package migrations

import (
	"database/sql"
	"errors"
	"sort"
	"strings"
)

// ErrStopped returned if the migration couldn't rollback due to a /stop modifier
var ErrStopped = errors.New("stopped rollback due to /stop modifier")

// CreateMigrationsRollbacks creates the migrations.rollbacks table in the database if it doesn't already
// exist.
func CreateMigrationsRollbacks(tx *sql.Tx) error {
	if MissingMigrationsRollbacks(tx) {
		if _, err := tx.Exec("create table migrations.rollbacks(migration varchar(1024) not null primary key, down text)"); err != nil {
			return err
		}
	}

	return nil
}

// MissingMigrationsRollbacks returns true if there is no migrations.rollbacks table in the database.
func MissingMigrationsRollbacks(tx *sql.Tx) bool {
	row := tx.QueryRow("select not(exists(select 1 from pg_catalog.pg_class c " +
		"join pg_catalog.pg_namespace n " +
		"on n.oid = c.relnamespace " +
		"where n.nspname = 'migrations' and c.relname = 'rollbacks'))")

	var result bool
	if err := row.Scan(&result); err != nil {
		return true
	}

	return result
}

// UpdateRollback adds the migration's "down" SQL to the rollbacks table.
func UpdateRollback(tx *sql.Tx, path string) error {
	var err error
	filename := Filename(path)

	row := tx.QueryRow("select exists(select 1 from migrations.rollbacks where migration = $1)", filename)
	var exists bool
	if err := row.Scan(&exists); err != nil {
		return err
	}

	if exists {
		return nil
	}

	downSQL, mods, err := ReadSQL(path, Down)
	if err != nil {
		return err
	}

	downSQL = SQL(strings.TrimSpace(string(downSQL)))

	// Record that the rollback should stop here, as indicated by the annotation on the Down
	// indicator in the SQL
	if mods.Has("/stop") {
		_, err = tx.Exec("insert into migrations.rollbacks (migration, down) values ($1, '/stop')", filename)
		return err
	}

	_, err = tx.Exec("insert into migrations.rollbacks (migration, down) values ($1, $2)", filename, downSQL)
	return err
}

// ApplyRollbacks collects any migrations stored in the database that are higher than the desired
// revision and runs the "down" migration to roll them back.
func ApplyRollbacks(db *sql.DB, revision int) error {
	migrations, err := Applied(db)
	if err != nil {
		return err
	}

	// Run the migrations in reverse order
	sort.Sort(SortDown(migrations))

	for _, migration := range migrations {
		tx, err := db.Begin()
		if err != nil {
			return err
		}

		migrationRevision, err := Revision(migration)
		if err != nil {
			_ = tx.Rollback()
			return err
		}

		// Stop when we reach the desired revision
		if migrationRevision <= revision {
			_ = tx.Rollback()
			break
		}

		var downSQL string
		row := tx.QueryRow("select down from migrations.rollbacks where migration = $1", migration)
		if err := row.Scan(&downSQL); errors.Is(err, sql.ErrNoRows) {
			continue
		} else if err != nil {
			_ = tx.Rollback()
			return err
		}

		if downSQL == "/stop" {
			_ = tx.Rollback()
			return ErrStopped

		} else if downSQL != "" {
			_, err = tx.Exec(downSQL)
			if err != nil {
				_ = tx.Rollback()
				return err
			}
		}

		// Clean out the migration now that it's been rolled back
		if _, err := tx.Exec("delete from migrations.rollbacks where migration = $1", migration); err != nil {
			_ = tx.Rollback()
			return err
		}

		if _, err := tx.Exec("delete from migrations.applied where migration = $1", migration); err != nil {
			_ = tx.Rollback()
			return err
		}

		if err := tx.Commit(); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	return nil
}

// HandleEmbeddedRollbacks updates the rollbacks and then applies any missing and necessary
// rollbacks to get the database to the implied versions.
func HandleEmbeddedRollbacks(db *sql.DB, directory string, version int) error {
	if version == Latest {
		version = LatestRevision(directory)
	}

	// Apply the db-based rollbacks as needed
	if err := ApplyRollbacks(db, version); err != nil {
		return err
	}

	return nil
}
