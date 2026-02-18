package database

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL Driver
)

type DB struct {
	conn *sql.DB
}

func Open(dsn string, migrationsFS fs.FS) (*DB, error) {
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Recommended pool configuration for production
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(25)
	conn.SetConnMaxLifetime(5 * time.Minute)

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Run Migrations
	if err := runMigrations(conn, dsn, migrationsFS); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return &DB{conn: conn}, nil
}

func runMigrations(conn *sql.DB, dsn string, migrationsFS fs.FS) error {
	driver, err := postgres.WithInstance(conn, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("could not create migration driver: %w", err)
	}

	var m *migrate.Migrate

	if migrationsFS != nil {
		// Use embedded migrations
		d, err := iofs.New(migrationsFS, "migrations")
		if err != nil {
			return fmt.Errorf("could not create iofs source: %w", err)
		}
		m, err = migrate.NewWithInstance(
			"iofs",
			d,
			"postgres",
			driver,
		)
	} else {
		// Fallback to file system (useful for dev without build)
		m, err = migrate.NewWithDatabaseInstance(
			"file://db/migrations",
			"postgres",
			driver,
		)
	}

	if err != nil {
		return fmt.Errorf("could not create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("an error occurred while syncing the database: %w", err)
	}

	log.Println("Database migrations applied successfully")
	return nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) HasUsers() (bool, error) {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count > 0, err
}

func (db *DB) GetSetting(key string) (string, error) {
	var value string
	// Updated for Postgres placeholders
	err := db.conn.QueryRow("SELECT value FROM settings WHERE key = $1", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (db *DB) SetSetting(key, value string) error {
	// Updated for Postgres upsert syntax and placeholders
	_, err := db.conn.Exec(
		"INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT(key) DO UPDATE SET value = $3",
		key, value, value,
	)
	return err
}

func (db *DB) EnsureSessionSecret() (string, error) {
	secret, err := db.GetSetting("session_secret")
	if err != nil {
		return "", err
	}
	if secret != "" {
		return secret, nil
	}
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate session secret: %w", err)
	}
	secret = hex.EncodeToString(b)
	if err := db.SetSetting("session_secret", secret); err != nil {
		return "", err
	}
	log.Println("Generated new session secret")
	return secret, nil
}
