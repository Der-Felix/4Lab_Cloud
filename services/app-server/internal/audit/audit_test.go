package audit_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/4labscloud/4labscloud/services/app-server/internal/audit"
	"github.com/google/uuid"
)

// TestPseudonymization_HMAC ueberprueft die kryptographische Pseudonymisierung gemaess DSGVO Art. 17.
func TestPseudonymization_HMAC(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}

	userID1 := uuid.New()
	userID2 := uuid.New()

	pseudo1 := audit.CalculatePseudonym(userID1, key)
	pseudo2 := audit.CalculatePseudonym(userID2, key)

	if pseudo1 == "" || pseudo2 == "" {
		t.Fatal("Pseudonym darf nicht leer sein")
	}
	if pseudo1 == pseudo2 {
		t.Fatal("Unterschiedliche User-IDs muessen unterschiedliche Pseudonyme erzeugen")
	}

	// Deterministische Pruefung
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(userID1.String()))
	expected := hex.EncodeToString(mac.Sum(nil))

	if pseudo1 != expected {
		t.Fatalf("Erwartetes Pseudonym %s, erhalten: %s", expected, pseudo1)
	}

	// Nochmalige Berechnung fuer dieselbe ID muss identisch sein
	pseudo1Repeat := audit.CalculatePseudonym(userID1, key)
	if pseudo1 != pseudo1Repeat {
		t.Fatalf("Pseudonymisierung muss deterministisch sein")
	}
}

// TestDecodeHMACKey ueberprueft das Dekodieren gueltiger und ungueltiger HMAC-Keys.
func TestDecodeHMACKey(t *testing.T) {
	// 32 Bytes Base64
	validBase64 := "kGMSPVyZKTduhuyLX8ctc650jFQ1tfc7vvVEUSMDAZc="
	key, err := audit.DecodeHMACKey(validBase64)
	if err != nil {
		t.Fatalf("Gueltiger Key schlug fehl: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("Key-Laenge muss 32 Bytes sein, ist %d", len(key))
	}

	// Ungueltige Laenge
	invalidBase64 := "c2hvcnQ=" // "short"
	_, err = audit.DecodeHMACKey(invalidBase64)
	if err == nil {
		t.Fatal("Zu kurzer Key haette fehlschlagen muessen")
	}
}
