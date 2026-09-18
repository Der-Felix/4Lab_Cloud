#!/usr/bin/env bash
# 4labscloud - End-to-End Test (Etappe D3)
# Prueft den vollstaendigen Upload-, Verschluesselungs- und Complete-Flow ueber Nginx.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Umgebungsvariablen laden
if [ -f "$ROOT_DIR/.env" ]; then
    export $(grep -v '^#' "$ROOT_DIR/.env" | xargs)
fi

BASE_URL="https://localhost:8443"
JWT_SECRET="${JWT_SECRET:-1ac913fb473984fce095ad0d5506e7e95874e44522c48528368dc9bf87cf0c3c}"
USER_A="11111111-1111-1111-1111-111111111111"
USER_B="22222222-2222-2222-2222-222222222222"

echo "=== 4labscloud E2E Test (Etappe D3) ==="

# Test-User in PostgreSQL sicherstellen
echo "-> Stelle Test-Benutzer in Datenbank bereit..."
podman exec -i 4labs-postgres psql -U 4labs -d 4labscloud -c "
INSERT INTO users (id, email, password_hash) VALUES 
('$USER_A', 'user_a@4labs.internal', 'hash_test_a'),
('$USER_B', 'user_b@4labs.internal', 'hash_test_b')
ON CONFLICT (id) DO NOTHING;
" > /dev/null

# JWT Tokens generieren ueber cmd/jwtgen
echo "-> Generiere JWT Tokens via cmd/jwtgen..."
TOKEN_A=$(JWT_SECRET="$JWT_SECRET" go -C "$ROOT_DIR/services/app-server" run cmd/jwtgen/main.go -user "$USER_A")
TOKEN_B=$(JWT_SECRET="$JWT_SECRET" go -C "$ROOT_DIR/services/app-server" run cmd/jwtgen/main.go -user "$USER_B")

# TEST 1: Kompletter Flow (Init -> Tus PATCH -> Complete -> Checksum)
echo "--- TEST 1: Kompletter Upload Flow & SHA-256 Verifikation ---"
PAYLOAD="4labscloud secure encrypted payload test 2026 - verified end-to-end!"
PAYLOAD_LEN=$(printf "%s" "$PAYLOAD" | wc -c | tr -d ' ')
EXPECTED_SHA=$(printf "%s" "$PAYLOAD" | shasum -a 256 | awk '{print $1}')

# 1.1 Init
INIT_RES=$(curl -k -s -X POST "$BASE_URL/api/v1/uploads/init" \
    -H "Authorization: Bearer $TOKEN_A" \
    -H "Content-Type: application/json" \
    -d "{\"filename\": \"e2e_doc.txt\", \"size_bytes\": $PAYLOAD_LEN}")
UPLOAD_ID=$(echo "$INIT_RES" | grep -o '"upload_id":"[^"]*' | cut -d'"' -f4)
if [ -z "$UPLOAD_ID" ]; then
    echo "FEHLER: Init fehlgeschlagen: $INIT_RES"
    exit 1
fi
echo "✓ Upload init erfolgreich: $UPLOAD_ID"

# 1.2 Tus PATCH Chunk
HTTP_CODE=$(curl -k -s -o /dev/null -w "%{http_code}" -X PATCH "$BASE_URL/api/v1/uploads/tus/$UPLOAD_ID" \
    -H "Tus-Resumable: 1.0.0" \
    -H "Upload-Offset: 0" \
    -H "Content-Type: application/offset+octet-stream" \
    --data-binary "$PAYLOAD")
if [ "$HTTP_CODE" != "204" ]; then
    echo "FEHLER: Tus PATCH fehlgeschlagen (HTTP $HTTP_CODE)"
    exit 1
fi
echo "✓ Tus Chunk Upload erfolgreich (HTTP 204)"

# 1.3 Complete
COMPLETE_RES=$(curl -k -s -X POST "$BASE_URL/api/v1/uploads/complete" \
    -H "Authorization: Bearer $TOKEN_A" \
    -H "Content-Type: application/json" \
    -d "{\"upload_id\": \"$UPLOAD_ID\"}")
FILE_ID=$(echo "$COMPLETE_RES" | grep -o '"file_id":"[^"]*' | cut -d'"' -f4)
if [ -z "$FILE_ID" ]; then
    echo "FEHLER: Complete fehlgeschlagen: $COMPLETE_RES"
    exit 1
fi
echo "✓ Complete erfolgreich. File-ID: $FILE_ID"

# 1.4 Pruefe DB-Eintrag und SHA-256 Pruefsumme
DB_INFO=$(podman exec -i 4labs-postgres psql -U 4labs -d 4labscloud -t -A -c \
    "SELECT checksum_sha256, storage_path FROM files WHERE id = '$FILE_ID';")
DB_SHA=$(echo "$DB_INFO" | cut -d'|' -f1)
STORAGE_PATH=$(echo "$DB_INFO" | cut -d'|' -f2)

if [ "$DB_SHA" != "$EXPECTED_SHA" ]; then
    echo "FEHLER: SHA-256 Checksumme stimmt nicht ueberein!"
    echo "Erwartet: $EXPECTED_SHA"
    echo "DB:        $DB_SHA"
    exit 1
fi
echo "✓ Checksummen-Verifikation erfolgreich (Klartext SHA-256 == DB-Checksum)"

# 1.5 Pruefe Datei auf Disk (relativer Pfad)
podman exec 4labs-upload-service test -f "/data/storage/$STORAGE_PATH"
echo "✓ Verschluesselte Datei auf Disk vorhanden: /data/storage/$STORAGE_PATH"

# 1.6 Pruefe Session-Cleanup in upload_sessions
SESSIONS_COUNT=$(podman exec -i 4labs-postgres psql -U 4labs -d 4labscloud -t -A -c \
    "SELECT count(*) FROM upload_sessions WHERE id = '$UPLOAD_ID';")
if [ "$SESSIONS_COUNT" != "0" ]; then
    echo "FEHLER: Upload-Session wurde nach Complete nicht aus upload_sessions geloescht!"
    exit 1
fi
echo "✓ Session-Cleanup erfolgreich (upload_sessions geloescht)"


# TEST 2: Idempotenz (zweimal Complete aufrufen)
echo "--- TEST 2: Idempotenz-Pruefung ---"
RETRY_RES=$(curl -k -s -X POST "$BASE_URL/api/v1/uploads/complete" \
    -H "Authorization: Bearer $TOKEN_A" \
    -H "Content-Type: application/json" \
    -d "{\"upload_id\": \"$UPLOAD_ID\"}")
RETRY_FILE_ID=$(echo "$RETRY_RES" | grep -o '"file_id":"[^"]*' | cut -d'"' -f4)

if [ "$RETRY_FILE_ID" != "$FILE_ID" ]; then
    echo "FEHLER: Idempotenter Aufruf lieferte nicht dieselbe File-ID!"
    echo "Erwartet: $FILE_ID, Erhalten: $RETRY_FILE_ID"
    exit 1
fi

IDEM_AUDIT=$(podman exec -i 4labs-postgres psql -U 4labs -d 4labscloud -t -A -c \
    "SELECT count(*) FROM audit_log WHERE action = 'upload_complete_retry' AND target_id = '$FILE_ID' AND result = 'ok';")
if [ "$IDEM_AUDIT" -lt 1 ]; then
    echo "FEHLER: Audit-Log fuer upload_complete_retry fehlt!"
    exit 1
fi
echo "✓ Idempotenz bestaetigt: Zweiter Aufruf liefert HTTP 200 + gleiche file_id + upload_complete_retry Audit"


# TEST 3: Race Condition (Parallele Complete-Aufrufe)
echo "--- TEST 3: Race Condition (Parallele Complete-Aufrufe) ---"
RACE_PAYLOAD="race condition data test"
RACE_LEN=$(printf "%s" "$RACE_PAYLOAD" | wc -c | tr -d ' ')

RACE_INIT=$(curl -k -s -X POST "$BASE_URL/api/v1/uploads/init" \
    -H "Authorization: Bearer $TOKEN_A" \
    -H "Content-Type: application/json" \
    -d "{\"filename\": \"race.txt\", \"size_bytes\": $RACE_LEN}")
RACE_UPLOAD_ID=$(echo "$RACE_INIT" | grep -o '"upload_id":"[^"]*' | cut -d'"' -f4)

curl -k -s -X PATCH "$BASE_URL/api/v1/uploads/tus/$RACE_UPLOAD_ID" \
    -H "Tus-Resumable: 1.0.0" \
    -H "Upload-Offset: 0" \
    -H "Content-Type: application/offset+octet-stream" \
    --data-binary "$RACE_PAYLOAD" > /dev/null

TMP_DIR=$(mktemp -d)
curl -k -s -X POST "$BASE_URL/api/v1/uploads/complete" \
    -H "Authorization: Bearer $TOKEN_A" \
    -H "Content-Type: application/json" \
    -d "{\"upload_id\": \"$RACE_UPLOAD_ID\"}" > "$TMP_DIR/resp1.json" &
curl -k -s -X POST "$BASE_URL/api/v1/uploads/complete" \
    -H "Authorization: Bearer $TOKEN_A" \
    -H "Content-Type: application/json" \
    -d "{\"upload_id\": \"$RACE_UPLOAD_ID\"}" > "$TMP_DIR/resp2.json" &
wait

FILE_ID_1=$(grep -o '"file_id":"[^"]*' "$TMP_DIR/resp1.json" | cut -d'"' -f4)
FILE_ID_2=$(grep -o '"file_id":"[^"]*' "$TMP_DIR/resp2.json" | cut -d'"' -f4)
rm -rf "$TMP_DIR"

if [ "$FILE_ID_1" != "$FILE_ID_2" ]; then
    echo "FEHLER: Parallele Aufrufe lieferten unterschiedliche File-IDs: $FILE_ID_1 vs $FILE_ID_2"
    exit 1
fi

RACE_COUNT=$(podman exec -i 4labs-postgres psql -U 4labs -d 4labscloud -t -A -c \
    "SELECT count(*) FROM files WHERE upload_id = '$RACE_UPLOAD_ID';")
if [ "$RACE_COUNT" != "1" ]; then
    echo "FEHLER: files-Tabelle enthaelt nicht genau 1 Eintrag (Count: $RACE_COUNT)!"
    exit 1
fi
echo "✓ Race Condition abgefangen: Exakt 1 files-Eintrag erzeugt, beide Requests erhielten gleiche file_id"


# TEST 4: 403 Forbidden bei fremdem User & sofortiges Cleanup
echo "--- TEST 4: Sicherheitspruefung 403 (fremder User) & Session-Cleanup ---"
ATTACK_INIT=$(curl -k -s -X POST "$BASE_URL/api/v1/uploads/init" \
    -H "Authorization: Bearer $TOKEN_A" \
    -H "Content-Type: application/json" \
    -d '{"filename": "user_a_private.txt", "size_bytes": 10}')
ATTACK_UPLOAD_ID=$(echo "$ATTACK_INIT" | grep -o '"upload_id":"[^"]*' | cut -d'"' -f4)

# User B versucht Complete auf Upload von User A
ATTACK_CODE=$(curl -k -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/api/v1/uploads/complete" \
    -H "Authorization: Bearer $TOKEN_B" \
    -H "Content-Type: application/json" \
    -d "{\"upload_id\": \"$ATTACK_UPLOAD_ID\"}")

if [ "$ATTACK_CODE" != "403" ]; then
    echo "FEHLER: Erwartet HTTP 403 fuer fremden User, erhalten: $ATTACK_CODE"
    exit 1
fi
echo "✓ Zugriff verweigert: HTTP 403 erhalten"

# Session muss in Rust / DB sofort geloescht sein
CLEANUP_CHECK=$(podman exec -i 4labs-postgres psql -U 4labs -d 4labscloud -t -A -c \
    "SELECT count(*) FROM upload_sessions WHERE id = '$ATTACK_UPLOAD_ID';")
if [ "$CLEANUP_CHECK" != "0" ]; then
    echo "FEHLER: Session wurde nach 403-Zugriff nicht geloescht!"
    exit 1
fi

DENIED_AUDIT=$(podman exec -i 4labs-postgres psql -U 4labs -d 4labscloud -t -A -c \
    "SELECT count(*) FROM audit_log WHERE action = 'upload_complete_denied' AND result = 'forbidden_wrong_user';")
if [ "$DENIED_AUDIT" -lt 1 ]; then
    echo "FEHLER: Audit-Log fuer upload_complete_denied fehlt!"
    exit 1
fi
echo "✓ 403-Sicherheitstest bestanden: Session sofort geloescht & upload_complete_denied geloggt"

echo ""
echo "=== ALLE E2E-TESTS ERFOLGREICH ABGESCHLOSSEN ==="
