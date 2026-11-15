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
	err := DBFrom(ctx, r.db).Preload("Creator").Preload("Members").Where("id = ?", id).First(&pool).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &pool, nil
}

func (r *PoolRepository) FindByUserId(ctx context.Context, userID uuid.UUID) ([]*models.Pool, error) {
	var pools []*models.Pool
	err := DBFrom(ctx, r.db).
		Preload("Creator").
		Preload("Members").
		Joins("JOIN f1_api.pool_members pm ON pm.pool_id = pools.id").
		Where("pm.user_id = ?", userID).
		Find(&pools).Error
	return pools, err
}

func (r *PoolRepository) FindByInviteCode(ctx context.Context, inviteCode string) (*models.Pool, error) {
	var pool models.Pool
	err := DBFrom(ctx, r.db).Preload("Creator").Preload("Members").Where("invite_code = ?", inviteCode).First(&pool).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &pool, nil
}

// Save creates or updates the pool row. Do NOT use it to add or remove members:
// saving a pool only inserts missing member links, it never deletes links of
// members you removed from the slice (that is why "leave pool" and "kick member"
// used to leave the user in the pool). Use AddMember / RemoveMember for that.
func (r *PoolRepository) Save(ctx context.Context, pool *models.Pool) (*models.Pool, error) {
	err := DBFrom(ctx, r.db).Save(pool).Error
	return pool, err
}

// AddMember links a user to a pool. Adding an existing member is a no-op.
func (r *PoolRepository) AddMember(ctx context.Context, poolID, userID uuid.UUID) error {
	return DBFrom(ctx, r.db).Exec(`
		INSERT INTO f1_api.pool_members (pool_id, user_id)
		SELECT ?::uuid, ?::uuid
		WHERE NOT EXISTS (
			SELECT 1 FROM f1_api.pool_members WHERE pool_id = ?::uuid AND user_id = ?::uuid
		)`, poolID, userID, poolID, userID).Error
}

// RemoveMember deletes the membership link.
func (r *PoolRepository) RemoveMember(ctx context.Context, poolID, userID uuid.UUID) error {
	return DBFrom(ctx, r.db).Exec(
		`DELETE FROM f1_api.pool_members WHERE pool_id = ?::uuid AND user_id = ?::uuid`,
		poolID, userID).Error
}

// Delete removes a pool together with its membership links. gorm does not do the
// links itself, so without this the delete fails (or leaves orphans) when the
// database has no ON DELETE CASCADE.
func (r *PoolRepository) Delete(ctx context.Context, pool *models.Pool) error {
	db := DBFrom(ctx, r.db)
	if err := db.Exec(`DELETE FROM f1_api.pool_members WHERE pool_id = ?::uuid`, pool.ID).Error; err != nil {
		return err
	}
	return db.Delete(pool).Error
}
