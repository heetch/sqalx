package sqalx

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/rogpeppe/fastuuid"
)

var (
	// ErrNotInTransaction is returned when using Commit
	// outside of a transaction.
	ErrNotInTransaction = errors.New("not in transaction")

	// ErrIncompatibleOption is returned when using an option incompatible
	// with the selected driver.
	ErrIncompatibleOption = errors.New("incompatible option")
)

var uuids = fastuuid.MustNewGenerator()

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
	// Tx returns the underlying transaction.
	Tx() *sqlx.Tx
}

// A Driver can query the database. It can either be a *sqlx.DB or a *sqlx.Tx
// and therefore is limited to the methods they have in common.
type Driver interface {
	sqlx.Execer
	sqlx.ExecerContext
	sqlx.Queryer
	sqlx.QueryerContext
	sqlx.Preparer
	sqlx.PreparerContext
	BindNamed(query string, arg interface{}) (string, []interface{}, error)
	DriverName() string
	Get(dest interface{}, query string, args ...interface{}) error
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	MustExec(query string, args ...interface{}) sql.Result
	MustExecContext(ctx context.Context, query string, args ...interface{}) sql.Result
	NamedExec(query string, arg interface{}) (sql.Result, error)
	NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error)
	NamedQuery(query string, arg interface{}) (*sqlx.Rows, error)
	PrepareNamed(query string) (*sqlx.NamedStmt, error)
	PrepareNamedContext(ctx context.Context, query string) (*sqlx.NamedStmt, error)
	Preparex(query string) (*sqlx.Stmt, error)
	PreparexContext(ctx context.Context, query string) (*sqlx.Stmt, error)
	QueryRow(string, ...interface{}) *sql.Row
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
	Rebind(query string) string
	Select(dest interface{}, query string, args ...interface{}) error
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
}

// New creates a new Node with the given DB.
func New(db *sqlx.DB, options ...Option) (Node, error) {
	n := node{
		db:     db,
		Driver: db,
	}

	for _, opt := range options {
		err := opt(&n)
		if err != nil {
			return nil, err
		}
	}

	return &n, nil
}

// NewFromTransaction creates a new Node from the given transaction.
func NewFromTransaction(tx *sqlx.Tx, options ...Option) (Node, error) {
	n := node{
		tx:     tx,
		Driver: tx,
	}

	for _, opt := range options {
		err := opt(&n)
		if err != nil {
			return nil, err
		}
	}

	return &n, nil
}

// Connect to a database.
func Connect(driverName, dataSourceName string, options ...Option) (Node, error) {
	db, err := sqlx.Connect(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}

	node, err := New(db, options...)
	if err != nil {
		// the connection has been opened within this function, we must close it
		// on error.
		db.Close()
		return nil, err
	}

	return node, nil
}

type node struct {
	Driver
	db               *sqlx.DB
	tx               *sqlx.Tx
	savePointID      string
	savePointEnabled bool
	nested           bool
}

func (n *node) Close() error {
	return n.db.Close()
}

func (n node) Beginx() (Node, error) {
	var err error

	switch {
	case n.tx == nil:
		// new actual transaction
		n.tx, err = n.db.Beginx()
		n.Driver = n.tx
	case n.savePointEnabled:
		// already in a transaction: using savepoints
		n.nested = true
		// savepoints name must start with a char and cannot contain dashes (-)
		n.savePointID = "sp_" + strings.Replace(uuids.Hex128(), "-", "_", -1)
		_, err = n.tx.Exec("SAVEPOINT " + n.savePointID)
	default:
		// already in a transaction: reusing current transaction
		n.nested = true
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

	if n.savePointEnabled && n.savePointID != "" {
		_, err = n.tx.Exec("ROLLBACK TO SAVEPOINT " + n.savePointID)
	} else if !n.nested {
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
		_, err = n.tx.Exec("RELEASE SAVEPOINT " + n.savePointID)
	} else if !n.nested {
		err = n.tx.Commit()
	}

	if err != nil {
		return err
	}

	n.tx = nil
	n.Driver = nil

	return nil
}

// Tx returns the underlying transaction.
func (n *node) Tx() *sqlx.Tx {
	return n.tx
}

// Option to configure sqalx
type Option func(*node) error

// SavePoint option enables PostgreSQL and SQLite Savepoints for nested
// transactions.
func SavePoint(enabled bool) Option {
	return func(n *node) error {
		driverName := n.Driver.DriverName()
		if enabled && driverName != "postgres" && driverName != "sqlite3" {
			return ErrIncompatibleOption
		}
		n.savePointEnabled = enabled
		return nil
	}
}
