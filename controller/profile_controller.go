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
	if err != nil {
		SendServiceError(c, err)
		return
	}
	if profile == nil {
		SendError(c, http.StatusNotFound, "User profile not found.")
		return
	}

	// The model is sent as-is, so the bcrypt password hash would be in the JSON
	// unless the struct tag hides it. Work on a copy and blank the hash so it can
	// never leave the server, whatever the tags say. Every other field is unchanged.
	safe := *profile
	safe.Password = nil
	c.JSON(http.StatusOK, safe)
}
