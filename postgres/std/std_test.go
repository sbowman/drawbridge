package std_test

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sbowman/drawbridge"
	"github.com/sbowman/drawbridge/postgres"
	"github.com/sbowman/drawbridge/postgres/std"
	"github.com/stretchr/testify/assert"
	"testing"
)

const (
	// TestDB is the test database URI.  Created by the Makefile.
	TestDB = "postgres://drawbridge@localhost:5432/drawbridge_test?sslmode=disable"
)

// Can we connect to the database and execute a command?
func TestOpen(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	db := stdConnect(t)

	_, err := db.Exec(ctx, `select 1`)
	assert.Nil(err)
}

// Do operations rollback properly when the transaction closes without a commit?
func TestClose(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	db := stdConnect(t)

	tx, err := db.Begin(ctx)
	assert.Nil(err)
	defer drawbridge.TxClose(ctx, tx)

	_, err = tx.Exec(ctx, `create table sample (id serial primary key, name varchar(64) not null)`)
	assert.Nil(err)
}

// Open a database connection to the TestDB.  Panics if the test user or test database
// doesn't exist, or the database isn't available.
func stdConnect(t *testing.T) drawbridge.Span {
	conn, err := std.Open(TestDB)
	if err != nil {
		t.Fatalf("Unable to connect to %s: %s", postgres.SafeURI(TestDB), err)
	}

	t.Cleanup(func() {
		if err := conn.Shutdown(); err != nil {
			t.Fatalf("Unable to shutdown database connection: %s\n", err)
		}
	})

	return conn
}

// Open a database connection pool to the TestDB.  Panics if the test user or test
// database doesn't exist, or the database isn't available.
func stdPool(t *testing.T) drawbridge.Span {
	pool, err := pgxpool.New(context.Background(), TestDB)
	if err != nil {
		t.Fatalf("Unable to connect to %s: %s", TestDB, err)
	}

	conn := std.FromPool(pool)

	t.Cleanup(func() {
		if err := conn.Shutdown(); err != nil {
			t.Fatalf("Unable to shutdown database connection: %s\n", err)
		}
	})

	return conn
}
