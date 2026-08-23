package converter

import "f1-api-go/internal/dto"

type DriverTeamSeasonConverter struct{}

func NewDriverTeamSeasonConverter() *DriverTeamSeasonConverter {
	return &DriverTeamSeasonConverter{}
}

func (c *DriverTeamSeasonConverter) TeamResponseConverter(team dto.TeamWithPoints) dto.TeamWithPointsResponse {
	return dto.TeamWithPointsResponse{
		ID:          team.GetID(),
		Name:        team.GetName(),
		ShortName:   team.GetShortName(),
		Country:     team.GetCountry(),
		Base:        team.GetBase(),
		TotalPoints: team.GetTotalPoints(),
	}
}