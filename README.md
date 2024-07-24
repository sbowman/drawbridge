# Drawbridge Database Support

[![PkgGoDev](https://pkg.go.dev/badge/github.com/sbowman/drawbridge)](https://pkg.go.dev/github.com/sbowman/drawbridge)

Drawbridge supplements `database/sql` and various database drivers such as `jackc/pgx`.  
It provides two things:

* a standard interface covering both a database connection and a transaction with wrapper
  classes that support it
* a migrations package to manage database schemas through version control

Drawbridge is an amalgamation of two open source packages I created years ago. Since they
are used together every time I create a database application, I thought it made sense with
the new version to clean up and organize old code and combine the two separate projects
into a single package.

* https://github.com/sbowman/hermes-pgx
* https://github.com/sbowman/hermes (deprecated lib/pq version of hermes)
* https://github.com/sbowman/migrations

Note if you've used `sbowman/hermes-pgx`, you'll notice a number of features have been
removed. Drawbridge simplifies its interfaces to only be about querying and updating
the database. Other functionality like locks and timers are left to the application or
separate packages.

## Standard, Shared Interface

While it might be atypical for a package to provide and implement interfaces, Drawbridge
does so with the goal of allowing developers to combine database-related functions so
they may pass either a database connection or a transaction into any function. This
allows the developer to write smaller functions that can be combined into larger
functions.

It also allows for easier testing. A test case may create a transaction, pass it into a
number of functions, then rollback the transaction at the end of the test without
affecting the database.

There are two interfaces currently available in Drawbridge. The first, `drawbridge.Span`
is available with support for `database/sql` by leveraging the `jackc/pgx/stdlib` package.
The other interface, `postgres.Span` is more closely related to the `pgx` packages,
including additional methods like CopyFrom and SendBatch. There are plans to support
SQLite3 and MySQL in the future.

The `postgres` package support `jackc/pgx/v5`. To leverage this version:

    go get github.com/sbowman/drawbridge/postgres

### Usage

    // Sample can take either a reference to the pgx database connection pool, or to a 
    // transaction.
    func Sample(span postgres.Span, name string) error {
        tx, err := span.Begin()
        if err != nil {
            return err
        }
        
        // Will automatically rollback if an error short-circuits the return
        // before tx.Commit() is called...
        defer tx.Close() 

        res, err := conn.Exec("insert into samples (name) values ($1)", name)
        if err != nil {
            return err
        }

        check, err := res.RowsAffected()
        if check == 0 {
            return fmt.Errorf("Failed to insert row (%s)", err)
        }

        return tx.Commit()
    }

    func main() {
        // Create a connection pool with max 10 connections, min 2 idle connections...
        span, err := postgres.Open("postgres://postgres@127.0.0.1/my_db?sslmode=disable")
        if err != nil {
            return err
        }

        // This works...
        if err := Sample(span, "Bob"); err != nil {
            fmt.Println("Bob failed!", err.Error())
        }

        // So does this...
        tx, err := span.Begin()
        if err != nil {
            panic(err)
        }

        // Will automatically rollback if call to sample fails...
        defer tx.Close() 

        if err := Sample(tx, "Frank"); err != nil {
            fmt.Println("Frank failed!", err.Error())
            return
        }

        // Don't forget to commit, or you'll automatically rollback on 
        // "defer tx.Close()" above!
        if err := tx.Commit(); err != nil {
            fmt.Println("Unable to save changes to the database:", err.Error())
        }
    }

Using a `postgres.Span` parameter in a function also opens up *in situ* testing of
database functionality. You can create a transaction in the test case and pass it to a
function that takes a `postgres.Span`, run any tests on the results of that function, and
simply let the transaction rollback at the end of the test to clean up.

    var DB *postgres.DB
    
    // We'll just open one database connection pool to speed up testing, so 
    // we're not constantly opening and closing connections.
    func TestMain(m *testing.M) {
	    db, err := postgres.Open(DBTestURI)
	    if err != nil {
	        fmt.Fprintf(os.Stderr, "Unable to open a database connection: %s\n", err)
	        os.Exit(1)
    	}
    	defer db.Shutdown()
    	
    	DB = db
    	
    	os.Exit(m.Run())
    }
    
    // Test getting a user account from the database.  The signature for the
    // function is:  `func GetUser(conn hermes.Conn, email string) (User, error)`
    // 
    // Passing a hermes.Conn value to the function means we can pass in either
    // a reference to the database pool and really update the data, or we can
    // pass in the same transaction reference to both the SaveUser and GetUser
    // functions.  If we use a transaction, we can let the transaction roll back 
    // after we test these functions, or at any failure point in the test case,
    // and we know the data is cleaned up. 
    func TestGetUser(t *testing.T) {
        u := User{
            Email: "jdoe@nowhere.com",
            Name: "John Doe",
        }
        
        tx, err := DB.Begin()
        if err != nil {
            t.Fatal(err)
        }
        defer tx.Close()
        
        if err := tx.SaveUser(tx, u); err != nil {
            t.Fatalf("Unable to create a new user account: %s", err)
        }
        
        check, err := tx.GetUser(tx, u.Email)
        if err != nil {
            t.Fatalf("Failed to get user by email address: %s", err)
        }
        
        if check.Email != u.Email {
            t.Errorf("Expected user email to be %s; was %s", u.Email, check.Email)
        } 
        
        if check.Name != u.Name {
            t.Errorf("Expected user name to be %s; was %s", u.Name, check.Name)
        } 
        
        // Note:  do nothing...when the test case ends, the `defer tx.Close()`
        // is called, and all the data in this transaction is rolled back out.
    }

Using transactions, even if a test case fails a returns prematurely, the database
transaction is automatically closed, thanks to `defer`. The database is cleaned up without
any fuss or need to remember to delete the data you created at any point in the test.

### Shutting down the connection pool

Note that because Drawbridge overloads the concept of `db.Close()` and `tx.Close()`,
`db.Close()` doesn't actually do anything. In pgx, `db.Close()` would close the connection
pool, which we don't want. So instead, call `postgres.DB.Shutdown()` to clean up your
connection pool when your app shuts down.

    db, err := postgres.Open(DBTestURI)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Unable to open a database connection: %s\n", err)
        os.Exit(1)
    }
    defer db.Shutdown()

## Migrations

The `migrations` package provides an agile approach to managing database schema revisions
for your Go applications. Using versioned files containing SQL commands to modify the
schema, you can make small changes to your database schema over time and version the
changes using source control.

The basic workflow:

* Include the `migrations` package in your application.
* Create a `./sql` directory in your project and populate it with files containing the
  SQL schema changes required for each release.
* In your startup code, include the call to `Apply` the migrations.
* Deploy the application (either include the `./sql` files with the deployment or embed
  them), which applies the migrations on startup.

The migrations package works in a similar fashion to Ruby on Rails ActiveRecord
migrations:  create a directory for your migrations, add numberically ordered SQL files
with an "up" and "down" section, then write your "up" SQL commands to update the database,
and "down" SQL command to roll back those changes if needed. You may then call functions
in the package from your application to apply changes to the database schema on
deployment, or leverage the `migrate` command line application to manage the database
schema separately from the application.

### Migrations Integration and CLI

The `migrations` package may be included in your product binary, or run separately as a
standalone application. When embedded in the application, typically you configure your
application to apply the database migrations on startup.

It may also be run as a standalone tool, either from the command line or a Docker
container.

#### Installing the Command-Line Tool

To install the command-line tool, run:

    $ go install github.com/sbowman/drawbridge/migrations/cli@v0.9.1

This will install the `migrate` binary into your `$GOPATH/bin` directory. If that
directory is on your PATH, you should be able to run migrations from anywhere.

If you need help using the command-line tool, run `migrate help` for information.

### Available Options

TBD

#### Supported Environment Variables

The `migrations` package supports supplying configuration options via environment
variables.

* migrate the database to the given revision (`DB_REVISION=<num>`)
* where to located the migration files (`DB_MIGRATIONS=<path>`)
* disable embedded rollbacks (`DB_EMBED=false`)

### The API

#### Adding Migrations to Your Application

There are a few primary functions to add to your applications to use the `migrations`
package.

#### Create an Empty Migration File

The migrations package can create empty migration files with the "up" and "down" sections
inserted for you (see below).

    migrations.Create("/path/to/migrations", "migration name")

This will create a migration file in the migrations path. The name will be modified to
remove the space, and a revision number will be inserted. If there were five other SQL
migrations in `/path/to/migrations`, You'd end up with a file like so:

    $ ls /path/to/migrations
    total 16
    -rw-r--r--  1 user  staff   99 May 22 15:39 1-create-sample.sql
    -rw-r--r--  1 user  staff  168 May 22 15:40 2-add-email-to-sample.sql
    -rw-r--r--  1 user  staff  207 May 22 15:50 3-create-my-proc.sql
    -rw-r--r--  1 user  staff   96 May 22 16:32 4-add-username-to-sample.sql
    -rw-r--r--  1 user  staff  347 May 22 17:55 5-rename-sample-to-users.sql
    -rw-r--r--  1 user  staff   22 May 26 19:03 6-migration-name.sql

#### Apply the Migrations

Once you've created your migration files and added the appropriate SQL to generate your
database schema and seed any data, you'll want to apply the migrations to the database by
calling:

    migrations.Apply(db)

Where `db` is a `*sql.DB` database connection (from the `database/sql` Go package).

This will attempt to run the migrations to the latest version as defined in the default
`./sql` directory, relative to where the binary was run.

You can override this directory in the `migrations.Options` or using the `DB_MIGRATIONS`
environment variable:

    migrations.WithDirectory("/etc/app/sql").Apply(db)

To migrate to a specific revision, modify the options or use the `DB_REVISION` environment
variable:

    migrations.WithDirectory("/etc/app/sql").WithRevision(33).Apply(db)

The revision number allows you to apply just the migrations appropriate for the current
version of your application. Day to day, you'll likely just use the default value `-1`,
which applies any and all existing migrations in order of their revision number.

Suppose you have six migrations, but you just want to apply the first four migrations to
your database and hold off on the last two? Migrations start at `1`, so Set the revision
to `4` and call:

    migrations.WithRevision(4).Apply(db)

The migrations package will apply just the SQL from the migration files from `1` to `4`,
in order.

What if you've already run all six migrations listed above? When you call
`migrations.Apply` with `WithRevision` set to `4`, the migrations package applies the SQL
from the "down" section in migration `6`, followed by the "down" section of migration `5`,
until the schema version is `4`.

We call this "rolling back" a migration, and it allows you to develop your application,
make some changes, apply them, then roll them back if they don't work.

Migrations also support something called "embedded roll backs."  The migrations package
tracks the migrations applied to the database in a metadata table in your database (by
default this is in the `drawbridge` schema, a table named `schema_migrations`, but it's
configurable). When a migration file is applied to the database, the "down" section of
the file in stored in the schema_migrations table. This allows migrations to be rolled
back without the migration files being present, just in case you need to rollback a
production deployment.

### Migration Files

Typically you'll deploy your migration files to a directory when you deploy your
application.

Create a directory in your application for the SQL migration files:

    $ mkdir ./sql
    $ cd sql

Now create a SQL migration. The filename must start with a number, followed by
a dash, followed by some description of the migration. For example:

    $ vi 1-create-users.sql

If you're using the migrations CLI, you can use the `create` command to create a migration
SQL file complete with "up" and "down" sections:

    $ migrations create create-users
    Created new migration ./sql/1-create_users.sql

An empty migration file looks like this:

    --- !Up
    
    --- !Down

Under the "up" section, add the changes you'd like to make to the database in this
migration. You may insert as many database commands as you like, but ideally each
migration carries out the simplest, smallest unit of work that makes for a useful
database, e.g. create a database table and indexes; make a modification to a table; or
create a stored procedure.

Note the above line formats are required, including the exclamation points, or
`migrations` won't be able to tell "up" from "down."

The "down" section should contain the code necessary to rollback the "up" changes.

So our "create_users" migration may look something like this:

    --- !Up
    create table users (
        id serial primary key,
        username varchar(64) not null,
        email varchar(1024) not null,
        password varchar(40) not null,
        enabled bool not null default true
    );
    
    create unique index idx_users_username on users (username);
    create unique index idx_users_emails on users (email);
    
    --- !Down
    drop table users;

The migrations package simply passes the raw SQL in the appropriate section ("up" or
"down"), to the database. The SQL calls are wrapped in a single transaction, so that if
there's a problem with the SQL, the database can rollback the entire migration.  _This
is not supported in MySQL or Maria, as they do not support transactionable schema
modifications._

Note that while running the migrations, the schema migrations metadata table will be
locked, if possible. If you're deploying your application across multiple instances, this
ensures a single instance will be responsible for the migrations. When the lock releases,
the other instances realize the migrations have already been applied.

Some databases, such as PostgreSQL, support nearly all schema modification commands
(`CREATE TABLE`, `ALTER TABLE`, etc.) in a transaction. Databases like MySQL have some
support for this. Your mileage may vary. If your database doesn't support transactionable
schema modifications, you may have to manually repair your databases should a migration
partially fail. This is another reason to keep each migration modification small and
targeted to a specific change, and not put everything in one revision file:  if the
migration fails for whatever reason, it's easier to clean up.

#### Embedding Migrations

TBD

### Embedded Rollbacks

Migrations stores each rollback ("down") SQL migration in the database. With this the
migrations package doesn't need the SQL files to be present to rollback, which makes it
easier to rollback an application's database migrations when using deployment tools like
Ansible or Terraform. You can simply deploy a previous version of the application, and the
migrations package can apply the rollbacks stored in the database to restore the database
to its previous schema version.

For example, you could deploy version 1.3 of your application, realize there is a bug,
then redeploy version 1.2. The migrations package can recognize the highest version of SQL
files available is lower than the migrations applied to the database, and can run the
rollback using the SQL embedded in the schema migrations table.

### Using the Migrations command-line tool

The Migrations v2 includes a CLI tool to run migrations standalone, without
needing to embed them in your application. Simply install the CLI tool and
make sure the Go `bin` directory is in your path:

    $ go install github.com/sbowman/migrations/v2/migrate
    $ migrate --revision=12 --migrations=./sql --uri='postgres://localhost/myapp_db?
    sslmode=disable' 

Use `migrate --help` for details on the available commands and parameters.

## References

* https://github.com/jackc/pgx