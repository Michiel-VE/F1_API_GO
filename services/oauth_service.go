package services

import (
	"context"
	"errors"
	"fmt"
	"log"

	"f1-api-go/internal/models"
	"f1-api-go/repository"
)

const providerGoogle = "google"

// ErrAccountExists is returned when a Google sign-in uses an email that already
// belongs to a password account. We do not silently merge them: anyone could
// have registered that email with a password first (this app does not verify
// emails), and would keep access to the account after the real owner signs in
// with Google.
var ErrAccountExists = errors.New("an account with this email already exists")

type GoogleProfile struct {
	Subject string
	Email   string
	Name    string
	Picture string
}

type OAuthService struct {
	userRepository *repository.UserRepository
}

func NewOAuthService(userRepo *repository.UserRepository) *OAuthService {
	return &OAuthService{userRepository: userRepo}
}

// FindOrCreateGoogleUser resolves a verified Google profile to a user:
//  1. same Google account as before -> that user
//  2. same email, account has no password (created through Google earlier,
//     for example by the previous Spring app) -> that user, now linked
//  3. same email, account has a password -> ErrAccountExists
//  4. otherwise -> a new user
func (s *OAuthService) FindOrCreateGoogleUser(ctx context.Context, p GoogleProfile) (*models.User, error) {
	if p.Subject == "" || p.Email == "" {
		return nil, errors.New("incomplete google profile")
	}

	user, err := s.userRepository.FindByProviderAndProviderID(providerGoogle, p.Subject)
	if err != nil {
		return nil, fmt.Errorf("provider lookup failed: %w", err)
	}
	if user != nil {
		return user, nil
	}

	existing, err := s.userRepository.FindByEmail(ctx, p.Email)
	if err != nil {
		return nil, fmt.Errorf("email lookup failed: %w", err)
	}
	if existing != nil {
		if existing.Password != nil && *existing.Password != "" {
			return nil, ErrAccountExists
		}
		if err := s.userRepository.LinkOAuthProvider(ctx, existing.ID, providerGoogle, p.Subject, p.Picture); err != nil {
			// Logging in is still correct; only the link could not be stored.
			log.Printf("WARN: could not link google account to user %s: %v", existing.ID, err)
		}
		return existing, nil
	}

	created, err := s.userRepository.CreateOAuthUser(ctx, p.Name, p.Email, providerGoogle, p.Subject, p.Picture)
	if err != nil {
		return nil, fmt.Errorf("creating user failed: %w", err)
	}
	return created, nil
}
