package std_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sbowman/drawbridge"
	"github.com/sbowman/drawbridge/postgres/std"
	"github.com/stretchr/testify/assert"
)

const (
	// TestDB is the test database URI.  Created by the Makefile.
	TestDB = "postgres://drawbridge@localhost:5432/drawbridge_test?sslmode=disable"
)

var (
	// Pooled connection for use in all the tests, so we hopefully don't overrun the
	// available database connections.
	db *std.DB
)

func TestMain(m *testing.M) {
	pool, err := pgxpool.New(context.Background(), TestDB)
	if err != nil {
		panic(fmt.Sprintf("Unable to connect to %s: %s", TestDB, err))
	}

	db = std.FromPool(pool)

	defer func() {
		if err := db.Shutdown(); err != nil {
			panic(fmt.Sprintf("Unable to shutdown database connection: %s\n", err))
		}
	}()

	os.Exit(m.Run())
}

// Can we connect to the database and execute a command?
func TestOpen(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	_, err := db.Exec(ctx, `select 1`)
	assert.Nil(err)
}

// Do operations rollback properly when the transaction closes without a commit?
func TestClose(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	tx, err := db.Begin(ctx)
	assert.Nil(err)

	_, err = tx.Exec(ctx, `create table sample (id serial primary key, name varchar(64) not null)`)
	assert.Nil(err)

	drawbridge.TxClose(ctx, tx)

	_, err = db.Exec(ctx, `insert into sample (name) values ('Bob')`)
	assert.Error(err)

	var pgerr *pgconn.PgError
	assert.ErrorAs(err, &pgerr)
	assert.Equal(pgerr.Code, "42P01") // indicates the table doesn't exist
}

func TestCommit(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	tx, err := db.Begin(ctx)
	assert.Nil(err)

	defer func() {
		_, err := db.Exec(ctx, `drop table if exists goody`)
		assert.Nil(err)
	}()

	_, err = tx.Exec(ctx, `create table goody (id serial primary key, shoes varchar(64) not null, num integer)`)
	assert.Nil(err)

	row := tx.QueryRow(ctx, `insert into goody (shoes, num) values ('sneakers', 2) returning id`)

	var id int
	err = row.Scan(&id)
	assert.Nil(err)
	assert.Greater(id, 0)

	err = tx.Commit()
	assert.Nil(err)

	// Should be ok to call close after a commit, so you can always defer tx.Close(ctx)
	err = tx.Close(ctx)
	assert.Nil(err)

	row = db.QueryRow(ctx, `select id, shoes, num from goody where id = $1`, id)

	var existing int
	var shoes string
	var num int

	err = row.Scan(&existing, &shoes, &num)

	assert.Nil(err)
	assert.Equal(id, existing)
	assert.Equal("sneakers", shoes)
	assert.Equal(2, num)
}

// Does committing a transaction multiple levels deep work?
func TestSubCommit(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	tx, err := db.Begin(ctx)
	assert.Nil(err)

	defer func() {
		_, err := db.Exec(ctx, `drop table if exists goody`)
		assert.Nil(err)
	}()

	fn := func(ctx context.Context, span drawbridge.Span, shoes string, num int) (int, error) {
		tx, err := span.Begin(ctx)
		if err != nil {
			return 0, err
		}
		defer drawbridge.TxClose(ctx, tx)

		var id int

		row := tx.QueryRow(ctx, `insert into goody (shoes, num) values ($1, $2) returning id`, shoes, num)
		if err := row.Scan(&id); err != nil {
			return 0, err
		}

		if err := tx.Commit(); err != nil {
			return 0, err
		}

		return id, nil
	}

	_, err = tx.Exec(ctx, `create table goody (id serial primary key, shoes varchar(64) not null, num integer)`)
	assert.Nil(err)

	id, err := fn(ctx, tx, "boots", 3)
	assert.Nil(err)
	assert.Greater(id, 0)

	err = tx.Commit()
	assert.Nil(err)

	row := db.QueryRow(ctx, `select id, shoes, num from goody where id = $1`, id)

	var existing int
	var shoes string
	var num int

	err = row.Scan(&existing, &shoes, &num)

	assert.Nil(err)
	assert.Equal(id, existing)
	assert.Equal("boots", shoes)
	assert.Equal(3, num)
}

// Does rolling back a transaction several levels deep work?
func TestSubRollback(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	tx, err := db.Begin(ctx)
	assert.Nil(err)

	defer func() {
		_, err := db.Exec(ctx, `drop table if exists goody`)
		assert.Nil(err)
	}()

	fn := func(ctx context.Context, span drawbridge.Span, shoes string, num int) (int, error) {
		tx, err := span.Begin(ctx)
		if err != nil {
			return 0, err
		}
		defer drawbridge.TxClose(ctx, tx)

		if num < 0 {
			return 0, fmt.Errorf("can't have a negative number of shoes: %d", num)
		}

		var id int

		row := tx.QueryRow(ctx, `insert into goody (shoes, num) values ($1, $2) returning id`, shoes, num)
		if err := row.Scan(&id); err != nil {
			return 0, err
		}

		if err := tx.Commit(); err != nil {
			return 0, err
		}

		return id, nil
	}

	_, err = tx.Exec(ctx, `create table goody (id serial primary key, shoes varchar(64) not null, num integer)`)
	assert.Nil(err)

	id, err := fn(ctx, tx, "sandals", -1)
	assert.Error(err)
	assert.Equal(id, 0)

	err = tx.Commit()
	assert.Error(err)
	assert.ErrorIs(err, drawbridge.ErrRolledBack)

	row := db.QueryRow(ctx, `select id, shoes, num from goody where id = $1`, id)

	var existing int
	var shoes string
	var num int

	err = row.Scan(&existing, &shoes, &num)
	assert.Error(err)
}
