package models

import (
	"time"

	"github.com/google/uuid"
)

type Race struct {
	BaseEntry
	Name          string       `gorm:"column:name;not null;unique" json:"name"`
	Country       string       `gorm:"column:country;not null" json:"country"`
	RaceStartDate time.Time    `gorm:"column:race_start_date;type:date;not null" json:"race_start_date"`
	RaceEndDate   time.Time    `gorm:"column:race_end_date;type:date;not null" json:"race_end_date"`
	ExtraInfo     *string      `gorm:"column:extra_info;type:text" json:"extra_info,omitempty"`
	IsSprint      bool         `gorm:"column:is_sprint;not null;default:false" json:"is_sprint"`
	StatusID      uuid.UUID    `gorm:"column:status_id;type:uuid;not null" json:"status_id"`
	Status        RaceStatus   `gorm:"foreignKey:StatusID" json:"status,omitempty"`
	RaceSeasons   []RaceSeason `gorm:"foreignKey:RaceID;constraint:OnDelete:CASCADE" json:"race_seasons,omitempty"`
}

func (Race) TableName() string {
	return "f1_api.race"
}
