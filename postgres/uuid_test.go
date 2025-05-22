package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestUUIDType(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Can't create a transaction: %s", err)
	}
	defer TxClose(t, ctx, tx)

	_, err = tx.Exec(ctx, `
create table sample_uuid (
	id uuid primary key default gen_random_uuid(),
	name varchar(64) not null
)`)
	assert.Nil(err, "Couldn't create sample_uuid table")

	// Test passing in a UUID
	id := uuid.New()
	name := "manual example"

	_, err = tx.Exec(ctx, "insert into sample_uuid (id, name) values ($1, $2)", id, name)
	assert.Nilf(err, "Couldn't insert sample record with UUID")

	row := tx.QueryRow(ctx, "select id, name from sample_uuid where name = $1 limit 1", name)

	var cid uuid.UUID
	var cname string

	err = row.Scan(&cid, &cname)
	assert.Nil(err)
	assert.Equal(cid.String(), id.String())
	assert.Equal(cname, name)

	// Test getting a generated ID
	name = "generated example"

	_, err = tx.Exec(ctx, "insert into sample_uuid (name) values ($1)", name)
	assert.Nil(err, "Couldn't insert sample record without uuid")

	row = tx.QueryRow(ctx, "select id, name from sample_uuid where name = $1 limit 1", name)

	err = row.Scan(&cid, &cname)
	assert.Nil(err)
	assert.NotEmpty(cid.String())
	assert.NotEqual(cid.String(), id.String())
	assert.Equal(cname, name)
}
