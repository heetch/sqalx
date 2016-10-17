package sqalx_test

import (
	"testing"

	"github.com/heetch/sqalx"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	sqlmock "gopkg.in/DATA-DOG/go-sqlmock.v1"
)

func prepareDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	return sqlx.NewDb(db, "mock"), mock, func() {
		db.Close()
	}
}

func TestSqalxSimpleQuery(t *testing.T) {
	db, mock, cleanup := prepareDB(t)
	defer cleanup()

	mock.ExpectExec("UPDATE products").WillReturnResult(sqlmock.NewResult(1, 1))

	node := sqalx.New(db)

	_, err := node.Exec("UPDATE products SET views = views + 1")
	require.NoError(t, err)
}

func TestSqalxTopLevelTransaction(t *testing.T) {
	db, mock, cleanup := prepareDB(t)
	defer cleanup()
	var err error

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE products").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	node := sqalx.New(db)

	node, err = node.Beginx()
	require.NoError(t, err)
	require.NotNil(t, node)

	_, err = node.Exec("UPDATE products SET views = views + 1")
	require.NoError(t, err)

	err = node.Commit()
	require.NoError(t, err)
}
