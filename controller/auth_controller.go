package controller

import (
	"net/http"
	"os"

	"f1-api-go/internal/requests"
	"f1-api-go/services"
	"github.com/gin-gonic/gin"
)

type AuthController struct {
	authService *services.AuthService
	jwtService  *services.JWTService
}

func NewAuthController(authService *services.AuthService, jwtService *services.JWTService) *AuthController {
	return &AuthController{
		authService: authService,
		jwtService:  jwtService,
	}
}

func (a *AuthController) Register(c *gin.Context) {
	var req requests.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := a.authService.RegisterUser(req); err != nil {
		SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.String(http.StatusOK, "Registration successful")
}

func (a *AuthController) Login(c *gin.Context) {
	var req requests.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	isProduction := os.Getenv("BASE_URL") != ""
	expirationMs := a.jwtService.GetExpiration().Milliseconds()

	cookieHeader, err := a.authService.LoginAndGetCookieHeader(req, isProduction, expirationMs)
	if err != nil {
		SendError(c, http.StatusUnauthorized, err.Error())
		return
	}

	c.Header("Set-Cookie", cookieHeader)
	c.Status(http.StatusOK)
}

func (a *AuthController) Logout(c *gin.Context) {
	isProduction := os.Getenv("BASE_URL") != ""
	domain := ""
	if isProduction {
		domain = "michielve.be"
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("jwt", "", -1, "/", domain, isProduction, true)
	c.Status(http.StatusOK)
}