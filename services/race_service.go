package services

import (
	"context"
	"encoding/json"
	"fmt"

	converter "f1-api-go/internal/convertor"
	"f1-api-go/internal/models"
	"f1-api-go/internal/response"
	"f1-api-go/repository"

	"github.com/redis/go-redis/v9"
)

type RaceService struct {
	raceRepository *repository.RaceRepository
	redisClient    *redis.Client
	cacheEnabled   bool
}

func NewRaceService(
	rRepo *repository.RaceRepository,
	redisClient *redis.Client,
	cacheEnabled bool,
) *RaceService {
	return &RaceService{
		raceRepository: rRepo,
		redisClient:    redisClient,
		cacheEnabled:   cacheEnabled,
	}
}

func (s *RaceService) GetAllRacesForSeason(ctx context.Context, season string) ([]response.RaceResponse, error) {
	cacheKey := fmt.Sprintf("races::season:%s", season)

	if s.cacheEnabled && s.redisClient != nil {
		if cachedData, err := s.redisClient.Get(ctx, cacheKey).Bytes(); err == nil {
			var cachedResp []response.RaceResponse
			if err := json.Unmarshal(cachedData, &cachedResp); err == nil {
				return cachedResp, nil
			}
		}
	}

	races, err := s.raceRepository.FindAllRacesBySeasonName(season)
	if err != nil {
		return nil, err
	}

	result := s.convertList(races)

	if s.cacheEnabled && s.redisClient != nil {
		if bytes, err := json.Marshal(result); err == nil {
			s.redisClient.Set(ctx, cacheKey, bytes, 0)
		}
	}

	return result, nil
}

func (s *RaceService) convertList(races []models.Race) []response.RaceResponse {
	result := make([]response.RaceResponse, 0, len(races))
	for _, race := range races {
		result = append(result, converter.RaceToResponse(race))
	}
	return result
}
