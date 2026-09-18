package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	TokenIssuer       = "4labscloud"
	PurposeAccess     = "access"
	PurposeMFAPending = "mfa_pending"
)

// Claims definiert die JWT-Payload fuer 4labscloud inklusive Token-Verwendungszweck.
type Claims struct {
	jwt.RegisteredClaims
	Purpose string `json:"purpose,omitempty"`
}

// GenerateToken erstellt ein neues signiertes Access-JWT fuer eine gegebene User-ID.
func GenerateToken(userID uuid.UUID, secret string, ttl time.Duration) (string, error) {
	return generateTokenWithPurpose(userID, secret, ttl, PurposeAccess)
}

// GenerateMFAPendingToken erstellt ein kurzlebiges Token (5 Min) fuer den zweiten Faktor.
func GenerateMFAPendingToken(userID uuid.UUID, secret string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return generateTokenWithPurpose(userID, secret, ttl, PurposeMFAPending)
}

func generateTokenWithPurpose(userID uuid.UUID, secret string, ttl time.Duration, purpose string) (string, error) {
	if secret == "" {
		return "", errors.New("jwt secret darf nicht leer sein")
	}

	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    TokenIssuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		Purpose: purpose,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("fehler beim signieren des tokens: %w", err)
	}

	return signedToken, nil
}

// ParseToken prueft ein Access-JWT und verweigert Tokens mit Zweck mfa_pending strikt.
func ParseToken(tokenString string, secret string) (uuid.UUID, error) {
	claims, err := parseClaims(tokenString, secret)
	if err != nil {
		return uuid.Nil, err
	}

	// mfa_pending darf NIE als regulaeres Access-Token akzeptiert werden
	if claims.Purpose == PurposeMFAPending {
		return uuid.Nil, errors.New("mfa_pending token kann nicht als access-token verwendet werden")
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("ungueltige user-id im token-subject: %w", err)
	}

	return userID, nil
}

// ParseMFAPendingToken prueft ein Token und akzeptiert AUSSCHLIESSLICH purpose=mfa_pending.
func ParseMFAPendingToken(tokenString string, secret string) (uuid.UUID, error) {
	claims, err := parseClaims(tokenString, secret)
	if err != nil {
		return uuid.Nil, err
	}

	if claims.Purpose != PurposeMFAPending {
		return uuid.Nil, errors.New("ungueltiger token-zweck, mfa_pending erforderlich")
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("ungueltige user-id im token-subject: %w", err)
	}

	return userID, nil
}

func parseClaims(tokenString string, secret string) (*Claims, error) {
	if secret == "" {
		return nil, errors.New("jwt secret darf nicht leer sein")
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unerwarteter signaturalgorithmus: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("ungueltiges token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("ungueltige token-claims")
	}

	return claims, nil
}
