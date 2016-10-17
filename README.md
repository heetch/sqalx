# sqalx

sqalx is a library that allows to seamlessly create nested transactions and to avoid thinking about whether or not a function is called within a transaction.
With sqalx you can easily create reusable and composable functions that can be called within or out of transactions and that can create transactions themselves.

It is built on top of [sqlx](https://github.com/jmoiron/sqlx) and currently supports only PostgreSQL.

## Getting started

```sh
$ go get github.com/heetch/sqalx
```

### Import sqlax

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
	// Connect to PostgreSQL with sqlx
	db, err := sqlx.Connect("postgres", "user=foo dbname=bar sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// create a sqalx.Node. A Node is a wrapper around sqlx.DB or sqlx.Tx
	node := sqalx.New(db)

	err = createUser(node)
	if err != nil {
		log.Fatal(err)
	}
}

func createUser(node sqalx.Node) error {
	// Exec a query
	_, _ = tx.Exec("INSERT INTO ....") // you can use a node as if it were a *sqlx.DB or a *sqlx.Tx

	// let's create a transaction
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, _ = tx.Exec("UPDATE ...")

	err = updateGroups(tx)
	if err != nil {
		return nil
	}

	return tx.Commit()
}

func updateGroups(node *sqlax.Node) error {
	// Notice we are creating a new transaction.
	// This would cause a dead lock without sqalx.
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
