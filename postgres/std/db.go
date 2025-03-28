package std

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/sbowman/drawbridge"
)

// DB implements the [drawbridge.Span] interface on top of [sql.DB], using pgx's
// stdlib support.
type DB struct {
	*sql.DB

	pool *pgxpool.Pool
}

// FromPool returns a [db.Span]-compatible [sql.DB] wrapper instance.  It leverages the
// pgx stdlib library to provide a standard Go sql.DB-compatible interface.
func FromPool(pool *pgxpool.Pool) *DB {
	conn := stdlib.OpenDBFromPool(pool)
	return &DB{conn, pool}
}

// Open works like sql.Open, but returns a [Span]-compatible database connection.
func Open(uri string) (*DB, error) {
	db, err := sql.Open("pgx", uri)
	if err != nil {
		return nil, err
	}

	return &DB{db, nil}, err
}

func (db *DB) Begin(ctx context.Context) (drawbridge.Span, error) {
	return db.newTx(ctx)
}

// Close does nothing at the DB level.  See [DB.Shutdown] to properly close the
// connection or pool.
func (db *DB) Close(_ context.Context) error {
	return nil
}

// Shutdown closes down the underlying [pgxpool.Pool] or [pgx.Conn].
func (db *DB) Shutdown() error {
	if err := db.DB.Close(); err != nil {
		return err
	}

	if db.pool != nil {
		db.pool.Close()
	}

	return nil
}

// Commit does nothing at the DB level.
func (db *DB) Commit() error {
	return nil
}

func (db *DB) Exec(ctx context.Context, sql string, arguments ...any) (sql.Result, error) {
	return db.DB.ExecContext(ctx, sql, arguments...)
}

func (db *DB) Query(ctx context.Context, sql string, args ...any) (*sql.Rows, error) {
	return db.DB.QueryContext(ctx, sql, args...)
}

func (db *DB) QueryRow(ctx context.Context, sql string, args ...any) *sql.Row {
	return db.DB.QueryRowContext(ctx, sql, args...)
}

func (db *DB) InTx() bool {
	return false
}
