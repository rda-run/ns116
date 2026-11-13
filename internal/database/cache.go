package database

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"ns116/internal/model"
)

const cacheTTL = 5 * time.Minute

func (db *DB) CacheZones(zones []model.HostedZone) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // no-op after Commit
	if _, err := tx.Exec("DELETE FROM zones_cache"); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO zones_cache (zone_id, name, record_count, comment, label) VALUES ($1, $2, $3, $4, $5)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, z := range zones {
		if _, err := stmt.Exec(z.ID, z.Name, z.RecordCount, z.Comment, z.Label); err != nil {
			return fmt.Errorf("zone %s: %w", z.ID, err)
		}
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
	defer tx.Rollback() // no-op after Commit
	if _, err := tx.Exec("DELETE FROM dns_cache WHERE zone_id = $1", zoneID); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO dns_cache
		(zone_id, record_name, record_type, ttl, values_json, is_alias, alias_target, alias_zone_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range records {
		vJSON, err := json.Marshal(r.Values)
		if err != nil {
			return err
		}
		isAlias := 0
		if r.IsAlias {
			isAlias = 1
		}
		if _, err := stmt.Exec(zoneID, r.Name, r.Type, r.TTL, string(vJSON), isAlias, r.AliasTarget, r.AliasZoneID); err != nil {
			return fmt.Errorf("record %s %s: %w", r.Name, r.Type, err)
		}
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
	if _, err := db.conn.Exec("DELETE FROM dns_cache WHERE zone_id = $1", zoneID); err != nil {
		log.Printf("cache: failed to invalidate records of zone %s: %v", zoneID, err)
	}
}

func (db *DB) InvalidateAllCache() {
	for _, table := range []string{"dns_cache", "zones_cache"} {
		if _, err := db.conn.Exec("DELETE FROM " + table); err != nil {
			log.Printf("cache: failed to clear %s: %v", table, err)
		}
	}
}
