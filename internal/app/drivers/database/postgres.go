package database

import (
	"database/sql"
	"fmt"
	"konsulin-service/internal/app/config"
	"log"

	_ "github.com/lib/pq"
)

func NewPostgresDB(driverConfig *config.DriverConfig) *sql.DB {
	connectionString := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		driverConfig.PostgresDB.Host,
		driverConfig.PostgresDB.Port,
		driverConfig.PostgresDB.Username,
		driverConfig.PostgresDB.Password,
		driverConfig.PostgresDB.DBName)

	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		log.Fatalf("Failed to open postgres database connection: %s", err.Error())
	}

	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to connect to postgres database: %s", err.Error())
	}

	log.Println("Successfully connected to postgres database")

	return db
}
