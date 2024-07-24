// Package postgres enables a standard interface based on the pgx/v5
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"reflect"
	"regexp"
	"strings"
)

// FromPool creates a postgres.Span-compatible object.  Note this approach does not
// include the uuid functionality by default; you must register that separately.
func FromPool(pool *pgxpool.Pool) *DB {
	return &DB{pool}
}

// Open wraps the [pgxpool.Pool] so it supports the [Span] interface.  See
// [pgxpool.ParseConfig] for details on the format of the URI string.
func Open(uri string) (*DB, error) {
	config, err := pgxpool.ParseConfig(uri)
	if err != nil {
		return nil, err
	}

	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		Register(conn.TypeMap())
		return nil
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, err
	}

	return &DB{pool}, nil
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
		return pgerr.Severity == "ERROR" && pgerr.Code == CodeUniqueViolation
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

// Register registers the github.com/google/uuid integration with a pgtype.Map.
func Register(m *pgtype.Map) {
	m.TryWrapEncodePlanFuncs = append([]pgtype.TryWrapEncodePlanFunc{TryWrapUUIDEncodePlan}, m.TryWrapEncodePlanFuncs...)
	m.TryWrapScanPlanFuncs = append([]pgtype.TryWrapScanPlanFunc{TryWrapUUIDScanPlan}, m.TryWrapScanPlanFuncs...)

	m.RegisterType(&pgtype.Type{
		Name:  "uuid",
		OID:   pgtype.UUIDOID,
		Codec: UUIDCodec{},
	})

	registerDefaultPgTypeVariants := func(name, arrayName string, value interface{}) {
		// T
		m.RegisterDefaultPgType(value, name)

		// *T
		valueType := reflect.TypeOf(value)
		m.RegisterDefaultPgType(reflect.New(valueType).Interface(), name)

		// []T
		sliceType := reflect.SliceOf(valueType)
		m.RegisterDefaultPgType(reflect.MakeSlice(sliceType, 0, 0).Interface(), arrayName)

		// *[]T
		m.RegisterDefaultPgType(reflect.New(sliceType).Interface(), arrayName)

		// []*T
		sliceOfPointerType := reflect.SliceOf(reflect.TypeOf(reflect.New(valueType).Interface()))
		m.RegisterDefaultPgType(reflect.MakeSlice(sliceOfPointerType, 0, 0).Interface(), arrayName)

		// *[]*T
		m.RegisterDefaultPgType(reflect.New(sliceOfPointerType).Interface(), arrayName)
	}

	registerDefaultPgTypeVariants("uuid", "_uuid", uuid.UUID{})
	registerDefaultPgTypeVariants("uuid", "_uuid", UUID{})
}
