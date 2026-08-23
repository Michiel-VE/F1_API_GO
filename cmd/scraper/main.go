package main

import (
	"context"
	"log"

	"f1-api-go/config"
	"f1-api-go/repository"
	"f1-api-go/services/scrapers"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	config.InitConfig()

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
	log.Println("Successfully connected to database for scraping job.")

	ctx := context.Background()
	driverRepo := repository.NewDriverRepository(db)
	driverScraper := scrapers.NewScrapedDriverService(driverRepo, nil)

	if err := driverScraper.UpdateDriversFromScraper(ctx); err != nil {
		log.Fatalf("Failed to update drivers: %v", err)
	}

	log.Println("Driver scraper job completed successfully.")
}
