package sqlite

import (
	"database/sql"
	"log"
	"strings"

	"github.com/mattn/go-sqlite3"
	"github.com/nkanaev/yarr/src/content/htmlutil"
)

func init() {
	sql.Register("sqlite3_yarr", &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			return conn.RegisterFunc("strip_html", htmlutil.ExtractText, true)
		},
	})
}

type SQLiteStorage struct {
	db *sql.DB
}

func New(path string) (*SQLiteStorage, error) {
	if pos := strings.IndexRune(path, '?'); pos == -1 {
		// _txlock=immediate: transactions take the write lock up front, so
		// concurrent writers wait (busy_timeout) instead of failing when a
		// deferred transaction cannot upgrade its lock.
		params := "_journal=WAL&_sync=NORMAL&_busy_timeout=5000&_txlock=immediate"
		// Shared-cache mode reports table-level contention as SQLITE_LOCKED,
		// which bypasses busy_timeout and made concurrent refresh writes fail
		// with "database is locked". It is only needed so that all pooled
		// connections see the same in-memory database.
		if strings.Contains(path, ":memory:") {
			params += "&cache=shared"
		}
		log.Printf("opening db with params: %s", params)
		path = path + "?" + params
	}

	db, err := sql.Open("sqlite3_yarr", path)
	if err != nil {
		return nil, err
	}

	if err = migrate(db); err != nil {
		return nil, err
	}
	return &SQLiteStorage{db: db}, nil
}

func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// Ping verifies the database is reachable and its schema readable.
func (s *SQLiteStorage) Ping() error {
	var n int
	return s.db.QueryRow("select count(*) from settings").Scan(&n)
}
