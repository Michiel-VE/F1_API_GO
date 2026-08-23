package models

import "github.com/google/uuid"

type RaceSeason struct {
	BaseEntry
	RaceID   uuid.UUID `gorm:"column:race_id;type:uuid" json:"race_id"`
	Race     Race      `gorm:"foreignKey:RaceID" json:"race,omitempty"`
	SeasonID uuid.UUID `gorm:"column:season_id;type:uuid" json:"season_id"`
	Season   Season    `gorm:"foreignKey:SeasonID" json:"season,omitempty"`
}

func (RaceSeason) TableName() string {
	return "f1_api.race_season"
}