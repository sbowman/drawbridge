package postgres

import (
	"context"
	"database/sql"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/sbowman/drawbridge"
)

// StdDB implements the [drawbridge.Span] interface on top of [sql.DB], using pgx's
// stdlib support.
type StdDB struct {
	*sql.DB

	pool *pgxpool.Pool
}

// StdFromPool returns a [db.Span]-compatible [sql.DB] wrapper instance.  It leverages the
// pgx stdlib library to provide a standard Go sql.DB-compatible interface.
func StdFromPool(pool *pgxpool.Pool) *StdDB {
	conn := stdlib.OpenDBFromPool(pool)
	return &StdDB{conn, pool}
}

// StdOpen works like sql.Open, but returns a [Span]-compatible database connection.
func StdOpen(uri string) (*StdDB, error) {
	db, err := sql.Open("pgx", uri)
	if err != nil {
		return nil, err
	}

	return &StdDB{db, nil}, err
}

func (db *StdDB) Begin(ctx context.Context) (*StdTx, error) {
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &StdTx{Tx: tx}, nil
}

// Close does nothing at the DB level.  See [StdDB.Shutdown] to properly close the
// connection or pool.
func (db *StdDB) Close(_ context.Context) error {
	return nil
}

// Shutdown closes down the underlying [pgxpool.Pool] or [pgx.Conn].
func (db *StdDB) Shutdown() error {
	if err := db.DB.Close(); err != nil {
		return err
	}

	if db.pool != nil {
		db.pool.Close()
	}

	return nil
}

// Commit does nothing at the DB level.
func (db *StdDB) Commit(_ context.Context) error {
	return nil
}

func (db *StdDB) Exec(ctx context.Context, sql string, arguments ...any) (sql.Result, error) {
	return db.DB.ExecContext(ctx, sql, arguments...)
}

func (db *StdDB) Query(ctx context.Context, sql string, args ...any) (*sql.Rows, error) {
	return db.DB.QueryContext(ctx, sql, args...)
}

func (db *StdDB) QueryRow(ctx context.Context, sql string, args ...any) *sql.Row {
	return db.DB.QueryRowContext(ctx, sql, args...)
}

// Tracks subtransaction commits and rollbacks
type txState int

const (
	txPending txState = iota
	txRollback
	txCommit
)

type StdTx struct {
	*sql.Tx

	current  txState
	history  []txState
	rollback bool
}

// Begin manages multi-level transactions in the context of the [drawbridge.Span]
// interface.
func (tx *StdTx) Begin(_ context.Context) (*StdTx, error) {
	if tx.rollback {
		return nil, drawbridge.ErrRolledBack
	}

	tx.save()

	return tx, nil
}

// Commit the transaction.  If the transaction is a "sub transaction," pops the history
// based on the rollback state.
func (tx *StdTx) Commit(_ context.Context) error {
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
func (tx *StdTx) Close(_ context.Context) error {
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

func (tx *StdTx) Exec(ctx context.Context, sql string, arguments ...any) (sql.Result, error) {
	return tx.Tx.ExecContext(ctx, sql, arguments...)
}

func (tx *StdTx) Query(ctx context.Context, sql string, args ...any) (*sql.Rows, error) {
	return tx.Tx.QueryContext(ctx, sql, args...)
}

func (tx *StdTx) QueryRow(ctx context.Context, sql string, args ...any) *sql.Row {
	return tx.Tx.QueryRowContext(ctx, sql, args...)
}

// Save the state of the transaction on the stack.
func (tx *StdTx) save() {
	tx.history = append(tx.history, tx.current)
	tx.current = txPending
}

// Remove the transaction state from the stack, and update the current state.
func (tx *StdTx) release() {
	tx.current, tx.history = tx.history[len(tx.history)-1], tx.history[:len(tx.history)-1]
}
