package pgxtest

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"os"
	"strings"
	"testing"

	"github.com/sbowman/drawbridge/migrations"
	"github.com/sbowman/drawbridge/postgres/std"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// TableExists queries for the table in the PostgreSQL metadata.
	TableExists = `
select exists 
    (select from information_schema.tables 
            where table_schema = $1 and 
                  table_name = $2)`
)

var db *std.DB

func TestMain(m *testing.M) {
	var err error

	db, err = std.Open("postgres://postgres@localhost/migrations_test?sslmode=disable")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to connect to migrations_test database: %s\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// TestMatch checks that we can match "up" and "down" sections in the files.
func TestMatch(t *testing.T) {
	assert := assert.New(t)
	reader := &migrations.DiskReader{}

	doc, err := migrations.ReadSQL(reader, "./testdata/1-create-sample.sql", migrations.Up)
	assert.Nil(err)
	assert.Contains(doc, "create table samples")
	assert.NotContains(doc, "drop table samples")

	doc, err = migrations.ReadSQL(reader, "./testdata/1-create-sample.sql", migrations.Down)
	assert.Nil(err)
	assert.NotContains(doc, "create table samples")
	assert.Contains(doc, "drop table samples")
}

// TestUp confirms upward bound migrations work.
func TestUp(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	defer clean(t, ctx)

	options := migrations.WithDirectory("./testdata")

	err := options.WithRevision(1).Apply(ctx, db)
	assert.Nil(err)

	err = tableExists(ctx, "drawbridge.schema_migrations")
	assert.Nil(err)

	err = tableExists(ctx, "samples")
	assert.Nil(err)

	_, err = db.Exec(ctx, "insert into samples (name) values ('Bob')")
	assert.Nil(err)

	rows, err := db.Query(ctx, "select name from samples where name = 'Bob'")
	require.Nil(t, err)

	var name string
	for rows.Next() {
		err := rows.Scan(&name)
		assert.Nil(err)
		assert.Equal(name, "Bob")
	}

	assert.NotEmpty(name)

	// Check that rollbacks are loaded in the database
	rows, err = db.Query(ctx, "select migration, rollback from drawbridge.schema_migrations")
	assert.Nil(err)

	var found int

	var migration string
	var down sql.NullString

	for rows.Next() {
		found++

		err = rows.Scan(&migration, &down)
		assert.Nil(err)

		SQL, err := migrations.ReadSQL(options.Reader, "./testdata/"+migration, migrations.Down)
		assert.Nil(err)

		SQL = strings.TrimSpace(SQL)
		assert.NotEqual(SQL, down.String)
	}

	assert.NotEqual(found, 0)
}

// Make sure revisions, i.e. partial migrations, are working.
func TestRevisions(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	defer clean(t, ctx)

	options := migrations.WithDirectory("./testdata")

	err := options.WithRevision(1).Apply(ctx, db)
	assert.Nil(err, "Unable to run migration to revision 1")

	_, err = db.Exec(ctx, "insert into samples (name, email) values ('Bob', 'bob@home.com')")
	assert.Error(err, "Expected inserting an email address to fail")

	err = options.WithRevision(2).Apply(ctx, db)
	assert.Nil(err, "Unable to run migration to revision 2")

	_, err = db.Exec(ctx, "insert into samples (name, email) values ('Bob', 'bob@home.com')")
	assert.Nil(err, "Expected to be able to insert email address after revision 2")

	rows, err := db.Query(ctx, "select email from samples where name = 'Bob'")
	require.Nil(t, err)

	var email string
	for rows.Next() {
		err := rows.Scan(&email)
		assert.Nil(err, "Failed to get email from database")
		assert.Equal(email, "bob@home.com")
	}

	assert.NotEmpty(email)
}

// Make sure migrations can be rolled back.
func TestDown(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	defer clean(t, ctx)

	options := migrations.WithDirectory("./testdata")

	err := options.WithRevision(2).Apply(ctx, db)
	require.Nil(t, err)

	_, err = db.Exec(ctx, "insert into samples (name, email) values ('Bob', 'bob@home.com')")
	assert.Nil(err)

	rows, err := db.Query(ctx, "select email from samples where name = 'Bob'")
	require.Nil(t, err)

	var email string
	for rows.Next() {
		err := rows.Scan(&email)
		assert.Nil(err)
		assert.Equal(email, "bob@home.com")
	}

	assert.NotEmpty(email)

	// Rollback
	err = options.WithRevision(1).Apply(ctx, db)
	require.Nil(t, err)

	// Is the rollback in the database gone?
	row := db.QueryRow(ctx, "select exists(select migration from drawbridge.schema_migrations where migration = '2-add-email-to-sample.sql')")

	var found bool
	err = row.Scan(&found)
	assert.Nil(err)
	assert.False(found)

	_, err = db.Exec(ctx, "insert into samples (name, email) values ('Alice', 'alice@home.com')")
	assert.Error(err)

	var pgerr *pgconn.PgError
	assert.ErrorAs(err, &pgerr)
	assert.Equal(pgerr.Code, "42703") // i.e., column does not exist

	_, err = db.Query(ctx, "select email from samples where name = 'Bob'")
	assert.Error(err)

	rows, err = db.Query(ctx, "select name from samples where name = 'Alice'")
	require.Nil(t, err)

	for rows.Next() {
		t.Errorf("Did not expect results from the query")
	}
}

// Is the simplified Rollback function working?
func TestRollback(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	defer clean(t, ctx)

	options := migrations.WithDirectory("./testdata")

	err := options.WithRevision(2).Apply(ctx, db)
	assert.Nil(err)

	_, err = db.Exec(ctx, "insert into samples (name, email) values ('Bob', 'bob@home.com')")
	assert.Nil(err)

	err = options.Rollback(ctx, db, 1)
	require.Nil(t, err)

	_, err = db.Query(ctx, "select email from samples where name = 'Bob'")
	assert.Error(err)
}

// Under normal circumstances, if part of a migration fails, the whole migration false.
func TestTransactions(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	defer clean(t, ctx)

	options := migrations.WithDirectory("./testdata")

	err := options.WithRevision(3).Apply(ctx, db)
	assert.Error(err)

	rows, err := db.Query(ctx, "select name from samples where name = 'abc'")
	require.Nil(t, err)

	for rows.Next() {
		var name string
		err := rows.Scan(&name)
		assert.Nil(err)
		assert.Equal(name, "abc")
	}
}

// TODO: test different metadata table name
// TODO: test metadata table in public, i.e. no schema

// Clean out the database.
func clean(t *testing.T, ctx context.Context) {
	if err := tableExists(ctx, "drawbridge.schema_migrations"); err == nil {
		if _, err := db.Exec(ctx, "delete from drawbridge.schema_migrations"); err != nil {
			t.Fatalf("Unable to clear the migrations.applied table: %s", err)
		}
	}

	rows, err := db.Query(ctx, "select table_name from information_schema.tables where table_schema='public'")
	if err != nil {
		t.Fatalf("Couldn't query for table names: %s", err)
	}

	var name string
	for rows.Next() {
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("Failed to get table name: %s", err)
		}

		// Note: not exactly safe, but this is just a test case
		if _, err := db.Exec(ctx, "drop table if exists "+name+" cascade"); err != nil {
			t.Fatalf("Couldn't drop table %s: %s", name, err)
		}
	}
}

// Check if the table exists.  Returns nil if the table exists.
func tableExists(ctx context.Context, table string) error {
	parts := strings.Split(table, ".")

	var schema string
	if len(parts) == 1 {
		schema = "public"
		table = parts[0]
	} else {
		schema = parts[0]
		table = parts[1]
	}

	rows, err := db.Query(ctx, TableExists, schema, table)
	if err != nil {
		return err
	}

	if rows.Next() {
		var found bool
		if err := rows.Scan(&found); err != nil {
			return err
		}

		if found {
			return nil
		}
	}

	return sql.ErrNoRows
}
