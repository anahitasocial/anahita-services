package data

import (
	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/go-sql-driver/mysql"

	"database/sql"
	"log"
	"time"
)

var counts uint32

func ConnectToDB(dsn string) *sql.DB {
	for {
		connection, err := openDB(dsn)

		if err != nil {
			log.Println("Database not ready...")
			counts++
		} else {
			log.Println("Connected to database!")
			return connection
		}

		if counts > 10 {
			log.Println(err)
			return nil
		}

		log.Println("Backing off for two seconds...")
		time.Sleep(2 * time.Second)
		continue
	}
}

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, nil
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
