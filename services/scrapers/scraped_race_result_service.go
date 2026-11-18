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
	staleAfter = 72 * time.Hour
)

type ScrapedRaceResultService struct {
	baseUrl                    string
	driverRepository           RaceDriverRepository
	raceRepository             RaceRepository
	seasonRepository           SeasonRepository
	driverRaceResultRepository DriverRaceResultRepository
	txManager                  TransactionManager
	fetcher                    *pageFetcher
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
		fetcher:                    newPageFetcher(15*time.Second, logger),
		logger:                     logger,
	}
}

var driverCodePattern = regexp.MustCompile(`([A-Z]{3})$`)
var raceResultUrlPattern = regexp.MustCompile(`/races/\d+/([a-z0-9-]+)/race-result`)

func (s *ScrapedRaceResultService) UpdatePendingRaceResults(ctx context.Context, year string) error {
	s.logger.Info("Syncing completed race results up to current date.", slog.String("year", year))

	now := time.Now()

	if err := s.raceRepository.UpdatePastScheduledRacesToCompleted(ctx, now, year); err != nil {
		s.logger.Error("Failed to update past scheduled races to completed", slog.String("error", err.Error()))
	}

	season, err := s.seasonRepository.FindBySeasonName(ctx, year)
	if err != nil {
		return fmt.Errorf("season lookup failed for %s: %w", year, err)
	}
	if season == nil {
		return fmt.Errorf("%w: %s", ErrSeasonNotFound, year)
	}

	pendingModelRaces, err := s.raceRepository.FindAllFinishedRacesMissingResults(ctx, now, year)
	if err != nil {
		return fmt.Errorf("failed to look up races missing results: %w", err)
	}
	if len(pendingModelRaces) == 0 {
		s.logger.Info("All finished races already have results recorded.", slog.String("year", year))
		return nil
	}

	discovered, err := s.discoverRaceResults(ctx, year)
	if err != nil {
		return fmt.Errorf("failed to discover race result pages: %w", err)
	}

	pendingRaces := make([]Race, 0, len(pendingModelRaces))
	for _, m := range pendingModelRaces {
		pendingRaces = append(pendingRaces, Race{
			ID:            m.ID,
			Name:          m.Name,
			Country:       m.Country,
			RaceStartDate: m.RaceStartDate,
			RaceEndDate:   m.RaceEndDate,
		})
	}

	var (
		errs      []error
		unmatched []string
		stale     []string
		used      = make(map[string]struct{})
	)

	for i := range pendingRaces {
		race := &pendingRaces[i]

		match, method, ok := matchRace(discovered, race, used)
		if !ok {
			unmatched = append(unmatched, race.Name)
			attrs := []any{
				slog.String("race", race.Name),
				slog.String("country", race.Country),
				slog.Time("race_start_date", race.RaceStartDate),
				slog.Time("race_end_date", race.RaceEndDate),
				slog.Int("discovered_count", len(discovered)),
				slog.String("discovered", summarizeDiscovered(discovered)),
			}
			if isStaleRace(race, now) {
				stale = append(stale, race.Name)
				s.logger.Error("F1.com result page not found for race (overdue) - will retry next run.", attrs...)
			} else {
				s.logger.Warn("F1.com result page not found for race (may not be published yet) - will retry next run.", attrs...)
			}
			continue
		}

		used[match.url] = struct{}{}
		s.logger.Info("Matched race to F1.com result page",
			slog.String("race", race.Name),
			slog.String("slug", match.slug),
			slog.String("method", string(method)),
			slog.String("url", match.url))

		if err := s.ScrapeAndPersistRaceResult(ctx, match.url, season, race); err != nil {
			s.logger.Error("Failed to scrape race result", slog.String("race", race.Name), slog.String("error", err.Error()))
			errs = append(errs, fmt.Errorf("scrape %q: %w", race.Name, err))
		}
	}

	if len(unmatched) > 0 {
		s.logger.Warn("Some races could not be matched this run.",
			slog.Int("count", len(unmatched)),
			slog.String("races", strings.Join(unmatched, ", ")))
	}
	if len(stale) > 0 {
		errs = append(errs, fmt.Errorf("%d race(s) finished more than %s ago but have no matching F1.com result page: %s",
			len(stale), staleAfter, strings.Join(stale, ", ")))
	}

	return errors.Join(errs...)
}

func isStaleRace(race *Race, now time.Time) bool {
	if race.RaceStartDate.IsZero() && race.RaceEndDate.IsZero() {
		return false
	}
	ref := race.RaceEndDate
	if ref.IsZero() || ref.Before(race.RaceStartDate) {
		ref = race.RaceStartDate.AddDate(0, 0, defaultWeekendDays)
	}
	return now.Sub(ref) > staleAfter
}

func summarizeDiscovered(discovered []discoveredRace) string {
	parts := make([]string, 0, len(discovered))
	for _, d := range discovered {
		parts = append(parts, d.slug+"@"+d.date)
	}
	return strings.Join(parts, ",")
}

func (s *ScrapedRaceResultService) discoverRaceResults(ctx context.Context, year string) ([]discoveredRace, error) {
	yearInt, err := strconv.Atoi(strings.TrimSpace(year))
	if err != nil {
		return nil, fmt.Errorf("invalid season year %q: %w", year, err)
	}

	indexUrl := s.baseUrl + "/en/results/" + year + "/races"
	doc, err := s.fetcher.fetchDocument(ctx, indexUrl)
	if err != nil {
		return nil, err
	}

	var results []discoveredRace
	undated := 0
	doc.Find("table tbody tr").Each(func(i int, row *goquery.Selection) {
		cells := row.Find("td")
		if cells.Length() < 2 {
			return
		}
		link := cells.Eq(0).Find("a[href*='/race-result']").First()
		href, exists := link.Attr("href")
		if !exists {
			href, exists = cells.Eq(0).Find("a").First().Attr("href")
			if !exists {
				return
			}
		}
		if !strings.HasPrefix(href, "http") {
			href = s.baseUrl + href
		}

		slug := ""
		if m := raceResultUrlPattern.FindStringSubmatch(href); len(m) > 1 {
			slug = m[1]
		}

		dateText := strings.TrimSpace(cells.Eq(1).Text())
		raceDate, ok := parseIndexDate(dateText, yearInt)
		if !ok {
			undated++
		}

		results = append(results, discoveredRace{
			name:     strings.TrimSpace(cells.Eq(0).Text()),
			slug:     slug,
			date:     dateText,
			raceDate: raceDate,
			url:      href,
		})
	})

	if len(results) == 0 {
		return nil, fmt.Errorf("no race rows found at %s - page layout may have changed", indexUrl)
	}
	s.logger.Info("Discovered F1.com race result pages",
		slog.Int("count", len(results)),
		slog.Int("undated", undated))
	return results, nil
}

func (s *ScrapedRaceResultService) ScrapeAndPersistRaceResult(ctx context.Context, resultUrl string, season *Season, race *Race) error {
	s.logger.Info("Scraping results", slog.String("url", resultUrl))

	doc, err := s.fetcher.fetchDocument(ctx, resultUrl)
	if err != nil {
		return err
	}

	rows := doc.Find("table tbody tr")
	if rows.Length() == 0 {
		return fmt.Errorf("no result rows found at %s - page layout may have changed", resultUrl)
	}

	sprintUrl := strings.Replace(resultUrl, "/race-result", "/sprint-results", 1)
	sprintPointsMap := s.scrapeSprintPoints(ctx, sprintUrl)

	return s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		var saved, existing, skipped int
		var rowErr error

		rows.EachWithBreak(func(i int, row *goquery.Selection) bool {
			outcome, reason, err := s.parseAndSaveRow(txCtx, row, race, season, sprintPointsMap)
			if err != nil {
				rowErr = fmt.Errorf("row %d: %w", i, err)
				return false
			}
			switch outcome {
			case rowSaved:
				saved++
			case rowExisting:
				existing++
			default:
				skipped++
				s.logger.Warn("Skipped result row",
					slog.String("race", race.Name),
					slog.Int("row", i),
					slog.String("reason", reason))
			}
			return true
		})
		if rowErr != nil {
			return rowErr
		}

		s.logger.Info("Finished scraping race",
			slog.String("race", race.Name),
			slog.Int("saved", saved),
			slog.Int("already_recorded", existing),
			slog.Int("skipped", skipped))

		if saved+existing == 0 {
			return fmt.Errorf("parsed 0 results for race %s at %s - page layout may have changed", race.Name, resultUrl)
		}
		return nil
	})
}

func (s *ScrapedRaceResultService) scrapeSprintPoints(ctx context.Context, sprintUrl string) map[string]float64 {
	pointsMap := make(map[string]float64)

	sprintDoc, err := s.fetcher.fetchDocument(ctx, sprintUrl)
	if err != nil {
		return pointsMap
	}

	sprintRows := sprintDoc.Find("table tbody tr")
	if sprintRows.Length() == 0 {
		return pointsMap
	}

	sprintRows.Each(func(i int, row *goquery.Selection) {
		cells := row.Find("td")
		if cells.Length() < 7 {
			return
		}
		driverCode := extractDriverCode(cells.Eq(2))
		if driverCode == "" {
			return
		}
		pts, err := parsePoints(cells.Eq(6).Text())
		if err == nil && pts > 0 {
			pointsMap[driverCode] = pts
			s.logger.Info("Parsed sprint points", slog.String("driver", driverCode), slog.Float64("points", pts))
		}
	})

	return pointsMap
}

type rowOutcome int

const (
	rowSaved rowOutcome = iota
	rowExisting
	rowSkipped
)

func (s *ScrapedRaceResultService) parseAndSaveRow(ctx context.Context, row *goquery.Selection, race *Race, season *Season, sprintPointsMap map[string]float64) (rowOutcome, string, error) {
	cells := row.Find("td")
	if cells.Length() < 7 {
		return rowSkipped, fmt.Sprintf("expected at least 7 cells, got %d", cells.Length()), nil
	}

	position := strings.TrimSpace(cells.Eq(0).Text())

	driverCode := extractDriverCode(cells.Eq(2))
	if driverCode == "" {
		return rowSkipped, fmt.Sprintf("could not extract driver code from %q", strings.TrimSpace(cells.Eq(2).Text())), nil
	}

	driver, err := s.driverRepository.FindByDriverCode(ctx, driverCode)
	if err != nil {
		return rowSkipped, "", fmt.Errorf("driver lookup failed for code %q: %w", driverCode, err)
	}
	if driver == nil {
		return rowSkipped, "", fmt.Errorf("no driver in the database with driver_code %q", driverCode)
	}

	exists, err := s.driverRaceResultRepository.ExistsByDriverAndRaceAndSeason(ctx, driver, race, season)
	if err != nil {
		return rowSkipped, "", fmt.Errorf("existence check failed for driver %q: %w", driverCode, err)
	}
	if exists {
		return rowExisting, fmt.Sprintf("result already recorded for driver %q", driverCode), nil
	}

	points, err := parsePoints(cells.Eq(6).Text())
	if err != nil {
		return rowSkipped, "", fmt.Errorf("unparseable points for driver %q: %w", driverCode, err)
	}

	sprintPoints := sprintPointsMap[driverCode]
	lapsCompleted, _ := strconv.Atoi(strings.TrimSpace(cells.Eq(4).Text()))

	timeOrRetired := strings.TrimSpace(cells.Eq(5).Text())
	status := deriveStatus(position, timeOrRetired)

	now := time.Now()
	res := &DriverRaceResult{
		ID:            uuid.New(),
		Driver:        driver,
		Race:          race,
		Season:        season,
		Points:        points,
		SprintPoints:  sprintPoints,
		LapsCompleted: lapsCompleted,
		Status:        status,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.driverRaceResultRepository.Save(ctx, res); err != nil {
		return rowSkipped, "", fmt.Errorf("save failed for driver %q: %w", driverCode, err)
	}

	return rowSaved, "", nil
}

func parsePoints(raw string) (float64, error) {
	text := strings.TrimSpace(raw)
	switch text {
	case "", "-", "–", "—":
		return 0, nil
	}
	return strconv.ParseFloat(text, 64)
}

func extractDriverCode(cell *goquery.Selection) string {
	if code := strings.TrimSpace(cell.Find("span.uppercase").Last().Text()); driverCodePattern.MatchString(code) {
		return driverCodePattern.FindString(code)
	}
	text := strings.TrimSpace(cell.Text())
	if match := driverCodePattern.FindStringSubmatch(text); len(match) > 1 {
		return match[1]
	}
	return ""
}

func deriveStatus(position, timeOrRetired string) string {
	switch strings.ToUpper(strings.TrimSpace(timeOrRetired)) {
	case "DSQ":
		return "DSQ"
	case "DNS":
		return "DNS"
	case "DNF":
		return "DNF"
	}
	if strings.EqualFold(strings.TrimSpace(position), "NC") {
		return "DNF"
	}
	return "Finished"
}
