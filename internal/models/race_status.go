package models

import "github.com/google/uuid"

type RaceStatus struct {
	ID   uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name string    `gorm:"column:name;not null;unique" json:"name"`
}

func (RaceStatus) TableName() string {
	return "f1_api.race_status"
}