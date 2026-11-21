package services

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func validSecretBase64(t *testing.T) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
}

func TestNewJWTService_Validation(t *testing.T) {
	validSecret := validSecretBase64(t)

	if _, err := NewJWTService("not-valid-base64!!!", 60000); err == nil {
		t.Fatal("expected error for invalid base64 secret")
	}

	shortSecret := base64.StdEncoding.EncodeToString([]byte("short-secret"))
	if _, err := NewJWTService(shortSecret, 60000); err == nil {
		t.Fatal("expected error for secret shorter than 32 bytes")
	}

	if _, err := NewJWTService(validSecret, 0); err == nil {
		t.Fatal("expected error for non-positive expiration")
	}

	svc, err := NewJWTService(validSecret, 15000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.GetExpiration() != 15*time.Second {
		t.Errorf("GetExpiration() = %v, want %v", svc.GetExpiration(), 15*time.Second)
	}
}

func TestJWTService_GenerateAndParse(t *testing.T) {
	svc, err := NewJWTService(validSecretBase64(t), time.Hour.Milliseconds())
	if err != nil {
		t.Fatalf("failed to create JWTService: %v", err)
	}

	principal := &CustomUserPrincipal{
		ID:       uuid.New(),
		Username: "driver@example.com",
		Role:     "ROLE_ADMIN",
	}

	token, err := svc.GenerateToken(principal)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	if !svc.IsTokenValid(token) {
		t.Fatal("expected generated token to be valid")
	}

	username, err := svc.ExtractUsername(token)
	if err != nil {
		t.Fatalf("ExtractUsername failed: %v", err)
	}
	if username != "driver@example.com" {
		t.Errorf("username = %q, want %q", username, "driver@example.com")
	}

	extracted, err := svc.ExtractPrincipal(token)
	if err != nil {
		t.Fatalf("ExtractPrincipal failed: %v", err)
	}
	if extracted.ID != principal.ID {
		t.Errorf("ID = %s, want %s", extracted.ID, principal.ID)
	}
	if extracted.Username != principal.Username {
		t.Errorf("Username = %q, want %q", extracted.Username, principal.Username)
	}
	if extracted.Role != "ADMIN" {
		t.Errorf("Role = %q, want %q", extracted.Role, "ADMIN")
	}

	validatedObj, ok := svc.ValidateToken(token)
	if !ok {
		t.Fatal("expected ValidateToken ok=true")
	}
	validatedPrincipal, ok := validatedObj.(*CustomUserPrincipal)
	if !ok {
		t.Fatalf("expected *CustomUserPrincipal, got %T", validatedObj)
	}
	if validatedPrincipal.ID != principal.ID || validatedPrincipal.Role != "ADMIN" {
		t.Errorf("unexpected validated principal: %+v", validatedPrincipal)
	}
}

func TestJWTService_RejectsInvalidAndExpiredTokens(t *testing.T) {
	svc, err := NewJWTService(validSecretBase64(t), time.Minute.Milliseconds())
	if err != nil {
		t.Fatalf("failed to create JWTService: %v", err)
	}

	if svc.IsTokenValid("invalid.token.string") {
		t.Error("expected malformed token to be invalid")
	}
	if _, ok := svc.ValidateToken("invalid.token.string"); ok {
		t.Error("expected ValidateToken to return false for malformed token")
	}

	otherSvc, err := NewJWTService(base64.StdEncoding.EncodeToString([]byte("fedcba9876543210fedcba9876543210")), time.Minute.Milliseconds())
	if err != nil {
		t.Fatalf("failed to create other JWTService: %v", err)
	}
	foreignToken, err := otherSvc.GenerateToken(&CustomUserPrincipal{
		ID:       uuid.New(),
		Username: "user@example.com",
		Role:     "USER",
	})
	if err != nil {
		t.Fatalf("failed to generate foreign token: %v", err)
	}
	if svc.IsTokenValid(foreignToken) {
		t.Error("expected token signed with different secret to be rejected")
	}

	// Token without exp claim must be rejected because WithExpirationRequired() is enforced.
	noExpClaims := JWTClaims{
		UserID: uuid.New().String(),
		Role:   "USER",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: "noexp@example.com",
		},
	}
	noExpToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, noExpClaims).SignedString(svc.secretKey)
	if err != nil {
		t.Fatalf("failed signing token without exp: %v", err)
	}
	if _, err := svc.ExtractPrincipal(noExpToken); err == nil {
		t.Error("expected error for token missing exp claim")
	}

	// Expired token must be rejected.
	expiredClaims := JWTClaims{
		UserID: uuid.New().String(),
		Role:   "USER",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "expired@example.com",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}
	expiredToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims).SignedString(svc.secretKey)
	if err != nil {
		t.Fatalf("failed signing expired token: %v", err)
	}
	if svc.IsTokenValid(expiredToken) {
		t.Error("expected expired token to be invalid")
	}

	// Invalid UUID in userId claim should fail principal extraction.
	badIDClaims := JWTClaims{
		UserID: "not-a-uuid",
		Role:   "USER",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "baduuid@example.com",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	badIDToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, badIDClaims).SignedString(svc.secretKey)
	if err != nil {
		t.Fatalf("failed signing token with invalid UUID: %v", err)
	}
	if _, err := svc.ExtractPrincipal(badIDToken); err == nil || !strings.Contains(err.Error(), "invalid user id") {
		t.Errorf("expected invalid user id error, got %v", err)
	}
}
