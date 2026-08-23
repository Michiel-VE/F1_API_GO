package security

import (
	"github.com/google/uuid"
)

// CustomUserPrincipal represents the authenticated user context within the application.
type CustomUserPrincipal struct {
	ID       uuid.UUID
	Email    string
	Password string
	Role     string
}

// NewCustomUserPrincipal initializes a principal instance.
func NewCustomUserPrincipal(id uuid.UUID, email, password, role string) *CustomUserPrincipal {
	return &CustomUserPrincipal{
		ID:       id,
		Email:    email,
		Password: password,
		Role:     role,
	}
}

func (p *CustomUserPrincipal) GetID() uuid.UUID {
	return p.ID
}

func (p *CustomUserPrincipal) GetUsername() string {
	return p.Email
}

func (p *CustomUserPrincipal) GetRole() string {
	return p.Role
}