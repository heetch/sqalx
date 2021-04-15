POSTGRESQL_DATASOURCE ?= postgresql://sqalx:sqalx@localhost:5432/sqalx?sslmode=disable
MYSQL_DATASOURCE ?= sqalx:sqalx@tcp(localhost:3306)/sqalx
SQLITE_DATASOURCE ?= :memory:

.PHONY: test

test:
	POSTGRESQL_DATASOURCE="$(POSTGRESQL_DATASOURCE)" \
	MYSQL_DATASOURCE="$(MYSQL_DATASOURCE)" \
	SQLITE_DATASOURCE="$(SQLITE_DATASOURCE)" \
	go test -v -cover -race -timeout=1m ./... && echo OK || (echo FAIL && exit 1)
