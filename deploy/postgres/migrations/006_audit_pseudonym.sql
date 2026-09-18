-- Migration 006: Audit-Log Pseudonymisierung gemaess DSGVO Art. 17 & Session-Metadaten
-- Fuegt pseudonym_hash zu audit_log hinzu und user_agent/ip_address zu refresh_tokens.

-- 1. audit_log erweitern
ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS pseudonym_hash TEXT;
CREATE INDEX IF NOT EXISTS idx_audit_pseudonym ON audit_log(pseudonym_hash);

-- 2. refresh_tokens erweitern fuer Session-Management
ALTER TABLE refresh_tokens ADD COLUMN IF NOT EXISTS user_agent TEXT;
ALTER TABLE refresh_tokens ADD COLUMN IF NOT EXISTS ip_address TEXT;
