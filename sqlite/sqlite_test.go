package sqlite_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/sbowman/drawbridge"
	"github.com/sbowman/drawbridge/sqlite3"
)

// TestDB is the test database connection string.
var db *sqlite.DB

func TestMain(m *testing.M) {
	var err error
	db, err = sqlite.Open(":memory:")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to create a SQLite3 database in memory, %s\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(nil); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Unable to close the database, %s\n", err)
		}
	}()

	os.Exit(m.Run())
}

// TxClose wraps the tx.Close functionality to rollback the connection and return it to
// the pool.
func TxClose(t *testing.T, ctx context.Context, tx drawbridge.Span) {
	if err := tx.Close(ctx); err != nil {
		panic(fmt.Sprintf("Unable to rollback transaction: %s", err))
	}
}
