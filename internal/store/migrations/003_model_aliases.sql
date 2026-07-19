-- Migration 003: Model alias mapping
-- Allows mapping friendly model names (aliases) to real backend model names.

CREATE TABLE IF NOT EXISTS model_aliases (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_id TEXT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    alias       TEXT NOT NULL,
    real_name   TEXT NOT NULL,
    created_at  INTEGER NOT NULL,
    UNIQUE(provider_id, alias)
);

CREATE INDEX IF NOT EXISTS idx_model_aliases_provider ON model_aliases(provider_id);
