package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

const (
	GCMNonceSize = 12
	KeySizeAES256 = 32
)

// DecodeMFAKey dekodiert einen 32-Byte Base64-Schluessel.
func DecodeMFAKey(base64Key string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("ungueltiges base64-format fuer mfa_key: %w", err)
	}
	if len(key) != KeySizeAES256 {
		return nil, fmt.Errorf("mfa_key muss genau 32 bytes lang sein, erhalten %d", len(key))
	}
	return key, nil
}

// EncryptMFASecret verschluesselt das Klartext-Secret via AES-256-GCM.
// Rueckgabeformat: Nonce (12 Bytes) || Ciphertext mit Auth-Tag.
func EncryptMFASecret(key []byte, plaintext string) ([]byte, error) {
	if len(key) != KeySizeAES256 {
		return nil, errors.New("ungueltige schluessellaenge fuer aes-256")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("fehler beim initialisieren der aes-blockchiffre: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("fehler beim initialisieren von gcm: %w", err)
	}

	nonce := make([]byte, GCMNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("fehler beim generieren der nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return ciphertext, nil
}

// DecryptMFASecret entschluesselt ein via AES-256-GCM geschuetztes Secret.
func DecryptMFASecret(key []byte, encrypted []byte) (string, error) {
	if len(key) != KeySizeAES256 {
		return "", errors.New("ungueltige schluessellaenge fuer aes-256")
	}

	if len(encrypted) < GCMNonceSize {
		return "", errors.New("verschluesselte daten sind kuerzer als die nonce-laenge")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("fehler beim initialisieren der aes-blockchiffre: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("fehler beim initialisieren von gcm: %w", err)
	}

	nonce := encrypted[:GCMNonceSize]
	ciphertext := encrypted[GCMNonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("entschluesselung oder authentifizierung fehlgeschlagen: %w", err)
	}

	return string(plaintext), nil
}

// GenerateCSRFToken generiert ein kryptografisch sicheres 32-Byte CSRF-Token (Base64url).
func GenerateCSRFToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("fehler beim generieren des csrf-tokens: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
