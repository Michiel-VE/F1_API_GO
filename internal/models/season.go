package models

type Season struct {
	BaseEntry
	SeasonName        string             `gorm:"column:season_name;not null;unique" json:"season_name"`
	DriverTeamSeasons []DriverTeamSeason `gorm:"foreignKey:SeasonID;constraint:OnDelete:CASCADE" json:"driver_team_seasons,omitempty"`
}

func (Season) TableName() string {
	return "f1_api.season"
}