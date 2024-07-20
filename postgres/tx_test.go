package postgres_test

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sbowman/drawbridge/postgres"
	"github.com/stretchr/testify/assert"
	"testing"
)

// TestTransaction tests simple queries against a [postgres.Span] backed by [postgres.Tx].
func TestTransaction(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	tx, err := db.Begin(ctx)
	assert.Nil(err)

	_, err = tx.Exec(ctx, "create table simple(id serial primary key, email varchar(255))")
	assert.Nil(err)

	var id int
	row := tx.QueryRow(ctx, "insert into simple(email) values('jdoe@nowhere.com') returning id")
	err = row.Scan(&id)
	assert.Nil(err)

	var email string
	row = tx.QueryRow(ctx, "select email from simple where id = $1", id)
	err = row.Scan(&email)
	assert.Nil(err)

	assert.Equal("jdoe@nowhere.com", email)

	// This should roll everything back
	postgres.TxClose(ctx, tx)

	// Transaction should be dead
	row = tx.QueryRow(ctx, "select email from simple where id = $1", id)
	err = row.Scan(&email)
	assert.ErrorIs(err, pgx.ErrTxClosed)
	assert.NotNil(err)

	// Table should be gone...
	row = db.QueryRow(ctx, "select email from simple where id = $1", id)
	err = row.Scan(&email)

	var pgerr *pgconn.PgError
	assert.ErrorAs(err, &pgerr)
	assert.Equal(pgerr.Code, postgres.CodeUndefinedTable)
}

// Do subtransactions commit and rollback properly.
func TestSubTransactionRollback(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	addEmail := func(ctx context.Context, span postgres.Span, email string) (int, error) {
		tx, err := span.Begin(ctx)
		if err != nil {
			return -1, err
		}
		defer postgres.TxClose(ctx, tx)

		id := -1

		row := tx.QueryRow(ctx, "insert into subtxtest(email) values($1) returning id", email)
		if err = row.Scan(&id); err != nil {
			return -1, err
		}

		err = tx.Commit(ctx)
		return id, err
	}

	getUser := func(ctx context.Context, span postgres.Span, id int) (string, error) {
		row := span.QueryRow(ctx, "select email from subtxtest where id = $1", id)

		var email string
		if err := row.Scan(&email); err != nil {
			return "", err
		}

		return email, nil
	}

	testCases := []struct {
		email string
	}{
		{email: "userA@nowhere.com"},
		{email: "userB@nowhere.com"},
	}

	tx, err := db.Begin(ctx)
	assert.Nil(err)

	_, err = tx.Exec(ctx, "create table subtxtest(id serial primary key, email varchar(255))")
	assert.Nil(err)

	for _, testCase := range testCases {
		id, err := addEmail(ctx, tx, testCase.email)
		assert.Nil(err)

		email, err := getUser(ctx, tx, id)
		assert.Nil(err)
		assert.Equal(testCase.email, email)
	}

	// This should roll everything back
	postgres.TxClose(ctx, tx)

	// Transaction should be dead
	var id int
	row := tx.QueryRow(ctx, "select id from subtxtest where email = 'abc@nowhere.com'")
	err = row.Scan(&id)
	assert.ErrorIs(err, pgx.ErrTxClosed)
	assert.NotNil(err)

	// Table should be gone...
	row = db.QueryRow(ctx, "select id from subtxtest where email = 'abc@nowhere.com'")
	err = row.Scan(&id)

	var pgerr *pgconn.PgError
	assert.ErrorAs(err, &pgerr)
	assert.Equal(pgerr.Code, postgres.CodeUndefinedTable)
}

// Do subtransactions commit and rollback properly.
func TestSubTransactionCommit(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	addEmail := func(ctx context.Context, span postgres.Span, email string) (int, error) {
		tx, err := span.Begin(ctx)
		if err != nil {
			return -1, err
		}
		defer postgres.TxClose(ctx, tx)

		id := -1

		row := tx.QueryRow(ctx, "insert into subtxcommit(email) values($1) returning id", email)
		if err = row.Scan(&id); err != nil {
			return -1, err
		}

		err = tx.Commit(ctx)
		return id, err
	}

	getUser := func(ctx context.Context, span postgres.Span, id int) (string, error) {
		row := span.QueryRow(ctx, "select email from subtxcommit where id = $1", id)

		var email string
		if err := row.Scan(&email); err != nil {
			return "", err
		}

		return email, nil
	}

	testCases := []struct {
		email string
	}{
		{email: "userA@nowhere.com"},
		{email: "userB@nowhere.com"},
	}

	tx, err := db.Begin(ctx)
	assert.Nil(err)
	defer postgres.TxClose(ctx, tx)

	_, err = tx.Exec(ctx, "create table subtxcommit(id serial primary key, email varchar(255))")
	assert.Nil(err)

	for _, testCase := range testCases {
		id, err := addEmail(ctx, tx, testCase.email)
		assert.Nil(err)

		//fmt.Println("Created user", id)

		email, err := getUser(ctx, tx, id)
		assert.Nil(err)
		assert.Equal(testCase.email, email)
	}

	// This should commit everything
	err = tx.Commit(ctx)
	assert.Nil(err)

	// Transaction should be dead
	var id int
	row := tx.QueryRow(ctx, "select id from subtxcommit where email = 'abc@nowhere.com'")
	err = row.Scan(&id)
	assert.ErrorIs(err, pgx.ErrTxClosed)
	assert.NotNil(err)

	for _, testCase := range testCases {
		var id int
		row = db.QueryRow(ctx, "select id from subtxcommit where email = $1", testCase.email)
		err = row.Scan(&id)
		assert.Nil(err)
		assert.Greater(id, 0)

		//fmt.Println("found", testCase.email, "with ID", id)
	}

	// Let's wipe out this test table
	_, err = db.Exec(ctx, "drop table subtxcommit")
	assert.Nil(err)
}
