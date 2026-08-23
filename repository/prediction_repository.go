package repository

import (
	"errors"

	"f1-api-go/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PredictionRepository struct {
	db *gorm.DB
}

func NewPredictionRepository(db *gorm.DB) *PredictionRepository {
	return &PredictionRepository{db: db}
}

func (r *PredictionRepository) FindByUserIDAndRaceID(userID, raceID uuid.UUID) (*models.Prediction, error) {
	var prediction models.Prediction
	err := r.db.Where("user_id = ? AND race_id = ?", userID, raceID).First(&prediction).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &prediction, err
}

func (r *PredictionRepository) ExistsByUserIDAndRaceID(userID, raceID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.Model(&models.Prediction{}).
		Where("user_id = ? AND race_id = ?", userID, raceID).
		Count(&count).Error
	return count > 0, err
}