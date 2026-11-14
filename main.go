package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"f1-api-go/config"
	"f1-api-go/controller"
	"f1-api-go/internal/models"
	"f1-api-go/repository"
	"f1-api-go/services"
	"f1-api-go/services/scrapers"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Adapters between the scrapers package interfaces and the repositories.
// None of them touch *gorm.DB directly any more: every write goes through a
// repository method that uses repository.DBFrom(ctx, ...), so it joins the
// transaction carried by ctx.
// ---------------------------------------------------------------------------

type seasonRepoAdapter struct {
	repo *repository.SeasonRepository
}

func (a *seasonRepoAdapter) FindBySeasonName(ctx context.Context, name string) (*scrapers.Season, error) {
	m, err := a.repo.FindBySeasonName(ctx, name)
	if err != nil || m == nil {
		return nil, err
	}
	return &scrapers.Season{ID: m.ID, SeasonName: m.SeasonName, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}, nil
}

func (a *seasonRepoAdapter) Save(ctx context.Context, s *scrapers.Season) (*scrapers.Season, error) {
	m := &models.Season{ID: s.ID, SeasonName: s.SeasonName, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt}
	if err := a.repo.Save(ctx, m); err != nil {
		return nil, err
	}
	return s, nil
}

type raceRepoAdapter struct {
	repo *repository.RaceRepository
}

func toScraperRace(m *models.Race) *scrapers.Race {
	return &scrapers.Race{
		ID:            m.ID,
		Name:          m.Name,
		Country:       m.Country,
		RaceStartDate: m.RaceStartDate,
		RaceEndDate:   m.RaceEndDate,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}

func (a *raceRepoAdapter) FindAllFinishedRacesMissingResults(ctx context.Context, now time.Time, season string) ([]models.Race, error) {
	return a.repo.FindAllFinishedRacesMissingResults(ctx, now, season)
}

func (a *raceRepoAdapter) FindByNameAndRaceStartDateAndRaceEndDate(ctx context.Context, name string, start, end time.Time) (*scrapers.Race, error) {
	m, err := a.repo.FindByNameAndRaceStartDateAndRaceEndDate(ctx, name, start, end)
	if err != nil || m == nil {
		return nil, err
	}
	return toScraperRace(m), nil
}

func (a *raceRepoAdapter) FindByRaceStartDateInSeason(ctx context.Context, seasonName string, start time.Time) (*scrapers.Race, error) {
	m, err := a.repo.FindByRaceStartDateInSeason(ctx, seasonName, start)
	if err != nil || m == nil {
		return nil, err
	}
	return toScraperRace(m), nil
}

func (a *raceRepoAdapter) UpdateDetails(ctx context.Context, raceID uuid.UUID, name, country string, start, end time.Time) error {
	return a.repo.UpdateDetails(ctx, raceID, name, country, start, end)
}

func (a *raceRepoAdapter) UpdatePastScheduledRacesToCompleted(ctx context.Context, now time.Time, season string) error {
	return a.repo.UpdatePastScheduledRacesToCompleted(ctx, now, season)
}

func (a *raceRepoAdapter) Save(ctx context.Context, r *scrapers.Race) (*scrapers.Race, error) {
	m := &models.Race{
		ID:            r.ID,
		Name:          r.Name,
		Country:       r.Country,
		RaceStartDate: r.RaceStartDate,
		RaceEndDate:   r.RaceEndDate,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
	if r.Status != nil {
		m.StatusID = r.Status.ID
	}
	if err := a.repo.Save(ctx, m); err != nil {
		return nil, err
	}
	r.ID = m.ID
	return r, nil
}

type raceStatusAdapter struct {
	repo *repository.RaceStatusRepository
}

func (a *raceStatusAdapter) FindByName(ctx context.Context, name string) (*scrapers.RaceStatus, error) {
	m, err := a.repo.FindByName(ctx, name)
	if err != nil || m == nil {
		return nil, err
	}
	return &scrapers.RaceStatus{ID: m.ID, Name: m.Name}, nil
}

type raceSeasonAdapter struct {
	repo *repository.RaceSeasonRepository
}

func (a *raceSeasonAdapter) ExistsByRaceAndSeason(ctx context.Context, r *scrapers.Race, s *scrapers.Season) (bool, error) {
	return a.repo.ExistsByRaceAndSeason(ctx, r.ID, s.ID)
}

func (a *raceSeasonAdapter) Save(ctx context.Context, rs *scrapers.RaceSeason) error {
	return a.repo.Save(ctx, &models.RaceSeason{
		ID:        rs.ID,
		RaceID:    rs.Race.ID,
		SeasonID:  rs.Season.ID,
		CreatedAt: rs.CreatedAt,
		UpdatedAt: rs.UpdatedAt,
	})
}

type teamRepoAdapter struct {
	repo *repository.TeamRepository
}

func toScraperTeams(teams []models.Team) []*scrapers.Team {
	res := make([]*scrapers.Team, 0, len(teams))
	for i := range teams {
		t := teams[i]
		// CreatedAt/UpdatedAt must be copied: the scraper saves these structs
		// back, and a zero created_at would otherwise overwrite the real one.
		res = append(res, &scrapers.Team{
			ID: t.ID, Name: t.Name, ShortName: t.ShortName, Country: t.Country, Base: t.Base,
			CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
		})
	}
	return res
}

func (a *teamRepoAdapter) FindAll(ctx context.Context) ([]*scrapers.Team, error) {
	teams, err := a.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	return toScraperTeams(teams), nil
}

func (a *teamRepoAdapter) FindAllByShortName(ctx context.Context, shortName string) ([]*scrapers.Team, error) {
	teams, err := a.repo.FindAllByShortName(ctx, shortName)
	if err != nil {
		return nil, err
	}
	return toScraperTeams(teams), nil
}

func (a *teamRepoAdapter) Save(ctx context.Context, t *scrapers.Team) error {
	return a.repo.Save(ctx, &models.Team{
		ID:        t.ID,
		Name:      t.Name,
		ShortName: t.ShortName,
		Country:   t.Country,
		Base:      t.Base,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	})
}

func (a *teamRepoAdapter) Delete(ctx context.Context, t *scrapers.Team) error {
	return a.repo.Delete(ctx, t.ID)
}

type dtsRepoAdapter struct {
	repo *repository.DriverTeamSeasonRepository
}

func (a *dtsRepoAdapter) FindByDriverAndSeasonAndTeam(ctx context.Context, driver *models.Driver, season *scrapers.Season, team *scrapers.Team) (*scrapers.DriverTeamSeason, error) {
	m, err := a.repo.FindByDriverAndSeasonAndTeam(ctx, driver.ID, season.ID, team.ID)
	if err != nil || m == nil {
		return nil, err
	}
	return &scrapers.DriverTeamSeason{
		ID:        m.ID,
		Driver:    driver,
		Season:    season,
		Team:      team,
		Points:    m.Points,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}, nil
}

func (a *dtsRepoAdapter) Save(ctx context.Context, dts *scrapers.DriverTeamSeason) (*scrapers.DriverTeamSeason, error) {
	if dts == nil {
		return nil, fmt.Errorf("driver team season data is nil")
	}
	if dts.Driver == nil {
		return nil, fmt.Errorf("driver is nil for driver team season record")
	}
	if dts.Season == nil {
		return nil, fmt.Errorf("season is nil for driver team season record")
	}
	if dts.Team == nil {
		return nil, fmt.Errorf("team is nil for driver team season record")
	}

	m := &models.DriverTeamSeason{
		ID:        dts.ID,
		DriverID:  dts.Driver.ID,
		SeasonID:  dts.Season.ID,
		TeamID:    dts.Team.ID,
		Points:    dts.Points,
		CreatedAt: dts.CreatedAt,
		UpdatedAt: dts.UpdatedAt,
	}
	if err := a.repo.Save(ctx, m); err != nil {
		return nil, err
	}
	return dts, nil
}

type resultRepoAdapter struct {
	repo *repository.DriverRaceResultRepository
}

func (a *resultRepoAdapter) ExistsByDriverAndRaceAndSeason(ctx context.Context, driver *models.Driver, race *scrapers.Race, season *scrapers.Season) (bool, error) {
	return a.repo.ExistsByDriverAndRaceAndSeason(ctx, driver.ID, race.ID, season.ID)
}

func (a *resultRepoAdapter) Save(ctx context.Context, res *scrapers.DriverRaceResult) error {
	return a.repo.Save(ctx, &models.DriverRaceResult{
		ID:            res.ID,
		DriverID:      res.Driver.ID,
		RaceID:        res.Race.ID,
		SeasonID:      res.Season.ID,
		Points:        res.Points,
		LapsCompleted: &res.LapsCompleted,
		Status:        &res.Status,
		CreatedAt:     res.CreatedAt,
		UpdatedAt:     res.UpdatedAt,
	})
}

type driverRepoAdapter struct {
	repo *repository.DriverRepository
}

func (a *driverRepoAdapter) FindByDriverCode(ctx context.Context, driverCode string) (*models.Driver, error) {
	return a.repo.FindByDriverCode(ctx, driverCode)
}

func (a *driverRepoAdapter) FindByFirstNameAndLastNameIgnoreCase(ctx context.Context, firstname, lastname string) (*models.Driver, error) {
	return a.repo.FindByFirstNameAndLastNameIgnoreCase(ctx, firstname, lastname)
}

func (a *driverRepoAdapter) FindAllByLastNameIgnoreCase(ctx context.Context, lastname string) ([]*models.Driver, error) {
	drivers, err := a.repo.FindAllByLastNameIgnoreCase(ctx, lastname)
	if err != nil {
		return nil, err
	}
	res := make([]*models.Driver, 0, len(drivers))
	for i := range drivers {
		res = append(res, &drivers[i])
	}
	return res, nil
}

func (a *driverRepoAdapter) Save(ctx context.Context, driver *models.Driver) error {
	return a.repo.Save(ctx, driver)
}

// ---------------------------------------------------------------------------

func openDB() (*gorm.DB, error) {
	return gorm.Open(postgres.New(postgres.Config{
		DSN:                  config.AppCfg.DatabaseURL,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		PrepareStmt: false,
	})
}

func openRedis() (*redis.Client, error) {
	if config.AppCfg.RedisURL == "" {
		return nil, nil
	}
	opt, err := redis.ParseURL(config.AppCfg.RedisURL)
	if err != nil {
		return nil, err
	}
	return redis.NewClient(opt), nil
}

func main() {
	config.InitConfig()

	db, err := openDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	redisClient, redisErr := openRedis()

	runMode := os.Getenv("RUN_MODE")
	if runMode != "" {
		// A Redis problem must never stop a scraper, it only means no eviction.
		if redisErr != nil {
			log.Printf("WARN: Redis unavailable, caches will not be evicted after scraping: %v", redisErr)
			redisClient = nil
		}

		if os.Getenv("AWS_EXECUTION_ENV") != "" {
			http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
				log.Printf("Executing scraper task for RUN_MODE: %s", runMode)
				if err := runScraper(r.Context(), db, redisClient, runMode); err != nil {
					log.Printf("Scraper failed: %v", err)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			})

			port := os.Getenv("PORT")
			if port == "" {
				port = "8080"
			}
			log.Printf("Scraper container listening for Lambda Web Adapter events on port %s", port)
			if err := http.ListenAndServe(":"+port, nil); err != nil {
				log.Fatalf("Scraper HTTP server failed: %v", err)
			}
			return
		}

		if err := runScraper(context.Background(), db, redisClient, runMode); err != nil {
			log.Fatalf("Failed execution: %v", err)
		}
		return
	}

	if redisErr != nil {
		log.Fatalf("Failed to parse Redis URL: %v", redisErr)
	}

	driverRepoService := repository.NewDriverRepository(db)
	teamRepoService := repository.NewTeamRepository(db)
	raceRepoService := repository.NewRaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	driverTeamSeasonRepo := repository.NewDriverTeamSeasonRepository(db)
	driverRaceResultRepo := repository.NewDriverRaceResultRepository(db)

	poolRepo := repository.NewPoolRepository(db)
	teamPredictionRepo := repository.NewTeamPredictionRepository(db)
	predictionTeamRepo := repository.NewPredictionTeamRepository(db)
	txManager := repository.NewTransactionManager(db)

	driverService := services.NewDriverService(driverRepoService, redisClient, config.AppCfg.CacheEnabled)
	driverRaceResultsService := services.NewDriverRaceResultsService(driverRepoService, driverRaceResultRepo)
	teamService := services.NewTeamService(teamRepoService, driverTeamSeasonRepo, redisClient, config.AppCfg.CacheEnabled)
	raceService := services.NewRaceService(raceRepoService, redisClient, config.AppCfg.CacheEnabled)
	userService := services.NewUserService(userRepo)
	predictionService := services.NewPredictionService(
		poolRepo,
		teamPredictionRepo,
		predictionTeamRepo,
		userRepo,
		txManager,
	)

	jwtService, err := services.NewJWTService(config.AppCfg.JWTSecretKey, config.AppCfg.JwtExpiration.Milliseconds())
	if err != nil {
		log.Fatalf("Failed to initialize JWT service: %v", err)
	}
	authService := services.NewAuthService(userRepo, jwtService)

	authController := controller.NewAuthController(authService, jwtService)
	profileController := controller.NewProfileController(userService)
	driverController := controller.NewDriverController(driverService, driverRaceResultsService)
	teamController := controller.NewTeamController(teamService)
	raceController := controller.NewRaceController(raceService)
	predictionController := controller.NewPredictionController(predictionService)

	r := config.SecurityConfig(
		authController,
		profileController,
		driverController,
		teamController,
		raceController,
		predictionController,
		jwtService,
		nil,
	)

	serverAddr := fmt.Sprintf(":%s", config.AppCfg.ServerPort)
	server := &http.Server{
		Addr:         serverAddr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	fmt.Printf("Server running on http://localhost%s\n", serverAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// cachesToEvict lists the Redis cache regions each scraper invalidates. Before,
// nothing ever evicted them and entries had no TTL, so the API kept serving
// whatever it cached first.
var cachesToEvict = map[string][]string{
	"scraper-race":        {"races", "races_hist"},
	"scraper-driver-team": {"teams", "teams_hist"},
	"scraper-driver":      {"drivers", "drivers_hist"},
	"scraper-standing":    {"drivers", "drivers_hist", "teams", "teams_hist"},
	"scraper-race-result": {"results", "results_hist"},
}

// runScraper runs a scraper mode and then evicts the caches it affects. It
// evicts even when the run reported errors, because a partially failed run may
// still have changed data.
func runScraper(ctx context.Context, db *gorm.DB, redisClient *redis.Client, runMode string) error {
	runErr := executeRunMode(ctx, db, runMode)

	if redisClient != nil {
		if names := cachesToEvict[runMode]; len(names) > 0 {
			evictCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			services.NewCacheEvictionService(redisClient).EvictAndStamp(evictCtx, names...)
		}
	}
	return runErr
}

// seasonsToProcess returns the current season, plus the previous one in
// January/February: the last race's result page and the final standings can
// appear after 1 January, when "current year" has already rolled over.
func seasonsToProcess(now time.Time) []string {
	years := []string{strconv.Itoa(now.Year())}
	if now.Month() <= time.February {
		years = append(years, strconv.Itoa(now.Year()-1))
	}
	return years
}

// forEachSeason runs fn per season and joins the errors. A previous season that
// simply isn't in the database is skipped rather than reported as a failure.
func forEachSeason(years []string, fn func(year string) error) error {
	var errs []error
	for i, year := range years {
		if err := fn(year); err != nil {
			if i > 0 && errors.Is(err, scrapers.ErrSeasonNotFound) {
				log.Printf("Skipping previous season %s: %v", year, err)
				continue
			}
			errs = append(errs, fmt.Errorf("season %s: %w", year, err))
		}
	}
	return errors.Join(errs...)
}

func executeRunMode(ctx context.Context, db *gorm.DB, runMode string) error {
	now := time.Now()
	currentYear := strconv.Itoa(now.Year())
	seasons := seasonsToProcess(now)
	txMgr := repository.NewTransactionManager(db)

	switch runMode {
	case "scraper-race":
		raceScraper := scrapers.NewScrapedRaceService(
			"",
			&seasonRepoAdapter{repo: repository.NewSeasonRepository(db)},
			&raceRepoAdapter{repo: repository.NewRaceRepository(db)},
			&raceStatusAdapter{repo: repository.NewRaceStatusRepository(db)},
			&raceSeasonAdapter{repo: repository.NewRaceSeasonRepository(db)},
			txMgr,
			nil,
		)
		return raceScraper.UpdateRaceFromScraperForSeason(ctx, currentYear)

	case "scraper-driver-team":
		teamScraper := scrapers.NewScrapedTeamService(
			&teamRepoAdapter{repo: repository.NewTeamRepository(db)},
			nil,
		)
		return teamScraper.UpdateTeamsFromScraper(ctx)

	case "scraper-driver":
		// New, opt-in mode: this scraper existed but was never wired to any
		// RUN_MODE, so drivers were never created/updated automatically.
		driverScraper := scrapers.NewScrapedDriverService(
			&driverRepoAdapter{repo: repository.NewDriverRepository(db)},
			nil,
		)
		return driverScraper.UpdateDriversFromScraper(ctx)

	case "scraper-standing":
		standingScraper := scrapers.NewScrapedStandingService(
			&driverRepoAdapter{repo: repository.NewDriverRepository(db)},
			&seasonRepoAdapter{repo: repository.NewSeasonRepository(db)},
			&teamRepoAdapter{repo: repository.NewTeamRepository(db)},
			&dtsRepoAdapter{repo: repository.NewDriverTeamSeasonRepository(db)},
			txMgr,
			nil,
		)
		return forEachSeason(seasons, func(year string) error {
			return standingScraper.UpdateDriverAndPointsFromScraperForSeason(ctx, year)
		})

	case "scraper-race-result":
		resultScraper := scrapers.NewScrapedRaceResultService(
			"",
			&driverRepoAdapter{repo: repository.NewDriverRepository(db)},
			&raceRepoAdapter{repo: repository.NewRaceRepository(db)},
			&seasonRepoAdapter{repo: repository.NewSeasonRepository(db)},
			&resultRepoAdapter{repo: repository.NewDriverRaceResultRepository(db)},
			txMgr,
			nil,
		)
		return forEachSeason(seasons, func(year string) error {
			return resultScraper.UpdatePendingRaceResults(ctx, year)
		})

	default:
		return fmt.Errorf("unknown RUN_MODE: %s", runMode)
	}
}
