package models

import "github.com/google/uuid"

type DriverTeamSeason struct {
	BaseEntry
	DriverID uuid.UUID `gorm:"column:driver_id;type:uuid;not null" json:"driver_id"`
	Driver   Driver    `gorm:"foreignKey:DriverID" json:"driver,omitempty"`
	TeamID   uuid.UUID `gorm:"column:team_id;type:uuid;not null" json:"team_id"`
	Team     Team      `gorm:"foreignKey:TeamID" json:"team,omitempty"`
	SeasonID uuid.UUID `gorm:"column:season_id;type:uuid;not null" json:"season_id"`
	Season   Season    `gorm:"foreignKey:SeasonID" json:"season,omitempty"`
	Points   int       `gorm:"column:points;not null" json:"points"`
}

func (DriverTeamSeason) TableName() string {
	return "f1_api.driver_team_season"
}