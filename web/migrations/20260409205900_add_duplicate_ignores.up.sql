CREATE TABLE IF NOT EXISTS duplicate_ignores (
    id          SERIAL PRIMARY KEY,
    value_hash  TEXT NOT NULL,
    value_text  TEXT NOT NULL,
    ignored_by  TEXT NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_dup_ignore_hash
    ON duplicate_ignores(value_hash);
