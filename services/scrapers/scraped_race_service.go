package scrapers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
)

const (
	raceCardSelector     = "div.grid.justify-items-stretch a.group"
	descriptionSelector  = "p.typography-module_display-xl-bold__Gyl5W"
	nameSelector         = "span.typography-module_body-xs-semibold__Fyfwn"
	dateSelector         = "span.typography-module_technical-m-bold__JDsxP"
	dateFallbackSelector = "span.typography-module_technical-xs-regular__-W0Gs"
)

var (
	crossMonthDatePattern = regexp.MustCompile(`(?P<startDay>\d{2}).*?(?P<startMonth>[a-zA-Z]{3}).*?(?P<endDay>\d{2}).*?(?P<endMonth>[a-zA-Z]{3})`)
	sameMonthDatePattern  = regexp.MustCompile(`(?P<startDay>\d{2}).*?(?P<endDay>\d{2}).*?(?P<endMonth>[a-zA-Z]{3})`)
)

type RaceStatus struct {
	ID   uuid.UUID
	Name string
}

type RaceSeason struct {
	ID        uuid.UUID
	Race      *Race
	Season    *Season
	CreatedAt time.Time
	UpdatedAt time.Time
}

type RaceStatusRepository interface {
	FindByName(ctx context.Context, name string) (*RaceStatus, error)
}

type RaceSeasonRepository interface {
	ExistsByRaceAndSeason(ctx context.Context, race *Race, season *Season) (bool, error)
	Save(ctx context.Context, raceSeason *RaceSeason) error
}

type ScrapedRaceService struct {
	baseUrl              string
	seasonRepository     SeasonRepository
	raceRepository       RaceRepository
	raceStatusRepository RaceStatusRepository
	raceSeasonRepository RaceSeasonRepository
	txManager            TransactionManager
	fetcher              *pageFetcher
	logger               *slog.Logger
}

func NewScrapedRaceService(
	baseUrl string,
	seasonRepo SeasonRepository,
	raceRepo RaceRepository,
	statusRepo RaceStatusRepository,
	raceSeasonRepo RaceSeasonRepository,
	txManager TransactionManager,
	logger *slog.Logger,
) *ScrapedRaceService {
	if baseUrl == "" {
		baseUrl = "https://www.formula1.com"
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &ScrapedRaceService{
		baseUrl:              baseUrl,
		seasonRepository:     seasonRepo,
		raceRepository:       raceRepo,
		raceStatusRepository: statusRepo,
		raceSeasonRepository: raceSeasonRepo,
		txManager:            txManager,
		fetcher:              newPageFetcher(15*time.Second, logger),
		logger:               logger,
	}
}

func (s *ScrapedRaceService) ScrapeF1Race(ctx context.Context, year string) ([]*Race, error) {
	startTime := time.Now()
	var races []*Race
	s.logger.Info("Starting scrape of F1 races", slog.String("year", year))

	url := fmt.Sprintf("%s/en/racing/%s", s.baseUrl, year)
	doc, err := s.fetcher.fetchDocument(ctx, url)
	if err != nil {
		s.logger.Error("Error loading F1 schedule", slog.String("year", year), slog.String("error", err.Error()))
		return nil, err
	}

	cards := doc.Find(raceCardSelector)
	skipped := 0
	cards.Each(func(i int, card *goquery.Selection) {
		description := s.safeSelectText(card, descriptionSelector)
		raceName := s.safeSelectText(card, nameSelector)
		dateText := s.safeSelectText(card, dateSelector)
		if dateText == "" {
			dateText = s.safeSelectText(card, dateFallbackSelector)
		}

		if raceName == "" {
			skipped++
			s.logger.Warn("Skipping race card without a name", slog.String("year", year), slog.Int("card", i))
			return
		}

		startDate, endDate, err := s.parseDateRange(dateText, year)
		if err != nil {
			skipped++
			s.logger.Warn("Could not parse race card", slog.String("year", year), slog.String("race", raceName), slog.String("error", err.Error()))
			return
		}

		races = append(races, &Race{
			Name:          raceName,
			Country:       description,
			RaceStartDate: startDate,
			RaceEndDate:   endDate,
		})
	})

	if len(races) == 0 {
		return nil, fmt.Errorf("no races parsed from %s (%d cards found, %d skipped) - page layout may have changed", url, cards.Length(), skipped)
	}

	s.logger.Info("Scraping for year completed.",
		slog.String("year", year),
		slog.Int("totalParsed", len(races)),
		slog.Int("skipped", skipped),
		slog.Duration("duration", time.Since(startTime)),
	)

	return races, nil
}

func (s *ScrapedRaceService) UpdateRaceFromScraperForSeason(ctx context.Context, year string) error {
	startTime := time.Now()
	s.logger.Info("Starting race update for season.", slog.String("year", year))

	scrapedData, err := s.ScrapeF1Race(ctx, year)
	if err != nil {
		return err
	}

	var season *Season
	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		var txErr error
		season, txErr = s.GetOrCreateSeason(txCtx, year)
		return txErr
	})
	if err != nil {
		return err
	}

	scheduledStatus, err := s.raceStatusRepository.FindByName(ctx, "SCHEDULED")
	if err != nil {
		return fmt.Errorf("looking up RaceStatus 'SCHEDULED' failed: %w", err)
	}
	if scheduledStatus == nil {
		return errors.New("RaceStatus 'SCHEDULED' not found")
	}

	s.logger.Info("Processing scraped races for season.", slog.Int("count", len(scrapedData)), slog.String("year", year))

	var errs []error
	for _, scrapedRace := range scrapedData {
		err := s.txManager.WithTx(ctx, func(txCtx context.Context) error {
			return s.ProcessSingleRace(txCtx, scrapedRace, season, scheduledStatus)
		})
		if err != nil {
			s.logger.Error("Failed to process race", slog.String("name", scrapedRace.Name), slog.String("year", year), slog.String("error", err.Error()))
			errs = append(errs, fmt.Errorf("race %q: %w", scrapedRace.Name, err))
		}
	}

	s.logger.Info("Race update for season completed.", slog.String("year", year), slog.Duration("duration", time.Since(startTime)))
	return errors.Join(errs...)
}

func (s *ScrapedRaceService) GetOrCreateSeason(ctx context.Context, year string) (*Season, error) {
	season, err := s.seasonRepository.FindBySeasonName(ctx, year)
	if err != nil {
		return nil, fmt.Errorf("season lookup failed for %s: %w", year, err)
	}
	if season != nil {
		return season, nil
	}

	s.logger.Info("Season not found. Creating new season record.", slog.String("year", year))
	now := time.Now()
	newSeason := &Season{
		ID:         uuid.New(),
		SeasonName: year,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	return s.seasonRepository.Save(ctx, newSeason)
}

func (s *ScrapedRaceService) ProcessSingleRace(ctx context.Context, scrapedRace *Race, season *Season, defaultStatus *RaceStatus) error {
	race, err := s.raceRepository.FindByNameAndRaceStartDateAndRaceEndDate(ctx, scrapedRace.Name, scrapedRace.RaceStartDate, scrapedRace.RaceEndDate)
	if err != nil {
		return fmt.Errorf("race lookup failed: %w", err)
	}

	if race == nil {
		// Same race, new sponsor name? Match on start date within the season and
		// update it instead of inserting a duplicate that would never match an
		// F1.com result page.
		race, err = s.raceRepository.FindByRaceStartDateInSeason(ctx, season.SeasonName, scrapedRace.RaceStartDate)
		if err != nil {
			return fmt.Errorf("race lookup by start date failed: %w", err)
		}
		if race != nil {
			s.logger.Info("Existing race found by start date - updating its details.",
				slog.String("old_name", race.Name), slog.String("new_name", scrapedRace.Name))
			if err := s.raceRepository.UpdateDetails(ctx, race.ID, scrapedRace.Name, scrapedRace.Country, scrapedRace.RaceStartDate, scrapedRace.RaceEndDate); err != nil {
				return fmt.Errorf("updating race details failed: %w", err)
			}
			race.Name = scrapedRace.Name
			race.Country = scrapedRace.Country
			race.RaceStartDate = scrapedRace.RaceStartDate
			race.RaceEndDate = scrapedRace.RaceEndDate
		}
	}

	if race == nil {
		s.logger.Info("Race not found. Saving new race record.", slog.String("name", scrapedRace.Name))
		now := time.Now()
		newRace := &Race{
			ID:            uuid.New(),
			Name:          scrapedRace.Name,
			Country:       scrapedRace.Country,
			RaceStartDate: scrapedRace.RaceStartDate,
			RaceEndDate:   scrapedRace.RaceEndDate,
			Status:        defaultStatus,
			CreatedAt:     now,
			UpdatedAt:     now,
		}

		race, err = s.raceRepository.Save(ctx, newRace)
		if err != nil {
			return err
		}
	}

	exists, err := s.raceSeasonRepository.ExistsByRaceAndSeason(ctx, race, season)
	if err != nil {
		return fmt.Errorf("race/season link lookup failed: %w", err)
	}
	if !exists {
		s.logger.Info("Linking race to season.", slog.String("race", race.Name), slog.String("season", season.SeasonName))
		now := time.Now()
		return s.raceSeasonRepository.Save(ctx, &RaceSeason{
			ID:        uuid.New(),
			Race:      race,
			Season:    season,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	return nil
}

func (s *ScrapedRaceService) safeSelectText(parent *goquery.Selection, cssSelector string) string {
	return strings.TrimSpace(parent.Find(cssSelector).First().Text())
}

func (s *ScrapedRaceService) parseMonth(monthStr string) (time.Month, error) {
	parsedTime, err := time.Parse("Jan", monthStr)
	if err != nil {
		return 0, fmt.Errorf("invalid month: %s", monthStr)
	}
	return parsedTime.Month(), nil
}

func (s *ScrapedRaceService) parseDateRange(rangeStr, year string) (time.Time, time.Time, error) {
	yearInt, err := strconv.Atoi(year)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid year format: %s", year)
	}

	var startDate, endDate time.Time

	if m := crossMonthDatePattern.FindStringSubmatch(rangeStr); m != nil {
		startDay, _ := strconv.Atoi(m[crossMonthDatePattern.SubexpIndex("startDay")])
		endDay, _ := strconv.Atoi(m[crossMonthDatePattern.SubexpIndex("endDay")])
		startMonth, err1 := s.parseMonth(m[crossMonthDatePattern.SubexpIndex("startMonth")])
		endMonth, err2 := s.parseMonth(m[crossMonthDatePattern.SubexpIndex("endMonth")])
		if err1 != nil || err2 != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("failed to parse month in range: %s", rangeStr)
		}
		startDate = time.Date(yearInt, startMonth, startDay, 0, 0, 0, 0, time.UTC)
		endDate = time.Date(yearInt, endMonth, endDay, 0, 0, 0, 0, time.UTC)
	} else if m := sameMonthDatePattern.FindStringSubmatch(rangeStr); m != nil {
		startDay, _ := strconv.Atoi(m[sameMonthDatePattern.SubexpIndex("startDay")])
		endDay, _ := strconv.Atoi(m[sameMonthDatePattern.SubexpIndex("endDay")])
		month, err := s.parseMonth(m[sameMonthDatePattern.SubexpIndex("endMonth")])
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("failed to parse month in range: %s", rangeStr)
		}
		startDate = time.Date(yearInt, month, startDay, 0, 0, 0, 0, time.UTC)
		endDate = time.Date(yearInt, month, endDay, 0, 0, 0, 0, time.UTC)
	} else {
		return time.Time{}, time.Time{}, fmt.Errorf("unknown date format: %q", rangeStr)
	}

	if endDate.Before(startDate) {
		return time.Time{}, time.Time{}, fmt.Errorf("end date before start date in range: %q", rangeStr)
	}
	return startDate, endDate, nil
}
