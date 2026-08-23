package repository

import (
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

func (r *RaceStatusRepository) FindByName(name string) (*models.RaceStatus, error) {
	var status models.RaceStatus
	err := r.db.Where("name = ?", name).First(&status).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &status, err
}