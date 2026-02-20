package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// NewSQLite creates a new SQLite database connection
func NewSQLite(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// RunMigrations runs database migrations
func RunMigrations(db *sql.DB) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS agents (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			image TEXT NOT NULL,
			agent_type TEXT DEFAULT 'docker',
			status TEXT DEFAULT 'stopped',
			container_id TEXT,
			endpoint TEXT,
			listen_port INTEGER DEFAULT 9000,
			bearer_token TEXT,
			agent_card TEXT,
			config TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS channels (
			id TEXT PRIMARY KEY,
			members TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS task_logs (
			id TEXT PRIMARY KEY,
			channel_id TEXT,
			sender_id TEXT,
			recipient_id TEXT,
			status TEXT,
			messages TEXT,
			artifacts TEXT,
			direction TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_task_logs_channel_id ON task_logs(channel_id)`,
		`CREATE INDEX IF NOT EXISTS idx_task_logs_sender_id ON task_logs(sender_id)`,
		`CREATE INDEX IF NOT EXISTS idx_task_logs_created_at ON task_logs(created_at)`,
		`ALTER TABLE agents ADD COLUMN agent_type TEXT DEFAULT 'docker'`,
	}

	for _, migration := range migrations {
		_, _ = db.Exec(migration) // Ignore errors for ALTER TABLE if column exists
	}

	return nil
}
