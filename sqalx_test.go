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

func TestSqalxTransactionViolations(t *testing.T) {
	node := sqalx.New(nil)

	require.Panics(t, func() {
		node.Exec("UPDATE products SET views = views + 1")
	})

	require.Panics(t, func() {
		node.Beginx()
	})

	// calling Rollback after a transaction is closed does nothing
	err := node.Rollback()
	require.NoError(t, err)

	err = node.Commit()
	require.Equal(t, err, sqalx.ErrNotInTransaction)
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
	defer func() {
		err = node.Rollback()
		require.NoError(t, err)
	}()

	_, err = node.Exec("UPDATE products SET views = views + 1")
	require.NoError(t, err)

	err = node.Commit()
	require.NoError(t, err)
}

func TestSqalxNestedTransactions(t *testing.T) {
	db, mock, cleanup := prepareDB(t)
	defer cleanup()

	var err error
	const query = "UPDATE products SET views = views + 1"

	mock.ExpectExec("UPDATE products").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE products").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("SAVEPOINT").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE products").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("ROLLBACK TO").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("SAVEPOINT").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE products").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("RELEASE TO").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	node := sqalx.New(db)

	_, err = node.Exec(query)
	require.NoError(t, err)

	n1, err := node.Beginx()
	require.NoError(t, err)
	require.NotNil(t, n1)

	_, err = n1.Exec(query)
	require.NoError(t, err)

	n1_1, err := n1.Beginx()
	require.NoError(t, err)
	require.NotNil(t, n1_1)

	_, err = n1_1.Exec(query)
	require.NoError(t, err)

	err = n1_1.Rollback()
	require.NoError(t, err)

	err = n1_1.Commit()
	require.Equal(t, sqalx.ErrNotInTransaction, err)

	n1_1, err = n1.Beginx()
	require.NoError(t, err)
	require.NotNil(t, n1_1)

	_, err = n1_1.Exec(query)
	require.NoError(t, err)

	err = n1_1.Commit()
	require.NoError(t, err)

	err = n1_1.Commit()
	require.Equal(t, sqalx.ErrNotInTransaction, err)

	err = n1_1.Rollback()
	require.NoError(t, err)

	err = n1.Commit()
	require.NoError(t, err)
}
