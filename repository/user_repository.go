package repository

import (
	"context"
	"errors"
	"time"

	"f1-api-go/internal/domain"
	"f1-api-go/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// FindByProviderAndProviderID keeps its original (context-less) signature
// because it is probably called from OAuth code I haven't seen.
func (r *UserRepository) FindByProviderAndProviderID(provider, providerID string) (*models.User, error) {
	var user models.User
	err := r.db.Where("provider = ? AND provider_id = ?", provider, providerID).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := DBFrom(ctx, r.db).Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	err := DBFrom(ctx, r.db).Where("id = ?", id).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	return DBFrom(ctx, r.db).Create(user).Error
}

// LinkOAuthProvider records which OAuth account belongs to an existing user.
func (r *UserRepository) LinkOAuthProvider(ctx context.Context, id uuid.UUID, provider, providerID, picture string) error {
	updates := map[string]any{"provider": provider, "provider_id": providerID}
	if picture != "" {
		updates["picture"] = picture
	}
	return DBFrom(ctx, r.db).Model(&models.User{}).Where("id = ?", id).Updates(updates).Error
}

// CreateOAuthUser inserts a user that signed in through an OAuth provider (no password).
// It writes by column name so it does not depend on the exact Go types of the
// provider fields in models.User. If an insert fails with "column ... does not
// exist", adjust the column names below to match your table.
func (r *UserRepository) CreateOAuthUser(ctx context.Context, name, email, provider, providerID, picture string) (*models.User, error) {
	id := uuid.New()
	err := DBFrom(ctx, r.db).Model(&models.User{}).Create(map[string]any{
		"id":          id,
		"name":        name,
		"email":       email,
		"role":        string(domain.RoleUser),
		"provider":    provider,
		"provider_id": providerID,
		"picture":     picture,
		"created_at":  time.Now(),
	}).Error
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}
