.PHONY: test
test: test_postgres

PG_SRC := \
	postgres/db.go \
	postgres/postgres.go \
	postgres/span.go \
	postgres/std/db.go \
	postgres/std/tx.go \
	postgres/tx.go \

PG_TEST := \
	postgres/std/std_test.go

.PHONY: test_postgres
test_postgres: db_postgres $(PG_SRC) $(PG_TEST)
	@cd postgres && go test ./...

.PHONY: db_postgres
db_postgres:
	@psql -U drawbridge template1 -c "select 1;" > /dev/null 2>&1 || createuser -d drawbridge
	@psql -U drawbridge drawbridge_test -c "select 1;" > /dev/null 2>&1 || createdb -U drawbridge drawbridge_test

.PHONY: tidy
tidy:
	@go mod tidy
	@cd postgres && go mod tidy
	@cd migrations/pgxtest && go mod tidy
	@cd migrations/cli && go mod tidy

migrate: migrations/cli
	@go build -C migrations/cli
	@mv migrations/cli/migrate .

