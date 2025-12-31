package database

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// Config holds database configuration
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// NewPostgresDB creates a new PostgreSQL database connection
func NewPostgresDB(cfg Config) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	return db, nil
}

// InitSchema initializes the database schema
func InitSchema(db *sql.DB) error {
	schema := `
	-- Users table
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		email VARCHAR(255) UNIQUE NOT NULL,
		name VARCHAR(100) NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

	-- Message metadata table (audit log)
	-- CRITICAL: This stores WHO sent to WHOM, but NEVER the content
	-- Content remains ephemeral in Redis
	CREATE TABLE IF NOT EXISTS message_metadata (
		id SERIAL PRIMARY KEY,
		message_id VARCHAR(255) UNIQUE NOT NULL,
		sender_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		recipient_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		status VARCHAR(20) NOT NULL DEFAULT 'pending',
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		read_at TIMESTAMP,
		expires_at TIMESTAMP NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_metadata_message_id ON message_metadata(message_id);
	CREATE INDEX IF NOT EXISTS idx_metadata_sender_id ON message_metadata(sender_id);
	CREATE INDEX IF NOT EXISTS idx_metadata_recipient_id ON message_metadata(recipient_id);
	CREATE INDEX IF NOT EXISTS idx_metadata_status ON message_metadata(status);

	-- Add encryption_key column if it doesn't exist (for recipient link generation)
	DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns
					   WHERE table_name='message_metadata' AND column_name='encryption_key') THEN
			ALTER TABLE message_metadata ADD COLUMN encryption_key TEXT;
		END IF;
	END $$;

	-- Add is_admin column if it doesn't exist
	DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns
					   WHERE table_name='users' AND column_name='is_admin') THEN
			ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT false;
		END IF;
	END $$;

	-- Make default admin account an admin
	UPDATE users SET is_admin = true WHERE email = 'admin@vanish.local' AND is_admin = false;
	`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	return nil
}
