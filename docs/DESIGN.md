# 4LabCloud Design-System & UI/UX-Regeln

> Alle Hex-Werte in diesem Dokument sind gegen `frontend/src/routes/layout.css` geprueft.
> Aendert sich ein Token dort, wird es hier nachgezogen. Eine Richtlinie, die von der
> Implementierung abweicht, ist schlimmer als keine: sie erzeugt falsches Vertrauen.

## 1. Grundsaetze & Design-Richtung
- **Stil**: Seafile-Sachlichkeit (kompakte, funktionale Tabellen und Werkzeuge) kombiniert mit Nextcloud-Struktur (klare linke Navigation, Kopfzeile mit Kontext).
- **Dark Mode First**: Standardmaessig dunkles Farbschema, umschaltbar auf Light Mode.
  Konsequenz: Der Light Mode wird leicht uebersehen. Jede Farbaenderung ist in **beiden**
  Modi gegen Abschnitt 3 zu pruefen.
- **Keine externen CDNs**: Alle Fonts (`@fontsource/inter`, `@fontsource/great-vibes`) und Icons (`@lucide/svelte`) ausschliesslich lokal gebundelt.
- **Kein Linear-Minimalismus**, keine 3D-Effekte, keine verspielten Farbverlaeufe.
- **Zu Glasmorphismus**: `backdrop-blur` ist fuer *temporaere und schwebende* Schichten
  erlaubt und erwuenscht - Modals, Toasts, Sticky-Header, Lightbox-Overlays, Bild-Overlays.
  Verboten ist es als Gestaltungsmittel fuer *dauerhafte Inhaltsflaechen*: Karten, Listen
  und Tabellen bleiben deckend. (Die frueher pauschale Formulierung "kein Glasmorphism"
  widersprach 10 Dateien im Code und war damit wirkungslos.)

## 2. Token-Architektur (KEINE Hex-Werte im Svelte-Code)
Hex-Werte sind ausschliesslich in `layout.css` (`@theme`) und `Logo.svelte` erlaubt.
Im Svelte-Code werden ausschliesslich Token-Klassen verwendet.

### Basis-Token (`@theme`, gelten im Dark Mode unveraendert)

| Token | Klasse | Wert | Verwendung |
|---|---|---|---|
| primary-dark | `text-primary-dark` | `#0F3A6E` | Logo-Wortmarke hell |
| primary | `bg-primary` `text-primary` | `#1E6FD9` | Primaeraktionen, Links |
| primary-hover | - | `#1560C4` | Hover primaerer Buttons |
| primary-light | `text-primary-light` | `#3BC1E8` | Icons, Akzente im Dark Mode |
| accent | `text-accent` | `#2DD4BF` | Erfolg, Bestaetigung |
| bg-dark / bg-light | `bg-bg-dark` | `#0A0A0B` / `#F4F4F5` | Seitenhintergrund |
| surface-dark / -light | `bg-surface-dark` | `#141416` / `#FFFFFF` | Karten, Panels |
| sidebar-light | `bg-sidebar-light` | `#EFEFF1` | Sidebar hell (dunkel: `surface-dark`) |
| border-dark / -light | `border-border-dark` | `#26262A` / `#E4E4E7` | Rahmen, Trennlinien |
| text-dark / -light | `text-text-dark` | `#F5F5F7` / `#18181B` | Fliesstext |
| muted-dark / -light | `text-muted-dark` | `#A1A1A6` / `#5B5B63` | Sekundaertext |
| danger | `text-danger` | `#DC2626` | Zerstoerende Aktionen |
| amber | `text-amber` | `#F59E0B` | Warnung, Freigaben |
| rose / violet / slate | - | `#F43F5E` / `#8B5CF6` / `#64748B` | Kategorien im Audit-Log |

### Modus-Ueberschreibung im Light Mode
`accent`, `amber`, `primary` und `muted-light` sind fuer dunkle Flaechen entworfen und
verfehlen auf Weiss die Lesbarkeitsschwelle. Sie werden deshalb in `layout.css` unter
`html:not(.dark)` **umdefiniert** - an einer Stelle, nicht an jeder Fundstelle:

| Token | Dark | Light | Grund |
|---|---|---|---|
| accent | `#2DD4BF` | `#0F766E` | 1.86:1 -> 5.47:1 auf Weiss |
| amber | `#F59E0B` | `#B45309` | 2.15:1 -> 5.02:1 |
| primary | `#1E6FD9` | `#1A5FBF` | Links auf `bg-light` lagen bei 4.41:1 |
| muted-light | - | `#5B5B63` | 4.40:1 -> 6.12:1 auf `bg-light` |

**Regel**: Eine Farbe, die in beiden Modi mit derselben Klasse verwendet wird, muss in
beiden Modi bestehen. Ist das mit einem Wert nicht moeglich, wird das Token pro Modus
umdefiniert - niemals 70+ Fundstellen einzeln mit `dark:`-Varianten geflickt.

## 3. Kontrast (verbindlich, nachrechenbar)
- Fliesstext und Sekundaertext: **mindestens 4.5:1** (WCAG 2.1 AA).
- Text ab 18px bzw. 14px fett: mindestens 3:1.
- Nicht-textliche Bedienelemente (Rahmen aktiver Felder, Icons mit Bedeutung): mindestens 3:1.
- Gegen **beide** moeglichen Hintergruende pruefen: `surface` (Karten) **und** `bg` (Seite).
  `muted-light` bestand frueher auf Weiss (4.83:1) und fiel auf `bg-light` durch (4.40:1) -
  genau dieser Fall wird sonst uebersehen.
- Vor dem Aendern einer Farbe den Kontrast ausrechnen, nicht schaetzen.

## 4. Typografie
- **Schriftart**: Inter lokal. Kalligrafisch (`font-calligraphic`, Great Vibes) ausschliesslich fuer den Marken-Slogan.
- **FOUT-Schutz**: WOFF2-Kernschnitte in `app.html` per `rel="preload"`.
- **Skala** - nur diese Stufen, **keine handgesetzten px-Werte**:

| Klasse | Groesse | Verwendung |
|---|---|---|
| `text-3xs` | 10px | Badges, Zaehler. Absolute Untergrenze. |
| `text-2xs` | 11px | Metadaten, Zeitstempel, Hilfstexte |
| `text-xs` | 12px | Standard fuer dichte UI (Listen, Tabellen, Buttons) |
| `text-sm` | 14px | Fliesstext, Formularbeschriftungen |
| `text-base` | 16px | Hervorgehobener Fliesstext |
| `text-lg` / `text-xl` | 18/20px | Abschnittsueberschriften |
| `text-2xl` aufwaerts | 24px+ | Seitentitel, Hero |

  Alles unter 10px ist verboten. Frueher existierten 63 handgesetzte Groessen
  (`text-[11px]`, `text-[10px]`, `text-[9px]`) als zweite, undokumentierte Skala.
- **Zahlen & Daten**: Dateigroessen, Zeitstempel, IDs und Metriken immer mit `tabular-nums`.

## 5. Dichte & Layout
- **Keine festen Hoehen fuer bildschirmfuellende Elemente.** Der Hero stand auf `h-[280px]`
  und belegte damit auf einem 14-Zoll-Laptop 35% der sichtbaren Hoehe. Solche Elemente
  binden ihre Hoehe an den Viewport: `h-[clamp(150px,21vh,260px)]`.
- **Breakpoints loesen keine Hoehenprobleme.** `lg:`/`xl:` reagieren auf die Breite. Ist zu
  wenig vertikaler Platz das Problem, gehoeren `vh`/`dvh` oder `clamp()` her.
- **Leere Zustaende bekommen keine eigene grosse Karte.** "Sie haben nichts" in 400px Hoehe
  mit Illustration ist der schlechteste Tausch von Platz gegen Information. Der leere Fall
  gehoert in eine Kennzahl; die Liste rendert erst, wenn es Daten gibt.
- **Keine starren Spaltenpaare.** Eine kurze Inhaltsspalte neben einer langen Kartenspalte
  erzeugt zwangslaeufig tote Flaeche. Karten sind Kinder eines gemeinsamen Rasters.
- **Jedes Flex-Kind, das schrumpfen koennen muss, braucht `min-w-0`.** Ohne das schrumpft es
  nicht unter seine Inhaltsbreite und sprengt die Zeile.

## 6. Sidebar & Navigation
- **Breite**: Ausgeklappt 240px (`w-60`), eingeklappt 64px (`w-16`).
- **Responsiv**: Unterhalb `md` (768px) wird **immer** eingeklappt dargestellt
  (`matchMedia('(max-width: 767px)')`), unabhaengig von der Nutzerpraeferenz. Bei 240px auf
  einem 390px-Bildschirm bleiben sonst 150px fuer den gesamten Inhalt.
- **Collapse-Persistenz**: Cookie `sidebar_collapsed=true|false; path=/; max-age=31536000; SameSite=Strict; Secure`, kein `httpOnly` (SSR-Sync ohne Flash).
- **Aktiv-Zustand**: `bg-primary/15 text-primary dark:text-primary-light border-l-[3px] border-primary`.
- **Icons**: einheitlich 16px (`w-4 h-4`).
  **Offene Schuld:** Es sind zwei Icon-Bibliotheken im Einsatz - `@lucide/svelte` (18 Dateien)
  und `@tabler/icons-svelte` (13 Dateien). Das ist unbeabsichtigt gewachsen, vergroessert das
  Bundle und fuehrt zu leicht unterschiedlichen Strichstaerken. Neuer Code verwendet Lucide;
  Tabler wird schrittweise abgeloest.

## 7. Zustandsspeicherung
- **Sessions und Auth**: ausschliesslich sichere Cookies.
- **Reine Ansichtseinstellungen** (Theme, Sidebar, sichtbare Dashboard-Bereiche):
  `localStorage` bzw. Cookie. Bewusster Nachteil: geraetegebunden. Soll eine Einstellung
  geraeteuebergreifend gelten, gehoert sie in die Nutzereinstellungen samt Migration.

## 8. Verbotene Anti-Patterns
- Keine zentrierten Hero-Abschnitte ohne Nutzwert.
- Keine violetten/blauen Gradienten (nur 4labs-Logofarben).
- Keine Kartenschlachten: Datenlogik durch Linien (`border-t`, `divide-y`) strukturieren statt Karten in Karten zu verschachteln.
- Keine Pill-Buttons ohne klaren funktionalen Grund.
- Absolutes Emoji-Verbot: Symbole ausschliesslich als Lucide-SVG.
- Keine Spinner bei regulaeren Ladevorgaengen: massgeschneiderte Skeleton-Loader.
- Keine Marketing-Badges in der Anwendung. Compliance-Aussagen ("BSI konform",
  "AES-256-GCM at rest") gehoeren in die Dokumentation, nicht ins Dashboard: sie belegen
  dauerhaft Platz und sagen dem angemeldeten Nutzer nichts, was er beeinflussen kann.

## 9. Nielsen 10 UX-Heuristiken Checkliste
1. **Sichtbarkeit des Systemstatus**: Ladefortschritte, Upload-Status, Toast-Meldungen bei jeder Aktion.
2. **Uebereinstimmung mit der realen Welt**: Vertraute Dateisystem-Metaphern.
3. **Benutzerkontrolle & Freiheit**: Abbrechen in allen Dialogen, ESC schliesst Overlays.
4. **Konsistenz & Standards**: Einheitliche Button-Hierarchien (`primary`, `secondary`, `ghost`, `danger`).
5. **Fehlervermeidung**: Bestaetigungsdialoge bei destruktiven Aktionen.
6. **Wiedererkennen statt Erinnern**: Sichtbare Toolbars, Breadcrumbs, Kontextmenues.
7. **Flexibilitaet & Effizienz**: Tastaturbedienung, Listen- vs. Grid-Ansicht, ausblendbare Dashboard-Bereiche.
8. **Aesthetisches & minimalistisches Design**: Fokus auf Daten und Dateioperationen.
9. **Fehlerbehandlung**: Klare deutsche Fehlermeldungen mit Handlungsanweisung.
10. **Hilfe & Dokumentation**: Klare Inline-Hinweise.
