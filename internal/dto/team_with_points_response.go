package dto

import "github.com/google/uuid"

type TeamWithPointsResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	ShortName   string    `json:"shortName"`
	Country     string    `json:"country"`
	Base        string    `json:"base"`
	TotalPoints int       `json:"totalPoints"`
}