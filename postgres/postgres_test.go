package postgres_test

import (
	"context"
	"fmt"
	"github.com/sbowman/drawbridge/postgres"
	"os"
	"testing"
)

// TestDB is the test database connection string.
const TestDB = "postgres://postgres@localhost/drawbridge_test?sslmode=disable&pool_max_conns=5&pool_min_conns=2"

var db *postgres.DB

func TestMain(m *testing.M) {
	var err error
	db, err = postgres.Open(TestDB)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to connect to %s, %s\n", postgres.SafeURI(TestDB), err)
		os.Exit(1)
	}
	defer db.Shutdown()

	os.Exit(m.Run())
}

// TxClose wraps the tx.Close functionality to rollback the connection and return it to
// the pool.
func TxClose(t *testing.T, ctx context.Context, tx postgres.Span) {
	if err := tx.Close(ctx); err != nil {
		t.Fatalf("Unable to rollback transaction: %s", err)
	}
}
