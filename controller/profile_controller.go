package controller

import (
	"net/http"

	"f1-api-go/services"

	"github.com/gin-gonic/gin"
)

type ProfileController struct {
	userService *services.UserService
}

func NewProfileController(userService *services.UserService) *ProfileController {
	return &ProfileController{
		userService: userService,
	}
}

func (p *ProfileController) GetProfile(c *gin.Context) {
	user, ok := GetAuthUser(c)
	if !ok {
		return
	}

	profile, err := p.userService.FindByEmail(c.Request.Context(), user.Username)
	if err != nil || profile == nil {
		SendError(c, http.StatusNotFound, "User profile not found.")
		return
	}

	c.JSON(http.StatusOK, profile)
}
