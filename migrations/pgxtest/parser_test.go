package pgxtest

import (
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/sbowman/drawbridge/migrations"
)

// Check that a single query parses correctly, with and without the semicolon.
func TestParseSingle(t *testing.T) {
	assert := assert.New(t)
	reader := &StringReader{}

	doc := `
--- !Up
create table sample(
	id serial primary key,
	name varchar(60) not null);

--- !Down
drop table sample;`

	sql, err := migrations.ReadSQL(reader, doc, migrations.Up)
	assert.Nil(err)
	assert.Greater(len(sql), 1)

	assert.Contains(sql, "create table sample")
	assert.Contains(sql, "id serial primary key")
	assert.Contains(sql, "name varchar(60) not null")

	assert.NotContains(sql, "drop table sample")

	sql, err = migrations.ReadSQL(reader, doc, migrations.Down)
	assert.Nil(err)
	assert.Greater(len(sql), 1)

	assert.NotContains(sql, "create table sample")
	assert.NotContains(sql, "id serial primary key")
	assert.NotContains(sql, "name varchar(60) not null")

	assert.Contains(sql, "drop table sample")
}

// Test that multiple queries come out ok.
func TestParseMulti(t *testing.T) {
	assert := assert.New(t)
	reader := &StringReader{}

	doc := `
--- !Up
create table sample(
	id serial primary key,
	name varchar(60) not null
);

create unique index idx_sample_name on sample (name);

--- !Down
drop table sample;`

	sql, err := migrations.ReadSQL(reader, doc, migrations.Up)
	assert.Nil(err)
	assert.Greater(len(sql), 1)

	assert.Contains(sql, "create table sample")
	assert.Contains(sql, "id serial primary key")
	assert.Contains(sql, "name varchar(60) not null")
	assert.Contains(sql, "create unique index idx_sample_name on sample (name)")

	assert.NotContains(sql, "drop table sample")
}

// Test that quotes within quotes works.
func TestParseQuotes(t *testing.T) {
	assert := assert.New(t)
	reader := &StringReader{}

	doc := `
--- !Up
insert into sample ("dog's name") values ('Maya');
insert into sample ("dog--but no -- cats") values ('Maya');
insert into sample (name) values ('Maya "the dog" Dog');
insert into sample (location) values ('King\'s Ransom');
insert into sample (location) values ('King'''s Ransom');
insert into sample (phrase) values ('for whom; the bell tolls');
insert into sample (phrase) values ('for whom -- the bell tolls');
`

	sql, err := migrations.ReadSQL(reader, doc, migrations.Up)
	assert.Nil(err)
	assert.Greater(len(sql), 1)

	expected := []string{
		`insert into sample ("dog's name") values ('Maya')`,
		`insert into sample ("dog--but no -- cats") values ('Maya')`,
		`insert into sample (name) values ('Maya "the dog" Dog')`,
		`insert into sample (location) values ('King\'s Ransom')`,
		`insert into sample (location) values ('King'''s Ransom')`,
		`insert into sample (phrase) values ('for whom; the bell tolls')`,
		`insert into sample (phrase) values ('for whom -- the bell tolls')`,
	}

	for _, cmd := range expected {
		assert.Contains(sql, cmd)
	}
}

func TestParseComments(t *testing.T) {
	assert := assert.New(t)
	reader := &StringReader{}

	doc := `
--- !Up
-- Creating a sample table
create table sample(
	id serial primary key,
	name varchar(60) not null -- name must be unique; don't overload this
);

-- Make sure the sample name is unique!
create unique index idx_sample_name on sample (name);

insert into sample (name) values ('--');
insert into sample ("weird -- column") values('hello');
`

	sql, err := migrations.ReadSQL(reader, doc, migrations.Up)
	assert.Nil(err)
	assert.Greater(len(sql), 1)

	expected := []string{
		"create table sample",
		"id serial primary key",
		"name varchar(60) not null",
		"create unique index idx_sample_name on sample (name)",
		"insert into sample (name) values ('--')",
		"insert into sample (\"weird -- column\") values('hello')",
	}

	for _, cmd := range expected {
		assert.Contains(sql, cmd)
	}
}
