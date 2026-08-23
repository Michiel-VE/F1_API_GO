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

const tableBodyContentSelector = "table tbody"

type ScrapedStanding struct {
	Position    int
	FirstName   string
	LastName    string
	DriverCode  string
	Nationality string
	Team        string
	Points      int
}

type DriverTeamSeason struct {
	ID        uuid.UUID
	Driver    *Driver
	Season    *Season
	Team      *Team
	Points    int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Team struct {
	ID        uuid.UUID
	Name      string
	ShortName string
	Country   string
	Base      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TeamRepository interface {
	FindAll(ctx context.Context) ([]*Team, error)
	FindAllByShortName(ctx context.Context, shortName string) ([]*Team, error)
	Save(ctx context.Context, team *Team) error
	Delete(ctx context.Context, team *Team) error
}

type DriverTeamSeasonRepository interface {
	FindByDriverAndSeasonAndTeam(ctx context.Context, driver *Driver, season *Season, team *Team) (*DriverTeamSeason, error)
	Save(ctx context.Context, dts *DriverTeamSeason) (*DriverTeamSeason, error)
}

type DriverStandingRepository interface {
	FindByLastnameIgnoreCase(ctx context.Context, lastname string) (*Driver, error)
	FindByFirstnameIgnoreCase(ctx context.Context, firstname string) (*Driver, error)
}

type TransactionManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type ScrapedStandingService struct {
	driverRepository           DriverStandingRepository
	seasonRepository           SeasonRepository
	teamRepository             TeamRepository
	driverTeamSeasonRepository DriverTeamSeasonRepository
	txManager                  TransactionManager
	httpClient                 *http.Client
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
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

func (s *ScrapedStandingService) ScrapeF1Standings(ctx context.Context, year string) ([]*ScrapedStanding, error) {
	var scrapedStandings []*ScrapedStanding
	s.logger.Info("Starting to scrape F1 standings for year", slog.String("year", year))

	url := fmt.Sprintf("https://www.formula1.com/en/results/%s/drivers", year)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logger.Error("Error scraping F1 standings", slog.String("year", year), slog.String("error", err.Error()))
		return scrapedStandings, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.Warn("Failed to retrieve standings page", slog.String("year", year), slog.Int("status", resp.StatusCode))
		return scrapedStandings, nil
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		s.logger.Error("Error parsing standings response HTML", slog.String("year", year), slog.String("error", err.Error()))
		return scrapedStandings, err
	}

	tableBody := doc.Find(tableBodyContentSelector).First()
	if tableBody.Length() == 0 {
		s.logger.Warn("No standings table found on page for year. Scrape failed.", slog.String("year", year))
		return scrapedStandings, nil
	}

	upperCodeRegex := regexp.MustCompile(`[A-Z]{3}`)

	tableBody.Find("tr").Each(func(i int, row *goquery.Selection) {
		cells := row.Find("td")
		if cells.Length() >= 5 {
			posText := strings.TrimSpace(cells.Eq(0).Text())
			position, _ := strconv.Atoi(posText)

			nameData := strings.Fields(strings.TrimSpace(cells.Eq(1).Text()))
			if len(nameData) == 0 {
				return
			}

			firstName := nameData[0]
			lastName := "N/A"
			driverCode := "N/A"

			if len(nameData) > 1 {
				lastIndex := len(nameData) - 1
				lastElement := nameData[lastIndex]
				nameEndIndex := lastIndex

				if len(lastElement) == 3 && upperCodeRegex.MatchString(lastElement) && lastIndex > 0 {
					driverCode = lastElement
					nameEndIndex = lastIndex - 1
				} else if len(lastElement) > 3 && upperCodeRegex.MatchString(lastElement[len(lastElement)-3:]) {
					driverCode = lastElement[len(lastElement)-3:]
					nameData[lastIndex] = lastElement[:len(lastElement)-3]
				} else {
					driverCode = "N/A"
					nameEndIndex = lastIndex
				}

				var sb strings.Builder
				for idx := 1; idx <= nameEndIndex; idx++ {
					sb.WriteString(nameData[idx])
					if idx < nameEndIndex {
						sb.WriteString(" ")
					}
				}
				lastName = strings.TrimSpace(sb.String())
			}

			pointsText := strings.TrimSpace(cells.Eq(4).Text())
			points, _ := strconv.Atoi(pointsText)

			standing := &ScrapedStanding{
				Position:    position,
				FirstName:   firstName,
				LastName:    lastName,
				DriverCode:  driverCode,
				Nationality: strings.TrimSpace(cells.Eq(2).Text()),
				Team:        strings.TrimSpace(cells.Eq(3).Text()),
				Points:      points,
			}
			scrapedStandings = append(scrapedStandings, standing)
		}
	})

	s.logger.Info("Successfully scraped standings for year.", slog.Int("count", len(scrapedStandings)), slog.String("year", year))
	return scrapedStandings, nil
}

func (s *ScrapedStandingService) UpdateDriverAndPointsFromScraperForSeason(ctx context.Context, year string) error {
	startTime := time.Now()
	s.logger.Info("Starting point update for season:", slog.String("year", year))

	scrapedData, err := s.ScrapeF1Standings(ctx, year)
	if err != nil || len(scrapedData) == 0 {
		return err
	}

	season, err := s.seasonRepository.FindBySeasonName(ctx, year)
	if err != nil || season == nil {
		return fmt.Errorf("season %s not found", year)
	}

	driverCache := make(map[string]*Driver)

	for _, data := range scrapedData {
		err := s.txManager.WithTx(ctx, func(txCtx context.Context) error {
			return s.ProcessSingleStanding(txCtx, data, season, driverCache)
		})
		if err != nil {
			s.logger.Error("Failed to process standing for driver", slog.String("lastname", data.LastName), slog.String("error", err.Error()))
		}
	}

	s.logger.Info("Finished updating points for season.",
		slog.String("year", year),
		slog.Duration("duration", time.Since(startTime)),
	)
	return nil
}

func (s *ScrapedStandingService) ProcessSingleStanding(ctx context.Context, data *ScrapedStanding, season *Season, driverCache map[string]*Driver) error {
	driver, ok := driverCache[data.LastName]
	if !ok {
		var err error
		driver, err = s.driverRepository.FindByLastnameIgnoreCase(ctx, data.LastName)
		if err != nil || driver == nil {
			driver, err = s.driverRepository.FindByFirstnameIgnoreCase(ctx, data.LastName)
			if err != nil || driver == nil {
				return fmt.Errorf("driver not found: %s", data.LastName)
			}
		}
		driverCache[data.LastName] = driver
	}

	allTeams, err := s.teamRepository.FindAll(ctx)
	if err != nil {
		return err
	}

	scrapedTeamName := strings.ToLower(data.Team)
	var matchedTeam *Team
	for _, t := range allTeams {
		if strings.Contains(scrapedTeamName, strings.ToLower(t.ShortName)) {
			matchedTeam = t
			break
		}
	}

	if matchedTeam == nil {
		return fmt.Errorf("team not found for scraped name: %s", data.Team)
	}

	dts, err := s.driverTeamSeasonRepository.FindByDriverAndSeasonAndTeam(ctx, driver, season, matchedTeam)
	now := time.Now()

	if err == nil && dts != nil {
		if dts.Points != data.Points {
			s.logger.Info("Updating points for driver.",
				slog.String("firstname", driver.Firstname),
				slog.String("lastname", driver.Lastname),
				slog.Int("points", data.Points),
			)
			dts.Points = data.Points
			dts.UpdatedAt = now
			_, err = s.driverTeamSeasonRepository.Save(ctx, dts)
			return err
		}
		return nil
	}

	s.logger.Info("Creating new DTS for driver.",
		slog.String("firstname", driver.Firstname),
		slog.String("lastname", driver.Lastname),
		slog.String("team", matchedTeam.ShortName),
	)

	newDTS := &DriverTeamSeason{
		ID:        uuid.New(),
		Driver:    driver,
		Season:    season,
		Team:      matchedTeam,
		Points:    data.Points,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err = s.driverTeamSeasonRepository.Save(ctx, newDTS)
	return err
}