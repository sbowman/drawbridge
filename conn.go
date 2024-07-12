package db

import (
	"context"
	"database/sql"
)

// Conn presents the standard [sql.Conn] and [sql.Tx] with a common interface.  The
// underlying implementations allow you to write functions that accept a [db.Conn], which
// may be either a [sql.Conn] or a [sql.Tx].  This allows you to break down your database
// updates into discrete functions that can be called individually (with [sql.Conn]),
// combined into a transaction (with [sql.Tx]) or tested using transactions.
//
// The Conn interface also does one other thing:  it enforces contexts without requiring
// the `Context` at the end of every function.  For example in this interface, [Begin]
// will map to [sql.Conn.BeginTx].
//
// Note the comments for each of these function were modified from the corresponding
// `Context` functions in the [database/sql] package.  If this is a problem, please let
// me know and I will rewrite them from scratch.
type Conn interface {
	// Begin starts a transaction.
	//
	// The provided context is used until the transaction is committed or rolled back.
	// If the context is canceled, the sql package will roll back the transaction.
	// [sql.Tx.Commit] will return an error if the context provided to Begin is
	// canceled.
	//
	// The provided [sql.TxOptions] is optional and may be nil if defaults should be
	// used.  If a non-default isolation level is used that the driver doesn't
	// support, an error will be returned.
	//
	// If Conn already represents a transaction underneath, calling Begin creates
	// a checkpoint, if supported.
	//
	// If checkpoints are not supported, it will internally track that we are in a
	// "subtransaction."  Any call to [Close] or [Commit] will back out of the
	// subtransaction.  Note that a "subtransaction" has no special  functionality.
	// It is simply a counter that increments with a Begin and decrements with a
	// [Close] or [Commit].
	Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)

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
	Close() error

	// Exec executes a query without returning any rows.  The args are for any
	// placeholder parameters in the query.
	Exec(ctx context.Context, query string, args ...any) (sql.Result, error)

	// Prepare creates a prepared statement for later queries or executions.  Multiple
	// queries or executions may be run concurrently from the returned statement.  The
	// caller must call the statement's [*sql.Stmt.Close] method when the statement is
	// no longer needed.
	//
	// The provided context is used for the preparation of the statement, not for the
	// execution of the statement.
	Prepare(ctx context.Context, query string) (*sql.Stmt, error)

	// Query executes a query that returns rows, typically a SELECT.  The args are for
	// any placeholder parameters in the query.
	Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)

	// QueryRow executes a query that is expected to return at most one row. QueryRow
	// always returns a non-nil value. Errors are deferred until [sql.Row]'s Scan
	// method is called.  If the query selects no rows, the [*sql.Row.Scan] will
	// return [sql.ErrNoRows].  Otherwise, [*sql.Row.Scan] scans the first selected
	// row and discards the rest.
	QueryRow(ctx context.Context, query string, args ...any) *sql.Row
}
