package response

import (
	"time"

	"github.com/google/uuid"
)

type DriverCareerHistoryResponse struct {
	Season   string `json:"season"`
	Team     string `json:"team"`
	Position string `json:"position"`
	Points   string `json:"points"`
}

type DriverRaceResultsResponse struct {
	Firstname string          `json:"firstname"`
	Lastname  string          `json:"lastname"`
	Season    string          `json:"season"`
	Results   []RaceResultDTO `json:"results"`
}

type DriverResponse struct {
	DriverID        uuid.UUID  `json:"driverId"`
	Firstname       string     `json:"firstname"`
	Lastname        string     `json:"lastname"`
	PermanentNumber *int       `json:"permanentNumber,omitempty"`
	TeamName        string     `json:"teamName"`
	Points          int        `json:"points"`
	Birthday        string     `json:"birthday"`
	Country         string     `json:"country"`
	CountryCode     string     `json:"countryCode"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
}

type DriverWithSeasonsResponse struct {
	DriverID        uuid.UUID            `json:"driverId"`
	Firstname       string               `json:"firstname"`
	Lastname        string               `json:"lastname"`
	PermanentNumber *int                 `json:"permanentNumber,omitempty"`
	Birthday        string               `json:"birthday"`
	Country         string               `json:"country"`
	CountryCode     string               `json:"countryCode"`
	TeamSeasons     []TeamSeasonResponse `json:"teamSeasons"`
}
