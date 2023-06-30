
# :warning: Warning: This repository is considered inactive and no change will be made to it except for security updates.

# sqalx

[![GoDoc](https://godoc.org/github.com/heetch/sqalx?status.svg)](https://godoc.org/github.com/heetch/sqalx)
[![Go Report Card](https://goreportcard.com/badge/github.com/heetch/sqalx)](https://goreportcard.com/report/github.com/heetch/sqalx)

sqalx (pronounced 'scale-x') is a library built on top of [sqlx](https://github.com/jmoiron/sqlx) that allows to seamlessly create nested transactions and to avoid thinking about whether or not a function is called within a transaction.
With sqalx you can easily create reusable and composable functions that can be called within or out of transactions and that can create transactions themselves.

## Getting started

```sh
$ go get github.com/heetch/sqalx
```

### Import sqalx

```go
import "github.com/heetch/sqalx"
```

### Usage

```go
package main

import (
	"log"

	"github.com/heetch/sqalx"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	// Connect to PostgreSQL with sqlx.
	db, err := sqlx.Connect("postgres", "user=foo dbname=bar sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	// Pass the db to sqalx.
	// It returns a sqalx.Node. A Node is a wrapper around sqlx.DB or sqlx.Tx.
	node, err := sqalx.New(db)
	if err != nil {
		log.Fatal(err)
	}

	err = createUser(node)
	if err != nil {
		log.Fatal(err)
	}
}

func createUser(node sqalx.Node) error {
	// Exec a query
	_, _ = node.Exec("INSERT INTO ....") // you can use a node as if it were a *sqlx.DB or a *sqlx.Tx

	// Let's create a transaction.
	// A transaction is also a sqalx.Node.
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, _ = tx.Exec("UPDATE ...")

	// Now we call another function and pass it the transaction.
	err = updateGroups(tx)
	if err != nil {
		return nil
	}

	return tx.Commit()
}

func updateGroups(node sqalx.Node) error {
	// Notice we are creating a new transaction.
	// This would normally cause a dead lock without sqalx.
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, _ = tx.Exec("INSERT ...")
	_, _ = tx.Exec("UPDATE ...")
	_, _ = tx.Exec("DELETE ...")

	return tx.Commit()
}
```

### PostgreSQL Savepoints

When using the PostgreSQL driver, an option can be passed to `New` to enable the use of PostgreSQL [Savepoints](https://www.postgresql.org/docs/8.1/static/sql-savepoint.html) for nested transactions.

```go
node, err := sqalx.New(db, sqalx.SavePoint(true))
```

## Issue
Please open an issue if you encounter any problem.

## Development
sqalx is covered by a go test suite.  In order to test against specific databases we include a docker-compose file that runs Postgres and MySQL.

### Running all tests
To run the tests, first run `docker-compose up` to run both Postgres and MySQL in locally-exposed docker images.  Then run your tests via `make test` which sets up the above described data sources and runs all tests.

### Running specific tests
To test against the Postgres instance be sure to export the following DSN:

```sh
export POSTGRESQL_DATASOURCE="postgresql://sqalx:sqalx@localhost:5432/sqalx?sslmode=disable"
```

To test against the MySQL instance be sure to export the following DSN:

```sh
export MYSQL_DATASOURCE="sqalx:sqalx@tcp(localhost:3306)/sqalx"
```

To test against SQlite export the following DSN:

```sh
export SQLITE_DATASOURCE=":memory:"
```

_Note:_ If you are developing on an M1 Mac you will need to use the officially supported by Oracle image rather than the default `mysql:tag` image.  It is commented out in `docker-compose.yml`.

## License
 The library is released under the MIT license. See [LICENSE](LICENSE) file.
