package database

import (
	"encoding/json"
	"time"

	"ns116/internal/model"
)

const cacheTTL = 5 * time.Minute

func (db *DB) CacheZones(zones []model.HostedZone) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	_, _ = tx.Exec("DELETE FROM zones_cache")
	stmt, err := tx.Prepare(`INSERT INTO zones_cache (zone_id, name, record_count, comment, label) VALUES ($1, $2, $3, $4, $5)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, z := range zones {
		_, _ = stmt.Exec(z.ID, z.Name, z.RecordCount, z.Comment, z.Label)
	}
	return tx.Commit()
}

func (db *DB) GetCachedZones() ([]model.HostedZone, bool) {
	var cachedAt time.Time
	err := db.conn.QueryRow("SELECT cached_at FROM zones_cache LIMIT 1").Scan(&cachedAt)
	if err != nil {
		return nil, false
	}

	if time.Since(cachedAt) > cacheTTL {
		return nil, false
	}

	rows, err := db.conn.Query("SELECT zone_id, name, record_count, comment, label FROM zones_cache")
	if err != nil {
		return nil, false
	}
	defer rows.Close()

	var zones []model.HostedZone
	for rows.Next() {
		var z model.HostedZone
		if err := rows.Scan(&z.ID, &z.Name, &z.RecordCount, &z.Comment, &z.Label); err != nil {
			return nil, false
		}
		zones = append(zones, z)
	}
	return zones, len(zones) > 0
}

func (db *DB) CacheRecords(zoneID string, records []model.DNSRecord) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	// Use $1 for parameter
	_, _ = tx.Exec("DELETE FROM dns_cache WHERE zone_id = $1", zoneID)

	stmt, err := tx.Prepare(`INSERT INTO dns_cache
		(zone_id, record_name, record_type, ttl, values_json, is_alias, alias_target, alias_zone_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, r := range records {
		vJSON, _ := json.Marshal(r.Values)
		isAlias := 0
		if r.IsAlias {
			isAlias = 1
		}
		_, _ = stmt.Exec(zoneID, r.Name, r.Type, r.TTL, string(vJSON), isAlias, r.AliasTarget, r.AliasZoneID)
	}
	return tx.Commit()
}

func (db *DB) GetCachedRecords(zoneID string) ([]model.DNSRecord, bool) {
	var cachedAt time.Time
	err := db.conn.QueryRow("SELECT cached_at FROM dns_cache WHERE zone_id = $1 LIMIT 1", zoneID).Scan(&cachedAt)
	if err != nil {
		return nil, false
	}

	if time.Since(cachedAt) > cacheTTL {
		return nil, false
	}

	rows, err := db.conn.Query(
		`SELECT record_name, record_type, ttl, values_json, is_alias, alias_target, alias_zone_id
		 FROM dns_cache WHERE zone_id = $1`, zoneID)
	if err != nil {
		return nil, false
	}
	defer rows.Close()

	var records []model.DNSRecord
	for rows.Next() {
		var r model.DNSRecord
		var vJSON string
		var isAlias int
		if err := rows.Scan(&r.Name, &r.Type, &r.TTL, &vJSON, &isAlias, &r.AliasTarget, &r.AliasZoneID); err != nil {
			return nil, false
		}
		_ = json.Unmarshal([]byte(vJSON), &r.Values)
		r.IsAlias = isAlias == 1
		records = append(records, r)
	}
	return records, len(records) > 0
}

func (db *DB) InvalidateRecordCache(zoneID string) {
	_, _ = db.conn.Exec("DELETE FROM dns_cache WHERE zone_id = $1", zoneID)
}

func (db *DB) InvalidateAllCache() {
	_, _ = db.conn.Exec("DELETE FROM dns_cache")
	_, _ = db.conn.Exec("DELETE FROM zones_cache")
}
