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

var driverCodePattern = regexp.MustCompile(`([A-Z]{3})$`)
var raceResultUrlPattern = regexp.MustCompile(`/races/\d+/([a-z0-9-]+)/race-result`)

type discoveredRace struct {
	name string
	slug string
	date string
	url  string
}

func (s *ScrapedRaceResultService) UpdatePendingRaceResults(ctx context.Context, year string) error {
	s.logger.Info("Syncing completed race results up to current date.", slog.String("year", year))

	now := time.Now()

	if err := s.raceRepository.UpdatePastScheduledRacesToCompleted(ctx, now, year); err != nil {
		s.logger.Error("Failed to update past scheduled races to completed", slog.String("error", err.Error()))
	}

	season, err := s.seasonRepository.FindBySeasonName(ctx, year)
	if err != nil || season == nil {
		return fmt.Errorf("season not found: %s", year)
	}

	pendingModelRaces, err := s.raceRepository.FindAllFinishedRacesMissingResults(ctx, now, year)
	if err != nil {
		return fmt.Errorf("failed to look up races missing results: %w", err)
	}
	if len(pendingModelRaces) == 0 {
		s.logger.Info("All finished races already have results recorded.", slog.String("year", year))
		return nil
	}

	var pendingRaces []Race
	for _, m := range pendingModelRaces {
		pendingRaces = append(pendingRaces, Race{
			ID:            m.ID,
			Name:          m.Name,
			Country:       m.Country,
			RaceStartDate: m.RaceStartDate,
			RaceEndDate:   m.RaceEndDate,
		})
	}

	discovered, err := s.discoverRaceResults(ctx, year)
	if err != nil {
		return fmt.Errorf("failed to discover race result pages: %w", err)
	}

	var unmatched []string
	for i := range pendingRaces {
		race := &pendingRaces[i]

		match, ok := matchRace(discovered, race)
		if !ok {
			var rows []string
			for _, d := range discovered {
				rows = append(rows, fmt.Sprintf("%s|%s|%s", d.slug, d.date, d.name))
			}
			s.logger.Warn("F1.com result page not found for race - leaving unlinked, will retry next run.",
				slog.String("race", race.Name),
				slog.Time("race_start_date", race.RaceStartDate),
				slog.String("discovered_slug_date_name", strings.Join(rows, " || ")))
			unmatched = append(unmatched, race.Name)
			continue
		}

		if err := s.ScrapeAndPersistRaceResult(ctx, match.url, season, race); err != nil {
			s.logger.Error("Failed to scrape race result", slog.String("race", race.Name), slog.String("error", err.Error()))
		}
	}

	if len(unmatched) > 0 {
		s.logger.Warn("Some races could not be matched this run.",
			slog.Int("count", len(unmatched)),
			slog.String("races", strings.Join(unmatched, ", ")))
	}
	return nil
}

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
		re := regexp.MustCompile(`(\d{2}\s+[A-Za-z]{3})`)
		if match := re.FindString(dateText); match != "" {
			dateText = match
		}

		results = append(results, discoveredRace{
			name: strings.TrimSpace(cells.Eq(0).Text()),
			slug: slug,
			date: dateText,
			url:  href,
		})
	})
	return results, nil
}

func matchRace(discovered []discoveredRace, race *Race) (discoveredRace, bool) {
	if match, ok := matchByDate(discovered, race.RaceStartDate); ok {
		return match, true
	}
	return matchByName(discovered, race.Name)
}

func matchByDate(discovered []discoveredRace, raceDate time.Time) (discoveredRace, bool) {
	if raceDate.IsZero() {
		return discoveredRace{}, false
	}
	for _, d := range discovered {
		parsed, err := time.Parse("02 Jan", d.date)
		if err != nil {
			continue
		}
		if parsed.Day() == raceDate.Day() && parsed.Month() == raceDate.Month() {
			return d, true
		}
	}
	return discoveredRace{}, false
}

func matchByName(discovered []discoveredRace, raceName string) (discoveredRace, bool) {
	target := normalizeRaceName(raceName)
	if target == "" {
		return discoveredRace{}, false
	}
	for _, d := range discovered {
		if d.slug == "" {
			continue
		}
		slugNormalized := strings.ReplaceAll(d.slug, "-", "")

		slugKey := slugNormalized
		if slugKey == "greatbritain" {
			slugKey = "british"
		}

		if strings.Contains(target, slugNormalized) || strings.Contains(target, slugKey) {
			return d, true
		}
	}
	return discoveredRace{}, false
}

func normalizeRaceName(s string) string {
	s = strings.ToLower(s)
	re := regexp.MustCompile(`\b20\d{2}\b`)
	s = re.ReplaceAllString(s, "")

	replacer := strings.NewReplacer("grand prix", "", "gp", "", "-", "", " ", "", "'", "")
	return strings.TrimSpace(replacer.Replace(s))
}

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
