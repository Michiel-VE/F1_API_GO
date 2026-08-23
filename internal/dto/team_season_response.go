package dto

import "time"

type TeamSeasonResponse struct {
	TeamName   string    `json:"teamName"`
	ShortName  string    `json:"shortName"`
	SeasonName string    `json:"seasonName"`
	Points     int       `json:"points"`
	CreatedAt  time.Time `json:"created_at"`
}