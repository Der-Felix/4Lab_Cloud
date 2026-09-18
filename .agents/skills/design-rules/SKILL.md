---
name: design-rules
description: Verbindliche Offline-Design-Regeln fuer 4labscloud. Definiert Tokens, Typografie, Sidebar-Handling, Anti-Patterns und Nielsen-UX-Heuristiken.
---

# 4labscloud Design-System & UI/UX-Regeln

## 1. Grundsaetze & Design-Richtung
- **Stil**: Seafile-Sachlichkeit (kompakte, funktionale Tabellen und Werkzeuge) kombiniert mit Nextcloud-Struktur (klare linke Navigation, Kopfzeile mit Kontext).
- **Kein Linear-Minimalismus**, **kein Arc-Glasmorphism**, keine 3D-Effekte oder verspielte Farbverlaeufe.
- **Dark Mode First**: Standardmaessig dunkles Farbschema, umschaltbar auf Light Mode.
- **Keine externen CDNs**: Alle Fonts (`@fontsource/inter`) und Icons (`@lucide/svelte`) ausschliesslich lokal gebundelt.

## 2. Token-Architektur (KEINE Hex-Werte im Svelte-Code)
Hex-Werte sind ausschliesslich in `layout.css` (`@theme`) und `Logo.svelte` erlaubt.
Im gesamten Svelte-Code werden ausschliesslich die definierten Tailwind-Token-Klassen verwendet:

- **Primaerfarben**:
  - `bg-primary-dark`, `text-primary-dark`: `#0F3A6E` (Text, Logo)
  - `bg-primary`, `text-primary`, `border-primary`: `#1E6FD9` (Aktionen, primaere Buttons, Links)
  - `bg-primary-light`, `text-primary-light`: `#3BC1E8` (Icons, Akzente, Highlights)
- **Akzent & Erfolg**:
  - `bg-accent`, `text-accent`, `border-accent`: `#2DD4BF` (Erfolg, Sicherheits-Badges)
- **Hintergruende**:
  - `bg-bg-dark` (`#0B1220`) / `bg-bg-light` (`#F8FAFC`)
- **Flaechen & Karten**:
  - `bg-surface-dark` (`#111827`) / `bg-surface-light` (`#FFFFFF`)
- **Rahmen**:
  - `border-border-dark` (`#1F2937`) / `border-border-light` (`#E2E8F0`)
- **Text & Muted**:
  - `text-text-dark` (`#F1F5F9`) / `text-text-light` (`#0F172A`)
  - `text-muted-dark` (`#94A3B8`) / `text-muted-light` (`#64748B`)

## 3. Typografie & FOUT-Praevention
- **Schriftart**: Inter lokal (`@fontsource/inter`).
- **FOUT-Schutz**: WOFF2-Dateien der Kernschnitte werden in `app.html` mit `rel="preload"` referenziert, damit kein Text springt oder kurzzeitig unsichtbar ist.
- **Zahlen & Daten**: Dateigroessen, Zeitstempel, IDs und Metriken erhalten immer `tabular-nums` und Monospace/Inter-Tabellensatz.

## 4. Sidebar & Navigation
- **Breite**: Ausgeklappt 240px (`w-60`), eingeklappt 64px (`w-16`).
- **Sidebar-Collapse-Persistenz**:
  - Speicherung als Cookie: `sidebar_collapsed=true|false; path=/; max-age=31536000; SameSite=Strict; Secure`.
  - KEIN `httpOnly`, damit Client-Skripte und SSR ohne Flash synchron bleiben.
  - `localStorage` ist laut AGENTS.md nur fuer unkritische UI-Praeferenzen erlaubt; Sessions/Auth ausschliesslich ueber sichere Cookies.
- **Aktiv-Zustand**: `bg-primary/15 text-primary dark:text-primary-light border-l-[3px] border-primary`.
- **Icons**: Einheitlich Lucide-Icons mit 16px Groesse (`w-4 h-4`).

## 5. Verbotene Anti-Patterns
- Keine zentrierten Hero-Abschnitte ohne Nutzwert.
- Keine violetten/blauen Gradienten (nur 4labs-Logofarben).
- Keine Kartenschlachten: Datenlogik durch Linien (`border-t`, `divide-y`) strukturieren statt endlos Karten in Karten zu verschachteln.
- Keine Pill-Buttons ohne klaren funktionalen Grund.
- Absolutes Emoji-Verbot: Symbole ausschliesslich als Lucide-SVG rendern.
- Keine Spinner-Radierer bei regulaeren Ladevorgaengen: Immer massgeschneiderte Skeleton-Loader einsetzen.

## 6. Nielsen 10 UX-Heuristiken Checkliste
1. **Sichtbarkeit des Systemstatus**: Ladefortschritte, Upload-Status, Toast-Meldungen bei jeder Aktion.
2. **Uebereinstimmung mit der realen Welt**: Vertraute Dateisystem-Metaphern (Ordnerbaum, Upload, Download, Papierkorb).
3. **Benutzerkontrolle & Freiheit**: Abbrechen-Schaltflaechen in allen Dialogen, ESC-Taste schliesst Overlays/Kontextmenues.
4. **Konsistenz & Standards**: Gleiche Tastenkuerzel, einheitliche Button-Hierarchien (`primary`, `secondary`, `ghost`, `danger`).
5. **Fehlervermeidung**: Bestaetigungsdialoge bei destruktiven Aktionen (Loeschen).
6. **Wiedererkennen statt Erinnern**: Sichtbare Toolbars, Breadcrumbs und Kontextmenues.
7. **Flexibilitaet & Effizienz**: Tastaturbedienung (ESC, Enter), Listen- vs. Grid-Ansicht.
8. **Aesthetisches & minimalistisches Design**: Fokus auf Daten und Dateioperationen ohne visuelles Rauschen.
9. **Fehlerbehandlung**: Klare deutsche Fehlermeldungen mit Handlungsanweisung.
10. **Hilfe & Dokumentation**: Klare Inline-Hinweise (z.B. bei MFA-Recovery-Codes und Quotas).
