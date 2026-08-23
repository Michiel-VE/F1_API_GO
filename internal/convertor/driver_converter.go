package converter

import (
	"f1-api-go/internal/models"
	"f1-api-go/internal/response"
)

func DriverToWithSeasonsResponse(driver models.Driver) response.DriverWithSeasonsResponse {
	teamSeasons := make([]response.TeamSeasonResponse, len(driver.DriverTeamSeasons))
	for i, dts := range driver.DriverTeamSeasons {
		teamSeasons[i] = TeamSeasonToResponse(dts)
	}

	num := driver.PermanentNumber

	return response.DriverWithSeasonsResponse{
		DriverID:        driver.ID,
		Firstname:       driver.Firstname,
		Lastname:        driver.Lastname,
		PermanentNumber: &num,
		Birthday:        driver.Birthday.Format("2006-01-02"),
		Country:         driver.Country,
		CountryCode:     driver.CountryCode,
		TeamSeasons:     teamSeasons,
	}
}

func DriverToResponse(driver models.Driver) response.DriverResponse {
	num := driver.PermanentNumber

	return response.DriverResponse{
		DriverID:        driver.ID,
		Firstname:       driver.Firstname,
		Lastname:        driver.Lastname,
		TeamName:        driver.Team,
		PermanentNumber: &num,
		Birthday:        driver.Birthday.Format("2006-01-02"),
		Country:         driver.Country,
		CountryCode:     driver.CountryCode,
		CreatedAt:       driver.CreatedAt,
	}
}

func DriverToRaceResultsResponse(driver models.Driver, results []models.DriverRaceResult, season string) response.DriverRaceResultsResponse {
	dtoResults := make([]response.RaceResultDTO, len(results))
	for i, res := range results {
		var statusStr string
		if res.Status != nil {
			statusStr = *res.Status
		}

		dtoResults[i] = response.RaceResultDTO{
			RaceName:      res.Race.Name,
			Points:        res.Points,
			Status:        statusStr,
			LapsCompleted: res.LapsCompleted,
		}
	}

	return response.DriverRaceResultsResponse{
		Firstname: driver.Firstname,
		Lastname:  driver.Lastname,
		Season:    season,
		Results:   dtoResults,
	}
}

func TeamSeasonToResponse(dts models.DriverTeamSeason) response.TeamSeasonResponse {
	pts := dts.Points

	return response.TeamSeasonResponse{
		TeamName:   dts.Team.Name,
		ShortName:  dts.Team.ShortName,
		SeasonName: dts.Season.SeasonName,
		Points:     &pts,
		CreatedAt:  dts.CreatedAt,
	}
}