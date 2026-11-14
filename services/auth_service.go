package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"f1-api-go/internal/domain"
	"f1-api-go/internal/models"
	"f1-api-go/internal/requests"
	"f1-api-go/repository"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// bcrypt only uses the first 72 bytes of a password; newer x/crypto versions
// reject longer input with an error.
const maxPasswordBytes = 72

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

// RegisterUser now actually stores the user. It used to build the user object,
// discard it (`_ = newUser`) and return nil, so registration silently did nothing.
func (s *AuthService) RegisterUser(req requests.RegisterRequest) error {
	ctx := context.Background()

	if len(req.Password) > maxPasswordBytes {
		return fmt.Errorf("password must be at most %d bytes", maxPasswordBytes)
	}

	existingUser, err := s.userRepository.FindByEmail(ctx, req.Email)
	if err != nil {
		return fmt.Errorf("failed to check existing user: %w", err)
	}
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
		ID:        uuid.New(),
		Name:      &nameStr,
		Email:     req.Email,
		Password:  &hashedStr,
		Role:      domain.RoleUser,
		CreatedAt: time.Now(),
	}

	if err := s.userRepository.Create(ctx, newUser); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

var (
	dummyHashOnce sync.Once
	dummyHash     []byte
)

// compareAgainstDummy burns the same bcrypt time as a real comparison so that
// "unknown email" and "wrong password" can't be told apart by response time.
func compareAgainstDummy(password string) {
	dummyHashOnce.Do(func() {
		dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), bcrypt.DefaultCost)
	})
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
}

func (s *AuthService) LoginAndGetCookieHeader(req requests.LoginRequest, isProduction bool, expirationMs int64) (string, error) {
	user, err := s.userRepository.FindByEmail(context.Background(), req.Email)
	if err != nil || user == nil {
		compareAgainstDummy(req.Password)
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
