package data

import (
	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/go-sql-driver/mysql"

	"database/sql"
	"log"
	"time"
)

// Pool is how many connections a service keeps to MySQL. Zero leaves the
// database/sql default: no limit on open connections, two idle.
type Pool struct {
	MaxOpen int
	MaxIdle int
}

// Open connects to MySQL, waiting for it while it may still be starting:
// fifteen tries, two seconds apart. A connection is recycled after five
// minutes, before MySQL or a proxy between drops it.
func Open(dsn string, pool Pool) (*sql.DB, error) {
	var lastErr error

	for attempt := 1; attempt <= 15; attempt++ {
		db, err := sql.Open("mysql", dsn)
		if err == nil {
			if err = db.Ping(); err == nil {
				if pool.MaxOpen > 0 {
					db.SetMaxOpenConns(pool.MaxOpen)
				}
				if pool.MaxIdle > 0 {
					db.SetMaxIdleConns(pool.MaxIdle)
				}
				db.SetConnMaxLifetime(5 * time.Minute)
				return db, nil
			}
			_ = db.Close()
		}

		lastErr = err
		log.Printf("database not ready (attempt %d): %v", attempt, err)
		time.Sleep(2 * time.Second)
	}

	return nil, lastErr
}

// ConnectToDB is Open with the default pool, for the services that check for
// nil rather than an error.
func ConnectToDB(dsn string) *sql.DB {
	db, err := Open(dsn, Pool{})
	if err != nil {
		log.Println(err)
		return nil
	}
	log.Println("Connected to database!")
	return db
}

// NewMock returns store instance and SqlMock handler for testing
func NewMock() (*sql.DB, sqlmock.Sqlmock, error) {
	db, mock, err := sqlmock.New()
	if err != nil {
		return nil, mock, err
	}
	defer db.Close()

	return db, mock, nil
}
