package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
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

type Race struct {
	ID            uuid.UUID
	Name          string
	Country       string
	RaceStartDate time.Time
	RaceEndDate   time.Time
	Status        *RaceStatus
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Season struct {
	ID         uuid.UUID
	SeasonName string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

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
	httpClient           *http.Client
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
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger: logger,
	}
}

func (s *ScrapedRaceService) ScrapeF1Race(ctx context.Context, year string) ([]*Race, error) {
	startTime := time.Now()
	var races []*Race
	s.logger.Info("Starting scrape of F1 races", slog.String("year", year))

	url := fmt.Sprintf("%s/en/racing/%s", s.baseUrl, year)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logger.Error("Error loading F1 schedule", slog.String("year", year), slog.String("error", err.Error()))
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	doc.Find(raceCardSelector).Each(func(i int, card *goquery.Selection) {
		description := s.safeSelectText(card, descriptionSelector)
		raceName := s.safeSelectText(card, nameSelector)
		dateText := s.safeSelectText(card, dateSelector)
		if dateText == "" {
			dateText = s.safeSelectText(card, dateFallbackSelector)
		}

		startDate, endDate, err := s.parseDateRange(dateText, year)
		if err != nil {
			s.logger.Warn("Could not parse race card", slog.String("year", year), slog.String("error", err.Error()))
			return
		}

		race := &Race{
			Name:          raceName,
			Country:       description,
			RaceStartDate: startDate,
			RaceEndDate:   endDate,
		}
		races = append(races, race)
	})

	s.logger.Info("Scraping for year completed.",
		slog.String("year", year),
		slog.Int("totalParsed", len(races)),
		slog.Duration("duration", time.Since(startTime)),
	)

	return races, nil
}

func (s *ScrapedRaceService) UpdateRaceFromScraperForSeason(ctx context.Context, year string) error {
	startTime := time.Now()
	s.logger.Info("Starting race update for season.", slog.String("year", year))

	scrapedData, err := s.ScrapeF1Race(ctx, year)
	if err != nil || len(scrapedData) == 0 {
		s.logger.Warn("No scraped data found for season. Skipping update.", slog.String("year", year))
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
		return fmt.Errorf("RaceStatus 'SCHEDULED' not found: %w", err)
	}

	s.logger.Info("Processing scraped races for season.", slog.Int("count", len(scrapedData)), slog.String("year", year))

	for _, scrapedRace := range scrapedData {
		err := s.txManager.WithTx(ctx, func(txCtx context.Context) error {
			return s.ProcessSingleRace(txCtx, scrapedRace, season, scheduledStatus)
		})
		if err != nil {
			s.logger.Error("Failed to process race", slog.String("name", scrapedRace.Name), slog.String("year", year), slog.String("error", err.Error()))
		}
	}

	s.logger.Info("Race update for season completed.", slog.String("year", year), slog.Duration("duration", time.Since(startTime)))
	return nil
}

func (s *ScrapedRaceService) GetOrCreateSeason(ctx context.Context, year string) (*Season, error) {
	season, err := s.seasonRepository.FindBySeasonName(ctx, year)
	if err == nil && season != nil {
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
	if err != nil || race == nil {
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

		var saveErr error
		race, saveErr = s.raceRepository.Save(ctx, newRace)
		if saveErr != nil {
			return saveErr
		}
	}

	exists, err := s.raceSeasonRepository.ExistsByRaceAndSeason(ctx, race, season)
	if err == nil && !exists {
		s.logger.Info("Linking race to season.", slog.String("race", race.Name), slog.String("season", season.SeasonName))
		now := time.Now()
		raceSeason := &RaceSeason{
			ID:        uuid.New(),
			Race:      race,
			Season:    season,
			CreatedAt: now,
			UpdatedAt: now,
		}
		return s.raceSeasonRepository.Save(ctx, raceSeason)
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
	crossMonth := regexp.MustCompile(`(?P<startDay>\d{2}).*?(?P<startMonth>[a-zA-Z]{3}).*?(?P<endDay>\d{2}).*?(?P<endMonth>[a-zA-Z]{3})`)
	sameMonth := regexp.MustCompile(`(?P<startDay>\d{2}).*?(?P<endDay>\d{2}).*?(?P<endMonth>[a-zA-Z]{3})`)

	yearInt, err := strconv.Atoi(year)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid year format: %s", year)
	}

	if matches := crossMonth.FindStringSubmatch(rangeStr); matches != nil {
		startDay, _ := strconv.Atoi(matches[crossMonth.SubexpIndex("startDay")])
		endDay, _ := strconv.Atoi(matches[crossMonth.SubexpIndex("endDay")])
		startMonth, err1 := s.parseMonth(matches[crossMonth.SubexpIndex("startMonth")])
		endMonth, err2 := s.parseMonth(matches[crossMonth.SubexpIndex("endMonth")])
		if err1 != nil || err2 != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("failed to parse month in range: %s", rangeStr)
		}

		startDate := time.Date(yearInt, startMonth, startDay, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(yearInt, endMonth, endDay, 0, 0, 0, 0, time.UTC)
		return startDate, endDate, nil
	}

	if matches := sameMonth.FindStringSubmatch(rangeStr); matches != nil {
		startDay, _ := strconv.Atoi(matches[sameMonth.SubexpIndex("startDay")])
		endDay, _ := strconv.Atoi(matches[sameMonth.SubexpIndex("endDay")])
		month, err := s.parseMonth(matches[sameMonth.SubexpIndex("endMonth")])
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("failed to parse month in range: %s", rangeStr)
		}

		startDate := time.Date(yearInt, month, startDay, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(yearInt, month, endDay, 0, 0, 0, 0, time.UTC)
		return startDate, endDate, nil
	}

	return time.Time{}, time.Time{}, fmt.Errorf("unknown date format: %s", rangeStr)
}