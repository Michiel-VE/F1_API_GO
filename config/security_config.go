package config

import (
	"log"
	"net/http"
	"os"
	"strings"

	"f1-api-go/controller"
	"f1-api-go/exceptions"
	"f1-api-go/security"
	"f1-api-go/services"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type OAuthProperties struct {
	GoogleClientID     string
	GoogleClientSecret string
	OAuth2Config       *oauth2.Config
}

func NewOAuthProperties() *OAuthProperties {
	clientID := os.Getenv("SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_GOOGLE_CLIENT_SECRET")

	forcedRedirectURI := os.Getenv("SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_GOOGLE_REDIRECT_URI")
	redirectURI := forcedRedirectURI
	if redirectURI == "" {
		baseURL := os.Getenv("BASE_URL")
		redirectURI = baseURL + "/login/oauth2/code/google"
	}

	oauthConfig := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURI,
		Scopes:       []string{"openid", "profile", "email"},
		Endpoint:     google.Endpoint,
	}

	return &OAuthProperties{
		GoogleClientID:     clientID,
		GoogleClientSecret: clientSecret,
		OAuth2Config:       oauthConfig,
	}
}

func SecurityConfig(
	authController *controller.AuthController,
	profileController *controller.ProfileController,
	driverController *controller.DriverController,
	teamController *controller.TeamController,
	raceController *controller.RaceController,
	predictionController *controller.PredictionController,
	jwtService *services.JWTService,
	oAuthSuccessHandler *security.OAuth2AuthenticationSuccessHandler,
) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	// CORS MUST be absolute first so headers are attached to ALL responses (including errors, 401, 404)
	corsConfig := cors.Config{
		AllowOrigins: []string{
			"http://localhost:4200",
			"http://localhost:8080",
			"https://f1.michielve.be",
			"https://f1-api.michielve.be",
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-Requested-With", "Accept", "Origin"},
		ExposeHeaders:    []string{"Authorization"},
		AllowCredentials: true,
	}
	r.Use(cors.New(corsConfig))

	// --- TEMPORARY DEBUG MIDDLEWARE ---
	// Logs whether the Origin header arrived and whether the CORS
	// middleware actually set Access-Control-Allow-Origin on the response.
	// Remove once the missing-CORS-header issue is resolved.
	r.Use(func(c *gin.Context) {
		c.Next()
		log.Printf(
			"DEBUG origin=%q acao=%q acac=%q status=%d method=%s path=%s",
			c.Request.Header.Get("Origin"),
			c.Writer.Header().Get("Access-Control-Allow-Origin"),
			c.Writer.Header().Get("Access-Control-Allow-Credentials"),
			c.Writer.Status(),
			c.Request.Method,
			c.Request.URL.Path,
		)
	})
	// --- END TEMPORARY DEBUG MIDDLEWARE ---

	r.Use(exceptions.GlobalExceptionHandler())
	r.Use(security.JwtFilter(jwtService))

	auth := r.Group("/api/v1/auth")
	{
		auth.POST("/register", authController.Register)
		auth.POST("/login", authController.Login)
		auth.POST("/logout", func(c *gin.Context) {
			isProduction := strings.EqualFold(os.Getenv("SPRING_PROFILES_ACTIVE"), "prod") ||
				(os.Getenv("BASE_URL") != "" && strings.Contains(os.Getenv("BASE_URL"), "michielve.be"))

			domain := ""
			if isProduction {
				domain = "michielve.be"
			}

			c.SetSameSite(http.SameSiteLaxMode)
			c.SetCookie("jwt", "", -1, "/", domain, isProduction, true)
			authController.Logout(c)
		})
	}

	// PUBLIC ROUTES
	public := r.Group("/api/v1")
	{
		// Driver Routes
		public.GET("/drivers", driverController.GetAllDrivers)
		public.GET("/driver", driverController.GetDriverByPermanentNumber)
		public.GET("/drivers/lastname/:lastName", driverController.GetDriverByLastName)
		public.GET("/drivers/name/:name/career-history", driverController.GetDriverCareerHistory)
		public.GET("/drivers/:id/results", driverController.GetDriverRaceResults)
		public.GET("/drivers/season/:season", driverController.GetDriversBySeason)

		// Team Routes
		public.GET("/teams/:season", teamController.GetAllTeamsForSeason)
		public.GET("/team/:name", teamController.SearchTeamByName)

		// Race Routes
		public.GET("/races/:season", raceController.GetAllRacesForSeason)
	}

	// PROTECTED ROUTES
	protected := r.Group("/api/v1")
	protected.Use(RequireAuth())
	{
		// Profile Routes
		protected.GET("/profile", profileController.GetProfile)

		// Prediction & Pool Routes
		protected.POST("/predictions", predictionController.CreatePrediction)
		protected.POST("/predictions/team", predictionController.CreateTeamPrediction)
		protected.GET("/predictions/pools", predictionController.GetUserPools)
		protected.GET("/predictions/pools/:poolId", predictionController.GetPoolDetails)
		protected.GET("/predictions/status", predictionController.GetPredictionStatus)
		protected.GET("/predictions/team/personal", predictionController.GetPersonalTeamPrediction)
		protected.GET("/predictions/pools/:poolId/team", predictionController.GetPoolTeamPrediction)
		protected.POST("/predictions/pools", predictionController.CreatePool)
		protected.POST("/predictions/pools/join", predictionController.JoinPool)

		protected.DELETE("/predictions/pools/:poolId", predictionController.LeavePool)
		protected.DELETE("/predictions/pools/:poolId/members/:memberId", predictionController.KickMember)
	}

	r.NoRoute(func(c *gin.Context) {
		c.AbortWithStatus(http.StatusNotFound)
	})

	return r
}

func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, exists := c.Get("userPrincipal"); !exists {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	}
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
