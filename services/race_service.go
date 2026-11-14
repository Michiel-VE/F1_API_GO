package services

import (
	"context"
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

	var cached []response.RaceResponse
	if cacheGetJSON(ctx, s.redisClient, s.cacheEnabled, cacheKey, &cached) {
		return cached, nil
	}

	races, err := s.raceRepository.FindAllRacesBySeasonName(ctx, season)
	if err != nil {
		return nil, err
	}

	result := s.convertList(races)

	cacheSetJSON(ctx, s.redisClient, s.cacheEnabled, cacheKey, result)
	return result, nil
}

func (s *RaceService) convertList(races []models.Race) []response.RaceResponse {
	result := make([]response.RaceResponse, 0, len(races))
	for _, race := range races {
		result = append(result, converter.RaceToResponse(race))
	}
	return result
}
