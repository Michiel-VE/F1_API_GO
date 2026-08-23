package response

import (
	"time"

	"github.com/google/uuid"
)

type RaceResponse struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	Country   string     `json:"country"`
	StartDay  string     `json:"startDay"`
	EndDay    string     `json:"endDay"`
	ExtraInfo string     `json:"extraInfo"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

type RaceResultDTO struct {
	RaceName      string   `json:"raceName"`
	Points        float64  `json:"points"`
	Status        string   `json:"status"`
	LapsCompleted *int     `json:"lapsCompleted,omitempty"`
}