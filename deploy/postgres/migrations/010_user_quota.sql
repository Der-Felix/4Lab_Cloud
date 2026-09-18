-- Migration 010: Individuelle Benutzer-Quota (quota_bytes)
-- NULL bedeutet Rueckfall auf DEFAULT_QUOTA_GB
ALTER TABLE users ADD COLUMN IF NOT EXISTS quota_bytes BIGINT;
