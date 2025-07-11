package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Tx wraps the postgres.Tx interface and provides the missing hermes function wrappers.
type Tx struct {
	pgx.Tx
}

// Begin starts a pseudo nested transaction.
func (tx *Tx) Begin(ctx context.Context) (Span, error) {
	return tx.BeginTx(ctx, pgx.TxOptions{})
}

// BeginTx starts a transaction with custom isolation and other transaction options.
func (tx *Tx) BeginTx(ctx context.Context, _ pgx.TxOptions) (Span, error) {
	newTx, err := tx.Tx.Begin(ctx)
	if err != nil {
		return nil, err
	}

	return &Tx{newTx}, nil
}

// Close rolls back the transaction if this is a real transaction or rolls back to the
// savepoint if this is a pseudo nested transaction.
//
// Returns ErrTxClosed if the Conn is already closed, but is otherwise safe to call
// multiple times. Hence, a defer conn.Close() is safe even if conn.Commit() will be
// called first in a non-error condition.
//
// Any other failure of a real transaction will result in the connection being closed.
func (tx *Tx) Close(ctx context.Context) error {
	return tx.Tx.Rollback(ctx)
}

// InTx on a transaction always returns true.
func (tx *Tx) InTx() bool {
	return true
}
