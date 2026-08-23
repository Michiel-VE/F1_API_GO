package repository

import (
	"context"
	"errors"

	"f1-api-go/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PoolRepository struct {
	db *gorm.DB
}

func NewPoolRepository(db *gorm.DB) *PoolRepository {
	return &PoolRepository{db: db}
}

func (r *PoolRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.Pool, error) {
	var pool models.Pool
	err := r.db.WithContext(ctx).Preload("Creator").Preload("Members").Where("id = ?", id).First(&pool).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &pool, err
}

func (r *PoolRepository) FindByUserId(ctx context.Context, userID uuid.UUID) ([]*models.Pool, error) {
	var pools []*models.Pool
	err := r.db.WithContext(ctx).
		Preload("Creator").
		Preload("Members").
		Joins("JOIN f1_api.pool_members pm ON pm.pool_id = pools.id").
		Where("pm.user_id = ?", userID).
		Find(&pools).Error
	return pools, err
}

func (r *PoolRepository) FindByInviteCode(ctx context.Context, inviteCode string) (*models.Pool, error) {
	var pool models.Pool
	err := r.db.WithContext(ctx).Preload("Creator").Preload("Members").Where("invite_code = ?", inviteCode).First(&pool).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &pool, err
}

func (r *PoolRepository) Save(ctx context.Context, pool *models.Pool) (*models.Pool, error) {
	err := r.db.WithContext(ctx).Save(pool).Error
	return pool, err
}

func (r *PoolRepository) Delete(ctx context.Context, pool *models.Pool) error {
	return r.db.WithContext(ctx).Delete(pool).Error
}
