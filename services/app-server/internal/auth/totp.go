package auth

import (
	"fmt"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	TOTPIssuer     = "4labscloud"
	TOTPPeriodSec  = 30
	TOTPSecretSize = 20 // 20 Bytes gemaess Vorgabe
	TOTPSkew       = 1  // Verifikationsfenster ±1
)

// GenerateTOTPKey generiert ein neues 20-Byte TOTP-Secret und die zugehoerige otpauth:// URI.
func GenerateTOTPKey(email string) (secret string, uri string, err error) {
	opts := totp.GenerateOpts{
		Issuer:      TOTPIssuer,
		AccountName: email,
		Period:      TOTPPeriodSec,
		SecretSize:  TOTPSecretSize,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	}

	key, err := totp.Generate(opts)
	if err != nil {
		return "", "", fmt.Errorf("fehler beim generieren des totp-schluessels: %w", err)
	}

	return key.Secret(), key.URL(), nil
}

// ValidateTOTPCode validiert einen 6-stelligen Code gegen das Klartext-Secret mit Skew ±1.
func ValidateTOTPCode(code, secret string) bool {
	valid, err := totp.ValidateCustom(code, secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    TOTPPeriodSec,
		Skew:      TOTPSkew,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		return false
	}
	return valid
}

// GenerateCode generiert einen aktuellen TOTP-Code fuer ein gegebenes Secret (fuer Tests und Verifikation).
func GenerateCode(secret string, t time.Time) (string, error) {
	return totp.GenerateCode(secret, t)
}
