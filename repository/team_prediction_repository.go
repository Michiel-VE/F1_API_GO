package repository

import (
	"context"
	"errors"

	"f1-api-go/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TeamPredictionRepository struct {
	db *gorm.DB
}

func NewTeamPredictionRepository(db *gorm.DB) *TeamPredictionRepository {
	return &TeamPredictionRepository{db: db}
}

func (r *TeamPredictionRepository) FindPersonalByUserId(ctx context.Context, userID uuid.UUID) (*models.TeamPrediction, error) {
	var prediction models.TeamPrediction
	err := r.db.WithContext(ctx).Preload("PredictedTeams").
		Where("user_id = ? AND pool_id IS NULL", userID).
		First(&prediction).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &prediction, err
}

func (r *TeamPredictionRepository) FindByUserIdAndPoolId(ctx context.Context, userID, poolID uuid.UUID) (*models.TeamPrediction, error) {
	var prediction models.TeamPrediction
	err := r.db.WithContext(ctx).Preload("PredictedTeams").
		Where("user_id = ? AND pool_id = ?", userID, poolID).
		First(&prediction).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &prediction, err
}

func (r *TeamPredictionRepository) FindByPoolId(ctx context.Context, poolID uuid.UUID) ([]*models.TeamPrediction, error) {
	var predictions []*models.TeamPrediction
	err := r.db.WithContext(ctx).Preload("PredictedTeams").Preload("User").Where("pool_id = ?", poolID).Find(&predictions).Error
	return predictions, err
}

func (r *TeamPredictionRepository) Save(ctx context.Context, prediction *models.TeamPrediction) error {
	return r.db.WithContext(ctx).Save(prediction).Error
}

func (r *TeamPredictionRepository) Delete(ctx context.Context, prediction *models.TeamPrediction) error {
	return r.db.WithContext(ctx).Delete(prediction).Error
}

func (r *TeamPredictionRepository) DeleteByPoolId(ctx context.Context, poolID uuid.UUID) error {
	return r.db.WithContext(ctx).Where("pool_id = ?", poolID).Delete(&models.TeamPrediction{}).Error
}
