package database

import (
	"database/sql"

	"golang.org/x/crypto/bcrypt"

	"ns116/internal/model"
)

func (db *DB) GetUserByUsername(username string) (*model.User, error) {
	u := &model.User{}
	err := db.conn.QueryRow(
		"SELECT id, username, pass_hash, role, active, auth_source, created_at, updated_at FROM users WHERE username = $1",
		username,
	).Scan(&u.ID, &u.Username, &u.PassHash, &u.Role, &u.Active, &u.AuthSource, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (db *DB) ListUsers() ([]model.User, error) {
	rows, err := db.conn.Query(
		"SELECT id, username, pass_hash, role, active, auth_source, created_at, updated_at FROM users ORDER BY id",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Username, &u.PassHash, &u.Role, &u.Active, &u.AuthSource, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (db *DB) CreateUser(username, password, role string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}
	_, err = db.conn.Exec(
		"INSERT INTO users (username, pass_hash, role) VALUES ($1, $2, $3)",
		username, string(hash), role,
	)
	return err
}

func (db *DB) UpdateUserPassword(username, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return err
	}
	_, err = db.conn.Exec("UPDATE users SET pass_hash = $1, updated_at = NOW() WHERE username = $2",
		string(hash), username)
	return err
}

func (db *DB) SetUserActive(username string, active bool) error {
	activeInt := 0
	if active {
		activeInt = 1
	}
	// Postgres boolean is preferred, but schema uses INTEGER for compatibility with original design
	// Let's stick to INTEGER 0/1 as per schema.
	_, err := db.conn.Exec("UPDATE users SET active = $1, updated_at = NOW() WHERE username = $2",
		activeInt, username)
	return err
}

func (db *DB) DeleteUser(username string) error {
	_, err := db.conn.Exec("DELETE FROM users WHERE username = $1", username)
	return err
}

func (db *DB) AuthenticateUser(username, password string) (*model.User, error) {
	u, err := db.GetUserByUsername(username)
	if err != nil || u == nil || !u.Active {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PassHash), []byte(password)); err != nil {
		return nil, nil
	}
	return u, nil
}

func (db *DB) CreateLDAPUser(username, role string) error {
	_, err := db.conn.Exec(
		`INSERT INTO users (username, pass_hash, role, auth_source)
		 VALUES ($1, '', $2, 'ldap')
		 ON CONFLICT(username) DO UPDATE SET
		   role = $3, auth_source = 'ldap', updated_at = NOW()`,
		username, role, role,
	)
	return err
}
