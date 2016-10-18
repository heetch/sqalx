package sqalx

import (
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	uuid "github.com/satori/go.uuid"
)

// Errors
var (
	ErrNotInTransaction = errors.New("not in transaction")
)

// A Node is a database driver that can manage nested transactions.
type Node interface {
	Driver

	// Close the underlying sqlx connection.
	Close() error
	// Begin a new transaction.
	Beginx() (Node, error)
	// Rollback the associated transaction.
	Rollback() error
	// Commit the assiociated transaction.
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

// Connect to a database.
func Connect(driverName, dataSourceName string) (Node, error) {
	db, err := sqlx.Connect(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}

	return New(db), nil
}

type node struct {
	Driver
	db          *sqlx.DB
	tx          *sqlx.Tx
	savePointID string
}

func (n *node) Close() error {
	return n.db.Close()
}

func (n node) Beginx() (Node, error) {
	var err error

	if n.tx == nil {
		n.tx, err = n.db.Beginx()
		n.Driver = n.tx
	} else {
		n.savePointID = uuid.NewV1().String()
		_, err = n.tx.Exec("SAVEPOINT $1", n.savePointID)
	}

	if err != nil {
		return nil, err
	}

	return &n, nil
}

func (n *node) Rollback() error {
	if n.tx == nil {
		return nil
	}

	var err error

	if n.savePointID != "" {
		_, err = n.tx.Exec("ROLLBACK TO SAVEPOINT $1", n.savePointID)
	} else {
		err = n.tx.Rollback()
	}

	if err != nil {
		return err
	}

	n.tx = nil
	n.Driver = nil

	return nil
}

func (n *node) Commit() error {
	if n.tx == nil {
		return ErrNotInTransaction
	}

	var err error

	if n.savePointID != "" {
		_, err = n.tx.Exec("RELEASE TO SAVEPOINT $1", n.savePointID)
	} else {
		err = n.tx.Commit()
	}

	if err != nil {
		return err
	}

	n.tx = nil
	n.Driver = nil

	return nil
}
