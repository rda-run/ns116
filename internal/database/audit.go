package database

import "ns116/internal/model"

func (db *DB) LogAudit(entry model.AuditEntry) error {
	_, err := db.conn.Exec(
		`INSERT INTO audit_log (username, action, zone_id, record_name, record_type, detail, ip_address)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		entry.Username, entry.Action, entry.ZoneID, entry.RecordName,
		entry.RecordType, entry.Detail, entry.IPAddress,
	)
	return err
}

func (db *DB) ListAuditLog(limit, offset int) ([]model.AuditEntry, int, error) {
	var total int
	_ = db.conn.QueryRow("SELECT COUNT(*) FROM audit_log").Scan(&total)

	// Postgres uses $1, $2 for limit and offset
	rows, err := db.conn.Query(
		`SELECT id, username, action, zone_id, record_name, record_type, detail, ip_address, created_at
		 FROM audit_log ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []model.AuditEntry
	for rows.Next() {
		var e model.AuditEntry
		if err := rows.Scan(&e.ID, &e.Username, &e.Action, &e.ZoneID, &e.RecordName,
			&e.RecordType, &e.Detail, &e.IPAddress, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}
