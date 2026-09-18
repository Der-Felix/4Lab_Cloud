---
name: dsgvo-bsi-checklist
description: DSGVO- und BSI-Compliance-Regeln fuer 4labscloud. Bei jedem neuen Endpoint, jeder neuen Tabelle und jedem neuen Feature pruefen.
---

# DSGVO + BSI fuer 4labscloud

## Pflicht bei jedem neuen Code

### Verschluesselung
- Transport: TLS 1.3 (Nginx, BSI TR-02102-2)
- At Rest: AES-256-GCM (Rust, Dateien)
- DB: PostgreSQL mit pgcrypto fuer sensible Felder

### Zugriffskontrolle
- JWT mit kurzer Lebensdauer (15 Min) + Refresh-Token
- MFA (TOTP) fuer Login verpflichtend
- PostgreSQL RLS: CREATE POLICY ... USING (user_id = current_setting('app.user_id')::uuid)
- Service-Token zwischen Go und Rust

### Audit-Logging
Jede Aktion schreibt in audit_log:
- user_id, action, target_id, timestamp, ip_address, result
- Aktionen: login, logout, upload, download, share_create, share_delete, file_delete, user_delete

### Loeschkonzept (DSGVO Art. 17)
- DELETE /api/v1/users/me -> loescht User + alle Dateien + alle Shares
- Dateien werden HART geloescht (nicht Soft-Delete)
- Backups nach 90 Tagen automatisch bereinigen
- Loeschvorgang wird geloggt (ohne Inhalt)

### Datensparsamkeit (DSGVO Art. 5)
- Keine IP-Adressen in Logs laenger als 7 Tage
- Keine Datei-Inhalte lesen oder indexieren
- Keine Analytics, kein Tracking, keine externen CDNs

## Checkliste vor jedem Commit
- [ ] Endpoint hat Auth-Pruefung
- [ ] Endpoint schreibt Audit-Log
- [ ] Bei neuen Tabellen: RLS-Policy vorhanden
- [ ] Bei neuen Feldern: personenbezogen? Wenn ja, pgcrypto?
- [ ] Keine externen HTTP-Aufrufe (kein Google Fonts, kein CDN)

## Verboten
- console.log mit User-Daten
- Fehlermeldungen mit Stacktraces an den Client
- Externe Dienste ohne AVV
- Cloud ausserhalb EU/EWR
