package dto

import (
	"time"

	"github.com/google/uuid"
)

type DriverWithSeasonsResponse struct {
	DriverID        uuid.UUID            `json:"driverId"`
	Firstname       string               `json:"firstname"`
	Lastname        string               `json:"lastname"`
	PermanentNumber int                  `json:"permanentNumber"`
	Birthday        time.Time            `json:"birthday"`
	Country         string               `json:"country"`
	CountryCode     string               `json:"countryCode"`
	TeamSeasons     []TeamSeasonResponse `json:"teamSeasons"`
}