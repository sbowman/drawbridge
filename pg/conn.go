package pg

import (
	"context"
	"github.com/jackc/pgx/v5"
	db "github.com/sbowman/drawbridge"
)

// Conn extends the PostgreSQL database to support CopyFrom and SendBatch.
type Conn interface {
	db.Conn

	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}
