package services

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// DefaultCacheTTL is a safety net: even if nothing evicts a cache entry (for
// example a scraper Lambda that can't reach Redis), stale data disappears
// within this time. Entries used to be stored with no expiry at all.
const DefaultCacheTTL = time.Hour

// cacheGetJSON reports whether a valid cached value was found and decoded.
func cacheGetJSON(ctx context.Context, rc *redis.Client, enabled bool, key string, dest any) bool {
	if !enabled || rc == nil {
		return false
	}
	data, err := rc.Get(ctx, key).Bytes()
	if err != nil {
		return false
	}
	return json.Unmarshal(data, dest) == nil
}

// cacheSetJSON stores value with DefaultCacheTTL. Cache failures never fail a request.
func cacheSetJSON(ctx context.Context, rc *redis.Client, enabled bool, key string, value any) {
	if !enabled || rc == nil {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	if err := rc.Set(ctx, key, data, DefaultCacheTTL).Err(); err != nil {
		log.Printf("WARN: failed to cache key '%s': %v", key, err)
	}
}
