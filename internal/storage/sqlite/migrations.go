package sqlite

import "database/sql"

const schemaVersion = 1

const schemaV1 = `
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT PRIMARY KEY,
    title      TEXT NOT NULL DEFAULT '',
    status     TEXT NOT NULL DEFAULT 'active'
               CHECK(status IN ('active','running','completed','failed')),
    provider   TEXT NOT NULL DEFAULT '',
    model      TEXT NOT NULL DEFAULT '',
    profile    TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_updated ON sessions(updated_at DESC);

CREATE TABLE IF NOT EXISTS session_messages (
    session_id TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    messages   TEXT NOT NULL DEFAULT '[]',
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
`

func runMigrations(db *sql.DB) error {
	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return err
	}

	// Check current version
	var current int
	row := db.QueryRow("SELECT version FROM schema_version LIMIT 1")
	if err := row.Scan(&current); err != nil {
		// Table doesn't exist or is empty — run initial schema
		current = 0
	}

	if current >= schemaVersion {
		return nil
	}

	if current < 1 {
		if _, err := db.Exec(schemaV1); err != nil {
			return err
		}
	}

	// Upsert schema version
	_, err := db.Exec(`
		DELETE FROM schema_version;
		INSERT INTO schema_version (version) VALUES (?);
	`, schemaVersion)
	return err
}
