CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
    id         SERIAL PRIMARY KEY,
    username   TEXT    NOT NULL UNIQUE,
    pass_hash  TEXT    NOT NULL,
    role       TEXT    NOT NULL DEFAULT 'editor',
    active     INTEGER NOT NULL DEFAULT 1,
    auth_source TEXT   NOT NULL DEFAULT 'local',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
    token      TEXT PRIMARY KEY,
    username   TEXT    NOT NULL,
    csrf_token TEXT    NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP    NOT NULL,
    FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS dns_cache (
    id          SERIAL PRIMARY KEY,
    zone_id     TEXT NOT NULL,
    record_name TEXT NOT NULL,
    record_type TEXT NOT NULL,
    ttl         INTEGER,
    values_json TEXT,
    is_alias    INTEGER NOT NULL DEFAULT 0,
    alias_target  TEXT,
    alias_zone_id TEXT,
    cached_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_dns_cache_zone ON dns_cache(zone_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_dns_cache_record ON dns_cache(zone_id, record_name, record_type);

CREATE TABLE IF NOT EXISTS zones_cache (
    zone_id      TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    record_count INTEGER,
    comment      TEXT,
    label        TEXT,
    cached_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS audit_log (
    id          SERIAL PRIMARY KEY,
    username    TEXT    NOT NULL,
    action      TEXT    NOT NULL,
    zone_id     TEXT,
    record_name TEXT,
    record_type TEXT,
    detail      TEXT,
    ip_address  TEXT,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_log(username);
