package sqlite

import (
	"context"
	"errors"
	"fmt"
	"regexp"
)

var (
	// ErrInvalidTableName returned if the table name isn't in a valid format
	// (letters, numbers, underscores).
	ErrInvalidTableName = errors.New("metadata table name contains invalid characters")

	// ErrTableNameRequired returned if the table name is blank.
	ErrTableNameRequired = errors.New("metadata table name is required")
)

// CreateMetadata creates the table in the database used to track the state of the
// database migrations.  Note that SQLite3 does not support schemas, so the schema is
// ignored.
func (db *DB) CreateMetadata(ctx context.Context, _, table string) (string, error) {
	if err := isValidTableName(table); err != nil {
		return "", err
	}

	if _, err := db.Exec(ctx, createTableStmt(table)); err != nil {
		return "", err
	}

	return table, nil
}

// CreateMetadata creates the table in the database used to track the state of the
// database migrations.  Note that SQLite3 does not support schemas, so the schema is
// ignored.
func (tx *Tx) CreateMetadata(ctx context.Context, _, table string) (string, error) {
	if err := isValidTableName(table); err != nil {
		return "", err
	}

	if _, err := tx.Exec(ctx, createTableStmt(table)); err != nil {
		return "", err
	}

	return table, nil
}

// LockMetadata is unsupported, as it makes no sense in SQLite3.
func (db *DB) LockMetadata(_ context.Context, _ string) error {
	// Do nothing...
	return nil
}

// UnlockMetadata is unsupported, as it makes no sense in SQLite3.
func (db *DB) UnlockMetadata(_ context.Context, _ string) {
	// Do nothing...
}

// LockMetadata is unsupported, as it makes no sense in SQLite3.
func (tx *Tx) LockMetadata(ctx context.Context, metadataTable string) error {
	// Do nothing...
	return nil
}

// UnlockMetadata is unsupported, as it makes no sense in SQLite3.
func (tx *Tx) UnlockMetadata(_ context.Context, _ string) {
	// Do nothing...
}

var validObjName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// Validates the table name and returns the table name.
func isValidTableName(table string) error {
	if table == "" {
		return ErrTableNameRequired
	}

	if !validObjName.MatchString(table) {
		return ErrInvalidTableName
	}

	return nil
}

func createTableStmt(metadataTable string) string {
	return fmt.Sprintf("create table if not exists %s(migration varchar(1024) not null primary key, rollback text)", metadataTable)
}
