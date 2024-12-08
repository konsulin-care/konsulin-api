package migration

import (
	"database/sql"
	"log"

	migrate "github.com/rubenv/sql-migrate"
)

func Run(db *sql.DB) {
	migrations := &migrate.FileMigrationSource{
		Dir: "migration",
	}

	n, err := migrate.Exec(db, "postgres", migrations, migrate.Up)
	if err != nil {
		log.Fatalf("Error executing migration: %v", err)
	}

	log.Printf("Applied %d migrations!\n", n)
}
