package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps the *pgxpool.Pool and provides the missing function wrappers to support
// [Span].
type DB struct {
	*pgxpool.Pool
}

// Begin a new transaction with default isolation.
func (db *DB) Begin(ctx context.Context) (Span, error) {
	return db.BeginTx(ctx, pgx.TxOptions{})
}

// BeginTx starts a transaction with custom isolation and other transaction options.
func (db *DB) BeginTx(ctx context.Context, opts pgx.TxOptions) (Span, error) {
	tx, err := db.Pool.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}

	return &Tx{tx}, nil
}

// Commit does nothing on a connection, since you're not in a transaction.
func (db *DB) Commit(context.Context) error {
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

// InTx on a database connection returns false.
func (db *DB) InTx() bool {
	return false
}
