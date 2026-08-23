package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type CacheEvictionService struct {
	redisClient *redis.Client
}

func NewCacheEvictionService(redisClient *redis.Client) *CacheEvictionService {
	return &CacheEvictionService{
		redisClient: redisClient,
	}
}

func (s *CacheEvictionService) EvictAndStamp(ctx context.Context, cacheNames ...string) {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	for _, name := range cacheNames {
		pattern := fmt.Sprintf("%s:*", name)

		iter := s.redisClient.Scan(ctx, 0, pattern, 0).Iterator()
		var keys []string
		for iter.Next(ctx) {
			keys = append(keys, iter.Val())
		}
		if err := iter.Err(); err != nil {
			log.Printf("WARN: Error scanning cache '%s': %v", name, err)
			continue
		}

		if len(keys) > 0 {
			if err := s.redisClient.Unlink(ctx, keys...).Err(); err != nil {
				log.Printf("WARN: Failed to clear cache '%s': %v", name, err)
			}
		}

		stampKey := fmt.Sprintf("dataset:%s:lastUpdated", name)
		if err := s.redisClient.Set(ctx, stampKey, timestamp, 0).Err(); err != nil {
			log.Printf("WARN: Failed to set lastUpdated stamp for '%s': %v", name, err)
		} else {
			log.Printf("Evicted cache '%s', stamped lastUpdated=%s", name, timestamp)
		}
	}
}