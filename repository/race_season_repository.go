package repository

import (
	"context"

	"f1-api-go/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RaceSeasonRepository struct {
	db *gorm.DB
}

func NewRaceSeasonRepository(db *gorm.DB) *RaceSeasonRepository {
	return &RaceSeasonRepository{db: db}
}

func (r *RaceSeasonRepository) ExistsByRaceAndSeason(ctx context.Context, raceID, seasonID uuid.UUID) (bool, error) {
	var count int64
	err := DBFrom(ctx, r.db).Model(&models.RaceSeason{}).
		Where("race_id = ? AND season_id = ?", raceID, seasonID).
		Count(&count).Error
	return count > 0, err
}

func (r *RaceSeasonRepository) Save(ctx context.Context, raceSeason *models.RaceSeason) error {
	return DBFrom(ctx, r.db).Save(raceSeason).Error
}
