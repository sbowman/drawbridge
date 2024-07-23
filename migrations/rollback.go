package migrations

import (
	"context"
	"database/sql"
	"errors"
	"github.com/sbowman/drawbridge"
	"sort"
	"strings"
)

var (
	// ErrRollbackComplete returned when the rollbacks are past the desired revision.
	ErrRollbackComplete = errors.New("rollback complete")
)

// UpdateRollback adds the migration's "down" SQL to the rollbacks table.
func UpdateRollback(ctx context.Context, span Span, reader Reader, metadataTable, path string) error {
	var err error
	filename := Filename(path)

	row := span.QueryRow(ctx, "select exists(select 1 from "+metadataTable+" where migration = $1)", filename)
	var exists bool
	if err := row.Scan(&exists); err != nil {
		return err
	}

	if exists {
		return nil
	}

	downSQL, err := ReadSQL(reader, path, Down)
	if err != nil {
		return err
	}

	downSQL = strings.TrimSpace(downSQL)

	_, err = span.Exec(ctx, "update "+metadataTable+" set rollback = $1 where migration = $2", downSQL, filename)
	return err
}

// ApplyRollbacks collects any migrations stored in the database that are higher than the
// desired revision and runs the "down" migration to roll them back.
func (m Migration) ApplyRollbacks(ctx context.Context) error {
	migrations, err := Applied(ctx, m.span, m.metadataTable)
	if err != nil {
		return err
	}

	// Run the migrations in reverse order
	sort.Sort(SortDown(migrations))

	for _, migration := range migrations {
		if err := m.Rollback(ctx, migration); errors.Is(err, ErrRollbackComplete) {
			break
		} else if err != nil {
			return err
		}
	}

	return nil
}

// Rollback applies a "down" migration, provided it's greater than the desired revision.
func (m Migration) Rollback(ctx context.Context, migration string) error {
	tx, err := Begin(ctx, m.span)
	if err != nil {
		return err
	}
	defer drawbridge.TxClose(ctx, tx)

	migrationRevision, err := Revision(migration)
	if err != nil {
		return err
	}

	// Stop when we reach the desired revision
	if migrationRevision <= m.revision {
		return ErrRollbackComplete
	}

	var downSQL string
	row := tx.QueryRow(ctx, "select rollback from "+m.metadataTable+" where migration = $1", migration)
	if err := row.Scan(&downSQL); errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}

	if downSQL != "" {
		_, err = tx.Exec(ctx, downSQL)
		if err != nil {
			return err
		}
	}

	// Clean out the migration now that it's been rolled back
	if _, err := tx.Exec(ctx, "delete from "+m.metadataTable+" where migration = $1", migration); err != nil {
		return err
	}

	return tx.Commit()
}

// Applied returns the list of migrations that have already been applied to this database.
func Applied(ctx context.Context, span Span, metadataTable string) ([]string, error) {
	rows, err := span.Query(ctx, "select migration from "+metadataTable)
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

// HandleEmbeddedRollbacks updates the rollbacks and then applies any missing and necessary
// rollbacks to get the database to the implied versions.
func (m Migration) HandleEmbeddedRollbacks(ctx context.Context, directory string) error {
	if m.revision == Latest {
		m.revision = LatestRevision(m.reader, directory)
	}

	// Apply the db-based rollbacks as needed
	if err := m.ApplyRollbacks(ctx); err != nil {
		return err
	}

	return nil
}
