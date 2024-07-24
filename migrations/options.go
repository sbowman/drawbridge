package migrations

import (
	"os"
	"strconv"
	"strings"
)

const (
	// EnvMigrations is the environment variable that can be used to point to the
	// directory of SQL migrations.
	EnvMigrations = "DB_MIGRATIONS"

	// EnvRevision indicates the revision number to set the database to.
	EnvRevision = "DB_REVISION"

	// EnvEmbeddedRollbacks when false disables embedding the rollback SQL in the
	// database.
	EnvEmbeddedRollbacks = "DB_EMBED"
)

// Options manages the configuration of the migrations tool.
type Options struct {
	// Revision is the revision to forcibly move to.  Defaults to the latest revision
	// as indicated by the available SQL files (which could be a rollback if the
	// applied migrations exceed the latest SQL file.
	Revision int

	// Directory is the directory containing the SQL files.  Defaults to the "./sql"
	// directory.
	Directory string

	// EmbeddedRollbacks enables embedded rollbacks.  Defaults to true.
	EmbeddedRollbacks bool

	// MetadataTable points to the database table used to manage the schema
	// changes.
	MetadataTable struct {
		Schema string
		Name   string
	}

	// Reader defaults to the DiskReader for querying and ingesting migration files.
	Reader Reader
}

// DefaultOptions returns the defaults for the migrations package.  They include:
//
// * Revision: Latest (`DB_REVISION`)
// * Directory: ./sql (`DB_MIGRATIONS`)
// * EmbeddedRollbacks: true (`DB_EMBED`)
// * MetadataTable: drawbridge.schema_migrations
//
// Note that the schema migrations table is not configurable via an environment variable.
// It may be overridden by the application, but it's a bad idea to make this configurable.
// If it changes, your schema versioning will break without extraordinary measures.
func DefaultOptions() Options {
	revision := Latest
	directory := "./sql"
	embed := true
	schemaTable := "drawbridge.schema_migrations"

	if val := os.Getenv(EnvRevision); val != "" {
		rev, err := strconv.Atoi(val)
		if err == nil {
			revision = rev
		}
	}

	if val := os.Getenv(EnvMigrations); val != "" {
		directory = val
	}

	if val := os.Getenv(EnvEmbeddedRollbacks); val != "" {
		if strings.EqualFold(val, "false") {
			embed = false
		}
	}

	options := Options{
		Revision:          revision,
		Directory:         directory,
		EmbeddedRollbacks: embed,
		Reader:            &DiskReader{},
	}

	return options.WithSchemaTable(schemaTable)
}

// WithRevision manually indicates the revision to migrate the database to.  By default,
// the migrations to get the database to the revision indicated by the latest SQL
// migration file is used.
func WithRevision(revision int) Options {
	return DefaultOptions().WithRevision(revision)
}

// WithDirectory points to the directory of SQL migrations files that should be used to
// migrate the database schema.  Defaults to the "./sql" directory.
func WithDirectory(path string) Options {
	return DefaultOptions().WithDirectory(path)
}

// DisableEmbeddedRollbacks disables the embedded rollbacks functionality.  Rollbacks must
// be triggered manually, using WithRevision.
func DisableEmbeddedRollbacks() Options {
	return DefaultOptions().DisableEmbeddedRollbacks()
}

// WithSchemaTable overrides the default `drawbridge.schema_migrations` table to track the
// database schema versions.  Note that this is not configurable via environment
// variables, as it should never change once your app is deployed.  If you need to
// override this value, do it in a constant in your application.
func WithSchemaTable(schemaTable string) Options {
	return DefaultOptions().WithSchemaTable(schemaTable)
}

// WithRevision manually indicates the revision to migrate the database to.  By default,
// the migrations to get the database to the revision indicated by the latest SQL
// migration file is used.
func (options Options) WithRevision(revision int) Options {
	options.Revision = revision
	return options
}

// WithDirectory points to the directory of SQL migrations files that should be used to
// migrate the database schema.  Defaults to the "./sql" directory.
func (options Options) WithDirectory(path string) Options {
	options.Directory = path
	return options
}

// DisableEmbeddedRollbacks disables the embedded rollbacks functionality.  Rollbacks must
// be triggered manually, using WithRevision.
func (options Options) DisableEmbeddedRollbacks() Options {
	options.EmbeddedRollbacks = false
	return options
}

// WithSchemaTable overrides the default `drawbridge.schema_migrations` table to track the
// database schema versions.  Note that this is not configurable via environment
// variables, as it should never change once your app is deployed.  If you need to
// override this value, do it in a constant in your application.
func (options Options) WithSchemaTable(schemaTable string) Options {
	parts := strings.Split(schemaTable, ".")
	switch len(parts) {
	case 1:
		options.MetadataTable.Schema = "public"
		options.MetadataTable.Name = parts[0]

	case 2:
		options.MetadataTable.Schema = parts[0]
		options.MetadataTable.Name = parts[1]
	}

	return options
}
