package services

import (
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
)

var AllCacheNames = []string{
	"drivers", "drivers_hist",
	"teams", "teams_hist",
	"races", "races_hist",
	"results", "results_hist",
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