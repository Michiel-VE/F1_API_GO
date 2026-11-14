package repository

import (
	"context"
	"errors"

	"f1-api-go/internal/models"

	"gorm.io/gorm"
)

type RaceStatusRepository struct {
	db *gorm.DB
}

func NewRaceStatusRepository(db *gorm.DB) *RaceStatusRepository {
	return &RaceStatusRepository{db: db}
}

func (r *RaceStatusRepository) FindByName(ctx context.Context, name string) (*models.RaceStatus, error) {
	var status models.RaceStatus
	err := DBFrom(ctx, r.db).Where("name = ?", name).First(&status).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &status, nil
}
