package migrations

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/sbowman/drawbridge"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Direction is the direction to migrate
type Direction string

const (
	// Latest migrates to the latest migration.
	Latest int = -1

	// Up direction.
	Up Direction = "up"

	// Down direction.
	Down Direction = "down"

	// None direction.
	None Direction = "none"
)

var (
	// ErrNameRequired returned if the user failed to supply a name for the
	// migration.
	ErrNameRequired = errors.New("name required")

	// ErrInvalidStep returned when stepping back in a rollback.
	ErrInvalidStep = errors.New("invalid step")

	// Matches the Up/Down sections in the SQL migration file
	dirRe = regexp.MustCompile(`^---\s+!(Up|Down).*$`)
)

// Span extends the drawbridge.Span interface to support database migrations.
type Span interface {
	drawbridge.Span

	// CreateMetadata verifies if the schema and table exists, and if they don't, it
	// creates them.  Returns the name to use for the database queries related to
	// the migrations.  For example, if the schema is `drawbridge` and the table is
	// `schema_migrations`, CreateMetadata would return `drawbridge.schema_migrations`.
	//
	//
	CreateMetadata(ctx context.Context, schema, table string) (string, error)

	// LockMetadata locks the migrations package's metadata table to prevent other
	// processes from applying migrations.
	LockMetadata(ctx context.Context, metadataTable string) error

	// UnlockMetadata unlocks the migrations package's metadata table.  Some databases
	// require an unlock, whereas other databases unlock the table at the end of the
	// transaction, so this may do nothing.
	UnlockMetadata(ctx context.Context, metadataTable string)
}

// Create a new migration from the template.
func (options Options) Create(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ErrNameRequired
	}

	if err := os.MkdirAll(options.Directory, 0755); err != nil {
		return err
	}

	r := LatestRevision(options.Reader, options.Directory) + 1
	fullname := fmt.Sprintf("%d-%s.sql", r, trimmed)
	path := fmt.Sprintf("%s%c%s", options.Directory, os.PathSeparator, fullname)

	if err := os.WriteFile(path, []byte("--- !Up\n\n--- !Down\n\n"), 0644); err != nil {
		return err
	}

	return nil
}

// Apply any SQL migrations to the database.
//
// Any files that don't have entries in the migrations table will be run to bring the
// database to the indicated version.  Should the migrations in the database exceed the
// version indicated, the rollback or "down" migrations are applied to restore the
// database to the previous versions.  By default the database is migrated to the latest
// available version indicated by the SQL migration files.
//
// If the migrations table does not exist, this function automatically creates it.
//
// May return an ErrStopped if rolling back migrations and the Down portion has a /stop
// modifier.
//
// Note `span` should be a database connection or pool, not a transaction.
func (options Options) Apply(ctx context.Context, span Span) error {
	schema := options.SchemaMigrations.Schema
	table := options.SchemaMigrations.Table

	metadataTable, err := span.CreateMetadata(ctx, schema, table)
	if err != nil {
		return err
	}

	reader := options.Reader

	direction := Moving(ctx, span, metadataTable, options.Revision)
	migrations, err := Available(reader, options.Directory, direction)
	if err != nil {
		return err
	}

	m := Migration{span, reader, metadataTable, direction, options.Revision, options.EmbeddedRollbacks}

	for _, migration := range migrations {
		path := fmt.Sprintf("%s%c%s", options.Directory, os.PathSeparator, migration)
		if err := m.ReadAndApply(ctx, path); err != nil {
			return err
		}
	}

	if !options.EmbeddedRollbacks {
		return nil
	}

	return HandleEmbeddedRollbacks(ctx, span, reader, options.Directory, options.Revision)
}

// Migration defines the details about the migration being attempted.
type Migration struct {
	span          Span      // database transaction
	reader        Reader    // reads the migration files
	metadataTable string    // name of the metadata table in the database
	direction     Direction // direction to move to the revision
	revision      int       // move to this revision
	rollbacks     bool      // support embedded rollbacks?
}

// ReadAndApply reads the SQL from the migration file identified by `path` and applies the
// SQL for the direction, provided the revision is correct, all in a single transaction.
//
// Each migration file, when applied, is done so in a transaction with the metadata table
// locked, to prevent duplicate migrations across processes.
func (m Migration) ReadAndApply(ctx context.Context, path string) error {
	tx, err := m.span.Begin(ctx)
	if err != nil {
		return err
	}
	defer drawbridge.TxClose(ctx, tx)

	if err := m.span.LockMetadata(ctx, m.metadataTable); err != nil {
		return err
	}
	defer m.span.UnlockMetadata(ctx, m.metadataTable)

	if m.ShouldRun(ctx, path) {
		SQL, err := ReadSQL(m.reader, path, m.direction)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, SQL)
		if err != nil {
			return err
		}

		if err = m.Migrated(ctx, path); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// Rollback a number of migrations.
func (options Options) Rollback(ctx context.Context, span Span, metadataTable string, steps int) error {
	if steps < 1 {
		return ErrInvalidStep
	}

	latest, err := LatestMigration(ctx, span, metadataTable)
	if err != nil {
		return err
	}

	revision, err := Revision(latest)
	if err != nil {
		return err
	}

	version := revision - steps
	if version < 0 {
		version = 0
	}

	return options.WithRevision(version).Apply(ctx, span)
}

// Available returns the list of SQL migration paths in order.  If direction is
// Down, returns the migrations in reverse order (migrating down).
func Available(reader Reader, directory string, direction Direction) ([]string, error) {
	files, err := reader.Files(directory)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("invalid migrations directory, %s: %s", directory, err.Error())
	}

	var filenames []string
	for _, name := range files {
		if strings.HasSuffix(name, ".sql") {
			filenames = append(filenames, name)
		}
	}

	if direction == Down {
		sort.Sort(SortDown(filenames))
	} else {
		sort.Sort(SortUp(filenames))
	}

	return filenames, nil
}

// LatestRevision returns the latest revision available from the SQL files in
// the migrations directory.
func LatestRevision(reader Reader, directory string) int {
	migrations, err := Available(reader, directory, Down)
	if err != nil {
		return 0
	}

	if len(migrations) == 0 {
		return 0
	}

	// Find a valid filename
	for _, filename := range migrations {
		rev, err := Revision(filename)
		if err != nil {
			continue
		}

		return rev
	}

	return 0
}

// Revision extracts the revision number from a migration filename.
func Revision(filename string) (int, error) {
	segments := strings.SplitN(Filename(filename), "-", 2)
	if len(segments) == 1 {
		return 0, fmt.Errorf("invalid migration filename: %s", filename)
	}

	v, err := strconv.Atoi(segments[0])
	if err != nil {
		return 0, err
	}

	return v, nil
}

// Filename returns just the filename from the full path.
func Filename(path string) string {
	paths := strings.Split(path, string(os.PathSeparator))
	return paths[len(paths)-1]
}

// Moving determines the direction we're moving to reach the version.
func Moving(ctx context.Context, span Span, metadataTable string, version int) Direction {
	if version == Latest {
		return Up
	}

	latest, err := LatestMigration(ctx, span, metadataTable)
	if err != nil {
		return None
	}

	if latest == "" {
		return Up
	}

	revision, err := Revision(latest)
	if err != nil {
		return None
	}

	if revision < version {
		return Up
	} else if revision > version {
		return Down
	}

	return None
}

// ShouldRun decides if the migration should be applied or removed, based on
// the direction and desired version to reach.
func (m Migration) ShouldRun(ctx context.Context, migration string) bool {
	version, err := Revision(migration)
	if err != nil {
		return false
	}

	switch m.direction {
	case Up:
		return IsUp(version, m.revision) && !IsMigrated(ctx, m.span, m.metadataTable, migration)
	case Down:
		return IsDown(version, m.revision) && IsMigrated(ctx, m.span, m.metadataTable, migration)
	}
	return false
}

// IsUp returns true if the migration must roll up to reach the desired version.
func IsUp(version int, desired int) bool {
	return desired == Latest || version <= desired
}

// IsDown returns true if the migration must rollback to reach the desired
// version.
func IsDown(version int, desired int) bool {
	return version > desired
}

// ReadSQL reads the migration and filters for the up or down SQL commands.
func ReadSQL(reader Reader, path string, direction Direction) (string, error) {
	f, err := reader.Read(path)
	if err != nil {
		return "", nil
	}

	sqldoc := new(bytes.Buffer)
	parsing := false

	s := bufio.NewScanner(f)
	for s.Scan() {
		found := dirRe.FindStringSubmatch(s.Text())
		if len(found) == 1 {
			dir := strings.ToLower(found[1])

			if Direction(dir) == direction {
				parsing = true
				continue
			}

			parsing = false
		} else if parsing {
			sqldoc.Write(s.Bytes())
			sqldoc.WriteRune('\n')
		}
	}

	return sqldoc.String(), nil
}

// LatestMigration returns the name of the latest migration run against the database.
func LatestMigration(ctx context.Context, span Span, metadataTable string) (string, error) {
	var latest, migration string

	// PostgreSQL may not order the migrations by revision, so we need to compute which is
	// latest
	rows, err := span.Query(ctx, "select migration from "+metadataTable)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		if err := rows.Scan(&migration); err != nil {
			return "", err
		}

		m, _ := Revision(migration)
		l, _ := Revision(latest)

		if m > l {
			latest = migration
		}
	}

	return latest, nil
}

// Applied returns the list of migrations that have already been applied to this database.
func (m Migration) Applied(ctx context.Context) ([]string, error) {
	rows, err := m.span.Query(ctx, "select migration from "+m.metadataTable)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var migration string
	var results []string

	for rows.Next() {
		if err := rows.Scan(&migration); err != nil {
			return nil, err
		}

		results = append(results, migration)
	}

	return results, nil
}

// IsMigrated checks the migration has been applied to the database, i.e. is it
// in the migrations.applied table?
func IsMigrated(ctx context.Context, span Span, metadataTable string, migration string) bool {
	// If migrating, table should be locked, so no need to lock the row
	row := span.QueryRow(ctx, "select migration from "+metadataTable+" where migration = $1 limit 1", Filename(migration))
	return !errors.Is(row.Scan(), sql.ErrNoRows)
}

// Migrated adds or removes the migration record from the metadata table.
func (m Migration) Migrated(ctx context.Context, path string) error {
	filename := Filename(path)

	if m.direction == Down {
		if _, err := m.span.Exec(ctx, "delete from "+m.metadataTable+" where migration = $1", filename); err != nil {
			return err
		}
	} else {
		if _, err := m.span.Exec(ctx, "insert into "+m.metadataTable+" (migration) values ($1)", filename); err != nil {
			return err
		}

		if m.rollbacks {
			if err := m.UpdateRollback(ctx, path); err != nil {
				return err
			}
		}
	}

	return nil
}
