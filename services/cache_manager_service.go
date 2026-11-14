package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// DefaultCacheTTL is a safety net: even if nothing evicts a cache entry (for
// example a scraper Lambda that can't reach Redis), stale data disappears
// within this time. Entries used to be stored with no expiry at all.
const DefaultCacheTTL = time.Hour

var AllCacheNames = []string{
	"drivers", "drivers_hist",
	"teams", "teams_hist",
	"races", "races_hist",
	"results", "results_hist",
}

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

type CacheManagerService struct {
	redisClient *redis.Client
}

func NewCacheManagerService(redisClient *redis.Client) *CacheManagerService {
	return &CacheManagerService{
		redisClient: redisClient,
	}
}

func (s *CacheManagerService) EvictCache(ctx context.Context, cacheName string) {
	pattern := fmt.Sprintf("%s:*", cacheName)
	removed := s.deleteKeysMatching(ctx, pattern)
	log.Printf("Cleared entire cache region '%s', removed %d keys", cacheName, removed)
}

func (s *CacheManagerService) EvictSeasonAcrossAllCaches(ctx context.Context, season string) {
	for _, cacheName := range AllCacheNames {
		pattern := fmt.Sprintf("%s::season:%s*", cacheName, season)
		removed := s.deleteKeysMatching(ctx, pattern)
		if removed > 0 {
			log.Printf("Evicted %d key(s) matching '%s'", removed, pattern)
		}
	}
}

func (s *CacheManagerService) deleteKeysMatching(ctx context.Context, pattern string) int64 {
	var keysToDelete []string
	iter := s.redisClient.Scan(ctx, 0, pattern, 200).Iterator()

	for iter.Next(ctx) {
		keysToDelete = append(keysToDelete, iter.Val())
	}
	if err := iter.Err(); err != nil {
		log.Printf("WARN: Error scanning pattern '%s': %v", pattern, err)
		return 0
	}

	if len(keysToDelete) == 0 {
		return 0
	}

	count, err := s.redisClient.Unlink(ctx, keysToDelete...).Result()
	if err != nil {
		log.Printf("WARN: Error unlinking keys matching '%s': %v", pattern, err)
		return 0
	}

	return count
}
