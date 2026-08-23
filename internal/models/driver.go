package models

import "time"

type Driver struct {
	BaseEntry
	Firstname         string             `gorm:"column:firstname;not null" json:"firstname"`
	Lastname          string             `gorm:"column:lastname;not null" json:"lastname"`
	PermanentNumber   int                `gorm:"column:permanent_number;not null;unique" json:"permanent_number"`
	DriverCode        string             `gorm:"column:driver_code;type:varchar(3);not null;unique" json:"driver_code"`
	Birthday          time.Time          `gorm:"column:birthday;type:date;not null" json:"birthday"`
	Country           string             `gorm:"column:country;not null" json:"country"`
	CountryCode       string             `gorm:"column:country_code;type:varchar(3);not null" json:"country_code"`
	Team              string             `gorm:"column:team;not null" json:"team"`
	DriverTeamSeasons []DriverTeamSeason `gorm:"foreignKey:DriverID;constraint:OnDelete:CASCADE" json:"driver_team_seasons,omitempty"`
}

func (Driver) TableName() string {
	return "f1_api.driver"
}