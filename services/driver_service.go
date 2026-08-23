package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"f1-api-go/internal/models"
	"f1-api-go/internal/response"
	"f1-api-go/repository"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type DriverService struct {
	driverRepository *repository.DriverRepository
	redisClient      *redis.Client
	cacheEnabled     bool
}

func NewDriverService(
	driverRepo *repository.DriverRepository,
	redisClient *redis.Client,
	cacheEnabled bool,
) *DriverService {
	return &DriverService{
		driverRepository: driverRepo,
		redisClient:      redisClient,
		cacheEnabled:     cacheEnabled,
	}
}

func (s *DriverService) GetDriverById(ctx context.Context, id uuid.UUID) (*response.DriverWithSeasonsResponse, error) {
	driver, err := s.driverRepository.FindByID(id)
	if err != nil || driver == nil {
		return nil, fmt.Errorf("driver not found")
	}

	res := mapDriverToWithSeasonsResponse(driver)
	return &res, nil
}

func (s *DriverService) GetDriversByLastName(ctx context.Context, lastName string) (*response.DriverResponse, error) {
	driver, err := s.driverRepository.FindByLastNameIgnoreCase(lastName)
	if err != nil || driver == nil {
		return nil, err
	}

	res := mapDriverToResponse(driver)
	return &res, nil
}

func (s *DriverService) GetDriverCareerHistory(ctx context.Context, name string) ([]response.DriverWithSeasonsResponse, error) {
	drivers, err := s.driverRepository.FindCareerHistoryByLastNameIgnoreCase(name)
	if err != nil {
		return nil, err
	}

	var res []response.DriverWithSeasonsResponse
	for _, d := range drivers {
		res = append(res, mapDriverToWithSeasonsResponse(&d))
	}
	return res, nil
}

func mapDriverToResponse(d *models.Driver) response.DriverResponse {
	var permNum *int
	if d.PermanentNumber != 0 {
		permNum = &d.PermanentNumber
	}

	var updatedAt *time.Time
	if !d.UpdatedAt.IsZero() {
		updatedAt = &d.UpdatedAt
	}

	return response.DriverResponse{
		DriverID:        d.ID,
		Firstname:       d.Firstname,
		Lastname:        d.Lastname,
		PermanentNumber: permNum,
		Birthday:        d.Birthday.Format("2006-01-02"),
		Country:         d.Country,
		CountryCode:     d.CountryCode,
		CreatedAt:       d.CreatedAt,
		UpdatedAt:       updatedAt,
	}
}

func mapDriverToWithSeasonsResponse(d *models.Driver) response.DriverWithSeasonsResponse {
	var permNum *int
	if d.PermanentNumber != 0 {
		permNum = &d.PermanentNumber
	}

	var teamSeasons []response.TeamSeasonResponse
	for _, dts := range d.DriverTeamSeasons {
		seasonName := ""
		if dts.Season.ID != (uuid.UUID{}) {
			seasonName = dts.Season.SeasonName
		}
		teamName := ""
		if dts.Team.ID != (uuid.UUID{}) {
			teamName = dts.Team.Name
		}

		pointsVal := dts.Points

		teamSeasons = append(teamSeasons, response.TeamSeasonResponse{
			SeasonName: seasonName,
			TeamName:   teamName,
			Points:     &pointsVal,
		})
	}

	return response.DriverWithSeasonsResponse{
		DriverID:        d.ID,
		Firstname:       d.Firstname,
		Lastname:        d.Lastname,
		PermanentNumber: permNum,
		Birthday:        d.Birthday.Format("2006-01-02"),
		Country:         d.Country,
		CountryCode:     d.CountryCode,
		TeamSeasons:     teamSeasons,
	}
}

func (s *DriverService) GetDriversBySeason(ctx context.Context, season string) ([]response.DriverWithSeasonsResponse, error) {
	cacheKey := fmt.Sprintf("drivers::season:%s", season)

	if s.cacheEnabled && s.redisClient != nil {
		if cachedData, err := s.redisClient.Get(ctx, cacheKey).Bytes(); err == nil {
			var cachedResp []response.DriverWithSeasonsResponse
			if err := json.Unmarshal(cachedData, &cachedResp); err == nil {
				return cachedResp, nil
			}
		}
	}

	drivers, err := s.driverRepository.FindAllBySeasonName(season)
	if err != nil {
		return nil, err
	}

	var res []response.DriverWithSeasonsResponse
	for _, d := range drivers {
		res = append(res, mapDriverToWithSeasonsResponse(&d))
	}

	if s.cacheEnabled && s.redisClient != nil {
		if bytes, err := json.Marshal(res); err == nil {
			s.redisClient.Set(ctx, cacheKey, bytes, 0)
		}
	}

	return res, nil
}

func (s *DriverService) GetDriversByPermanentNumber(ctx context.Context, permNum int) ([]response.DriverResponse, error) {
	drivers, err := s.driverRepository.FindByPermanentNumber(permNum)
	if err != nil {
		return nil, err
	}

	var res []response.DriverResponse
	for _, d := range drivers {
		res = append(res, mapDriverToResponse(&d))
	}
	return res, nil
}

func (s *DriverService) GetAllDrivers(ctx context.Context) ([]response.DriverResponse, error) {
	cacheKey := "drivers::all"

	if s.cacheEnabled && s.redisClient != nil {
		if cachedData, err := s.redisClient.Get(ctx, cacheKey).Bytes(); err == nil {
			var cachedResp []response.DriverResponse
			if err := json.Unmarshal(cachedData, &cachedResp); err == nil {
				return cachedResp, nil
			}
		}
	}

	drivers, err := s.driverRepository.FindAll()
	if err != nil {
		return nil, err
	}

	var res []response.DriverResponse
	for _, d := range drivers {
		res = append(res, mapDriverToResponse(&d))
	}

	if s.cacheEnabled && s.redisClient != nil {
		if bytes, err := json.Marshal(res); err == nil {
			s.redisClient.Set(ctx, cacheKey, bytes, 0)
		}
	}

	return res, nil
}
