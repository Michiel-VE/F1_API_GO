package controller

import (
	"f1-api-go/services"

	"github.com/gin-gonic/gin"
)

type RaceController struct {
	raceService *services.RaceService
}

func NewRaceController(raceService *services.RaceService) *RaceController {
	return &RaceController{
		raceService: raceService,
	}
}

func (r *RaceController) GetAllRacesForSeason(c *gin.Context) {
	season := c.Param("season")
	if !RequireSeason(c, season) {
		return
	}

	races, err := r.raceService.GetAllRacesForSeason(c.Request.Context(), season)
	if err != nil {
		SendServiceError(c, err)
		return
	}
	SendListResponse(c, races)
}
