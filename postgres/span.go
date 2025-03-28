package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Span provides a more specific interface for PostgreSQL `jackc/pgx` users.  It abstracts
// the [pgx.Conn] struct and the [pgx.Tx] interface into a common interface, similar to
// [drawbridge.Span], but with pgx return values.
//
// See [drawbridge.Span] for details.
type Span interface {
	// Begin starts a transaction.  If Conn already represents a transaction, pgx will
	// create a savepoint instead.
	Begin(ctx context.Context) (Span, error)

	// BeginTx allows for richer control over the transaction.  Note that if this is
	// called in an existing transaction, the new `opts` are ignored and the options
	// for the wrapping transaction are used.
	BeginTx(ctx context.Context, opts pgx.TxOptions) (Span, error)

	// InTx returns true if this Span wraps a transaction.
	InTx() bool

	// Commit the transaction.
	Commit(ctx context.Context) error

	// Close will do one of these things:
	//
	// * If the underlying instance is a simple database connection, it does nothing.
	// * If the underlying instance is a transaction that hasn't been committed, it
	//   rolls back the transaction.
	// * If the underlying instance is a transaction that has been committed, it does
	//   nothing.
	Close(ctx context.Context) error

	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults

	// Exec executes a query without returning any rows.  The args are for any
	// placeholder parameters in the query.  Note in [postgres.Span], returns a
	// [pgconn.CommandTag] instead of [sql.Result].
	Exec(ctx context.Context, sql string, arguments ...interface{}) (commandTag pgconn.CommandTag, err error)

	// Query executes a query that returns rows, typically a SELECT.  The args are for
	// any placeholder parameters in the query.  Note in [postgres.Span], returns
	// [pgx.Rows] instead of [sql.Rows].
	//
	// Make sure to either consume all the rows or call [pgx.Rows.Close].
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)

	// QueryRow executes a query that is expected to return at most one row. QueryRow
	// always returns a non-nil value. Errors are deferred until [pgx.Row]'s Scan
	// method is called.  If the query selects no rows, the [*pgx.Row.Scan] will
	// return [pgx.ErrNoRows].  Otherwise, [*pgx.Row.Scan] scans the first selected
	// row and discards the rest.
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}
