package services

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const minJWTSecretBytes = 32

type CustomUserPrincipal struct {
	ID       uuid.UUID
	Username string
	Password string
	Role     string
}

type JWTClaims struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type JWTService struct {
	secretKey  []byte
	expiration time.Duration
}

func NewJWTService(base64Secret string, expirationMs int64) (*JWTService, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(base64Secret)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 jwt secret: %w", err)
	}
	if len(keyBytes) < minJWTSecretBytes {
		return nil, fmt.Errorf("jwt secret is too short: need at least %d bytes after base64 decoding, got %d", minJWTSecretBytes, len(keyBytes))
	}
	if expirationMs <= 0 {
		return nil, errors.New("jwt expiration must be positive")
	}

	return &JWTService{
		secretKey:  keyBytes,
		expiration: time.Duration(expirationMs) * time.Millisecond,
	}, nil
}

func (s *JWTService) GenerateToken(principal *CustomUserPrincipal) (string, error) {
	role := strings.TrimPrefix(principal.Role, "ROLE_")

	claims := JWTClaims{
		UserID: principal.ID.String(),
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   principal.Username,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.expiration)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretKey)
}

// extractAllClaims verifies signature, algorithm and expiry. Tokens without an
// exp claim are rejected: they used to parse fine and then crash IsTokenValid
// with a nil pointer.
func (s *JWTService) extractAllClaims(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&JWTClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return s.secretKey, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

func (s *JWTService) ExtractUsername(tokenString string) (string, error) {
	claims, err := s.extractAllClaims(tokenString)
	if err != nil {
		return "", err
	}
	return claims.Subject, nil
}

func principalFromClaims(claims *JWTClaims) (*CustomUserPrincipal, error) {
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id in token claims: %w", err)
	}

	return &CustomUserPrincipal{
		ID:       userID,
		Username: claims.Subject,
		Password: "",
		Role:     claims.Role,
	}, nil
}

func (s *JWTService) ExtractPrincipal(tokenString string) (*CustomUserPrincipal, error) {
	claims, err := s.extractAllClaims(tokenString)
	if err != nil {
		return nil, err
	}
	return principalFromClaims(claims)
}

func (s *JWTService) IsTokenValid(tokenString string) bool {
	claims, err := s.extractAllClaims(tokenString)
	if err != nil || claims.ExpiresAt == nil {
		return false
	}
	return claims.ExpiresAt.After(time.Now())
}

// ValidateToken parses the token once (it used to parse it twice).
func (s *JWTService) ValidateToken(tokenString string) (any, bool) {
	claims, err := s.extractAllClaims(tokenString)
	if err != nil || claims.ExpiresAt == nil || !claims.ExpiresAt.After(time.Now()) {
		return nil, false
	}
	principal, err := principalFromClaims(claims)
	if err != nil {
		return nil, false
	}
	return principal, true
}

func (s *JWTService) GetExpiration() time.Duration {
	return s.expiration
}
