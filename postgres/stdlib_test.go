package postgres_test

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	postgres "github.com/sbowman/drawbridge/pgx"
	"testing"
)

const (
	TestDB = "postgres://drawbridge@localhost:5432/drawbridge_test?sslmode=disable"
)

func TestStandard(t *testing.T) {
	ctx := context.Background()

	db := connect(ctx, t)
	defer func() {
		if err := db.Shutdown(); err != nil {
			t.Fatalf("Unable to shutdown database connection: %s\n", err)
		}
	}()

	if _, err := db.Exec(ctx, `create table sample (id serial primary key, name varchar(64) not null)`); err != nil {
		t.Fatalf("Unable to create sample table: %s", err)
	}
}

// Open a database connection to the TestDB.  Panics if the test user or test database
// doesn't exist, or the database isn't available.
func connect(ctx context.Context, t *testing.T) *postgres.StdDB {
	pool, err := pgxpool.New(ctx, TestDB)
	if err != nil {
		t.Fatalf("Unable to connect to the drawbridge_test database: %s", err)
	}

	return postgres.StdFromPool(pool)
}
