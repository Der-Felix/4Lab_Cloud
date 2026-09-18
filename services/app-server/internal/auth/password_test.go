package auth_test

import (
	"testing"

	"github.com/4labscloud/4labscloud/services/app-server/internal/auth"
)

func TestPassword_HashAndVerify(t *testing.T) {
	password := "StrengGeheim123!?"

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("fehler beim hashen des passworts: %v", err)
	}

	if hash == "" {
		t.Fatal("erzeugter hash ist leer")
	}

	// Gueltiges Passwort pruefen
	valid, err := auth.VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("unerwarteter fehler bei verify: %v", err)
	}
	if !valid {
		t.Fatal("verifikation mit korrektem passwort fehlgeschlagen")
	}

	// Falsches Passwort pruefen
	invalid, err := auth.VerifyPassword("FalschesPasswort", hash)
	if err != nil {
		t.Fatalf("unerwarteter fehler bei verify mit falschem passwort: %v", err)
	}
	if invalid {
		t.Fatal("verifikation mit falschem passwort ergab faelschlicherweise true")
	}
}

func TestPassword_DummyVerification(t *testing.T) {
	// Dummy-Verifikation darf nicht paniken und muss ausfuehrbar sein
	auth.VerifyDummyPassword("beliebigesPasswort")
}
