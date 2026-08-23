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

func (s *JWTService) extractAllClaims(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secretKey, nil
	})

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

func (s *JWTService) ExtractPrincipal(tokenString string) (*CustomUserPrincipal, error) {
	claims, err := s.extractAllClaims(tokenString)
	if err != nil {
		return nil, err
	}

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

func (s *JWTService) IsTokenValid(tokenString string) bool {
	claims, err := s.extractAllClaims(tokenString)
	if err != nil {
		return false
	}
	return claims.ExpiresAt.After(time.Now())
}

func (s *JWTService) ValidateToken(tokenString string) (any, bool) {
	principal, err := s.ExtractPrincipal(tokenString)
	if err != nil {
		return nil, false
	}
	if !s.IsTokenValid(tokenString) {
		return nil, false
	}
	return principal, true
}

func (s *JWTService) GetExpiration() time.Duration {
	return s.expiration
}