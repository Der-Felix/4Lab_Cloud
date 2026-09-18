package auth

import (
	"crypto/subtle"
	"fmt"

	"github.com/alexedwards/argon2id"
)

// DefaultArgon2Params definiert die gemaess BSI/Konvention verbindlichen Parameter.
// Memory: 64 MiB, Iterations: 3, Parallelism: 4, Salt: 16 Bytes, Key: 32 Bytes.
var DefaultArgon2Params = &argon2id.Params{
	Memory:      64 * 1024, // 64 MiB in KiB
	Iterations:  3,
	Parallelism: 4,
	SaltLength:  16,
	KeyLength:   32,
}

// DummyHash dient der Abwehr von Timing-Angriffen bei nicht existierenden Benutzern.
// Ein vorab mit den gleichen Argon2id-Parametern erzeugter Hash.
var DummyHash = "$argon2id$v=19$m=65536,t=3,p=4$dGVzdHNhbHQxMjM0NTY3OA$9Wv4K178yOhy1uXp9vVwS8M7x4Z3Q0kKqP6R9Y5tU2w"

// HashPassword erzeugt einen sicheren Argon2id-Hash des Klartext-Passworts.
func HashPassword(password string) (string, error) {
	hash, err := argon2id.CreateHash(password, DefaultArgon2Params)
	if err != nil {
		return "", fmt.Errorf("fehler beim erstellen des passwort-hashes: %w", err)
	}
	return hash, nil
}

// VerifyPassword vergleicht das Passwort mit dem Hash in konstanter Zeit.
// alexedwards/argon2id verwendet intern crypto/subtle.ConstantTimeCompare.
func VerifyPassword(password, hash string) (bool, error) {
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		return false, err
	}
	return match, nil
}

// VerifyDummyPassword fuehrt einen Hash-Vergleich gegen den Dummy-Hash aus,
// um Timing-Unterschiede bei nicht existierenden E-Mail-Adressen zu eliminieren.
func VerifyDummyPassword(password string) {
	_, _ = argon2id.ComparePasswordAndHash(password, DummyHash)
}

// ConstantTimeCompare verifiziert die Gleichheit zweier Byte-Slices in konstanter Zeit.
func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
