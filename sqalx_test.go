package sqalx_test

import (
	"context"
	"os"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	_ "github.com/go-sql-driver/mysql"
	"github.com/heetch/sqalx"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
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
		t.Log("skipping due to blank POSTGRESQL_DATASOURCE")
		t.Skip()
		return
	}

	testSqalxConnect(t, "postgres", dataSource)
	testSqalxConnect(t, "postgres", dataSource, sqalx.SavePoint(true))
}
func TestSqalxConnectPGX(t *testing.T) {
	dataSource := os.Getenv("POSTGRESQL_DATASOURCE")
	if dataSource == "" {
		t.Log("skipping due to blank POSTGRESQL_DATASOURCE")
		t.Skip()
		return
	}

	testSqalxConnect(t, "pgx", dataSource)
	testSqalxConnect(t, "pgx", dataSource, sqalx.SavePoint(true))
}

func TestSqalxConnectSqlite(t *testing.T) {
	dataSource := os.Getenv("SQLITE_DATASOURCE")
	if dataSource == "" {
		t.Skip()
		return
	}

	testSqalxConnect(t, "sqlite3", dataSource)
	testSqalxConnect(t, "sqlite3", dataSource, sqalx.SavePoint(true))
}

func TestSqalxConnectMySQL(t *testing.T) {
	dataSource := os.Getenv("MYSQL_DATASOURCE")
	if dataSource == "" {
		t.Log("skipping due to blank MYSQL_DATASOURCE")
		t.Skip()
		return
	}

	testSqalxConnect(t, "mysql", dataSource)
	testSqalxConnect(t, "mysql", dataSource, sqalx.SavePoint(true))
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
		//nolint:errcheck // the intended panic makes error checking irrelevant
		node.Exec("UPDATE products SET views = views + 1")
	})

	require.Panics(t, func() {
		//nolint:errcheck // the intended panic makes error checking irrelevant
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
	testSqalxNestedTransactions(t, "mock", false)
}

func TestSqalxNestedTransactionsWithSavePoint(t *testing.T) {
	for _, driver := range []string{
		"postgres",
		"pgx",
		"sqlite3",
		"mysql",
	} {
		t.Run(driver, func(t *testing.T) {
			testSqalxNestedTransactions(t, driver, true)
		})
	}
}

func testSqalxNestedTransactions(t *testing.T, driverName string, testSavePoint bool) {
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

	n1_1, err = n1.BeginTxx(context.Background(), nil)
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
	require.NoError(t, err)
	_, err = ntx.Exec("UPDATE products SET views = views + 1")
	require.NoError(t, err)

	err = ntx.Rollback()
	require.NoError(t, err)

	err = node.Rollback()
	require.NoError(t, err)
}
