package migration

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"

	migrate "github.com/rubenv/sql-migrate"
)

func Run(db *sql.DB) {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting working directory: %v", err)
	}

	migrations := &migrate.FileMigrationSource{
		Dir: filepath.Join(wd, "internal/migration"),
	}

	n, err := migrate.Exec(db, "postgres", migrations, migrate.Up)
	if err != nil {
		log.Fatalf("Error executing migration: %v", err)
	}

	log.Printf("Applied %d migrations!\n", n)
}
