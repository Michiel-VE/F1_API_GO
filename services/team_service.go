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

type TeamService struct {
	teamRepository *repository.TeamRepository
	redisClient    *redis.Client
}

func NewTeamService(
	tRepo *repository.TeamRepository,
	redisClient *redis.Client,
) *TeamService {
	return &TeamService{
		teamRepository: tRepo,
		redisClient:    redisClient,
	}
}

func (s *TeamService) GetAllTeamsForSeason(ctx context.Context, season string) ([]response.TeamResponse, error) {
	cacheKey := fmt.Sprintf("teams::season:%s", season)

	if cachedData, err := s.redisClient.Get(ctx, cacheKey).Bytes(); err == nil {
		var cachedResp []response.TeamResponse
		if err := json.Unmarshal(cachedData, &cachedResp); err == nil {
			return cachedResp, nil
		}
	}

	teams, err := s.teamRepository.FindAllTeamsBySeasonName(season)
	if err != nil {
		return nil, err
	}

	result := s.convertList(teams)

	if bytes, err := json.Marshal(result); err == nil {
		s.redisClient.Set(ctx, cacheKey, bytes, 0)
	}

	return result, nil
}

func (s *TeamService) convertList(teams []models.Team) []response.TeamResponse {
	result := make([]response.TeamResponse, 0, len(teams))
	for _, team := range teams {
		result = append(result, converter.TeamToResponse(team))
	}
	return result
}

func (s *TeamService) SearchTeamByName(ctx context.Context, name string) (*response.TeamResponse, error) {
	team, err := s.teamRepository.FindByName(name)
	if err != nil || team == nil {
		return nil, fmt.Errorf("team not found")
	}

	res := converter.TeamToResponse(*team)
	return &res, nil
}