package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// BlacklistUser traegt die User-ID nach einer Account-Loeschung in die Redis-Blacklist ein.
// Alle noch im Umlauf befindlichen JWTs dieses Users werden dadurch sofort ungueltig.
func BlacklistUser(ctx context.Context, rdb *redis.Client, userID uuid.UUID, ttl time.Duration) error {
	if rdb == nil {
		return nil
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	key := fmt.Sprintf("blacklist:user:%s", userID.String())
	return rdb.Set(ctx, key, "1", ttl).Err()
}

// IsUserBlacklisted prueft, ob die User-ID in Redis als gesperrt/geloescht markiert ist.
func IsUserBlacklisted(ctx context.Context, rdb *redis.Client, userID uuid.UUID) (bool, error) {
	if rdb == nil {
		return false, nil
	}
	key := fmt.Sprintf("blacklist:user:%s", userID.String())
	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, err
	}
	return val == "1", nil
}
