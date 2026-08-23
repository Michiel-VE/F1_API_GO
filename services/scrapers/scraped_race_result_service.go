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

// driverCodePattern matches a trailing 3-letter uppercase driver code glued
// directly onto a driver's name with no separator, which is what F1.com
// actually renders once markup is stripped, e.g. "George RussellRUS" -> "RUS".
var driverCodePattern = regexp.MustCompile(`([A-Z]{3})$`)

// discoveredRace is one row scraped from the season results index page
// (https://www.formula1.com/en/results/{year}/races).
type discoveredRace struct {
	name string // Grand Prix label as printed by F1, e.g. "Miami", "Barcelona-Catalunya"
	date string // as printed on the site, e.g. "08 Mar" - not currently parsed, kept for logging/debugging
	url  string // absolute URL to the /race-result page
}

// UpdatePendingRaceResults brings the DB up to date with any finished races
// that don't have driver results recorded yet. It processes every pending
// race in one call, not just the first, so a missed cron run doesn't leave a
// growing backlog.
func (s *ScrapedRaceResultService) UpdatePendingRaceResults(ctx context.Context, year string) error {
	s.logger.Info("Syncing completed race results up to current date.", slog.String("year", year))

	// Automatically flip past-due scheduled races to COMPLETED before looking
	// for missing results.
	if err := s.raceRepository.UpdatePastScheduledRacesToCompleted(ctx, time.Now(), year); err != nil {
		s.logger.Error("Failed to update past scheduled races to completed", slog.String("error", err.Error()))
	}

	season, err := s.seasonRepository.FindBySeasonName(ctx, year)
	if err != nil || season == nil {
		return fmt.Errorf("season not found: %s", year)
	}

	discovered, err := s.discoverRaceResults(ctx, year)
	if err != nil {
		return fmt.Errorf("failed to discover race result pages: %w", err)
	}
	if len(discovered) == 0 {
		s.logger.Warn("No race result rows found on the F1 results index page. Page layout may have changed.", slog.String("year", year))
		return nil
	}

	for {
		targetRace, err := s.raceRepository.FindFirstFinishedRaceMissingResults(ctx, time.Now(), year)
		if err != nil {
			return fmt.Errorf("failed to look up races missing results: %w", err)
		}
		if targetRace == nil {
			s.logger.Info("All finished races already have results recorded.", slog.String("year", year))
			return nil
		}

		match, ok := matchRace(discovered, targetRace)
		if !ok {
			// Don't loop forever retrying a race we can't match - log it
			// loudly and let the next scheduled run try again.
			s.logger.Warn("F1.com result page not found for race. Skipping for now.",
				slog.String("race", targetRace.Name))
			return nil
		}

		s.logger.Info("Matched race", slog.String("race", targetRace.Name), slog.String("f1_label", match.name), slog.String("url", match.url))

		if err := s.ScrapeAndPersistRaceResult(ctx, match.url, season, targetRace); err != nil {
			s.logger.Error("Failed to scrape race result", slog.String("race", targetRace.Name), slog.String("error", err.Error()))
			return err
		}
	}
}

// discoverRaceResults scrapes the season index page for every race's name,
// date, and race-result URL. Doing this instead of guessing a slug from
// race.Country avoids collisions where multiple races share a country
// (e.g. Miami / Las Vegas / United States all being "usa").
func (s *ScrapedRaceResultService) discoverRaceResults(ctx context.Context, year string) ([]discoveredRace, error) {
	indexUrl := s.baseUrl + "/en/results/" + year + "/races"
	doc, err := s.fetchDocument(ctx, indexUrl)
	if err != nil {
		return nil, err
	}

	var results []discoveredRace
	doc.Find("table tbody tr").Each(func(i int, row *goquery.Selection) {
		cells := row.Find("td")
		if cells.Length() < 2 {
			return
		}
		link := cells.Eq(0).Find("a[href*='/race-result']").First()
		href, exists := link.Attr("href")
		if !exists {
			// Fall back to any link in the row, in case markup changes.
			href, exists = cells.Eq(0).Find("a").First().Attr("href")
			if !exists {
				return
			}
		}
		if !strings.HasPrefix(href, "http") {
			href = s.baseUrl + href
		}
		results = append(results, discoveredRace{
			name: strings.TrimSpace(cells.Eq(0).Text()),
			date: strings.TrimSpace(cells.Eq(1).Text()),
			url:  href,
		})
	})
	return results, nil
}

// matchRace fuzzy-matches a DB race against the rows scraped from F1's index
// page by normalizing both names (lowercase, strip spaces/hyphens/"grand
// prix") and checking containment either direction. This tolerates things
// like the index cell text picking up flag alt-text (e.g.
// "Flag of AustraliaAustralia") and DB names like "Miami Grand Prix" vs F1's
// "Miami".
func matchRace(discovered []discoveredRace, race *Race) (discoveredRace, bool) {
	target := normalizeRaceName(race.Name)
	if target == "" {
		return discoveredRace{}, false
	}
	for _, d := range discovered {
		dn := normalizeRaceName(d.name)
		if dn == target || strings.Contains(dn, target) || strings.Contains(target, dn) {
			return d, true
		}
	}
	return discoveredRace{}, false
}

func normalizeRaceName(s string) string {
	s = strings.ToLower(s)
	replacer := strings.NewReplacer("grand prix", "", "gp", "", "-", "", " ", "", "'", "")
	return strings.TrimSpace(replacer.Replace(s))
}

// fetchDocument centralizes request setup (timeout, headers, status
// checking) so every scrape call behaves the same way.
func (s *ScrapedRaceResultService) fetchDocument(ctx context.Context, url string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to %s failed: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse html from %s: %w", url, err)
	}
	return doc, nil
}

// ScrapeAndPersistRaceResult fetches a single race-result page and saves
// every parseable driver row. The HTTP fetch happens outside the DB
// transaction so a slow/failed request doesn't hold a tx open.
func (s *ScrapedRaceResultService) ScrapeAndPersistRaceResult(ctx context.Context, resultUrl string, season *Season, race *Race) error {
	s.logger.Info("Scraping results", slog.String("url", resultUrl))

	doc, err := s.fetchDocument(ctx, resultUrl)
	if err != nil {
		return err
	}

	rows := doc.Find("table tbody tr")
	if rows.Length() == 0 {
		return fmt.Errorf("no result rows found at %s - page layout may have changed", resultUrl)
	}

	return s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		var saved, skipped int
		rows.Each(func(i int, row *goquery.Selection) {
			ok, reason := s.parseAndSaveRow(txCtx, row, race, season)
			if ok {
				saved++
				return
			}
			skipped++
			s.logger.Warn("Skipped result row",
				slog.String("race", race.Name),
				slog.Int("row", i),
				slog.String("reason", reason))
		})

		s.logger.Info("Finished scraping race",
			slog.String("race", race.Name),
			slog.Int("saved", saved),
			slog.Int("skipped", skipped))

		if saved == 0 {
			return fmt.Errorf("parsed 0 results for race %s at %s - page layout may have changed", race.Name, resultUrl)
		}
		return nil
	})
}

// parseAndSaveRow parses one <tr> from a race-result table. It returns
// whether the row was saved and, if not, a human-readable reason so failures
// are visible in logs instead of silently vanishing.
func (s *ScrapedRaceResultService) parseAndSaveRow(ctx context.Context, row *goquery.Selection, race *Race, season *Season) (bool, string) {
	cells := row.Find("td")
	if cells.Length() < 7 {
		return false, fmt.Sprintf("expected at least 7 cells, got %d", cells.Length())
	}

	position := strings.TrimSpace(cells.Eq(0).Text())

	driverCode := extractDriverCode(cells.Eq(2))
	if driverCode == "" {
		return false, fmt.Sprintf("could not extract driver code from %q", strings.TrimSpace(cells.Eq(2).Text()))
	}

	driver, err := s.driverRepository.FindByDriverCode(ctx, driverCode)
	if err != nil {
		return false, fmt.Sprintf("driver lookup failed for code %q: %v", driverCode, err)
	}
	if driver == nil {
		return false, fmt.Sprintf("no driver found for code %q", driverCode)
	}

	exists, err := s.driverRaceResultRepository.ExistsByDriverAndRaceAndSeason(ctx, driver, race, season)
	if err != nil {
		return false, fmt.Sprintf("existence check failed for driver %q: %v", driverCode, err)
	}
	if exists {
		return false, fmt.Sprintf("result already recorded for driver %q", driverCode)
	}

	pointsText := strings.TrimSpace(cells.Eq(6).Text())
	points, err := strconv.ParseFloat(pointsText, 64)
	if err != nil {
		points = 0
	}

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
		LapsCompleted: lapsCompleted,
		Status:        status,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.driverRaceResultRepository.Save(ctx, res); err != nil {
		return false, fmt.Sprintf("save failed for driver %q: %v", driverCode, err)
	}

	return true, ""
}

// extractDriverCode pulls the 3-letter driver code out of the driver cell.
// It tries a dedicated element first in case F1 wraps the code in its own
// tag, then falls back to the trailing-3-uppercase-letters pattern against
// the flattened cell text - confirmed against the live site to be
// "<Full Name><CODE>" with no separator, e.g. "George RussellRUS".
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

// deriveStatus determines a driver's race status. Critically, it checks the
// Time/Retired column FIRST regardless of position, because a driver who
// retires late but still completes enough of the race distance to be
// classified gets a numeric position with "DNF" in the time column - not
// position "NC". The previous version only checked DNF/DNS/DSQ when
// position == "NC", which mislabeled those drivers as "Finished".
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
