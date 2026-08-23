package models

import (
	"f1-api-go/internal/domain"
)

type User struct {
	BaseEntry
	Provider   *string     `gorm:"column:provider" json:"provider,omitempty"`
	ProviderID *string     `gorm:"column:provider_id;unique" json:"provider_id,omitempty"`
	Email      string      `gorm:"column:email;not null;unique" json:"email"`
	Password   *string     `gorm:"column:password" json:"-"`
	Name       *string     `gorm:"column:name" json:"name,omitempty"`
	Role       domain.Role `gorm:"column:role;type:varchar(20);not null" json:"role"`
	Picture    *string     `gorm:"column:picture;type:varchar(2048)" json:"picture,omitempty"`
}

func (User) TableName() string {
	return "f1_api.users"
}