package controller

import (
	"net/http"

	"f1-api-go/services"
	"github.com/gin-gonic/gin"
)

type TeamController struct {
	teamService *services.TeamService
}

func NewTeamController(teamService *services.TeamService) *TeamController {
	return &TeamController{
		teamService: teamService,
	}
}

func (t *TeamController) GetAllTeamsForSeason(c *gin.Context) {
	season := c.Param("season")
	teams, err := t.teamService.GetAllTeamsForSeason(c.Request.Context(), season)
	if err != nil {
		SendError(c, http.StatusInternalServerError, err.Error())
		return
	}

	SendListResponse(c, teams)
}

func (t *TeamController) SearchTeamByName(c *gin.Context) {
	name := c.Param("name")
	team, err := t.teamService.SearchTeamByName(c.Request.Context(), name)
	if err != nil {
		SendError(c, http.StatusNotFound, err.Error())
		return
	}

	c.JSON(http.StatusOK, team)
}