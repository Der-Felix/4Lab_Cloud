-- Migration 007: Tabelle export_jobs fuer asynchronen DSGVO-Datenexport
-- Speichert Status, Metadaten und Ablaufdatum von Export-ZIPs.

CREATE TABLE IF NOT EXISTS export_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_salt BYTEA,
    status TEXT NOT NULL DEFAULT 'pending',
    storage_path TEXT,
    size_bytes BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_export_jobs_user ON export_jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_export_jobs_status ON export_jobs(status);
CREATE INDEX IF NOT EXISTS idx_export_jobs_expires ON export_jobs(expires_at);

-- Row Level Security
ALTER TABLE export_jobs ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS export_jobs_user_isolation ON export_jobs;
CREATE POLICY export_jobs_user_isolation ON export_jobs
    USING (user_id = current_setting('app.user_id', true)::uuid);
