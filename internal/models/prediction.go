package models

import (
	"time"

	"github.com/google/uuid"
)

type Prediction struct {
	ID               uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID           uuid.UUID `gorm:"column:user_id;type:uuid;not null;uniqueIndex:idx_user_race" json:"user_id"`
	User             User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	RaceID           uuid.UUID `gorm:"column:race_id;type:uuid;not null;uniqueIndex:idx_user_race" json:"race_id"`
	Race             Race      `gorm:"foreignKey:RaceID" json:"race,omitempty"`
	PredictedDrivers string    `gorm:"column:predicted_drivers;type:jsonb;not null" json:"predicted_drivers"`
	Points           *int      `gorm:"column:points" json:"points,omitempty"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (Prediction) TableName() string {
	return "f1_api.prediction"
}