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
	historicalLookup           *historicalDriverClient
	backfillMode               bool
	seasonYear                 string
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
	return NewScrapedStandingServiceWithOptions(driverRepo, seasonRepo, teamRepo, dtsRepo, txManager, false, "", logger)
}

func NewScrapedStandingServiceWithOptions(
	driverRepo DriverStandingRepository,
	seasonRepo SeasonRepository,
	teamRepo TeamRepository,
	dtsRepo DriverTeamSeasonRepository,
	txManager TransactionManager,
	backfillMode bool,
	seasonYear string,
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
		historicalLookup:           newHistoricalDriverClient(logger),
		backfillMode:               backfillMode,
		seasonYear:                 strings.TrimSpace(seasonYear),
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
		pointsFloat, err := strconv.ParseFloat(pointsText, 64)
		if err != nil {
			skipped++
			s.logger.Warn("Skipping standings row: bad points",
				slog.String("driver", first+" "+last), slog.String("text", pointsText))
			return
		}
		points := int(pointsFloat)

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

	created := false
	if driver == nil {
		now := time.Now()
		driver = &models.Driver{
			ID:          uuid.New(),
			Firstname:   data.FirstName,
			Lastname:    data.LastName,
			DriverCode:  data.DriverCode,
			Country:     data.Nationality,
			CountryCode: getCountryCode(data.Nationality),
			Team:        data.Team,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		created = true
	}

	if s.backfillMode && s.historicalLookup != nil && (created || driver.Birthday.IsZero() || strings.TrimSpace(driver.Lastname) == "") {
		enriched, lookupErr := s.historicalLookup.lookup(ctx, s.seasonYear, driver.DriverCode, driver.Firstname, driver.Lastname)
		if lookupErr != nil {
			s.logger.Warn("Fallback driver enrichment failed",
				slog.String("driver", driver.Firstname+" "+driver.Lastname),
				slog.String("code", driver.DriverCode),
				slog.String("error", lookupErr.Error()))
		} else if enriched != nil {
			applyDriverUpdates(driver, enriched)
			driver.UpdatedAt = time.Now()
		}
	}

	if created {
		if err := s.driverRepository.Save(ctx, driver); err != nil {
			return nil, fmt.Errorf("failed to auto-create historical driver %s %s: %w", driver.Firstname, driver.Lastname, err)
		}
		s.logger.Info("Auto-created missing historical driver", slog.String("driver", driver.Firstname+" "+driver.Lastname), slog.String("code", driver.DriverCode))
	} else if s.backfillMode && !driver.Birthday.IsZero() {
		if err := s.driverRepository.Save(ctx, driver); err != nil {
			return nil, fmt.Errorf("failed to save enriched driver %s %s: %w", driver.Firstname, driver.Lastname, err)
		}
	}

	cache[key] = driver
	return driver, nil
}

// constructorKeys maps a normalized scraped team string to ordered candidate
// team names/short names in f1_api.team. Constructor checks come before
// engine/supplier checks so strings like "Haas Ferrari" or "Alfa Romeo Ferrari"
// never misclassify as Ferrari.
func constructorKeys(scraped string) []string {
	name := strings.ToLower(strings.TrimSpace(scraped))
	if name == "" {
		return nil
	}

	switch {
	case strings.Contains(name, "haas"):
		return []string{"tgr haas f1 team", "haas f1 team", "haas ferrari"}
	case strings.Contains(name, "alfa romeo") || strings.Contains(name, "kick sauber") || strings.Contains(name, "sauber"):
		return []string{
			"audi revolut f1 team",
			"audi",
			"stake f1 team kick sauber",
			"kick sauber",
			"kick sauber ferrari",
			"alfa romeo ferrari",
		}
	case strings.Contains(name, "aston martin") || strings.Contains(name, "racing point"):
		return []string{
			"aston martin aramco formula one team",
			"aston martin",
			"aston martin aramco mercedes",
		}
	case strings.Contains(name, "alpine") || strings.Contains(name, "renault"):
		return []string{
			"bwt alpine formula one team",
			"alpine",
			"alpine renault",
		}
	case strings.Contains(name, "toro rosso") || strings.Contains(name, "alphatauri") || strings.Contains(name, "racing bulls") || name == "rb" || strings.HasPrefix(name, "rb "):
		return []string{
			"visa cash app racing bulls formula one team",
			"racing bulls",
			"rb honda rbpt",
			"alphatauri honda rbpt",
			"scuderia toro rosso honda",
			"toro rosso",
		}
	case strings.Contains(name, "red bull"):
		return []string{
			"oracle red bull racing",
			"red bull racing",
			"red bull racing honda rbpt",
		}
	case strings.Contains(name, "mclaren"):
		return []string{
			"mclaren mastercard f1 team",
			"mclaren",
			"mclaren mercedes",
		}
	case strings.Contains(name, "williams"):
		return []string{
			"atlassian williams f1 team",
			"williams",
			"williams mercedes",
		}
	case strings.Contains(name, "audi"):
		return []string{"audi revolut f1 team", "audi"}
	case strings.Contains(name, "cadillac"):
		return []string{"cadillac formula 1 team", "cadillac"}
	case strings.Contains(name, "ferrari"):
		return []string{"scuderia ferrari hp", "ferrari"}
	case strings.Contains(name, "mercedes"):
		return []string{"mercedes-amg petronas formula one team", "mercedes"}
	default:
		return []string{name}
	}
}

func deriveSeasonShortName(scrapedTeam string, matchedTeam *Team) string {
	raw := strings.TrimSpace(scrapedTeam)
	if raw == "" {
		return matchedTeam.ShortName
	}
	lower := strings.ToLower(raw)

	switch {
	case strings.Contains(lower, "haas"):
		return "Haas F1 Team"
	case strings.Contains(lower, "alfa romeo"):
		return "Alfa Romeo"
	case strings.Contains(lower, "kick sauber"):
		return "Kick Sauber"
	case strings.Contains(lower, "sauber"):
		return "Sauber"
	case strings.Contains(lower, "aston martin"):
		return "Aston Martin"
	case strings.Contains(lower, "racing point"):
		return "Racing Point"
	case strings.Contains(lower, "alpine"):
		return "Alpine"
	case strings.Contains(lower, "renault"):
		return "Renault"
	case strings.Contains(lower, "toro rosso"):
		return "Toro Rosso"
	case strings.Contains(lower, "alphatauri"):
		return "AlphaTauri"
	case strings.Contains(lower, "racing bulls"):
		return "Racing Bulls"
	case lower == "rb" || strings.HasPrefix(lower, "rb "):
		return "RB"
	case strings.Contains(lower, "red bull"):
		return "Red Bull Racing"
	case strings.Contains(lower, "mclaren"):
		return "McLaren"
	case strings.Contains(lower, "williams"):
		return "Williams"
	case strings.Contains(lower, "audi"):
		return "Audi"
	case strings.Contains(lower, "cadillac"):
		return "Cadillac"
	case strings.Contains(lower, "ferrari"):
		return "Ferrari"
	case strings.Contains(lower, "mercedes"):
		return "Mercedes"
	default:
		if matchedTeam.ShortName != "" {
			return matchedTeam.ShortName
		}
		return raw
	}
}

func matchTeam(teams []*Team, scraped string) *Team {
	keys := constructorKeys(scraped)
	if len(keys) == 0 {
		return nil
	}

	for _, key := range keys {
		for _, t := range teams {
			shortCand := strings.ToLower(strings.TrimSpace(t.ShortName))
			nameCand := strings.ToLower(strings.TrimSpace(t.Name))
			if shortCand == key || nameCand == key {
				return t
			}
		}
	}

	return nil
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

	seasonTeamName := strings.TrimSpace(data.Team)
	if seasonTeamName == "" {
		seasonTeamName = matchedTeam.Name
	}
	seasonShortName := deriveSeasonShortName(seasonTeamName, matchedTeam)

	dts, err := s.driverTeamSeasonRepository.FindByDriverAndSeason(ctx, driver, season)
	if err != nil {
		return fmt.Errorf("driver/season lookup failed: %w", err)
	}
	now := time.Now()

	if dts != nil {
		teamChanged := dts.Team == nil || dts.Team.ID != matchedTeam.ID
		pointsChanged := dts.Points != data.Points
		nameChanged := dts.SeasonTeamName != seasonTeamName || dts.SeasonShortName != seasonShortName
		if !teamChanged && !pointsChanged && !nameChanged {
			return nil
		}

		s.logger.Info("Updating DTS for driver.",
			slog.String("firstname", driver.Firstname),
			slog.String("lastname", driver.Lastname),
			slog.Int("old_points", dts.Points),
			slog.Int("points", data.Points),
			slog.String("season_team", seasonTeamName),
		)
		dts.Team = matchedTeam
		dts.Points = data.Points
		dts.SeasonTeamName = seasonTeamName
		dts.SeasonShortName = seasonShortName
		dts.UpdatedAt = now
		_, err = s.driverTeamSeasonRepository.Save(ctx, dts)
		return err
	}

	s.logger.Info("Creating new DTS for driver.",
		slog.String("firstname", driver.Firstname),
		slog.String("lastname", driver.Lastname),
		slog.String("team", seasonShortName),
	)

	_, err = s.driverTeamSeasonRepository.Save(ctx, &DriverTeamSeason{
		ID:              uuid.New(),
		Driver:          driver,
		Season:          season,
		Team:            matchedTeam,
		Points:          data.Points,
		SeasonTeamName:  seasonTeamName,
		SeasonShortName: seasonShortName,
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	return err
}
