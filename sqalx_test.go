package sqalx_test

import (
	"os"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	_ "github.com/go-sql-driver/mysql"
	"github.com/heetch/sqalx"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func prepareDB(t *testing.T, driverName string) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	return sqlx.NewDb(db, driverName), mock, func() {
		db.Close()
	}
}

func TestSqalxConnectPostgreSQL(t *testing.T) {
	dataSource := os.Getenv("POSTGRESQL_DATASOURCE")
	if dataSource == "" {
		t.Skip()
		return
	}

	testSqalxConnect(t, "postgres", dataSource)
	testSqalxConnect(t, "postgres", dataSource, sqalx.SavePoint(true))
}

func TestSqalxConnectMySQL(t *testing.T) {
	dataSource := os.Getenv("MYSQL_DATASOURCE")
	if dataSource == "" {
		t.Skip()
		return
	}

	testSqalxConnect(t, "mysql", dataSource)

	node, err := sqalx.Connect("mysql", dataSource, sqalx.SavePoint(true))
	require.Equal(t, sqalx.ErrIncompatibleOption, err)
	require.Nil(t, node)
}

func testSqalxConnect(t *testing.T, driverName, dataSource string, options ...sqalx.Option) {
	node, err := sqalx.Connect(driverName, dataSource, options...)
	require.NoError(t, err)

	err = node.Close()
	require.NoError(t, err)
}

func TestSqalxTransactionViolations(t *testing.T) {
	node, err := sqalx.New(nil)
	require.NoError(t, err)

	require.Panics(t, func() {
		node.Exec("UPDATE products SET views = views + 1")
	})

	require.Panics(t, func() {
		node.Beginx()
	})

	// calling Rollback after a transaction is closed does nothing
	err = node.Rollback()
	require.NoError(t, err)

	err = node.Commit()
	require.Equal(t, err, sqalx.ErrNotInTransaction)
}

func TestSqalxSimpleQuery(t *testing.T) {
	db, mock, cleanup := prepareDB(t, "mock")
	defer cleanup()

	mock.ExpectExec("UPDATE products").WillReturnResult(sqlmock.NewResult(1, 1))

	node, err := sqalx.New(db)
	require.NoError(t, err)

	_, err = node.Exec("UPDATE products SET views = views + 1")
	require.NoError(t, err)
}

func TestSqalxTopLevelTransaction(t *testing.T) {
	db, mock, cleanup := prepareDB(t, "mock")
	defer cleanup()
	var err error

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE products").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	node, err := sqalx.New(db)
	require.NoError(t, err)

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
	testSqalxNestedTransactions(t, false)
}

func TestSqalxNestedTransactionsWithSavePoint(t *testing.T) {
	testSqalxNestedTransactions(t, true)
}

func testSqalxNestedTransactions(t *testing.T, testSavePoint bool) {
	driverName := "mock"
	if testSavePoint {
		driverName = "postgres"
	}

	db, mock, cleanup := prepareDB(t, driverName)
	defer cleanup()

	require.Equal(t, driverName, db.DriverName())

	var err error
	const query = "UPDATE products SET views = views + 1"

	mock.ExpectExec("UPDATE products").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE products").WillReturnResult(sqlmock.NewResult(1, 1))
	if testSavePoint {
		mock.ExpectExec("SAVEPOINT").WillReturnResult(sqlmock.NewResult(1, 1))
	}
	mock.ExpectExec("UPDATE products").WillReturnResult(sqlmock.NewResult(1, 1))
	if testSavePoint {
		mock.ExpectExec("ROLLBACK TO SAVEPOINT").WillReturnResult(sqlmock.NewResult(1, 1))
	}
	if testSavePoint {
		mock.ExpectExec("SAVEPOINT").WillReturnResult(sqlmock.NewResult(1, 1))
	}
	mock.ExpectExec("UPDATE products").WillReturnResult(sqlmock.NewResult(1, 1))
	if testSavePoint {
		mock.ExpectExec("RELEASE SAVEPOINT").WillReturnResult(sqlmock.NewResult(1, 1))
	}
	mock.ExpectCommit()

	node, err := sqalx.New(db, sqalx.SavePoint(testSavePoint))
	require.NoError(t, err)

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

func TestSqalxFromTransaction(t *testing.T) {
	db, mock, cleanup := prepareDB(t, "mock")
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE products").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE products").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectRollback()

	tx, err := db.Beginx()
	require.NoError(t, err)

	node, err := sqalx.NewFromTransaction(tx)
	require.NoError(t, err)

	_, err = node.Exec("UPDATE products SET views = views + 1")
	require.NoError(t, err)

	ntx, err := node.Beginx()
	_, err = ntx.Exec("UPDATE products SET views = views + 1")
	require.NoError(t, err)

	err = ntx.Rollback()
	require.NoError(t, err)

	err = node.Rollback()
	require.NoError(t, err)
}
