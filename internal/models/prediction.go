package models

import (
	"time"

	"github.com/google/uuid"
)

type Prediction struct {
	ID               uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID           uuid.UUID `gorm:"column:user_id;type:uuid;not null" json:"user_id"`
	RaceID           uuid.UUID `gorm:"column:race_id;type:uuid;not null" json:"race_id"`
	PredictedDrivers string    `gorm:"column:predicted_drivers;type:text;not null" json:"predicted_drivers"`
	Points           *int      `gorm:"column:points;type:integer" json:"points,omitempty"`
	CreatedAt        time.Time `gorm:"column:created_at;type:timestamp without time zone;autoCreateTime" json:"created_at"`
}

func (Prediction) TableName() string {
	return "f1_api.prediction"
}
