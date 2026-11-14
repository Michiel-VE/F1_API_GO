package services

import (
	"context"
	"fmt"

	converter "f1-api-go/internal/convertor"
	"f1-api-go/internal/response"
	"f1-api-go/repository"

	"github.com/redis/go-redis/v9"
)

type TeamService struct {
	teamRepository             *repository.TeamRepository
	driverTeamSeasonRepository *repository.DriverTeamSeasonRepository
	redisClient                *redis.Client
	cacheEnabled               bool
}

func NewTeamService(
	tRepo *repository.TeamRepository,
	dtsRepo *repository.DriverTeamSeasonRepository,
	redisClient *redis.Client,
	cacheEnabled bool,
) *TeamService {
	return &TeamService{
		teamRepository:             tRepo,
		driverTeamSeasonRepository: dtsRepo,
		redisClient:                redisClient,
		cacheEnabled:               cacheEnabled,
	}
}

func (s *TeamService) GetAllTeamsForSeason(ctx context.Context, season string) ([]response.TeamWithPointsResponse, error) {
	cacheKey := fmt.Sprintf("teams::season:%s", season)

	// cacheGetJSON/cacheSetJSON also check redisClient != nil (this method used
	// to dereference a nil client when caching was enabled without Redis).
	var cached []response.TeamWithPointsResponse
	if cacheGetJSON(ctx, s.redisClient, s.cacheEnabled, cacheKey, &cached) {
		return cached, nil
	}

	teams, err := s.driverTeamSeasonRepository.FindAllTeamsBySeasonName(ctx, season)
	if err != nil {
		return nil, err
	}

	result := make([]response.TeamWithPointsResponse, 0, len(teams))
	for _, team := range teams {
		result = append(result, converter.TeamWithPointsToResponse(team))
	}

	cacheSetJSON(ctx, s.redisClient, s.cacheEnabled, cacheKey, result)
	return result, nil
}

func (s *TeamService) SearchTeamByName(ctx context.Context, name string) (*response.TeamResponse, error) {
	team, err := s.teamRepository.FindByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, fmt.Errorf("%w: team not found", ErrNotFound)
	}

	res := converter.TeamToResponse(*team)
	return &res, nil
}
