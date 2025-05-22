package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/mattn/go-sqlite3"
	"github.com/sbowman/drawbridge"

	_ "github.com/mattn/go-sqlite3"
)

// Open a SQLite3 file.  Uses the Go `database/sql` pooling
func Open(filename string) (*DB, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=rwc", filename))
	if err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

// UniqueViolation returns true if the error is a pgconn.PgError with a code of 23505,
// unique violation.  In other words, did a query return an error because a value already
// exists?
func UniqueViolation(err error) bool {
	if err == nil {
		return false
	}

	var dberr sqlite3.Error
	if errors.As(err, &dberr) {
		return dberr.Code == sqlite3.ErrConstraint
	}

	return false
}

// NotFound returns true if the error contains a pgx.ErrorNoRows indicating no results
// were found for the database query.
func NotFound(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, sql.ErrNoRows) {
		return true
	}

	var dberr sqlite3.Error
	if errors.As(err, &dberr) {
		return dberr.Code == sqlite3.ErrNotFound
	}

	return false
}

// TxClose is a shorthand function to use in a defer statement.  If the transaction fails
// to close (commit or rollback), the function panics.
func TxClose(ctx context.Context, tx drawbridge.Span) {
	err := tx.Close(ctx)
	if err == nil {
		return
	}

	panic("Transaction failed to close: " + err.Error())
}
