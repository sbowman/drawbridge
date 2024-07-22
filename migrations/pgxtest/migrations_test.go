package pgxtest

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/sbowman/drawbridge/migrations"
	"github.com/sbowman/drawbridge/postgres/std"
	"github.com/stretchr/testify/assert"
	"os"
	"strings"
	"testing"
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

	reader := &migrations.DiskReader{}

	config := migrations.DefaultOptions()
	err := config.WithRevision(1).Apply(ctx, db)
	assert.Nil(err)

	err = tableExists(ctx, "drawbridge.schema_migrations")
	assert.Nil(err)

	err = tableExists(ctx, "samples")
	assert.Nil(err)

	_, err = db.Exec(ctx, "insert into samples (name) values ('Bob')")
	assert.Nil(err)

	rows, err := db.Query(ctx, "select name from samples where name = 'Bob'")
	assert.Nil(err)

	var name string
	for rows.Next() {
		err := rows.Scan(&name)
		assert.Nil(err)
		assert.Equal(name, "Bob")
	}

	assert.NotEmpty(name)

	// Check that rollbacks are loaded in the database
	rows, err = db.Query(ctx, "select migration, down from migrations.rollbacks")
	assert.Nil(err)

	var found int

	var migration, down string
	for rows.Next() {
		found++

		err = rows.Scan(&migration, &down)
		assert.Nil(err)

		SQL, err := migrations.ReadSQL(reader, "./testdata/"+migration, migrations.Down)
		assert.Nil(err)

		SQL = strings.TrimSpace(SQL)
		assert.NotEqual(SQL, down)
	}

	assert.NotEqual(found, 0)
}

//// Make sure revisions, i.e. partial migrations, are working.
//func TestRevisions(t *testing.T) {
//	defer clean(t)
//
//	if err := migrate(1); err != nil {
//		t.Fatalf("Unable to run migration to revision 1: %s", err)
//	}
//
//	if _, err := conn.Exec("insert into samples (name, email) values ('Bob', 'bob@home.com')"); err == nil {
//		t.Error("Expected inserting an email address to fail")
//	}
//
//	if err := migrate(2); err != nil {
//		t.Fatalf("Unable to run migration to revision 2: %s", err)
//	}
//
//	if _, err := conn.Exec("insert into samples (name, email) values ('Bob', 'bob@home.com')"); err != nil {
//		t.Errorf("Expected to be able to insert email address after revision 2: %s", err)
//	}
//
//	rows, err := conn.Query("select email from samples where name = 'Bob'")
//	if err != nil {
//		t.Errorf("Didn't find expected record in database: %s", err)
//	}
//
//	var email string
//	for rows.Next() {
//		if err := rows.Scan(&email); err != nil {
//			t.Errorf("Failed to get email from database: %s", err)
//		}
//
//		if email != "bob@home.com" {
//			t.Errorf("Expected email bob@home.com for Bob, got %s", email)
//		}
//	}
//
//	if email == "" {
//		t.Error("Email not found")
//	}
//}
//
//// Make sure migrations can be rolled back.
//func TestDown(t *testing.T) {
//	defer clean(t)
//
//	if err := migrate(2); err != nil {
//		t.Fatalf("Unable to run migration to revision 2: %s", err)
//	}
//
//	if _, err := conn.Exec("insert into samples (name, email) values ('Bob', 'bob@home.com')"); err != nil {
//		t.Errorf("Expected to be able to insert email address after revision 2: %s", err)
//	}
//
//	rows, err := conn.Query("select email from samples where name = 'Bob'")
//	if err != nil {
//		t.Errorf("Didn't find expected record in database: %s", err)
//	}
//
//	var email string
//	for rows.Next() {
//		if err := rows.Scan(&email); err != nil {
//			t.Errorf("Failed to get email from database: %s", err)
//		}
//
//		if email != "bob@home.com" {
//			t.Errorf("Expected email bob@home.com for Bob, got %s", email)
//		}
//	}
//
//	if email == "" {
//		t.Error("Email not found")
//	}
//
//	// Rollback
//	if err := migrate(1); err != nil {
//		t.Fatalf("Unable to run migration to revision 1: %s", err)
//	}
//
//	// Is the rollback in the database gone?
//	row := conn.QueryRow("select exists(select migration from migrations.rollbacks where migration = '2-add-email-to-sample.sql')")
//	if row == nil {
//		t.Errorf("Unable to query for rollback: %s", err)
//	} else {
//		var found bool
//		if err := row.Scan(&found); err != nil {
//			t.Errorf("Unable to query for rollback: %s", err)
//		} else if found {
//			t.Errorf("Failed to delete the rollback migration for 2-add-email-to-sample.sql")
//		}
//	}
//
//	if _, err := conn.Exec("insert into samples (name, email) values ('Alice', 'alice@home.com')"); err == nil {
//		t.Error("Expected inserting an email address to fail")
//	}
//
//	_, err = conn.Query("select email from samples where name = 'Bob'")
//	if err == nil {
//		t.Error("Expected an error, as the email column shouldn't exist")
//	}
//
//	rows, err = conn.Query("select name from samples where name = 'Alice'")
//	if err != nil {
//		t.Errorf("Unable to query for samples: %s", err)
//	}
//
//	for rows.Next() {
//		t.Errorf("Did not expect results from the query")
//	}
//}
//
//// Is the simplified Rollback function working?
//func TestRollback(t *testing.T) {
//	defer clean(t)
//
//	if err := migrate(2); err != nil {
//		t.Fatalf("Unable to run migration to revision 2: %s", err)
//	}
//
//	if _, err := conn.Exec("insert into samples (name, email) values ('Bob', 'bob@home.com')"); err != nil {
//		t.Errorf("Expected insert to succeed: %s", err)
//	}
//
//	if err := migrations.Rollback(conn, "./sql", 1); err != nil {
//		t.Fatalf("Unable to rollback migration to revision 1: %s", err)
//	}
//
//	_, err := conn.Query("select email from samples where name = 'Bob'")
//	if err == nil {
//		t.Error("Expected querying for the rolled-back column to fail")
//	}
//}
//
//// Under normal circumstances, if part of a migration fails, the whole migration false.
//func TestTransactions(t *testing.T) {
//	defer clean(t)
//
//	if err := migrate(3); err == nil {
//		t.Error("Expected migration to fail")
//	}
//
//	rows, err := conn.Query("select name from samples where name = 'abc'")
//	if err != nil {
//		t.Fatalf("Unable to query for sample names:%s", err)
//	}
//
//	for rows.Next() {
//		var name string
//		if err := rows.Scan(&name); err != nil {
//			t.Errorf("Unable to scan results: %s", err)
//			continue
//		}
//
//		if name == "abc" {
//			t.Error("Unexpected abc value")
//		}
//	}
//}
//
//// Shortcut to run the test migrations in the sql directory.
//func migrate(revision int) error {
//	return migrations.WithRevision(revision).Apply(conn)
//}

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
