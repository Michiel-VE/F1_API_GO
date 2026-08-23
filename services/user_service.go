package services

import (
	"context"
	"f1-api-go/internal/models"
	"fmt"
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

func (s *UserService) LoadUserByUsername(ctx context.Context, email string) (*CustomUserPrincipal, error) {
	user, err := s.userRepository.FindByEmail(ctx, email)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user not found with email: %s", email)
	}

	password := ""
	if user.Password != nil {
		password = *user.Password
	}
	if password == "" {
		password = "{noop}oauth2user"
	}

	roleName := "USER"
	if user.Role != "" {
		roleName = string(user.Role)
	}

	return &CustomUserPrincipal{
		ID:       user.ID,
		Username: user.Email,
		Password: password,
		Role:     roleName,
	}, nil
}

func (s *UserService) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	return s.userRepository.FindByEmail(ctx, email)
}
