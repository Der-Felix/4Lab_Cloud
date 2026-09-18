package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter prueft Anfragen gegen konfigurierte Grenzwerte.
type RateLimiter interface {
	CheckLoginLimit(ctx context.Context, ip string, email string) (bool, error)
	CheckMFAVerifyLimit(ctx context.Context, tokenHash string, userID string) (bool, error)
}

// RedisRateLimiter implementiert das Rate-Limiting ueber Redis.
type RedisRateLimiter struct {
	client *redis.Client
}

// NewRedisRateLimiter erstellt einen neuen Redis-basierten Rate-Limiter.
func NewRedisRateLimiter(client *redis.Client) *RedisRateLimiter {
	return &RedisRateLimiter{client: client}
}

// CheckLoginLimit prueft und erhoeht die Zaehler fuer IP (5/Min) und E-Mail (10/Std).
func (r *RedisRateLimiter) CheckLoginLimit(ctx context.Context, ip string, email string) (bool, error) {
	if r.client == nil {
		return true, nil
	}

	// 1. IP-Limit: Max 5 Versuche pro Minute
	if ip != "" {
		ipKey := fmt.Sprintf("rl:login:ip:%s", ip)
		cnt, err := r.increment(ctx, ipKey, 1*time.Minute)
		if err != nil {
			return true, err
		}
		if cnt > 5 {
			return false, nil
		}
	}

	// 2. E-Mail-Limit: Max 10 Versuche pro Stunde (gehasht zwecks Datensparsamkeit)
	if email != "" {
		normEmail := strings.ToLower(strings.TrimSpace(email))
		hash := sha256.Sum256([]byte(normEmail))
		emailKey := fmt.Sprintf("rl:login:email:%s", hex.EncodeToString(hash[:]))
		cnt, err := r.increment(ctx, emailKey, 1*time.Hour)
		if err != nil {
			return true, err
		}
		if cnt > 10 {
			return false, nil
		}
	}

	return true, nil
}

// CheckMFAVerifyLimit prueft: max 5 Versuche pro mfa_token, max 10 Versuche pro User / Stunde.
func (r *RedisRateLimiter) CheckMFAVerifyLimit(ctx context.Context, tokenHash string, userID string) (bool, error) {
	if r.client == nil {
		return true, nil
	}

	// 1. Token-Limit: Max 5 Versuche pro mfa_token (TTL 10 Minuten)
	if tokenHash != "" {
		tokenKey := fmt.Sprintf("rl:mfa:token:%s", tokenHash)
		cnt, err := r.increment(ctx, tokenKey, 10*time.Minute)
		if err != nil {
			return true, err
		}
		if cnt > 5 {
			return false, nil
		}
	}

	// 2. User-Limit: Max 10 Versuche pro Stunde
	if userID != "" {
		userKey := fmt.Sprintf("rl:mfa:user:%s", userID)
		cnt, err := r.increment(ctx, userKey, 1*time.Hour)
		if err != nil {
			return true, err
		}
		if cnt > 10 {
			return false, nil
		}
	}

	return true, nil
}

func (r *RedisRateLimiter) increment(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	pipe := r.client.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
	return incrCmd.Val(), nil
}

// InMemoryRateLimiter dient als Fallback und fuer isolierte Unit-Tests ohne Redis.
type InMemoryRateLimiter struct {
	mu     sync.Mutex
	counts map[string][]time.Time
}

// NewInMemoryRateLimiter instanziiert einen speicherbasierten Rate-Limiter.
func NewInMemoryRateLimiter() *InMemoryRateLimiter {
	return &InMemoryRateLimiter{
		counts: make(map[string][]time.Time),
	}
}

// CheckLoginLimit prueft die Limits im lokalen Speicher.
func (m *InMemoryRateLimiter) CheckLoginLimit(_ context.Context, ip string, email string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// IP-Pruefung (max 5 / 1 Min)
	if ip != "" {
		ipKey := "ip:" + ip
		validTimes := make([]time.Time, 0)
		for _, t := range m.counts[ipKey] {
			if now.Sub(t) < 1*time.Minute {
				validTimes = append(validTimes, t)
			}
		}
		if len(validTimes) >= 5 {
			m.counts[ipKey] = validTimes
			return false, nil
		}
		m.counts[ipKey] = append(validTimes, now)
	}

	// E-Mail-Pruefung (max 10 / 1 Stunde)
	if email != "" {
		emailKey := "email:" + strings.ToLower(strings.TrimSpace(email))
		validTimes := make([]time.Time, 0)
		for _, t := range m.counts[emailKey] {
			if now.Sub(t) < 1*time.Hour {
				validTimes = append(validTimes, t)
			}
		}
		if len(validTimes) >= 10 {
			m.counts[emailKey] = validTimes
			return false, nil
		}
		m.counts[emailKey] = append(validTimes, now)
	}

	return true, nil
}

// CheckMFAVerifyLimit prueft im Speicher: max 5 Versuche pro Token, max 10 Versuche pro User / Stunde.
func (m *InMemoryRateLimiter) CheckMFAVerifyLimit(_ context.Context, tokenHash string, userID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// Token-Pruefung: Max 5 Versuche (Gueltigkeit 10 Minuten)
	if tokenHash != "" {
		tokenKey := "mfa:token:" + tokenHash
		validTimes := make([]time.Time, 0)
		for _, t := range m.counts[tokenKey] {
			if now.Sub(t) < 10*time.Minute {
				validTimes = append(validTimes, t)
			}
		}
		if len(validTimes) >= 5 {
			m.counts[tokenKey] = validTimes
			return false, nil
		}
		m.counts[tokenKey] = append(validTimes, now)
	}

	// User-Pruefung: Max 10 Versuche pro Stunde
	if userID != "" {
		userKey := "mfa:user:" + userID
		validTimes := make([]time.Time, 0)
		for _, t := range m.counts[userKey] {
			if now.Sub(t) < 1*time.Hour {
				validTimes = append(validTimes, t)
			}
		}
		if len(validTimes) >= 10 {
			m.counts[userKey] = validTimes
			return false, nil
		}
		m.counts[userKey] = append(validTimes, now)
	}

	return true, nil
}
