package services

import (
	"context"

	"f1-api-go/internal/models"
)

type UserRepository interface {
	FindByEmail(ctx context.Context, email string) (*models.User, error)
}

type UserService struct {
	userRepository UserRepository
}

func NewUserService(userRepo UserRepository) *UserService {
	return &UserService{
		userRepository: userRepo,
	}
}

func (s *UserService) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	return s.userRepository.FindByEmail(ctx, email)
}
