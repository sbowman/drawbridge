package postgres

import (
	"context"
	"database/sql"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"regexp"
	"strings"
)

const (
	// PgUniqueViolation is the PostgreSQL error code indicating a unique constraint
	// violation. In other words, the insert or update tried to store a duplicate
	// value.
	PgUniqueViolation = "23505"
)

// Open wraps the [pgxpool.Pool] so it supports the [Span] interface.
func Open(pool *pgxpool.Pool) *DB {
	return &DB{pool}
}

// SafeURI replaces the password in the PostgreSQL DB URI with asterisks, for secure
// logging purposes.  If outputting a database URI to a log message, be sure to wrap it in
// a SafeURI call.
func SafeURI(pgURI string) string {
	re := regexp.MustCompile("postgres://(?:[^:]+:([^@]*)@)?.*/")
	sub := re.FindStringSubmatchIndex(pgURI)
	if len(sub) < 4 || sub[2] == -1 {
		return pgURI
	}

	var b strings.Builder

	b.WriteString(pgURI[:sub[2]])
	b.WriteString("****")
	b.WriteString(pgURI[sub[3]:])

	return b.String()
}

// UniqueViolation returns true if the error is a pgconn.PgError with a code of 23505,
// unique violation.  In other words, did a query return an error because a value already
// exists?
func UniqueViolation(err error) bool {
	if err == nil {
		return false
	}

	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) {
		return pgerr.Severity == "ERROR" && pgerr.Code == PgUniqueViolation
	}

	return false
}

// NotFound returns true if the error contains a pgx.ErrorNoRows indicating no results
// were found for the database query.
func NotFound(err error) bool {
	return err != nil && errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows)
}

// TxClose is a shorthand function to use in a defer statement.  If the transaction fails
// to close (commit or rollback), the function panics.
func TxClose(ctx context.Context, tx Span) {
	err := tx.Close(ctx)
	if err == nil || errors.Is(err, pgx.ErrTxClosed) {
		return
	}

	panic("Transaction failed to close: " + err.Error())
}
