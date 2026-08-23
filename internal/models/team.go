package models

type Team struct {
	BaseEntry
	Name              string             `gorm:"column:name;not null;unique" json:"name"`
	ShortName         string             `gorm:"column:short_name;not null;unique" json:"short_name"`
	Country           string             `gorm:"column:country;not null" json:"country"`
	Base              string             `gorm:"column:base;not null" json:"base"`
	DriverTeamSeasons []DriverTeamSeason `gorm:"foreignKey:TeamID;constraint:OnDelete:CASCADE" json:"driver_team_seasons,omitempty"`
}

func (Team) TableName() string {
	return "f1_api.team"
}