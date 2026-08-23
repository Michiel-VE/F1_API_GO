package repository

import (
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

func (r *RaceSeasonRepository) ExistsByRaceAndSeason(raceID, seasonID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.Model(&models.RaceSeason{}).
		Where("race_id = ? AND season_id = ?", raceID, seasonID).
		Count(&count).Error
	return count > 0, err
}