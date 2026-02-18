package database

import (
	"database/sql"
	"time"
)

func (db *DB) CreateSession(token, csrfToken, username string, expiresAt time.Time) error {
	_, err := db.conn.Exec(
		"INSERT INTO sessions (token, csrf_token, username, expires_at) VALUES ($1, $2, $3, $4)",
		token, csrfToken, username, expiresAt,
	)
	return err
}

func (db *DB) GetSession(token string) (string, string, time.Time, error) {
	var username, csrfToken string
	var expiresAt time.Time
	err := db.conn.QueryRow(
		"SELECT username, csrf_token, expires_at FROM sessions WHERE token = $1", token,
	).Scan(&username, &csrfToken, &expiresAt)
	if err == sql.ErrNoRows {
		return "", "", time.Time{}, nil
	}
	if err != nil {
		return "", "", time.Time{}, err
	}
	return username, csrfToken, expiresAt, nil
}

func (db *DB) DeleteSession(token string) error {
	_, err := db.conn.Exec("DELETE FROM sessions WHERE token = $1", token)
	return err
}

func (db *DB) PurgeExpiredSessions() error {
	_, err := db.conn.Exec("DELETE FROM sessions WHERE expires_at < NOW()")
	return err
}
