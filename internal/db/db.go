package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
}

// Open creates the chat directory if it doesn't exist and opens the SQLite database.
func Open(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	database, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := database.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := runMigrations(database); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &DB{database}, nil
}

func runMigrations(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS config (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		sender TEXT NOT NULL,
		text TEXT NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		edited BOOLEAN DEFAULT FALSE,
		deleted BOOLEAN DEFAULT FALSE,
		status TEXT DEFAULT 'sent' -- sent, delivered, read
	);

	CREATE TABLE IF NOT EXISTS sync_state (
		peer_id TEXT PRIMARY KEY,
		last_timestamp DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS telemetry (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		level TEXT NOT NULL,
		message TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS files (
		id TEXT PRIMARY KEY,
		message_id TEXT NOT NULL,
		filename TEXT NOT NULL,
		size INTEGER NOT NULL,
		local_path TEXT NOT NULL,
		FOREIGN KEY(message_id) REFERENCES messages(id) ON DELETE CASCADE
	);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return err
	}

	log.Println("Database migrations applied successfully.")
	return nil
}
