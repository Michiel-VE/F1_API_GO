package converter

import (
	"f1-api-go/internal/dto"
	"f1-api-go/internal/models"
	"f1-api-go/internal/response"
)

func TeamToResponse(team models.Team) response.TeamResponse {
	return response.TeamResponse{
		ID:        team.ID,
		Name:      team.Name,
		ShortName: team.ShortName,
		Country:   team.Country,
		Base:      team.Base,
		CreatedAt: team.CreatedAt,
	}
}

func TeamWithPointsToResponse(team dto.TeamWithPoints) response.TeamWithPointsResponse {
	pts := team.GetTotalPoints()

	return response.TeamWithPointsResponse{
		ID:          team.GetID(),
		Name:        team.GetName(),
		ShortName:   team.GetShortName(),
		Country:     team.GetCountry(),
		Base:        team.GetBase(),
		TotalPoints: &pts,
	}
}