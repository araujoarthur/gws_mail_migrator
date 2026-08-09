package migrator

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	"github.com/araujoarthur/gws_mail_migrator/lib/utils"
	_ "modernc.org/sqlite"
)

const schemaEmailsTable = `
		CREATE TABLE IF NOT EXISTS emails (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			message_id TEXT NOT NULL,
			filename TEXT NOT NULL,
			file_hash BLOB NOT NULL,
			sender TEXT NOT NULL,
			dest TEXT NOT NULL,
			date DATETIME NOT NULL,
			migration_target_address TEXT NOT NULL,
			migration_status INTEGER NOT NULL DEFAULT 0
				CHECK (migration_status IN (0, 1, 2, 3)),
			retry_count INTEGER NOT NULL DEFAULT 0,
			last_claim_run_id TEXT,

			UNIQUE (
				file_hash,
				dest,
				migration_target_address
			)
		);

		CREATE INDEX IF NOT EXISTS idx_emails_pending
		ON emails(
    		migration_target_address,
    		migration_status,
    		date,
    		id
		);

		CREATE INDEX IF NOT EXISTS idx_emails_target_date
		ON emails(
    		migration_target_address,
    		date,
    		id
		);

		CREATE INDEX IF NOT EXISTS idx_emails_target_dest_date
		ON emails(
			migration_target_address,
			dest,
			date,
			id
		);
		`

func InitialDBSetup(logger *slog.Logger) error {
	if exists, err := utils.FileExists(utils.MANAGER_DB_PATH); exists || err != nil {
		if err != nil {
			return fmt.Errorf("failed to setup: %w", err)
		}

		return fmt.Errorf("db file already exists")
	}

	db, err := sql.Open("sqlite", utils.MANAGER_DB_PATH)
	if err != nil {
		return fmt.Errorf("failed to setup database (open): %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to setup database (ping): %w", err)
	}

	if _, err := db.Exec(schemaEmailsTable); err != nil {
		return fmt.Errorf("failed to setup schema for emails table: %w", err)
	}

	logger.Info("created the database schema successfully")
	return nil
}

func InitialFolderSetup() error {
	if exists, err := utils.FolderExists(utils.EMAILS_ROOT_PATH); exists || err != nil {
		if err != nil {
			return fmt.Errorf("failed to check root emails folder folder: %w", err)
		}

		return fmt.Errorf("root emails folder already exists")
	}

	if mkdirErr := os.Mkdir(utils.EMAILS_ROOT_PATH, 0o777); mkdirErr != nil {
		return fmt.Errorf("failed to create root emails folder: %w", mkdirErr)
	}

	fmt.Println("The root emails folder didn't exist but was succesfully created")
	return nil
}

func ResetDB() error {
	if exists, err := utils.FileExists(utils.MANAGER_DB_PATH); !exists || err != nil {
		if err != nil {
			return fmt.Errorf("failed to setup: %w", err)
		}

		return fmt.Errorf("db file does not exist")
	}

	db, err := sql.Open("sqlite", utils.MANAGER_DB_PATH)
	if err != nil {
		return fmt.Errorf("failed to reset database (open): %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to reset database (ping): %w", err)
	}

	schema := `
		DROP TABLE IF EXISTS emails;
		`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to reset schema: %w", err)
	}

	fmt.Println("Database was reset")
	return nil
}
