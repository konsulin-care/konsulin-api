package migration

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	migrate "github.com/rubenv/sql-migrate"
)

func Run(db *sql.DB) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting working directory: %w", err)
	}

	migrations := &migrate.FileMigrationSource{
		Dir: filepath.Join(wd, "migration"),
	}

	n, err := migrate.Exec(db, "postgres", migrations, migrate.Up)
	if err != nil {
		return fmt.Errorf("error executing migration: %w", err)
	}

	log.Printf("Applied %d migrations!\n", n)
	return nil
}
