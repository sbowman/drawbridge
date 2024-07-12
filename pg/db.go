package pg

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps the *pgxpool.Pool and provides the missing function wrappers to support
// [db.Conn].
type DB struct {
	*pgxpool.Pool
}

// Begin a new transaction.
func (db *DB) Begin(ctx context.Context) (Conn, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}

	return &Tx{tx}, nil
}

// Commit does nothing on a connection, since you're not in a transaction.
func (db *DB) Commit(_ context.Context) error {
	return nil
}

// Close does nothing.  Since this Close method is meant to be used interchangably with
// transactions, it doesn't actually close anything, because we don't want to close the
// underlying database pool at the end of every non-transactional request.  Instead, see
// [DB.Shutdown].
func (db *DB) Close(_ context.Context) error {
	return nil
}

// Shutdown the underlying pgx Pool.  You may call this when your application is exiting
// to release all the database pool connections.
func (db *DB) Shutdown() {
	db.Pool.Close()
}
