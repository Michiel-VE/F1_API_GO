package services

import (
	"context"
	"fmt"

	"f1-api-go/internal/response"
	"f1-api-go/repository"

	"github.com/google/uuid"
)

type DriverRaceResultsService struct {
	driverRepo            *repository.DriverRepository
	driverRaceResultsRepo *repository.DriverRaceResultRepository
}

func NewDriverRaceResultsService(
	driverRepo *repository.DriverRepository,
	driverRaceResultsRepo *repository.DriverRaceResultRepository,
) *DriverRaceResultsService {
	return &DriverRaceResultsService{
		driverRepo:            driverRepo,
		driverRaceResultsRepo: driverRaceResultsRepo,
	}
}

func (s *DriverRaceResultsService) GetDriverRaceResults(ctx context.Context, driverID uuid.UUID, seasonName string) (*response.DriverRaceResultsResponse, error) {
	driver, err := s.driverRepo.FindByID(ctx, driverID)
	if err != nil {
		return nil, fmt.Errorf("failed to load driver: %w", err)
	}
	if driver == nil {
		return nil, fmt.Errorf("%w: driver not found", ErrNotFound)
	}

	results, err := s.driverRaceResultsRepo.FindByDriverIDAndSeason(ctx, driverID, seasonName)
	if err != nil {
		return nil, fmt.Errorf("failed to get race results: %w", err)
	}

	raceResultDTOs := make([]response.RaceResultDTO, 0, len(results))
	for _, res := range results {
		raceName := ""
		if res.Race.Name != "" {
			raceName = res.Race.Name
		}

		status := ""
		if res.Status != nil {
			status = *res.Status
		}

		var lapsPtr *int
		if res.LapsCompleted != nil {
			lapsVal := *res.LapsCompleted
			lapsPtr = &lapsVal
		}

		raceResultDTOs = append(raceResultDTOs, response.RaceResultDTO{
			RaceName:      raceName,
			Points:        res.Points,
			SprintPoints:  res.SprintPoints,
			IsSprint:      res.Race.IsSprint,
			LapsCompleted: lapsPtr,
			Status:        status,
		})
	}

	return &response.DriverRaceResultsResponse{
		Firstname: driver.Firstname,
		Lastname:  driver.Lastname,
		Season:    seasonName,
		Results:   raceResultDTOs,
	}, nil
}
