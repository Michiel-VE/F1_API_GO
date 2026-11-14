package repository

import (
	"context"

	"f1-api-go/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DriverRaceResultRepository struct {
	db *gorm.DB
}

func NewDriverRaceResultRepository(db *gorm.DB) *DriverRaceResultRepository {
	return &DriverRaceResultRepository{db: db}
}

func (r *DriverRaceResultRepository) ExistsByDriverAndRaceAndSeason(ctx context.Context, driverID, raceID, seasonID uuid.UUID) (bool, error) {
	var count int64
	err := DBFrom(ctx, r.db).Model(&models.DriverRaceResult{}).
		Where("driver_id = ? AND race_id = ? AND season_id = ?", driverID, raceID, seasonID).
		Count(&count).Error
	return count > 0, err
}

func (r *DriverRaceResultRepository) Save(ctx context.Context, result *models.DriverRaceResult) error {
	return DBFrom(ctx, r.db).Save(result).Error
}

func (r *DriverRaceResultRepository) FindByDriverIDAndSeason(ctx context.Context, driverID uuid.UUID, seasonName string) ([]models.DriverRaceResult, error) {
	var results []models.DriverRaceResult
	err := DBFrom(ctx, r.db).
		Joins("JOIN f1_api.season s ON s.id = f1_api.driver_race_result.season_id").
		Joins("JOIN f1_api.race r ON r.id = f1_api.driver_race_result.race_id").
		Preload("Driver").
		Preload("Race").
		Preload("Season").
		Where("f1_api.driver_race_result.driver_id = ? AND s.season_name = ?", driverID, seasonName).
		Order("r.race_start_date ASC").
		Find(&results).Error
	return results, err
}
