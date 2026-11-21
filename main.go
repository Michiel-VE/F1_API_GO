package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"f1-api-go/alerts"
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
		IsSprint:      m.IsSprint,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}

func (a *raceRepoAdapter) FindAllFinishedRacesMissingResults(ctx context.Context, now time.Time, season string) ([]models.Race, error) {
	return a.repo.FindAllFinishedRacesMissingResults(ctx, now, season)
}

func (a *raceRepoAdapter) FindFirstFinishedRaceMissingResults(ctx context.Context, now time.Time, season string) (*models.Race, error) {
	return a.repo.FindFirstFinishedRaceMissingResults(ctx, now, season)
}

func (a *raceRepoAdapter) FindCompletedNonSprintRaces(ctx context.Context, now time.Time, season string) ([]models.Race, error) {
	return a.repo.FindCompletedNonSprintRaces(ctx, now, season)
}

func (a *raceRepoAdapter) FindByName(ctx context.Context, name string) (*scrapers.Race, error) {
	m, err := a.repo.FindByName(ctx, name)
	if err != nil || m == nil {
		return nil, err
	}
	return toScraperRace(m), nil
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

func (a *raceRepoAdapter) MarkCancelledRaces(ctx context.Context, year string) error {
	return a.repo.MarkCancelledRaces(ctx, year)
}

func (a *raceRepoAdapter) Save(ctx context.Context, r *scrapers.Race) (*scrapers.Race, error) {
	m := &models.Race{
		ID:            r.ID,
		Name:          r.Name,
		Country:       r.Country,
		RaceStartDate: r.RaceStartDate,
		RaceEndDate:   r.RaceEndDate,
		IsSprint:      r.IsSprint,
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

func toScraperTeam(t models.Team) *scrapers.Team {
	return &scrapers.Team{
		ID:        t.ID,
		Name:      t.Name,
		ShortName: t.ShortName,
		Country:   t.Country,
		Base:      t.Base,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

func toScraperTeams(teams []models.Team) []*scrapers.Team {
	res := make([]*scrapers.Team, 0, len(teams))
	for i := range teams {
		res = append(res, toScraperTeam(teams[i]))
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
	repo     *repository.DriverTeamSeasonRepository
	teamRepo *repository.TeamRepository
}

func (a *dtsRepoAdapter) resolveTeam(ctx context.Context, teamID uuid.UUID, fallback *scrapers.Team) (*scrapers.Team, error) {
	if fallback != nil && fallback.ID == teamID {
		return fallback, nil
	}
	if a.teamRepo == nil {
		return &scrapers.Team{ID: teamID}, nil
	}
	allTeams, err := a.teamRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	for i := range allTeams {
		if allTeams[i].ID == teamID {
			return toScraperTeam(allTeams[i]), nil
		}
	}
	return &scrapers.Team{ID: teamID}, nil
}

func (a *dtsRepoAdapter) FindByDriverAndSeasonAndTeam(ctx context.Context, driver *models.Driver, season *scrapers.Season, team *scrapers.Team) (*scrapers.DriverTeamSeason, error) {
	m, err := a.repo.FindByDriverAndSeasonAndTeam(ctx, driver.ID, season.ID, team.ID)
	if err != nil || m == nil {
		return nil, err
	}
	resolvedTeam, err := a.resolveTeam(ctx, m.TeamID, team)
	if err != nil {
		return nil, err
	}
	return &scrapers.DriverTeamSeason{
		ID:              m.ID,
		Driver:          driver,
		Season:          season,
		Team:            resolvedTeam,
		Points:          m.Points,
		SeasonTeamName:  m.SeasonTeamName,
		SeasonShortName: m.SeasonShortName,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}, nil
}

func (a *dtsRepoAdapter) FindByDriverAndSeason(ctx context.Context, driver *models.Driver, season *scrapers.Season) (*scrapers.DriverTeamSeason, error) {
	m, err := a.repo.FindByDriverAndSeason(ctx, driver.ID, season.ID)
	if err != nil || m == nil {
		return nil, err
	}
	resolvedTeam, err := a.resolveTeam(ctx, m.TeamID, nil)
	if err != nil {
		return nil, err
	}
	return &scrapers.DriverTeamSeason{
		ID:              m.ID,
		Driver:          driver,
		Season:          season,
		Team:            resolvedTeam,
		Points:          m.Points,
		SeasonTeamName:  m.SeasonTeamName,
		SeasonShortName: m.SeasonShortName,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
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
		ID:              dts.ID,
		DriverID:        dts.Driver.ID,
		SeasonID:        dts.Season.ID,
		TeamID:          dts.Team.ID,
		Points:          dts.Points,
		SeasonTeamName:  dts.SeasonTeamName,
		SeasonShortName: dts.SeasonShortName,
		CreatedAt:       dts.CreatedAt,
		UpdatedAt:       dts.UpdatedAt,
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
		SprintPoints:  res.SprintPoints,
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

func (a *driverRepoAdapter) FindAll(ctx context.Context) ([]models.Driver, error) {
	return a.repo.FindAll(ctx)
}

func (a *driverRepoAdapter) Save(ctx context.Context, driver *models.Driver) error {
	return a.repo.Save(ctx, driver)
}

// ---------------------------------------------------------------------------

func openDB() (*gorm.DB, error) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  config.AppCfg.DatabaseURL,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		PrepareStmt:    false,
		TranslateError: true,
	})
	if err != nil {
		return nil, err
	}
	if err := config.ApplyDBPool(db); err != nil {
		return nil, err
	}
	return db, nil
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

func newAlertNotifier() *alerts.Notifier {
	var recipients []string
	for _, part := range strings.Split(config.AppCfg.AlertEmailTo, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			recipients = append(recipients, trimmed)
		}
	}

	return alerts.NewNotifier(alerts.Config{
		APIKey: strings.TrimSpace(config.AppCfg.ResendAPIKey),
		From:   strings.TrimSpace(config.AppCfg.AlertEmailFrom),
		To:     recipients,
	})
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
		if redisErr != nil {
			log.Printf("WARN: Redis unavailable, caches will not be evicted after scraping: %v", redisErr)
			redisClient = nil
		}

		notifier := newAlertNotifier()

		if os.Getenv("AWS_EXECUTION_ENV") != "" {
			http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
				log.Printf("Executing scraper task for RUN_MODE: %s", runMode)
				if err := runScraper(r.Context(), db, redisClient, notifier, runMode); err != nil {
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

		if err := runScraper(context.Background(), db, redisClient, notifier, runMode); err != nil {
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
	seasonRepo := repository.NewSeasonRepository(db)
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
		seasonRepo,
		txManager,
	)

	jwtService, err := services.NewJWTService(config.AppCfg.JWTSecretKey, config.AppCfg.JwtExpiration.Milliseconds())
	if err != nil {
		log.Fatalf("Failed to initialize JWT service: %v", err)
	}
	authService := services.NewAuthService(userRepo, jwtService)
	oauthService := services.NewOAuthService(userRepo)

	authController := controller.NewAuthController(authService, jwtService)
	profileController := controller.NewProfileController(userService)
	driverController := controller.NewDriverController(driverService, driverRaceResultsService)
	teamController := controller.NewTeamController(teamService)
	raceController := controller.NewRaceController(raceService)
	predictionController := controller.NewPredictionController(predictionService)
	oauthController := controller.NewOAuthController(config.AppCfg.GoogleOAuth, config.AppCfg.GoogleUserInfoURI, oauthService, authService)

	r := config.SecurityConfig(
		authController,
		profileController,
		driverController,
		teamController,
		raceController,
		predictionController,
		jwtService,
		oauthController,
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

var cachesToEvict = map[string][]string{
	"scraper-race":        {"races", "races_hist"},
	"scraper-driver-team": {"teams", "teams_hist"},
	"scraper-driver":      {"drivers", "drivers_hist"},
	"scraper-standing":    {"drivers", "drivers_hist", "teams", "teams_hist"},
	"scraper-race-result": {"results", "results_hist"},
}

func runScraper(ctx context.Context, db *gorm.DB, redisClient *redis.Client, notifier *alerts.Notifier, runMode string) error {
	runErr := executeRunMode(ctx, db, runMode)

	if runErr != nil && notifier != nil {
		alertCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if notifyErr := notifier.NotifyScraperFailure(alertCtx, runMode, os.Getenv("RUN_YEAR"), runErr); notifyErr != nil {
			log.Printf("WARN: Failed to send scraper failure alert: %v", notifyErr)
		}
	}

	if redisClient != nil {
		if names := cachesToEvict[runMode]; len(names) > 0 {
			evictCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			services.NewCacheEvictionService(redisClient).EvictAndStamp(evictCtx, names...)
		}
	}
	return runErr
}

func seasonsToProcess(now time.Time, targetYear string) []string {
	if targetYear != "" {
		return []string{targetYear}
	}
	years := []string{strconv.Itoa(now.Year())}
	if now.Month() <= time.February {
		years = append(years, strconv.Itoa(now.Year()-1))
	}
	return years
}

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

	// Read RUN_YEAR if explicitly provided for local backfilling; otherwise fallback to current year
	targetYear := os.Getenv("RUN_YEAR")
	if targetYear == "" {
		targetYear = strconv.Itoa(now.Year())
	}

	backfillMode := os.Getenv("BACKFILL") == "true"
	seasons := seasonsToProcess(now, os.Getenv("RUN_YEAR"))
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
		return raceScraper.UpdateRaceFromScraperForSeason(ctx, targetYear)

	case "scraper-driver-team":
		teamScraper := scrapers.NewScrapedTeamService(
			&teamRepoAdapter{repo: repository.NewTeamRepository(db)},
			nil,
		)
		return teamScraper.UpdateTeamsFromScraper(ctx)

	case "scraper-driver":
		driverScraper := scrapers.NewScrapedDriverServiceWithOptions(
			&driverRepoAdapter{repo: repository.NewDriverRepository(db)},
			backfillMode,
			targetYear,
			nil,
		)
		return driverScraper.UpdateDriversFromScraper(ctx)

	case "scraper-standing":
		standingScraper := scrapers.NewScrapedStandingServiceWithOptions(
			&driverRepoAdapter{repo: repository.NewDriverRepository(db)},
			&seasonRepoAdapter{repo: repository.NewSeasonRepository(db)},
			&teamRepoAdapter{repo: repository.NewTeamRepository(db)},
			&dtsRepoAdapter{
				repo:     repository.NewDriverTeamSeasonRepository(db),
				teamRepo: repository.NewTeamRepository(db),
			},
			txMgr,
			backfillMode,
			targetYear,
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
