package std

import (
	"context"
	"database/sql"
	"github.com/sbowman/drawbridge"
)

// Tracks subtransaction commits and rollbacks
type txState int

const (
	txPending txState = iota
	txRollback
	txCommit
)

type Tx struct {
	*sql.Tx

	current  txState
	history  []txState
	rollback bool
}

// Begin manages multi-level transactions in the context of the [drawbridge.Span]
// interface.
func (tx *Tx) Begin(_ context.Context) (drawbridge.Span, error) {
	if tx.rollback {
		return nil, drawbridge.ErrRolledBack
	}

	tx.save()

	return tx, nil
}

// Commit the transaction.  If the transaction is a "sub transaction," pops the history
// based on the rollback state.
func (tx *Tx) Commit(_ context.Context) error {
	if tx.rollback {
		return drawbridge.ErrRolledBack
	}

	if len(tx.history) == 0 {
		return tx.Tx.Commit()
	}

	tx.current = txCommit
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
	defer tx.release()

	if tx.current != txPending {
		return nil
	}

	if err := tx.Tx.Rollback(); err != nil {
		return err
	}

	tx.current = txRollback
	tx.rollback = true

	return nil
}

func (tx *Tx) Exec(ctx context.Context, sql string, arguments ...any) (sql.Result, error) {
	return tx.Tx.ExecContext(ctx, sql, arguments...)
}

func (tx *Tx) Query(ctx context.Context, sql string, args ...any) (*sql.Rows, error) {
	return tx.Tx.QueryContext(ctx, sql, args...)
}

func (tx *Tx) QueryRow(ctx context.Context, sql string, args ...any) *sql.Row {
	return tx.Tx.QueryRowContext(ctx, sql, args...)
}

// Save the state of the transaction on the stack.
func (tx *Tx) save() {
	tx.history = append(tx.history, tx.current)
	tx.current = txPending
}

// Remove the transaction state from the stack, and update the current state.
func (tx *Tx) release() {
	if len(tx.history) == 0 {
		return
	}

	tx.current, tx.history = tx.history[len(tx.history)-1], tx.history[:len(tx.history)-1]
}
