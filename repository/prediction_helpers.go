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
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&teams).Error
	return teams, err
}

type TransactionManager struct {
	db *gorm.DB
}

func NewTransactionManager(db *gorm.DB) *TransactionManager {
	return &TransactionManager{db: db}
}

func (tm *TransactionManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tm.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(ctx)
	})
}
