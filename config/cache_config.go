package config

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type CacheConfig struct {
	Client *redis.Client
	TTLs   map[string]time.Duration
}

func NewCacheManager(redisURL string) (*CacheConfig, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis url: %w", err)
	}

	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	activeTTL := 5 * 24 * time.Hour
	historicalTTL := 365 * 24 * time.Hour

	ttls := map[string]time.Duration{
		"drivers":      activeTTL,
		"drivers_hist": historicalTTL,
		"teams":        activeTTL,
		"teams_hist":   historicalTTL,
		"races":        activeTTL,
		"races_hist":   historicalTTL,
		"results":      activeTTL,
		"results_hist": historicalTTL,
	}

	return &CacheConfig{
		Client: client,
		TTLs:   ttls,
	}, nil
}

func (c *CacheConfig) GetTTL(cacheName string) time.Duration {
	if ttl, exists := c.TTLs[cacheName]; exists {
		return ttl
	}
	return 5 * 24 * time.Hour
}