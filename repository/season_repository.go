package repository

import (
	"context"
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

// FindBySeasonName returns (nil, nil) when the season does not exist and a
// non-nil error only for real database failures.
func (r *SeasonRepository) FindBySeasonName(ctx context.Context, year string) (*models.Season, error) {
	var season models.Season
	err := DBFrom(ctx, r.db).Where("season_name = ?", year).First(&season).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &season, nil
}

func (r *SeasonRepository) Save(ctx context.Context, season *models.Season) error {
	return DBFrom(ctx, r.db).Save(season).Error
}
