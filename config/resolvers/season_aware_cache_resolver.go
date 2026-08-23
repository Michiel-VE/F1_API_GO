package resolvers

import (
	"fmt"
	"time"
)

type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
}

type CacheManager interface {
	GetCache(name string) (Cache, error)
}

type SeasonAwareCacheResolver interface {
	ActiveCacheName() string
	HistoricalCacheName() string
	ResolveCache(manager CacheManager, args ...interface{}) (Cache, error)
}

type BaseSeasonAwareCacheResolver struct {
	ActiveName     string
	HistoricalName string
}

func (b *BaseSeasonAwareCacheResolver) ActiveCacheName() string {
	return b.ActiveName
}

func (b *BaseSeasonAwareCacheResolver) HistoricalCacheName() string {
	return b.HistoricalName
}

func (b *BaseSeasonAwareCacheResolver) ResolveCache(manager CacheManager, args ...interface{}) (Cache, error) {
	season := b.extractSeason(args...)
	currentYear := fmt.Sprintf("%d", time.Now().Year())

	isCurrentSeason := season != "" && season == currentYear

	cacheName := b.HistoricalName
	if isCurrentSeason {
		cacheName = b.ActiveName
	}

	cache, err := manager.GetCache(cacheName)
	if err != nil || cache == nil {
		return nil, fmt.Errorf("no cache registered for name: %s — check CacheConfig", cacheName)
	}

	return cache, nil
}

func (b *BaseSeasonAwareCacheResolver) extractSeason(args ...interface{}) string {
	for _, arg := range args {
		if s, ok := arg.(string); ok {
			return s;
		}
	}
	return ""
}