package models

import "github.com/google/uuid"

type DriverRaceResult struct {
	BaseEntry
	DriverID      uuid.UUID `gorm:"column:driver_id;type:uuid;not null" json:"driver_id"`
	Driver        Driver    `gorm:"foreignKey:DriverID" json:"driver,omitempty"`
	RaceID        uuid.UUID `gorm:"column:race_id;type:uuid;not null" json:"race_id"`
	Race          Race      `gorm:"foreignKey:RaceID" json:"race,omitempty"`
	SeasonID      uuid.UUID `gorm:"column:season_id;type:uuid;not null" json:"season_id"`
	Season        Season    `gorm:"foreignKey:SeasonID" json:"season,omitempty"`
	Points        float64   `gorm:"column:points;type:numeric(5,2);not null;default:0" json:"points"`
	SprintPoints  float64   `gorm:"column:sprint_points;type:numeric(5,2);not null;default:0" json:"sprint_points"`
	Status        *string   `gorm:"column:status;type:varchar(50)" json:"status,omitempty"`
	LapsCompleted *int      `gorm:"column:laps_completed" json:"laps_completed,omitempty"`
}

func (DriverRaceResult) TableName() string {
	return "f1_api.driver_race_result"
}
