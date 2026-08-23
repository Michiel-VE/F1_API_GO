package response

import (
	"time"

	"github.com/google/uuid"
)

type TeamResponse struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	Country   string     `json:"country"`
	ShortName string     `json:"shortName"`
	Base      string     `json:"base"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

type TeamSeasonResponse struct {
	TeamName   string     `json:"teamName"`
	ShortName  string     `json:"shortName"`
	SeasonName string     `json:"seasonName"`
	Points     *int       `json:"points,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
}

type TeamWithPointsResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Country     string    `json:"country"`
	ShortName   string    `json:"shortName"`
	Base        string    `json:"base"`
	TotalPoints *int      `json:"totalPoints,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}