## Beschreibung
Beschreibe kurz die Änderungen in diesem Pull Request und den Hintergrund.

## Zugehörige Issues
Schließt # (Issue-Nummer angeben)

## Art der Änderung
- [ ] Fehlerbehebung (Bugfix)
- [ ] Neue Funktion (Feature)
- [ ] Dokumentation / Tooling
- [ ] Refactoring / Performance

## Compliance- & Sicherheits-Checkliste
- [ ] **DSGVO**: Datenminimierung gewahrt? Keine IP-Adressen in Logs?
- [ ] **BSI**: TLS 1.3 bzw. AES-256-GCM unverändert sicher?
- [ ] **MFA**: Bleibt rein optional (kein Zwang)?
- [ ] **Auth**: Jeder öffentliche Endpunkt hat eine Berechtigungsprüfung?
- [ ] **Audit**: Schreibende Aktionen werden im Audit-Log vermerkt?
- [ ] **Zero-SaaS**: Keine externen CDNs, Fonts oder Tracking eingebunden?

## Getestet mit:
- [ ] `go test ./...`
- [ ] `cargo test`
- [ ] `npm run check` & `npm run test:unit`
