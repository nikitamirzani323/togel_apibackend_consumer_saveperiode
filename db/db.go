package db

import (
	"database/sql"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/nikitamirzani323/togel_apibackend_consumer_saveperiode/helpers"
)

var db *sql.DB
var err error

func Init() {
	err := godotenv.Load()
	if err != nil {
		panic("Failed to load env file")
	}

	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	dbDriver := os.Getenv("DB_DRIVER")

	conString := dbUser + ":" + dbPass + "@tcp(" + dbHost + ":" + dbPort + ")/" + dbName

	db, err = sql.Open(dbDriver, conString)
	helpers.ErrorCheck(err)

	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(100)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(60 * time.Second)
	err = db.Ping()
	helpers.ErrorCheck(err)
}
func CreateCon() *sql.DB {
	return db
}
