package services

import (
	"context"
	"encoding/json"
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

	if s.cacheEnabled {
		if cachedData, err := s.redisClient.Get(ctx, cacheKey).Bytes(); err == nil {
			var cachedResp []response.TeamWithPointsResponse
			if err := json.Unmarshal(cachedData, &cachedResp); err == nil {
				return cachedResp, nil
			}
		}
	}

	teams, err := s.driverTeamSeasonRepository.FindAllTeamsBySeasonName(season)
	if err != nil {
		return nil, err
	}

	result := make([]response.TeamWithPointsResponse, 0, len(teams))
	for _, team := range teams {
		result = append(result, converter.TeamWithPointsToResponse(team))
	}

	if s.cacheEnabled {
		if bytes, err := json.Marshal(result); err == nil {
			s.redisClient.Set(ctx, cacheKey, bytes, 0)
		}
	}

	return result, nil
}

func (s *TeamService) SearchTeamByName(ctx context.Context, name string) (*response.TeamResponse, error) {
	team, err := s.teamRepository.FindByName(name)
	if err != nil || team == nil {
		return nil, fmt.Errorf("team not found")
	}

	res := converter.TeamToResponse(*team)
	return &res, nil
}
