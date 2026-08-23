package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"f1-api-go/internal/domain"
	"f1-api-go/internal/models"
	"f1-api-go/internal/requests"
	"f1-api-go/repository"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepository *repository.UserRepository
	jwtService     *JWTService
}

func NewAuthService(userRepo *repository.UserRepository, jwtService *JWTService) *AuthService {
	return &AuthService{
		userRepository: userRepo,
		jwtService:     jwtService,
	}
}

func (s *AuthService) RegisterUser(req requests.RegisterRequest) error {
	existingUser, _ := s.userRepository.FindByEmail(context.Background(), req.Email)
	if existingUser != nil {
		return errors.New("email is already in use")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	hashedStr := string(hashedPassword)
	nameStr := req.Name

	newUser := &models.User{
		Name:      &nameStr,
		Email:     req.Email,
		Password:  &hashedStr,
		Role:      domain.RoleUser,
		CreatedAt: time.Now(),
	}

	_ = newUser
	return nil
}

func (s *AuthService) LoginAndGetCookieHeader(req requests.LoginRequest, isProduction bool, expirationMs int64) (string, error) {
	user, err := s.userRepository.FindByEmail(context.Background(), req.Email)
	if err != nil || user == nil {
		return "", errors.New("invalid credentials")
	}

	passwordBytes := []byte{}
	if user.Password != nil {
		passwordBytes = []byte(*user.Password)
	}

	if err := bcrypt.CompareHashAndPassword(passwordBytes, []byte(req.Password)); err != nil {
		return "", errors.New("invalid credentials")
	}

	principal := &CustomUserPrincipal{
		ID:       user.ID,
		Username: user.Email,
		Role:     string(user.Role),
	}

	token, err := s.jwtService.GenerateToken(principal)
	if err != nil {
		return "", err
	}

	return BuildCookieHeader(token, isProduction, int(expirationMs/1000)), nil
}

func BuildCookieHeader(token string, isProduction bool, maxAgeSeconds int) string {
	domainName := ""
	if isProduction {
		domainName = "michielve.be"
	}

	cookie := http.Cookie{
		Name:     "jwt",
		Value:    token,
		Path:     "/",
		Domain:   domainName,
		MaxAge:   maxAgeSeconds,
		HttpOnly: true,
		Secure:   isProduction,
		SameSite: http.SameSiteLaxMode,
	}

	return cookie.String()
}
