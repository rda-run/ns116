package model

import "time"

type HostedZone struct {
	ID          string
	Name        string
	RecordCount int64
	Comment     string
	Label       string
}

type DNSRecord struct {
	Name        string
	Type        string
	TTL         int64
	Values      []string
	IsAlias     bool
	AliasTarget string
	AliasZoneID string
}

type RecordChangeRequest struct {
	Action string
	Name   string
	Type   string
	TTL    int64
	Values []string
}

type User struct {
	ID         int64
	Username   string
	PassHash   string
	Role       string
	Active     bool
	AuthSource string // "local" or "ldap"
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Session struct {
	Token     string
	Username  string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type AuditEntry struct {
	ID         int64
	Username   string
	Action     string
	ZoneID     string
	ZoneName   string
	RecordName string
	RecordType string
	Detail     string
	IPAddress  string
	CreatedAt  time.Time
}

type CachedRecord struct {
	ZoneID      string
	RecordName  string
	RecordType  string
	TTL         int64
	ValuesJSON  string
	IsAlias     bool
	AliasTarget string
	AliasZoneID string
	CachedAt    time.Time
}
