package services

import (
	"context"
	"fmt"
	"time"
)

type ScrapedStandingService interface {
	UpdateDriverAndPointsFromScraperForSeason(ctx context.Context, season string) error
}

type ScrapedRaceService interface {
	UpdateRaceFromScraperForSeason(ctx context.Context, season string) error
}

type ScrapedDriverService interface {
	UpdateDriversFromScraper(ctx context.Context) error
}

type ScrapedTeamService interface {
	UpdateTeamsFromScraper(ctx context.Context) error
}

type ScrapedRaceResultService interface {
	UpdatePendingRaceResults(ctx context.Context, season string) error
}

type CacheEvictionService interface {
	EvictAndStamp(ctx context.Context, keys ...string) error
}

type F1Scheduler struct {
	scrapedStandingService   ScrapedStandingService
	scrapedRaceService       ScrapedRaceService
	scrapedDriverService     ScrapedDriverService
	scrapedTeamService       ScrapedTeamService
	scrapedRaceResultService ScrapedRaceResultService
	cacheEvictionService     CacheEvictionService
}

func NewF1Scheduler(
	standingService ScrapedStandingService,
	raceService ScrapedRaceService,
	driverService ScrapedDriverService,
	teamService ScrapedTeamService,
	raceResultService ScrapedRaceResultService,
	cacheEvictionService CacheEvictionService,
) *F1Scheduler {
	return &F1Scheduler{
		scrapedStandingService:   standingService,
		scrapedRaceService:       raceService,
		scrapedDriverService:     driverService,
		scrapedTeamService:       teamService,
		scrapedRaceResultService: raceResultService,
		cacheEvictionService:     cacheEvictionService,
	}
}

func (s *F1Scheduler) getCurrentYear() string {
	return fmt.Sprintf("%d", time.Now().Year())
}

func (s *F1Scheduler) UpdateRacesSeason(ctx context.Context) error {
	year := s.getCurrentYear()
	if err := s.scrapedRaceService.UpdateRaceFromScraperForSeason(ctx, year); err != nil {
		return err
	}
	return s.cacheEvictionService.EvictAndStamp(ctx, "races")
}

func (s *F1Scheduler) UpdateCurrentDriversAndPoints(ctx context.Context) error {
	year := s.getCurrentYear()
	if err := s.scrapedStandingService.UpdateDriverAndPointsFromScraperForSeason(ctx, year); err != nil {
		return err
	}
	return s.cacheEvictionService.EvictAndStamp(ctx, "drivers", "results")
}

func (s *F1Scheduler) UpdateDriverAndTeam(ctx context.Context) error {
	if err := s.scrapedDriverService.UpdateDriversFromScraper(ctx); err != nil {
		return err
	}
	if err := s.scrapedTeamService.UpdateTeamsFromScraper(ctx); err != nil {
		return err
	}
	return s.cacheEvictionService.EvictAndStamp(ctx, "drivers", "teams")
}

func (s *F1Scheduler) UpdateRaceResults(ctx context.Context) error {
	year := s.getCurrentYear()
	if err := s.scrapedRaceResultService.UpdatePendingRaceResults(ctx, year); err != nil {
		return err
	}
	return s.cacheEvictionService.EvictAndStamp(ctx, "results")
}