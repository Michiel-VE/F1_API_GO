package controller

import (
	"net/http"

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
	races, err := r.raceService.GetAllRacesForSeason(c.Request.Context(), season)
	if err != nil {
		SendError(c, http.StatusInternalServerError, err.Error())
		return
	}
	SendListResponse(c, races)
}