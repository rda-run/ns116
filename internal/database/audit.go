package database

import (
	"database/sql"
	"strings"

	"ns116/internal/model"
)

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
		`SELECT a.id, a.username, a.action, a.zone_id, zc.name, a.record_name, a.record_type, a.detail, a.ip_address, a.created_at
		 FROM audit_log a
		 LEFT JOIN zones_cache zc ON a.zone_id = zc.zone_id
		 ORDER BY a.created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []model.AuditEntry
	for rows.Next() {
		var e model.AuditEntry
		var zoneID, zoneName, recordName, recordType, detail sql.NullString
		if err := rows.Scan(&e.ID, &e.Username, &e.Action, &zoneID, &zoneName, &recordName,
			&recordType, &detail, &e.IPAddress, &e.CreatedAt); err != nil {
			return nil, 0, err
		}

		e.ZoneID = zoneID.String
		if zoneName.Valid {
			e.ZoneName = zoneName.String
		} else {
			e.ZoneName = e.ZoneID
		}
		e.RecordName = recordName.String
		e.RecordType = recordType.String
		e.Detail = detail.String

		if e.ZoneName != "" && e.ZoneName != e.ZoneID && e.RecordName != "" {
			zoneDomin := strings.TrimSuffix(e.ZoneName, ".")
			recNameClean := strings.TrimSuffix(e.RecordName, ".")

			if recNameClean == zoneDomin {
				e.RecordName = "@"
			} else {
				suffix := "." + zoneDomin
				if strings.HasSuffix(recNameClean, suffix) {
					e.RecordName = recNameClean[:len(recNameClean)-len(suffix)]
				}
			}
		}

		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}
