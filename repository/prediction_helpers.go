package repository

import (
	"context"

	"f1-api-go/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PredictionTeamRepository struct {
	db *gorm.DB
}

func NewPredictionTeamRepository(db *gorm.DB) *PredictionTeamRepository {
	return &PredictionTeamRepository{db: db}
}

func (r *PredictionTeamRepository) FindAllById(ctx context.Context, ids []uuid.UUID) ([]*models.Team, error) {
	var teams []*models.Team
	err := DBFrom(ctx, r.db).Where("id IN ?", ids).Find(&teams).Error
	return teams, err
}
