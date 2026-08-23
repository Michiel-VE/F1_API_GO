package dto

type RaceResultDTO struct {
	RaceName      string  `json:"raceName"`
	Points        float64 `json:"points"`
	Status        string  `json:"status"`
	LapsCompleted *int    `json:"lapsCompleted"`
}