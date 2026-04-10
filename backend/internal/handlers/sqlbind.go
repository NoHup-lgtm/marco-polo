package handlers

import (
	"database/sql"
	"strconv"
	"strings"
)

func bindQuery(query string) string {
	var builder strings.Builder
	builder.Grow(len(query) + 16)
	idx := 0

	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			idx++
			builder.WriteByte('$')
			builder.WriteString(strconv.Itoa(idx))
			continue
		}
		builder.WriteByte(query[i])
	}

	return builder.String()
}

func dbExec(db *sql.DB, query string, args ...interface{}) (sql.Result, error) {
	return db.Exec(bindQuery(query), args...)
}

func dbQuery(db *sql.DB, query string, args ...interface{}) (*sql.Rows, error) {
	return db.Query(bindQuery(query), args...)
}

func dbQueryRow(db *sql.DB, query string, args ...interface{}) *sql.Row {
	return db.QueryRow(bindQuery(query), args...)
}

func txExec(tx *sql.Tx, query string, args ...interface{}) (sql.Result, error) {
	return tx.Exec(bindQuery(query), args...)
}

func txQueryRow(tx *sql.Tx, query string, args ...interface{}) *sql.Row {
	return tx.QueryRow(bindQuery(query), args...)
}

func insertAndReturnID(db *sql.DB, query string, args ...interface{}) (int64, error) {
	var id int64
	err := dbQueryRow(db, query+" RETURNING id", args...).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}
