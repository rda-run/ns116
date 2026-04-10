package database

import (
	"log"
)

// AddDuplicateIgnore insere um value na lista de ignorados
func (db *DB) AddDuplicateIgnore(valueHash, valueText, username string) error {
	_, err := db.conn.Exec(
		"INSERT INTO duplicate_ignores (value_hash, value_text, ignored_by) VALUES ($1, $2, $3) ON CONFLICT(value_hash) DO NOTHING",
		valueHash, valueText, username,
	)
	if err != nil {
		log.Printf("Error adding duplicate ignore: %v", err)
	}
	return err
}

// RemoveDuplicateIgnore remove um value da lista de ignorados
func (db *DB) RemoveDuplicateIgnore(valueHash string) error {
	_, err := db.conn.Exec("DELETE FROM duplicate_ignores WHERE value_hash = $1", valueHash)
	return err
}

// GetIgnoredDuplicates retorna o set de hashes ignorados (para filtragem rápida)
func (db *DB) GetIgnoredDuplicates() (map[string]bool, error) {
	rows, err := db.conn.Query("SELECT value_hash FROM duplicate_ignores")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ignored := make(map[string]bool)
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return nil, err
		}
		ignored[hash] = true
	}
	return ignored, nil
}

// ResetIgnoredDuplicates remove todos os ignores da tabela
func (db *DB) ResetIgnoredDuplicates() error {
	_, err := db.conn.Exec("DELETE FROM duplicate_ignores")
	return err
}

// GetIgnoredCount retorna o total de registros ignorados
func (db *DB) GetIgnoredCount() (int, error) {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM duplicate_ignores").Scan(&count)
	return count, err
}
