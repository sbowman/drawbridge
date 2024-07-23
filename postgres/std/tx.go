package std

import (
	"context"
	"database/sql"
	"github.com/sbowman/drawbridge"
)

// Tracks subtransaction commits and rollbacks
type txState uint8

const (
	txPending txState = iota
	txCommit
)

type Tx struct {
	*sql.Tx

	depth      []txState
	inRollback bool
}

// Create a new Span-compatible transaction that supports sub-transactions.
func (db *DB) newTx(ctx context.Context) (*Tx, error) {
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &Tx{
		Tx:         tx,
		depth:      []txState{txPending},
		inRollback: false,
	}, nil
}

// Begin manages multi-level transactions in the context of the [drawbridge.Span]
// interface.
func (tx *Tx) Begin(_ context.Context) (drawbridge.Span, error) {
	if tx.inRollback {
		return nil, drawbridge.ErrRolledBack
	}

	tx.depth = append(tx.depth, txPending)

	return tx, nil
}

// Commit the transaction.  If the transaction is a "sub transaction," pops the history
// based on the rollback state.
func (tx *Tx) Commit() error {
	if tx.inRollback {
		return drawbridge.ErrRolledBack
	}

	if tx.current() == txCommit {
		return drawbridge.ErrCommitted
	}

	tx.state(txCommit)

	if len(tx.depth) == 1 {
		return tx.Tx.Commit()
	}

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
	defer tx.pop()

	if tx.current() != txPending {
		return nil
	}

	tx.inRollback = true
	return tx.Tx.Rollback()
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

func (tx *Tx) InTx() bool {
	return true
}

func (tx *Tx) current() txState {
	return tx.depth[len(tx.depth)-1]
}

func (tx *Tx) state(value txState) {
	tx.depth[len(tx.depth)-1] = value
}

func (tx *Tx) pop() {
	tx.depth = tx.depth[:len(tx.depth)-1]
}
