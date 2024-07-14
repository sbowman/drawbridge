package postgres

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Span abstracts the *pgx.Conn struct and the postgres.Tx interface into a common
// interface.  This can be useful for building domain models more functionally, i.e the
// same function could be used for a single database query outside of a transaction, or
// included in a transaction with other function calls.
//
// It's also useful for testing, as you can pass a transaction into any database-related
// function, don't commit, and simply Close() at the end of the test to clean up the
// database.
type Span interface {
	// Begin starts a transaction.  If Conn already represents a transaction, pgx will
	// create a savepoint instead.
	Begin(ctx context.Context) (Span, error)

	BeginTx(ctx context.Context, opts pgx.TxOptions) (Span, error)

	// Commit the transaction.  Does nothing if Conn is a *pgxpool.Pool.  If the
	// transaction is a psuedo-transaction, i.e. a savepoint, releases the savepoint.
	// Otherwise commits the transaction.
	Commit(ctx context.Context) error

	// Close will do one of these things:
	//
	// * If the underlying instance is a simple database connection, returns the
	//   connection to the pool.
	// * If the underlying instance is a transaction that hasn't been committed, it
	//   rolls back the transaction.
	// * If the underlying instance is a transaction that has been committed, does
	//   nothing.
	// * If the underlying instance is a checkpoint transaction that hasn't been
	//   committed, rolls back to the checkpoint.
	// * If the underlying instance is a checkpoint transaction that has been
	//   committed, does nothing.
	Close(ctx context.Context) error

	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults

	// Exec executes a query without returning any rows.  The args are for any
	// placeholder parameters in the query.
	Exec(ctx context.Context, sql string, arguments ...interface{}) (commandTag pgconn.CommandTag, err error)

	// Query executes a query that returns rows, typically a SELECT.  The args are for
	// any placeholder parameters in the query.
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)

	// QueryRow executes a query that is expected to return at most one row. QueryRow
	// always returns a non-nil value. Errors are deferred until [pgx.Row]'s Scan
	// method is called.  If the query selects no rows, the [*pgx.Row.Scan] will
	// return [pgx.ErrNoRows].  Otherwise, [*pgx.Row.Scan] scans the first selected
	// row and discards the rest.
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}
