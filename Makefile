.PHONY: test
test: test_postgres

PG_SRC := \
	postgres/db.go \
	postgres/postgres.go \
	postgres/span.go \
	postgres/stdlib.go \
	postgres/tx.go \

PG_TEST := \
	postgres/stdlib_test.go

.PHONY: test_postgres
test_postgres: db_postgres $(PG_SRC) $(PG_TEST)
	@cd postgres && go test

.PHONY: db_postgres
db_postgres:
	@psql -U drawbridge template1 -c "select 1;" > /dev/null 2>&1 || createuser -d drawbridge
	@psql -U drawbridge drawbridge_test -c "select 1;" > /dev/null 2>&1 || createdb -U drawbridge drawbridge_test
