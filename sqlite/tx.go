package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/sbowman/drawbridge"
)

var (
	// ErrTooManyRollbacks returned if the client software closes the transaction more
	// times than it begins a transaction.
	ErrTooManyRollbacks = errors.New("too many rollbacks")
)

const (
	StateCommitted  = 1
	StateRolledBack = 2
)

// Tx wraps the postgres.Tx interface and provides the missing hermes function wrappers.
// TODO: use states for this?
type Tx struct {
	*sql.Tx
	parent *Tx
	state  int
}

func newTx(tx *sql.Tx, parent *Tx) *Tx {
	return &Tx{
		Tx:     tx,
		parent: parent,
	}
}

// Begin starts a pseudo nested transaction.
func (tx *Tx) Begin(ctx context.Context) (drawbridge.Span, error) {
	return tx.BeginTx(ctx, nil)
}

// BeginTx starts a transaction with custom isolation and other transaction options.
func (tx *Tx) BeginTx(_ context.Context, _ *sql.TxOptions) (drawbridge.Span, error) {
	return newTx(tx.Tx, tx), nil
}

// Exec executes a query without returning any rows.  The args are for any
// placeholder parameters in the query.
func (tx *Tx) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return tx.ExecContext(ctx, query, args...)
}

// Query executes a query that returns rows, typically a SELECT.  The args are for
// any placeholder parameters in the query.
func (tx *Tx) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return tx.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that is expected to return at most one row. QueryRow
// always returns a non-nil value. Errors are deferred until [sql.Row]'s Scan
// method is called.  If the query selects no rows, the [*sql.Row.Scan] will
// return [sql.ErrNoRows].  Otherwise, [*sql.Row.Scan] scans the first selected
// row and discards the rest.
func (tx *Tx) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return tx.QueryRowContext(ctx, query, args...)
}

// Commit commits the transaction if this is a real transaction or releases the
// savepoint if this is a pseudo nested transaction. Commit will return an error
// where errors.Is(ErrTxClosed) is true if the Tx is already closed, but is
// otherwise safe to call multiple times. If the commit fails with a rollback
// status (e.g. the transaction was already in a broken state) then an error where
// errors.Is(ErrTxCommitRollback) is true will be returned.
func (tx *Tx) Commit() error {
	if tx.parent == nil {
		if tx.state == StateRolledBack {
			return sql.ErrTxDone
		}

		tx.state = StateCommitted
		return tx.Tx.Commit()
	}

	if tx.state == StateRolledBack {
		return sql.ErrTxDone
	}

	tx.state = StateCommitted
	return nil
}

// Close rolls back the transaction if this is a real transaction or rolls back to the
// savepoint if this is a pseudo nested transaction.
//
// Returns ErrTxClosed if the Conn is already closed, but is otherwise safe to call
// multiple times. Hence, a defer conn.Close() is safe even if conn.Commit() will be
// called first in a non-error condition.
//
// Any other failure of a real transaction will result in the connection being closed.
func (tx *Tx) Close(_ context.Context) error {
	if tx.parent == nil {
		if tx.state == StateCommitted {
			return nil
		}

		return tx.Tx.Rollback()
	}

	if tx.state != StateCommitted {
		tx.parent.state = StateRolledBack
	}

	return nil
}

// InTx on a transaction always returns true.
func (tx *Tx) InTx() bool {
	return true
}
