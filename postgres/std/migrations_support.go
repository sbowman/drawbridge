package std

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/sbowman/drawbridge"
)

var (
	// ErrInvalidSchemaName returned if the schema name isn't in a valid format
	// (letters, numbers, underscores).
	ErrInvalidSchemaName = errors.New("metadata schema name contains invalid characters")

	// ErrInvalidTableName returned if the table name isn't in a valid format
	// (letters, numbers, underscores).
	ErrInvalidTableName = errors.New("metadata table name contains invalid characters")

	// ErrTableNameRequired returned if the table name is blank.
	ErrTableNameRequired = errors.New("metadata table name is required")
)

// CreateMetadata creates the migrations package's metadata table in the requested schema
// and table if it doesn't already exist.  Returns the table name to use for the metadata.
func (db *DB) CreateMetadata(ctx context.Context, schema, table string) (string, error) {
	if stmt, err := createSchemaStmt(schema); err != nil {
		return "", err
	} else if stmt != "" && missingMetadataSchema(ctx, db, schema) {
		if _, err := db.Exec(ctx, stmt); err != nil {
			return "", err
		}
	}

	name, err := metadataName(schema, table)
	if err != nil {
		return "", err
	}

	if missingMetadataTable(ctx, db, schema, table) {
		if _, err := db.Exec(ctx, createTableStmt(name)); err != nil {
			return "", err
		}
	}

	return name, nil
}

// CreateMetadata creates the migrations package's metadata table in the requested schema
// and table if it doesn't already exist.  Returns the table name to use for the metadata.
func (tx *Tx) CreateMetadata(ctx context.Context, schema, table string) (string, error) {
	fmt.Println("looking for", schema, table)
	if stmt, err := createSchemaStmt(schema); err != nil {
		return "", err
	} else if stmt != "" && missingMetadataSchema(ctx, tx, schema) {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			return "", err
		}
	}

	name, err := metadataName(schema, table)
	if err != nil {
		return "", err
	}

	if missingMetadataTable(ctx, tx, schema, table) {
		if _, err := tx.Exec(ctx, createTableStmt(name)); err != nil {
			return "", err
		}
	}

	return name, nil
}

// LockMetadata panics because it makes no sense to lock the table out of a transaction.
func (db *DB) LockMetadata(_ context.Context, _ string) error {
	panic("You may not lock a table outside a transaction")
}

// UnlockMetadata does nothing.
func (db *DB) UnlockMetadata(_ context.Context, _ string) {
	// Do nothing...
}

// LockMetadata locks the metadata table to prevent other processes from applying
// migrations simultaneously.
func (tx *Tx) LockMetadata(ctx context.Context, metadataTable string) error {
	_, err := tx.Exec(ctx, "lock table "+metadataTable+" in access exclusive mode")
	return err
}

// UnlockMetadata does nothing.  PostgreSQL unlocks the table at the end of the
// transaction.
func (tx *Tx) UnlockMetadata(_ context.Context, _ string) {
	// Do nothing...
}

// Returns true if the provided schema doesn't exist in the database.  If the schema is
// blank, returns false.
func missingMetadataSchema(ctx context.Context, span drawbridge.Span, schema string) bool {
	if schema == "" {
		return false
	}

	var result bool

	// fmt.Println("does", schema, "exist?")

	row := span.QueryRow(ctx, "SELECT not(exists(select schema_name FROM information_schema.schemata WHERE schema_name = $1))", schema)
	if err := row.Scan(&result); err != nil {
		panic(fmt.Sprintf("Unable to query for the metadata schema, %s", err))
	}

	// fmt.Println("metadata schema missing?", result)

	return result
}

// Returns true if the given table is missing from the database. If table is blank,
// assumes "public."
func missingMetadataTable(ctx context.Context, span drawbridge.Span, schema, table string) bool {
	if schema == "" {
		schema = "public"
	}

	row := span.QueryRow(ctx, "select not(exists(select 1 from pg_catalog.pg_class c "+
		"join pg_catalog.pg_namespace n "+
		"on n.oid = c.relnamespace "+
		"where n.nspname = $1 and c.relname = $2))", schema, table)

	var result bool
	if err := row.Scan(&result); err != nil {
		panic(fmt.Sprintf("Unable to query for the metadata table, %s", err))
	}

	return result
}

var validObjName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// Verify the schema and table names, and returns the combined name.  If schema is blank,
// returns just the table name.  Otherwise, returns "schema.table."
func metadataName(schema, table string) (string, error) {
	if schema != "" && !validObjName.MatchString(schema) {
		return "", ErrInvalidSchemaName
	}

	if table == "" {
		return "", ErrTableNameRequired
	}

	if !validObjName.MatchString(table) {
		return "", ErrInvalidTableName
	}

	return fmt.Sprintf("%s.%s", schema, table), nil
}

// Validates the schema name and returns the create schema statement, if necessary.  If
// the schema is blank, returns a blank string and no error, meaning no need to create
// the schema.
func createSchemaStmt(schema string) (string, error) {
	if schema == "" {
		return "", nil
	} else if !validObjName.MatchString(schema) {
		return "", ErrInvalidSchemaName
	}

	return fmt.Sprintf("create schema if not exists %s", schema), nil
}

// Validates the schema and table names and returns the table name and create table
// statement.
func createTableStmt(metadataTable string) string {
	return fmt.Sprintf("create table %s(migration varchar(1024) not null primary key, rollback text)", metadataTable)
}
