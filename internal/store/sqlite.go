package store

import (
	"database/sql"
	"fmt"
	"regexp"

	_ "modernc.org/sqlite"
)

// validIdentifierPattern matches valid SQL identifiers (alphanumeric and underscores only)
// This prevents SQL injection in ALTER TABLE statements
var validIdentifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// allowedTables is a whitelist of tables that can be altered during migrations
var allowedTables = map[string]bool{
	"agents": true,
	"models": true,
}

// validateIdentifier checks if a table or column name is safe to use in SQL
func validateIdentifier(name string) error {
	if !validIdentifierPattern.MatchString(name) {
		return fmt.Errorf("invalid identifier: %q (must match [a-zA-Z_][a-zA-Z0-9_]*)", name)
	}
	return nil
}

// NewSQLite creates a new SQLite database connection
func NewSQLite(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Enable WAL mode for concurrent read/write support and set a busy
	// timeout so writers retry instead of immediately returning SQLITE_BUSY.
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000")

	return db, nil
}

// RunMigrations runs database migrations
func RunMigrations(db *sql.DB) error {
	// One-time migration: if models table has 'provider' column, drop it to recreate with new schema
	// This is safe because models are auto-discovered from auth configs.
	var hasProvider bool
	rows, err := db.Query("PRAGMA table_info(models)")
	if err == nil {
		for rows.Next() {
			var cid int
			var name string
			var dtype string
			var notnull int
			var dfltValue interface{}
			var pk int
			if err := rows.Scan(&cid, &name, &dtype, &notnull, &dfltValue, &pk); err == nil {
				if name == "provider" {
					hasProvider = true
				}
			}
		}
		rows.Close()
	}
	if hasProvider {
		fmt.Println("[migration] dropping models table to remove legacy provider column")
		_, _ = db.Exec("DROP TABLE models")
	}

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
			listen_port INTEGER DEFAULT 17000,
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
		`CREATE TABLE IF NOT EXISTS models (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			auth_config_id TEXT,
			model_identifier TEXT NOT NULL,
			endpoint_url TEXT,
			default_params TEXT,
			is_default INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS auth_configs (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			provider TEXT NOT NULL,
			auth_type TEXT NOT NULL,
			credentials TEXT,
			endpoint_url TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_task_logs_channel_id ON task_logs(channel_id)`,
		`CREATE INDEX IF NOT EXISTS idx_task_logs_sender_id ON task_logs(sender_id)`,
		`CREATE INDEX IF NOT EXISTS idx_task_logs_created_at ON task_logs(created_at)`,
		`CREATE TABLE IF NOT EXISTS agent_identity_files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			agent_id TEXT NOT NULL,
			filename TEXT NOT NULL,
			content TEXT NOT NULL DEFAULT '',
			char_count INTEGER DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(agent_id, filename),
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_identity_files_agent_id ON agent_identity_files(agent_id)`,
	}

	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	// Add missing columns to existing tables
	alterMigrations := []struct {
		table, column, definition string
	}{
		{"agents", "agent_type", "TEXT DEFAULT 'docker'"},
		{"agents", "listen_port", "INTEGER DEFAULT 17000"},
		{"agents", "model_id", "TEXT"},
		{"agents", "auth_config_id", "TEXT"},
		{"models", "is_default", "INTEGER DEFAULT 0"},
		{"models", "auth_config_id", "TEXT DEFAULT ''"},
		{"agents", "peer_agent_ids", "TEXT DEFAULT '[]'"},
	}
	for _, m := range alterMigrations {
		// Security: Validate table name against whitelist
		if !allowedTables[m.table] {
			return fmt.Errorf("migration error: table %q is not in allowed list", m.table)
		}
		// Security: Validate column name to prevent SQL injection
		if err := validateIdentifier(m.column); err != nil {
			return fmt.Errorf("migration error: %w", err)
		}

		var count int
		err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`, m.table, m.column).Scan(&count)
		if err == nil && count == 0 {
			// Safe to use fmt.Sprintf here because we've validated identifiers
			_, _ = db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, m.table, m.column, m.definition))
		}
	}

	return nil
}
