package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"f1-api-go/config"
	"f1-api-go/controller"
	"f1-api-go/repository"
	"f1-api-go/services"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// 1. Initialize configuration
	config.InitConfig()

	// 2. Database Connection with simple protocol to prevent prepared statement errors on poolers
	dsn := config.AppCfg.DatabaseURL
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true, // Disables binary prepared statements to avoid SQLSTATE 42P05
	}), &gorm.Config{
		PrepareStmt: false,
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("Successfully connected to the database!")

	// 3. Redis Client Initialization
	var redisClient *redis.Client
	if config.AppCfg.RedisURL != "" {
		opt, err := redis.ParseURL(config.AppCfg.RedisURL)
		if err != nil {
			log.Fatalf("Failed to parse Redis URL: %v", err)
		}
		redisClient = redis.NewClient(opt)
	}

	// 4. Initialize repositories & managers
	driverRepo := repository.NewDriverRepository(db)
	teamRepo := repository.NewTeamRepository(db)
	raceRepo := repository.NewRaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	driverTeamSeasonRepo := repository.NewDriverTeamSeasonRepository(db)

	poolRepo := repository.NewPoolRepository(db)
	teamPredictionRepo := repository.NewTeamPredictionRepository(db)
	predictionTeamRepo := repository.NewPredictionTeamRepository(db)
	txManager := repository.NewTransactionManager(db)

	// 5. Initialize services
	driverService := services.NewDriverService(driverRepo, redisClient, config.AppCfg.CacheEnabled)
	teamService := services.NewTeamService(teamRepo, driverTeamSeasonRepo, redisClient, config.AppCfg.CacheEnabled)
	raceService := services.NewRaceService(raceRepo, redisClient, config.AppCfg.CacheEnabled)
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

	// 6. Initialize controllers
	authController := controller.NewAuthController(authService, jwtService)
	profileController := controller.NewProfileController(userService)
	driverController := controller.NewDriverController(driverService, nil)
	teamController := controller.NewTeamController(teamService)
	raceController := controller.NewRaceController(raceService)
	predictionController := controller.NewPredictionController(predictionService)

	// 7. Setup Router using SecurityConfig with all controllers passed
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

	// 8. Configure and start the HTTP server using config port
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
