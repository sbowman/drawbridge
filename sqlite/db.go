package sqlite

import (
	"context"
	"database/sql"

	"github.com/sbowman/drawbridge"
)

// DB wraps the *sql.DB to support SQLite3.
type DB struct {
	*sql.DB
}

// Begin a new transaction with default isolation.
func (db *DB) Begin(ctx context.Context) (drawbridge.Span, error) {
	return db.BeginTx(ctx, nil)
}

// BeginTx starts a transaction with custom isolation and other transaction options.
func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (drawbridge.Span, error) {
	tx, err := db.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}

	return newTx(tx, nil), nil
}

// Exec executes a query without returning any rows.  The args are for any
// placeholder parameters in the query.
func (db *DB) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.ExecContext(ctx, query, args...)
}

// Query executes a query that returns rows, typically a SELECT.  The args are for
// any placeholder parameters in the query.
func (db *DB) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that is expected to return at most one row. QueryRow
// always returns a non-nil value. Errors are deferred until [sql.Row]'s Scan
// method is called.  If the query selects no rows, the [*sql.Row.Scan] will
// return [sql.ErrNoRows].  Otherwise, [*sql.Row.Scan] scans the first selected
// row and discards the rest.
func (db *DB) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return db.QueryRowContext(ctx, query, args...)
}

// Commit does nothing on a connection, since you're not in a transaction.
func (db *DB) Commit() error {
	return nil
}

// Close does nothing.  Since this Close method is meant to be used interchangably with
// transactions, it doesn't actually close anything, because we don't want to close the
// underlying database pool at the end of every non-transactional request.  Instead, see
// [DB.Shutdown].
func (db *DB) Close(_ context.Context) error {
	return nil
}

// InTx on a database connection returns false.
func (db *DB) InTx() bool {
	return false
}
