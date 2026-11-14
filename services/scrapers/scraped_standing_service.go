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

	"f1-api-go/internal/models"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
)

const tableBodyContentSelector = "table tbody"

var (
	upperCodeExact  = regexp.MustCompile(`^[A-Z]{3}$`)
	upperCodeSuffix = regexp.MustCompile(`[A-Z]{3}$`)
)

type ScrapedStandingService struct {
	driverRepository           DriverStandingRepository
	seasonRepository           SeasonRepository
	teamRepository             TeamRepository
	driverTeamSeasonRepository DriverTeamSeasonRepository
	txManager                  TransactionManager
	fetcher                    *pageFetcher
	logger                     *slog.Logger
}

func NewScrapedStandingService(
	driverRepo DriverStandingRepository,
	seasonRepo SeasonRepository,
	teamRepo TeamRepository,
	dtsRepo DriverTeamSeasonRepository,
	txManager TransactionManager,
	logger *slog.Logger,
) *ScrapedStandingService {
	if logger == nil {
		logger = slog.Default()
	}
	return &ScrapedStandingService{
		driverRepository:           driverRepo,
		seasonRepository:           seasonRepo,
		teamRepository:             teamRepo,
		driverTeamSeasonRepository: dtsRepo,
		txManager:                  txManager,
		fetcher:                    newPageFetcher(15*time.Second, logger),
		logger:                     logger,
	}
}

// parseDriverCell splits the standings "Driver" cell ("Max Verstappen VER",
// "Max VerstappenVER", ...) into first name, last name and 3-letter code.
func parseDriverCell(text string) (first, last, code string) {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return "", "", ""
	}
	first = fields[0]
	rest := append([]string(nil), fields[1:]...)
	if len(rest) == 0 {
		return first, "", ""
	}

	lastField := rest[len(rest)-1]
	switch {
	case upperCodeExact.MatchString(lastField):
		code = lastField
		rest = rest[:len(rest)-1]
	case len(lastField) > 3 && upperCodeSuffix.MatchString(lastField):
		code = lastField[len(lastField)-3:]
		rest[len(rest)-1] = lastField[:len(lastField)-3]
	}
	return first, strings.TrimSpace(strings.Join(rest, " ")), code
}

func (s *ScrapedStandingService) ScrapeF1Standings(ctx context.Context, year string) ([]*ScrapedStanding, error) {
	s.logger.Info("Starting to scrape F1 standings for year", slog.String("year", year))

	url := fmt.Sprintf("https://www.formula1.com/en/results/%s/drivers", year)
	doc, err := s.fetcher.fetchDocument(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("fetching standings for %s: %w", year, err)
	}

	tableBody := doc.Find(tableBodyContentSelector).First()
	if tableBody.Length() == 0 {
		return nil, fmt.Errorf("no standings table found at %s - page layout may have changed", url)
	}

	var standings []*ScrapedStanding
	rowCount, skipped := 0, 0

	tableBody.Find("tr").Each(func(i int, row *goquery.Selection) {
		cells := row.Find("td")
		if cells.Length() < 5 {
			return
		}
		rowCount++

		position, err := strconv.Atoi(strings.TrimSpace(cells.Eq(0).Text()))
		if err != nil {
			skipped++
			s.logger.Warn("Skipping standings row: bad position", slog.Int("row", i), slog.String("text", strings.TrimSpace(cells.Eq(0).Text())))
			return
		}

		first, last, code := parseDriverCell(cells.Eq(1).Text())
		if first == "" {
			skipped++
			s.logger.Warn("Skipping standings row: empty driver cell", slog.Int("row", i))
			return
		}

		pointsText := strings.TrimSpace(cells.Eq(4).Text())
		points, err := strconv.Atoi(pointsText)
		if err != nil {
			// Never turn an unparseable value into 0: that would overwrite a real total.
			skipped++
			s.logger.Warn("Skipping standings row: bad points",
				slog.String("driver", first+" "+last), slog.String("text", pointsText))
			return
		}

		standings = append(standings, &ScrapedStanding{
			Position:    position,
			FirstName:   first,
			LastName:    last,
			DriverCode:  code,
			Nationality: strings.TrimSpace(cells.Eq(2).Text()),
			Team:        strings.TrimSpace(cells.Eq(3).Text()),
			Points:      points,
		})
	})

	if len(standings) == 0 {
		return nil, fmt.Errorf("no standings rows parsed from %s (%d rows seen, %d skipped) - page layout may have changed", url, rowCount, skipped)
	}

	s.logger.Info("Successfully scraped standings for year.",
		slog.String("year", year), slog.Int("count", len(standings)), slog.Int("skipped", skipped))
	return standings, nil
}

func (s *ScrapedStandingService) UpdateDriverAndPointsFromScraperForSeason(ctx context.Context, year string) error {
	startTime := time.Now()
	s.logger.Info("Starting point update for season:", slog.String("year", year))

	scrapedData, err := s.ScrapeF1Standings(ctx, year)
	if err != nil {
		return err
	}

	season, err := s.seasonRepository.FindBySeasonName(ctx, year)
	if err != nil {
		return fmt.Errorf("season lookup failed for %s: %w", year, err)
	}
	if season == nil {
		return fmt.Errorf("%w: %s", ErrSeasonNotFound, year)
	}

	teams, err := s.teamRepository.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("loading teams failed: %w", err)
	}
	if len(teams) == 0 {
		return errors.New("no teams in the database - run the team scraper first")
	}

	driverCache := make(map[string]*models.Driver)
	var errs []error
	updated := 0

	for _, data := range scrapedData {
		err := s.txManager.WithTx(ctx, func(txCtx context.Context) error {
			return s.ProcessSingleStanding(txCtx, data, season, teams, driverCache)
		})
		if err != nil {
			s.logger.Error("Failed to process standing for driver",
				slog.String("driver", data.FirstName+" "+data.LastName),
				slog.String("code", data.DriverCode),
				slog.String("error", err.Error()))
			errs = append(errs, fmt.Errorf("%s %s: %w", data.FirstName, data.LastName, err))
			continue
		}
		updated++
	}

	s.logger.Info("Finished updating points for season.",
		slog.String("year", year),
		slog.Int("processed", updated),
		slog.Int("failed", len(errs)),
		slog.Duration("duration", time.Since(startTime)),
	)
	return errors.Join(errs...)
}

func driverCacheKey(d *ScrapedStanding) string {
	if d.DriverCode != "" {
		return "code:" + d.DriverCode
	}
	return "name:" + strings.ToLower(d.FirstName+"|"+d.LastName)
}

// findDriver tries the driver code first (unambiguous), then first+last name,
// and only falls back to the surname alone when it is unique in the table
// (historical drivers share surnames, and a bare surname lookup used to return
// an arbitrary one of them).
func (s *ScrapedStandingService) findDriver(ctx context.Context, data *ScrapedStanding, cache map[string]*models.Driver) (*models.Driver, error) {
	key := driverCacheKey(data)
	if d, ok := cache[key]; ok {
		return d, nil
	}

	var driver *models.Driver
	var err error

	if len(data.DriverCode) == 3 {
		if driver, err = s.driverRepository.FindByDriverCode(ctx, data.DriverCode); err != nil {
			return nil, fmt.Errorf("driver lookup by code failed: %w", err)
		}
	}
	if driver == nil && data.FirstName != "" && data.LastName != "" {
		if driver, err = s.driverRepository.FindByFirstNameAndLastNameIgnoreCase(ctx, data.FirstName, data.LastName); err != nil {
			return nil, fmt.Errorf("driver lookup by name failed: %w", err)
		}
	}
	if driver == nil && data.LastName != "" {
		matches, err := s.driverRepository.FindAllByLastNameIgnoreCase(ctx, data.LastName)
		if err != nil {
			return nil, fmt.Errorf("driver lookup by surname failed: %w", err)
		}
		switch {
		case len(matches) == 1:
			driver = matches[0]
		case len(matches) > 1:
			return nil, fmt.Errorf("ambiguous driver: %d drivers share surname %q and no driver code matched", len(matches), data.LastName)
		}
	}
	if driver == nil {
		return nil, fmt.Errorf("driver not found (code=%q)", data.DriverCode)
	}

	cache[key] = driver
	return driver, nil
}

// matchTeam picks the team whose (non-empty) short name appears in the scraped
// team string, preferring the longest short name so "Aston Martin Aramco
// Mercedes" matches "Aston Martin" rather than "Mercedes".
func matchTeam(teams []*Team, scraped string) *Team {
	name := strings.ToLower(strings.TrimSpace(scraped))
	var best *Team
	bestLen := 0
	for _, t := range teams {
		short := strings.ToLower(strings.TrimSpace(t.ShortName))
		if short == "" {
			continue // an empty short name would match every team string
		}
		if strings.Contains(name, short) && len(short) > bestLen {
			best, bestLen = t, len(short)
		}
	}
	return best
}

func (s *ScrapedStandingService) ProcessSingleStanding(ctx context.Context, data *ScrapedStanding, season *Season, teams []*Team, driverCache map[string]*models.Driver) error {
	driver, err := s.findDriver(ctx, data, driverCache)
	if err != nil {
		return err
	}

	matchedTeam := matchTeam(teams, data.Team)
	if matchedTeam == nil {
		return fmt.Errorf("team not found for scraped name: %q", data.Team)
	}

	dts, err := s.driverTeamSeasonRepository.FindByDriverAndSeasonAndTeam(ctx, driver, season, matchedTeam)
	if err != nil {
		// A lookup error must not fall through to "create": that inserts duplicates.
		return fmt.Errorf("driver/team/season lookup failed: %w", err)
	}
	now := time.Now()

	if dts != nil {
		if dts.Points == data.Points {
			return nil
		}
		s.logger.Info("Updating points for driver.",
			slog.String("firstname", driver.Firstname),
			slog.String("lastname", driver.Lastname),
			slog.Int("old_points", dts.Points),
			slog.Int("points", data.Points),
		)
		dts.Points = data.Points
		dts.UpdatedAt = now
		_, err = s.driverTeamSeasonRepository.Save(ctx, dts)
		return err
	}

	s.logger.Info("Creating new DTS for driver.",
		slog.String("firstname", driver.Firstname),
		slog.String("lastname", driver.Lastname),
		slog.String("team", matchedTeam.ShortName),
	)

	_, err = s.driverTeamSeasonRepository.Save(ctx, &DriverTeamSeason{
		ID:        uuid.New(),
		Driver:    driver,
		Season:    season,
		Team:      matchedTeam,
		Points:    data.Points,
		CreatedAt: now,
		UpdatedAt: now,
	})
	return err
}
