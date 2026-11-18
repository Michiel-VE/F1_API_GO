package models

import (
	"time"

	"github.com/google/uuid"
)

type TeamPrediction struct {
	ID             uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID         uuid.UUID  `gorm:"column:user_id;type:uuid;not null" json:"user_id"`
	User           User       `gorm:"foreignKey:UserID" json:"user,omitempty"`
	PoolID         *uuid.UUID `gorm:"column:pool_id;type:uuid" json:"pool_id,omitempty"`
	Pool           *Pool      `gorm:"foreignKey:PoolID" json:"pool,omitempty"`
	CreatedAt      time.Time  `gorm:"column:created_at;type:timestamp without time zone;autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;type:timestamp without time zone;autoUpdateTime" json:"updated_at"`
	PredictedTeams []Team     `gorm:"many2many:f1_api.team_prediction_teams;joinForeignKey:prediction_id;joinReferences:team_id" json:"predicted_teams,omitempty"`
}

func (TeamPrediction) TableName() string {
	return "f1_api.team_predictions"
}
