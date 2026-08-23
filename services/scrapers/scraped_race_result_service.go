package scrapers

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

type ScrapedRaceResultService struct {
	baseUrl                    string
	driverRepository           RaceDriverRepository
	raceRepository             RaceRepository
	seasonRepository           SeasonRepository
	driverRaceResultRepository DriverRaceResultRepository
	txManager                  TransactionManager
	httpClient                 *http.Client
	logger                     *slog.Logger
}

func NewScrapedRaceResultService(
	baseUrl string,
	driverRepo RaceDriverRepository,
	raceRepo RaceRepository,
	seasonRepo SeasonRepository,
	resultRepo DriverRaceResultRepository,
	txManager TransactionManager,
	logger *slog.Logger,
) *ScrapedRaceResultService {
	if baseUrl == "" {
		baseUrl = "https://www.formula1.com"
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &ScrapedRaceResultService{
		baseUrl:                    baseUrl,
		driverRepository:           driverRepo,
		raceRepository:             raceRepo,
		seasonRepository:           seasonRepo,
		driverRaceResultRepository: resultRepo,
		txManager:                  txManager,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger: logger,
	}
}

func (s *ScrapedRaceResultService) UpdatePendingRaceResults(ctx context.Context, year string) error {
	s.logger.Info("Syncing completed race results up to current date.", slog.String("year", year))

	// Automatically update past-due scheduled races to COMPLETED before looking for missing results
	if err := s.raceRepository.UpdatePastScheduledRacesToCompleted(ctx, time.Now(), year); err != nil {
		s.logger.Error("Failed to update past scheduled races to completed", slog.String("error", err.Error()))
	}

	targetRace, err := s.raceRepository.FindFirstFinishedRaceMissingResults(ctx, time.Now(), year)
	if err != nil || targetRace == nil {
		s.logger.Info("All finished races already have results recorded.", slog.String("year", year))
		return nil
	}

	season, err := s.seasonRepository.FindBySeasonName(ctx, year)
	if err != nil || season == nil {
		return fmt.Errorf("season not found: %s", year)
	}

	s.logger.Info("Targeting race", slog.String("name", targetRace.Name))

	resultUrls := s.discoverRaceResultUrls(ctx, year)
	matchSlug := s.getF1WebsiteSlug(targetRace)

	var targetUrl string
	for _, url := range resultUrls {
		if strings.Contains(url, "/"+matchSlug+"/") {
			targetUrl = url
			break
		}
	}

	if targetUrl != "" {
		return s.ScrapeAndPersistRaceResult(ctx, targetUrl, season, targetRace)
	}

	s.logger.Warn("F1.com result page not found for slug. Skipping for now.", slog.String("slug", matchSlug))
	return nil
}

func (s *ScrapedRaceResultService) getF1WebsiteSlug(race *Race) string {
	country := strings.ToLower(race.Country)
	name := strings.ToLower(race.Name)

	if strings.Contains(name, "barcelona") {
		return "barcelona-catalunya"
	}
	if country == "saudi arabia" {
		return "saudi-arabia"
	}
	if country == "united states" || country == "usa" {
		return "usa"
	}
	if country == "great britain" {
		return "great-britain"
	}
	if country == "uae" || strings.Contains(name, "abu dhabi") {
		return "abu-dhabi"
	}

	return strings.ReplaceAll(country, " ", "-")
}

func (s *ScrapedRaceResultService) discoverRaceResultUrls(ctx context.Context, year string) []string {
	var urls []string
	indexUrl := s.baseUrl + "/en/results/" + year + "/races"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, indexUrl, nil)
	if err != nil {
		return urls
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logger.Error("Failed to fetch results index", slog.String("error", err.Error()))
		return urls
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return urls
	}

	doc.Find("a[href*=/race-result]").Each(func(i int, sel *goquery.Selection) {
		if href, exists := sel.Attr("href"); exists {
			if !strings.HasPrefix(href, "http") {
				href = s.baseUrl + href
			}
			urls = append(urls, href)
		}
	})

	return urls
}

func (s *ScrapedRaceResultService) ScrapeAndPersistRaceResult(ctx context.Context, resultUrl string, season *Season, race *Race) error {
	s.logger.Info("Scraping results", slog.String("url", resultUrl))

	return s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		req, err := http.NewRequestWithContext(txCtx, http.MethodGet, resultUrl, nil)
		if err != nil {
			return err
		}

		resp, err := s.httpClient.Do(req)
		if err != nil {
			s.logger.Error("Could not scrape results", slog.String("url", resultUrl), slog.String("error", err.Error()))
			return err
		}
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			return err
		}

		doc.Find("table tbody tr").Each(func(i int, row *goquery.Selection) {
			s.parseAndSaveRow(txCtx, row, race, season)
		})

		s.logger.Info("Saved results", slog.String("race", race.Name))
		return nil
	})
}

func (s *ScrapedRaceResultService) parseAndSaveRow(ctx context.Context, row *goquery.Selection, race *Race, season *Season) {
	cells := row.Find("td")
	if cells.Length() < 7 {
		return
	}

	driverCodeSpan := cells.Eq(2).Find("span.uppercase").Last()
	var driverCode string
	if driverCodeSpan.Length() > 0 {
		driverCode = strings.TrimSpace(driverCodeSpan.Text())
	} else {
		re := regexp.MustCompile(`.*?([A-Z]{3})$`)
		match := re.FindStringSubmatch(strings.TrimSpace(cells.Eq(2).Text()))
		if len(match) > 1 {
			driverCode = match[1]
		}
	}

	driver, err := s.driverRepository.FindByDriverCode(ctx, driverCode)
	if err != nil || driver == nil {
		return
	}

	exists, err := s.driverRaceResultRepository.ExistsByDriverAndRaceAndSeason(ctx, driver, race, season)
	if err != nil || exists {
		return
	}

	points, _ := strconv.ParseFloat(strings.TrimSpace(cells.Eq(6).Text()), 64)
	lapsCompleted, _ := strconv.Atoi(strings.TrimSpace(cells.Eq(4).Text()))
	status := s.deriveStatus(strings.TrimSpace(cells.Eq(0).Text()), strings.TrimSpace(cells.Eq(5).Text()))

	now := time.Now()
	res := &DriverRaceResult{
		ID:            uuid.New(),
		Driver:        driver,
		Race:          race,
		Season:        season,
		Points:        points,
		LapsCompleted: lapsCompleted,
		Status:        status,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	_ = s.driverRaceResultRepository.Save(ctx, res)
}

func (s *ScrapedRaceResultService) deriveStatus(pos, timeStr string) string {
	if !strings.EqualFold(pos, "NC") {
		return "Finished"
	}
	if strings.EqualFold(timeStr, "DSQ") {
		return "DSQ"
	}
	if strings.EqualFold(timeStr, "DNS") {
		return "DNS"
	}
	return "DNF"
}
