package models

import "github.com/google/uuid"

type TeamPrediction struct {
	BaseEntry
	UserID         uuid.UUID  `gorm:"column:user_id;type:uuid;not null" json:"user_id"`
	User           User       `gorm:"foreignKey:UserID" json:"user,omitempty"`
	PoolID         *uuid.UUID `gorm:"column:pool_id;type:uuid" json:"pool_id,omitempty"`
	Pool           *Pool      `gorm:"foreignKey:PoolID" json:"pool,omitempty"`
	PredictedTeams []Team     `gorm:"many2many:f1_api.team_prediction_teams;joinForeignKey:prediction_id;joinReferences:team_id" json:"predicted_teams,omitempty"`
}

func (TeamPrediction) TableName() string {
	return "f1_api.team_predictions"
}