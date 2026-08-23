package converter

import (
	"f1-api-go/internal/models"
	"f1-api-go/internal/response"
)

func RaceToResponse(race models.Race) response.RaceResponse {
	var extraInfoStr string
	if race.ExtraInfo != nil {
		extraInfoStr = *race.ExtraInfo
	}

	return response.RaceResponse{
		ID:        race.ID,
		Name:      race.Name,
		Country:   race.Country,
		StartDay:  race.RaceStartDate.Format("2006-01-02"),
		EndDay:    race.RaceEndDate.Format("2006-01-02"),
		ExtraInfo: extraInfoStr,
		Status:    race.Status.Name,
		CreatedAt: race.CreatedAt,
	}
}