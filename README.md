# Drawbridge Database Support

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

TODO

## References

https://github.com/jackc/pgx