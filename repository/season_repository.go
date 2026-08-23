package repository

import (
	"errors"

	"f1-api-go/internal/models"
	"gorm.io/gorm"
)

type SeasonRepository struct {
	db *gorm.DB
}

func NewSeasonRepository(db *gorm.DB) *SeasonRepository {
	return &SeasonRepository{db: db}
}

func (r *SeasonRepository) FindBySeasonName(year string) (*models.Season, error) {
	var season models.Season
	err := r.db.Where("season_name = ?", year).First(&season).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &season, err
}