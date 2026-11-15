package services

import (
	"context"
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

func (s *DriverService) GetDriversByLastName(ctx context.Context, lastName string) (*response.DriverResponse, error) {
	driver, err := s.driverRepository.FindByLastNameIgnoreCase(ctx, lastName)
	if err != nil {
		return nil, err
	}
	if driver == nil {
		// Previously this returned (nil, nil), which handlers then dereferenced.
		return nil, fmt.Errorf("%w: no driver with last name %q", ErrNotFound, lastName)
	}

	res := mapDriverToResponse(driver)
	return &res, nil
}

func (s *DriverService) GetDriverCareerHistory(ctx context.Context, name string) ([]response.DriverWithSeasonsResponse, error) {
	drivers, err := s.driverRepository.FindCareerHistoryByLastNameIgnoreCase(ctx, name)
	if err != nil {
		return nil, err
	}

	res := make([]response.DriverWithSeasonsResponse, 0, len(drivers))
	for i := range drivers {
		res = append(res, mapDriverToWithSeasonsResponse(&drivers[i]))
	}
	return res, nil
}

// mapDriverToResponse copies values before taking their address. Taking the
// address of a field of a loop variable (as the callers used to) makes every
// response share one pointer on Go < 1.22, so all drivers showed the same number.
func mapDriverToResponse(d *models.Driver) response.DriverResponse {
	var permNum *int
	if d.PermanentNumber != 0 {
		n := d.PermanentNumber
		permNum = &n
	}

	var updatedAt *time.Time
	if !d.UpdatedAt.IsZero() {
		u := d.UpdatedAt
		updatedAt = &u
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
		n := d.PermanentNumber
		permNum = &n
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

	var cached []response.DriverWithSeasonsResponse
	if cacheGetJSON(ctx, s.redisClient, s.cacheEnabled, cacheKey, &cached) {
		return cached, nil
	}

	drivers, err := s.driverRepository.FindAllBySeasonName(ctx, season)
	if err != nil {
		return nil, err
	}

	res := make([]response.DriverWithSeasonsResponse, 0, len(drivers))
	for i := range drivers {
		res = append(res, mapDriverToWithSeasonsResponse(&drivers[i]))
	}

	cacheSetJSON(ctx, s.redisClient, s.cacheEnabled, cacheKey, res)
	return res, nil
}

func (s *DriverService) GetDriversByPermanentNumber(ctx context.Context, permNum int) ([]response.DriverResponse, error) {
	drivers, err := s.driverRepository.FindByPermanentNumber(ctx, permNum)
	if err != nil {
		return nil, err
	}

	res := make([]response.DriverResponse, 0, len(drivers))
	for i := range drivers {
		res = append(res, mapDriverToResponse(&drivers[i]))
	}
	return res, nil
}

func (s *DriverService) GetAllDrivers(ctx context.Context) ([]response.DriverResponse, error) {
	cacheKey := "drivers::all"

	var cached []response.DriverResponse
	if cacheGetJSON(ctx, s.redisClient, s.cacheEnabled, cacheKey, &cached) {
		return cached, nil
	}

	drivers, err := s.driverRepository.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	res := make([]response.DriverResponse, 0, len(drivers))
	for i := range drivers {
		res = append(res, mapDriverToResponse(&drivers[i]))
	}

	cacheSetJSON(ctx, s.redisClient, s.cacheEnabled, cacheKey, res)
	return res, nil
}
