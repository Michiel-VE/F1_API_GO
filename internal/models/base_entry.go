package models

import (
	"time"

	"github.com/google/uuid"
)

type BaseEntry struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	CreatedAt time.Time `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;autoUpdateTime" json:"updated_at"`
}