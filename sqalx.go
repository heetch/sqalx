package sqalx

import (
	"database/sql"
	"errors"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
)

// Errors
var (
	ErrNotInTransaction = errors.New("not in transaction")
)

// A Node is database driver that can manages nested transactions
type Node interface {
	Driver
	Beginx() (Node, error)
	Rollback() error
	Commit() error
}

// A Driver can query the database. It can either be a *sqlx.DB or a *sqlx.Tx
// and therefore is limited to the methods they have in common.
type Driver interface {
	sqlx.Execer
	sqlx.Queryer
	sqlx.Preparer
	BindNamed(query string, arg interface{}) (string, []interface{}, error)
	DriverName() string
	Get(dest interface{}, query string, args ...interface{}) error
	MustExec(query string, args ...interface{}) sql.Result
	NamedExec(query string, arg interface{}) (sql.Result, error)
	NamedQuery(query string, arg interface{}) (*sqlx.Rows, error)
	PrepareNamed(query string) (*sqlx.NamedStmt, error)
	Preparex(query string) (*sqlx.Stmt, error)
	Rebind(query string) string
	Select(dest interface{}, query string, args ...interface{}) error
}

// New creates a new Node with the given DB.
func New(db *sqlx.DB) Node {
	return &node{
		db:     db,
		Driver: db,
	}
}

type node struct {
	Driver
	db          *sqlx.DB
	tx          *sqlx.Tx
	savePointID string
}

func (n node) Beginx() (Node, error) {
	var err error

	if n.tx == nil {
		n.tx, err = n.db.Beginx()
		n.Driver = n.tx
	} else {
		// TODO replace by a more deterministic thread safe random string generator.
		// Using time.Now temporarily to avoid complexity.
		n.savePointID = strconv.Itoa(int(time.Now().UnixNano()))
		_, err = n.tx.Exec("SAVEPOINT $1", n.savePointID)
	}

	if err != nil {
		return nil, err
	}

	return &n, nil
}

func (n *node) Rollback() error {
	if n.tx == nil {
		return ErrNotInTransaction
	}

	if n.savePointID != "" {
		_, err := n.tx.Exec("ROLLBACK TO SAVEPOINT $1", n.savePointID)
		n.tx = nil
		n.Driver = nil
		return err
	}

	return n.tx.Rollback()
}

func (n *node) Commit() error {
	if n.tx == nil {
		return ErrNotInTransaction
	}

	if n.savePointID != "" {
		_, err := n.tx.Exec("RELEASE TO SAVEPOINT $1", n.savePointID)
		n.tx = nil
		n.Driver = nil
		return err
	}

	return n.tx.Commit()
}
