-- 4labscloud - Migration 003: upload_id zu files hinzufuegen
-- Eindeutige Zuordnung fuer Idempotenz und Vermeidung von Race Conditions

ALTER TABLE files ADD COLUMN IF NOT EXISTS upload_id UUID UNIQUE;
CREATE INDEX IF NOT EXISTS idx_files_upload_id ON files(upload_id);
