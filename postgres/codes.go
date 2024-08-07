package postgres

// PostgreSQL error codes
const (
	CodeUndefinedTable      = "42P01"
	CodeUndefinedColumn     = "42703"
	CodeUniqueViolation     = "23505"
	CodeForeignKeyViolation = "23503"
)
