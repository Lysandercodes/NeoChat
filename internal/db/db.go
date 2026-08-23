package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
}

// Message mirrors a row in the messages table.
type Message struct {
	ID        string
	Sender    string
	Text      string
	Timestamp time.Time
	UpdatedAt time.Time
	Edited    bool
	Deleted   bool
	Status    string
}

// Open creates the chat directory if it doesn't exist and opens the SQLite database.
func Open(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	// WAL journal mode + busy timeout prevent SQLITE_BUSY under concurrent goroutines.
	dsn := dbPath + "?_journal=WAL&_foreign_keys=ON&_busy_timeout=5000"
	database, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Keep a single writer connection to avoid WAL contention.
	database.SetMaxOpenConns(1)

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

	CREATE INDEX IF NOT EXISTS idx_messages_updated_at ON messages(updated_at);

	CREATE TABLE IF NOT EXISTS sync_state (
		peer_id TEXT PRIMARY KEY,
		last_timestamp DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS telemetry (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		level TEXT NOT NULL,
		message TEXT NOT NULL,
		sent BOOLEAN DEFAULT FALSE
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

	if _, err := db.Exec(schema); err != nil {
		return err
	}

	log.Println("Database migrations applied successfully.")
	return nil
}

// ─────────────────────────────────────────────
//  Message operations
// ─────────────────────────────────────────────

// InsertMessage writes a new outgoing message.
func (d *DB) InsertMessage(msg Message) error {
	_, err := d.Exec(`
		INSERT INTO messages (id, sender, text, timestamp, updated_at, edited, deleted, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.Sender, msg.Text,
		msg.Timestamp.UTC(), msg.UpdatedAt.UTC(),
		msg.Edited, msg.Deleted, msg.Status,
	)
	return err
}

// UpsertMessage applies a received sync payload using "last-write-wins" semantics.
// A remote message only overwrites a local one if its updated_at is strictly newer.
func (d *DB) UpsertMessage(msg Message) error {
	_, err := d.Exec(`
		INSERT INTO messages (id, sender, text, timestamp, updated_at, edited, deleted, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			text       = excluded.text,
			updated_at = excluded.updated_at,
			edited     = excluded.edited,
			deleted    = excluded.deleted,
			status     = excluded.status
		WHERE excluded.updated_at > messages.updated_at`,
		msg.ID, msg.Sender, msg.Text,
		msg.Timestamp.UTC(), msg.UpdatedAt.UTC(),
		msg.Edited, msg.Deleted, msg.Status,
	)
	return err
}

// MessagesSince returns all messages with updated_at strictly after the given time.
// This is the "diff" used during the sync handshake.
func (d *DB) MessagesSince(since time.Time) ([]Message, error) {
	rows, err := d.Query(`
		SELECT id, sender, text, timestamp, updated_at, edited, deleted, status
		FROM messages
		WHERE updated_at > ?
		ORDER BY updated_at ASC`,
		since.UTC(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(
			&m.ID, &m.Sender, &m.Text,
			&m.Timestamp, &m.UpdatedAt,
			&m.Edited, &m.Deleted, &m.Status,
		); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// GetMessages returns all non-deleted messages ordered by timestamp for display.
func (d *DB) GetMessages() ([]Message, error) {
	rows, err := d.Query(`
		SELECT id, sender, text, timestamp, updated_at, edited, deleted, status
		FROM messages
		WHERE deleted = FALSE
		ORDER BY timestamp ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(
			&m.ID, &m.Sender, &m.Text,
			&m.Timestamp, &m.UpdatedAt,
			&m.Edited, &m.Deleted, &m.Status,
		); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// ─────────────────────────────────────────────
//  Sync state (High-Water Mark)
// ─────────────────────────────────────────────

// GetSyncState returns the last known updated_at timestamp we received from a peer.
// Returns the zero time if we have never synced with this peer.
func (d *DB) GetSyncState(peerID string) (time.Time, error) {
	var ts time.Time
	err := d.QueryRow(
		`SELECT last_timestamp FROM sync_state WHERE peer_id = ?`, peerID,
	).Scan(&ts)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	return ts, err
}

// SetSyncState updates the High-Water Mark for a peer after a successful sync.
func (d *DB) SetSyncState(peerID string, ts time.Time) error {
	_, err := d.Exec(`
		INSERT INTO sync_state (peer_id, last_timestamp) VALUES (?, ?)
		ON CONFLICT(peer_id) DO UPDATE SET last_timestamp = excluded.last_timestamp`,
		peerID, ts.UTC(),
	)
	return err
}

// ─────────────────────────────────────────────
//  Telemetry
// ─────────────────────────────────────────────

// LogTelemetry stores an error or info event locally.
func (d *DB) LogTelemetry(level, message string) error {
	_, err := d.Exec(
		`INSERT INTO telemetry (level, message) VALUES (?, ?)`,
		level, message,
	)
	return err
}

// GetUnsentTelemetry returns telemetry rows not yet pushed to the peer.
func (d *DB) GetUnsentTelemetry() ([]struct {
	ID      int64
	Level   string
	Message string
	Time    time.Time
}, error) {
	rows, err := d.Query(
		`SELECT id, level, message, timestamp FROM telemetry WHERE sent = FALSE ORDER BY id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []struct {
		ID      int64
		Level   string
		Message string
		Time    time.Time
	}
	for rows.Next() {
		var item struct {
			ID      int64
			Level   string
			Message string
			Time    time.Time
		}
		if err := rows.Scan(&item.ID, &item.Level, &item.Message, &item.Time); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// MarkTelemetrySent marks the given telemetry row IDs as sent.
func (d *DB) MarkTelemetrySent(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`UPDATE telemetry SET sent = TRUE WHERE id = ?`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, id := range ids {
		if _, err := stmt.Exec(id); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
