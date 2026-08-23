package dto

import (
	"time"

	"github.com/google/uuid"
)

type DriverResponse struct {
	DriverID        uuid.UUID `json:"driverId"`
	Firstname       string    `json:"firstname"`
	Lastname        string    `json:"lastname"`
	TeamName        string    `json:"teamName"`
	PermanentNumber int       `json:"permanentNumber"`
	Birthday        time.Time `json:"birthday"`
	Country         string    `json:"country"`
	CountryCode     string    `json:"countryCode"`
	CreatedAt       time.Time `json:"created_at"`
}