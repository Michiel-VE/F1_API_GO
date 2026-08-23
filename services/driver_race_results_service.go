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
	driver, err := s.driverRepo.FindByID(driverID)
	if err != nil || driver == nil {
		return nil, fmt.Errorf("driver not found: %w", err)
	}

	results, err := s.driverRaceResultsRepo.FindByDriverIDAndSeason(driverID, seasonName)
	if err != nil {
		return nil, fmt.Errorf("failed to get race results: %w", err)
	}

	_ = results

	return &response.DriverRaceResultsResponse{
		Firstname: driver.Firstname,
		Lastname:  driver.Lastname,
		Season:    seasonName,
		Results:   []response.RaceResultDTO{},
	}, nil
}