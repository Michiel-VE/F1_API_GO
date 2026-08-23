package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
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

type seasonRepoAdapter struct {
	repo *repository.SeasonRepository
	db   *gorm.DB
}

func (a *seasonRepoAdapter) FindBySeasonName(ctx context.Context, name string) (*scrapers.Season, error) {
	m, err := a.repo.FindBySeasonName(name)
	if err != nil || m == nil {
		return nil, err
	}
	return &scrapers.Season{ID: m.ID, SeasonName: m.SeasonName, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}, nil
}
func (a *seasonRepoAdapter) Save(ctx context.Context, s *scrapers.Season) (*scrapers.Season, error) {
	m := &models.Season{ID: s.ID, SeasonName: s.SeasonName, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt}
	err := a.db.WithContext(ctx).Save(m).Error
	return s, err
}

type raceRepoAdapter struct {
	repo *repository.RaceRepository
	db   *gorm.DB
}

func (a *raceRepoAdapter) FindAllFinishedRacesMissingResults(ctx context.Context, now time.Time, season string) ([]models.Race, error) {
	return a.repo.FindAllFinishedRacesMissingResults(now, season)
}

func (a *raceRepoAdapter) FindFirstFinishedRaceMissingResults(ctx context.Context, now time.Time, season string) (*scrapers.Race, error) {
	m, err := a.repo.FindFirstFinishedRaceMissingResults(now, season)
	if err != nil || m == nil {
		return nil, err
	}
	return &scrapers.Race{ID: m.ID, Name: m.Name, Country: m.Country, RaceStartDate: m.RaceStartDate, RaceEndDate: m.RaceEndDate}, nil
}
func (a *raceRepoAdapter) FindByNameAndRaceStartDateAndRaceEndDate(ctx context.Context, name string, start, end time.Time) (*scrapers.Race, error) {
	m, err := a.repo.FindByNameAndRaceStartDateAndRaceEndDate(name, start, end)
	if err != nil || m == nil {
		return nil, err
	}
	return &scrapers.Race{ID: m.ID, Name: m.Name, Country: m.Country, RaceStartDate: m.RaceStartDate, RaceEndDate: m.RaceEndDate}, nil
}
func (a *raceRepoAdapter) UpdatePastScheduledRacesToCompleted(ctx context.Context, now time.Time, season string) error {
	return a.repo.UpdatePastScheduledRacesToCompleted(now, season)
}
func (a *raceRepoAdapter) UpdateExternalResultURL(raceID uuid.UUID, url string) error {
	return nil
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
	err := a.db.WithContext(ctx).Save(m).Error
	if err != nil {
		return nil, err
	}
	r.ID = m.ID
	return r, nil
}

type raceStatusAdapter struct {
	repo *repository.RaceStatusRepository
}

func (a *raceStatusAdapter) FindByName(ctx context.Context, name string) (*scrapers.RaceStatus, error) {
	m, err := a.repo.FindByName(name)
	if err != nil || m == nil {
		return nil, err
	}
	return &scrapers.RaceStatus{ID: m.ID, Name: m.Name}, nil
}

type raceSeasonAdapter struct {
	repo *repository.RaceSeasonRepository
	db   *gorm.DB
}

func (a *raceSeasonAdapter) ExistsByRaceAndSeason(ctx context.Context, r *scrapers.Race, s *scrapers.Season) (bool, error) {
	return a.repo.ExistsByRaceAndSeason(r.ID, s.ID)
}
func (a *raceSeasonAdapter) Save(ctx context.Context, rs *scrapers.RaceSeason) error {
	m := &models.RaceSeason{
		ID:        rs.ID,
		RaceID:    rs.Race.ID,
		SeasonID:  rs.Season.ID,
		CreatedAt: rs.CreatedAt,
		UpdatedAt: rs.UpdatedAt,
	}
	return a.db.WithContext(ctx).Save(m).Error
}

type teamRepoAdapter struct {
	repo   *repository.TeamRepository
	db     *gorm.DB
	season string
}

func (a *teamRepoAdapter) FindAll(ctx context.Context) ([]*scrapers.Team, error) {
	teams, err := a.repo.FindAllTeamsBySeasonName(a.season)
	if err != nil {
		return nil, err
	}
	var res []*scrapers.Team
	for _, t := range teams {
		tCopy := t
		res = append(res, &scrapers.Team{ID: tCopy.ID, Name: tCopy.Name, ShortName: tCopy.ShortName, Country: tCopy.Country, Base: tCopy.Base})
	}
	return res, nil
}
func (a *teamRepoAdapter) FindAllByShortName(ctx context.Context, shortName string) ([]*scrapers.Team, error) {
	teams, err := a.repo.FindAllByShortName(shortName)
	if err != nil {
		return nil, err
	}
	var res []*scrapers.Team
	for _, t := range teams {
		tCopy := t
		res = append(res, &scrapers.Team{ID: tCopy.ID, Name: tCopy.Name, ShortName: tCopy.ShortName, Country: tCopy.Country, Base: tCopy.Base})
	}
	return res, nil
}
func (a *teamRepoAdapter) Save(ctx context.Context, t *scrapers.Team) error {
	m := &models.Team{
		ID:        t.ID,
		Name:      t.Name,
		ShortName: t.ShortName,
		Country:   t.Country,
		Base:      t.Base,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
	return a.db.WithContext(ctx).Save(m).Error
}
func (a *teamRepoAdapter) Delete(ctx context.Context, t *scrapers.Team) error {
	m := &models.Team{ID: t.ID}
	return a.db.WithContext(ctx).Delete(m).Error
}

type dtsRepoAdapter struct {
	repo *repository.DriverTeamSeasonRepository
	db   *gorm.DB
}

func (a *dtsRepoAdapter) FindByDriverAndSeasonAndTeam(ctx context.Context, driver *models.Driver, season *scrapers.Season, team *scrapers.Team) (*scrapers.DriverTeamSeason, error) {
	m, err := a.repo.FindByDriverAndSeasonAndTeam(driver.ID, season.ID, team.ID)
	if err != nil || m == nil {
		return nil, err
	}
	return &scrapers.DriverTeamSeason{
		ID:     m.ID,
		Driver: driver,
		Season: season,
		Team:   team,
		Points: m.Points,
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
	err := a.db.WithContext(ctx).Save(m).Error
	return dts, err
}

type resultRepoAdapter struct {
	repo *repository.DriverRaceResultRepository
	db   *gorm.DB
}

func (a *resultRepoAdapter) ExistsByDriverAndRaceAndSeason(ctx context.Context, driver *models.Driver, race *scrapers.Race, season *scrapers.Season) (bool, error) {
	return a.repo.ExistsByDriverAndRaceAndSeason(driver.ID, race.ID, season.ID)
}
func (a *resultRepoAdapter) Save(ctx context.Context, res *scrapers.DriverRaceResult) error {
	m := &models.DriverRaceResult{
		ID:            res.ID,
		DriverID:      res.Driver.ID,
		RaceID:        res.Race.ID,
		SeasonID:      res.Season.ID,
		Points:        res.Points,
		LapsCompleted: &res.LapsCompleted,
		Status:        &res.Status,
		CreatedAt:     res.CreatedAt,
		UpdatedAt:     res.UpdatedAt,
	}
	return a.db.WithContext(ctx).Save(m).Error
}

type driverRepoAdapter struct {
	repo *repository.DriverRepository
	db   *gorm.DB
}

func (a *driverRepoAdapter) FindByLastNameIgnoreCase(lastname string) (*models.Driver, error) {
	return a.repo.FindByLastNameIgnoreCase(lastname)
}
func (a *driverRepoAdapter) FindByFirstNameIgnoreCase(firstname string) (*models.Driver, error) {
	return a.repo.FindByFirstNameIgnoreCase(firstname)
}
func (a *driverRepoAdapter) FindByFirstNameAndLastNameIgnoreCase(firstname, lastname string) (*models.Driver, error) {
	return a.repo.FindByFirstNameAndLastNameIgnoreCase(firstname, lastname)
}
func (a *driverRepoAdapter) FindByDriverCode(ctx context.Context, driverCode string) (*models.Driver, error) {
	return a.repo.FindByDriverCode(driverCode)
}
func (a *driverRepoAdapter) Save(ctx context.Context, driver *models.Driver) error {
	return a.repo.Save(ctx, driver)
}

type txManagerAdapter struct{ db *gorm.DB }

func (tm *txManagerAdapter) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tm.db.Transaction(func(tx *gorm.DB) error {
		return fn(ctx)
	})
}

func main() {
	config.InitConfig()

	runMode := os.Getenv("RUN_MODE")
	if runMode != "" {
		dsn := config.AppCfg.DatabaseURL
		db, err := gorm.Open(postgres.New(postgres.Config{
			DSN:                  dsn,
			PreferSimpleProtocol: true,
		}), &gorm.Config{
			PrepareStmt: false,
		})
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}

		if os.Getenv("AWS_EXECUTION_ENV") != "" {
			http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
				log.Printf("Executing scraper task for RUN_MODE: %s", runMode)
				if err := executeRunMode(r.Context(), db, runMode); err != nil {
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

		ctx := context.Background()
		if err := executeRunMode(ctx, db, runMode); err != nil {
			log.Fatalf("Failed execution: %v", err)
		}
		return
	}

	dsn := config.AppCfg.DatabaseURL
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		PrepareStmt: false,
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	var redisClient *redis.Client
	if config.AppCfg.RedisURL != "" {
		opt, err := redis.ParseURL(config.AppCfg.RedisURL)
		if err != nil {
			log.Fatalf("Failed to parse Redis URL: %v", err)
		}
		redisClient = redis.NewClient(opt)
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

func executeRunMode(ctx context.Context, db *gorm.DB, runMode string) error {
	currentYear := fmt.Sprintf("%d", time.Now().Year())
	txMgr := &txManagerAdapter{db: db}

	switch runMode {
	case "scraper-race":
		rRepo := repository.NewRaceRepository(db)
		sRepo := repository.NewSeasonRepository(db)
		stRepo := repository.NewRaceStatusRepository(db)
		rsRepo := repository.NewRaceSeasonRepository(db)

		raceScraper := scrapers.NewScrapedRaceService(
			"",
			&seasonRepoAdapter{repo: sRepo, db: db},
			&raceRepoAdapter{repo: rRepo, db: db},
			&raceStatusAdapter{repo: stRepo},
			&raceSeasonAdapter{repo: rsRepo, db: db},
			txMgr,
			nil,
		)
		return raceScraper.UpdateRaceFromScraperForSeason(ctx, currentYear)

	case "scraper-driver-team":
		tRepo := repository.NewTeamRepository(db)
		teamScraper := scrapers.NewScrapedTeamService(&teamRepoAdapter{repo: tRepo, db: db, season: currentYear}, nil)
		return teamScraper.UpdateTeamsFromScraper(ctx)

	case "scraper-standing":
		dRepo := repository.NewDriverRepository(db)
		sRepo := repository.NewSeasonRepository(db)
		tRepo := repository.NewTeamRepository(db)
		dtsRepo := repository.NewDriverTeamSeasonRepository(db)

		standingScraper := scrapers.NewScrapedStandingService(
			&driverRepoAdapter{repo: dRepo, db: db},
			&seasonRepoAdapter{repo: sRepo, db: db},
			&teamRepoAdapter{repo: tRepo, db: db, season: currentYear},
			&dtsRepoAdapter{repo: dtsRepo, db: db},
			txMgr,
			nil,
		)
		return standingScraper.UpdateDriverAndPointsFromScraperForSeason(ctx, currentYear)

	case "scraper-race-result":
		dRepo := repository.NewDriverRepository(db)
		rRepo := repository.NewRaceRepository(db)
		sRepo := repository.NewSeasonRepository(db)
		resRepo := repository.NewDriverRaceResultRepository(db)

		resultScraper := scrapers.NewScrapedRaceResultService(
			"",
			&driverRepoAdapter{repo: dRepo, db: db},
			&raceRepoAdapter{repo: rRepo, db: db},
			&seasonRepoAdapter{repo: sRepo, db: db},
			&resultRepoAdapter{repo: resRepo, db: db},
			txMgr,
			nil,
		)
		return resultScraper.UpdatePendingRaceResults(ctx, currentYear)

	default:
		return fmt.Errorf("unknown RUN_MODE: %s", runMode)
	}
}
