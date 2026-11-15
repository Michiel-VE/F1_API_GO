package config

import (
	"net/http"
	"os"
	"strings"

	"f1-api-go/controller"
	"f1-api-go/exceptions"
	"f1-api-go/security"
	"f1-api-go/services"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// allowedOrigins lists who may call the API from a browser. The localhost
// origins are only allowed outside production, unless you opt in with
// CORS_ALLOW_LOCALHOST=true (for example to develop the frontend locally
// against the deployed API).
func allowedOrigins() []string {
	origins := []string{
		"https://f1.michielve.be",
		"https://f1-api.michielve.be",
	}
	if !IsProduction() || strings.EqualFold(os.Getenv("CORS_ALLOW_LOCALHOST"), "true") {
		origins = append(origins, "http://localhost:4200", "http://localhost:8080")
	}
	return origins
}

// SecurityConfig builds the gin engine: middleware first, then every route.
func SecurityConfig(
	authController *controller.AuthController,
	profileController *controller.ProfileController,
	driverController *controller.DriverController,
	teamController *controller.TeamController,
	raceController *controller.RaceController,
	predictionController *controller.PredictionController,
	jwtService *services.JWTService,
	oauthController *controller.OAuthController,
) *gin.Engine {
	// Debug mode (the default) prints every route at startup and is slower.
	if IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	// CORS must come first so headers are attached to ALL responses (including
	// errors, 401, 404). It also answers preflight requests itself and rejects
	// requests from origins that are not in the list.
	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins(),
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-Requested-With", "Accept", "Origin"},
		ExposeHeaders:    []string{"Authorization"},
		AllowCredentials: true,
	}))
	r.Use(exceptions.GlobalExceptionHandler())
	r.Use(security.JwtFilter(jwtService))

	registerAuthRoutes(r, authController)
	registerOAuthRoutes(r, oauthController)
	registerPublicRoutes(r, driverController, teamController, raceController)
	registerProtectedRoutes(r, profileController, predictionController)

	r.NoRoute(func(c *gin.Context) {
		controller.SendError(c, http.StatusNotFound, "Not found")
	})

	return r
}

func registerAuthRoutes(r *gin.Engine, auth *controller.AuthController) {
	g := r.Group("/api/v1/auth")
	g.POST("/register", auth.Register)
	g.POST("/login", auth.Login)
	g.POST("/logout", auth.Logout)
}

// registerOAuthRoutes adds Google sign-in. These exact paths are what API
// Gateway forwards (/oauth2/{proxy+} and /login/oauth2/{proxy+}) and what is
// registered as the redirect URI in the Google console.
func registerOAuthRoutes(r *gin.Engine, oauth *controller.OAuthController) {
	if oauth == nil {
		return
	}
	r.GET("/oauth2/authorization/google", oauth.Start)
	r.GET("/login/oauth2/code/google", oauth.Callback)
}

func registerPublicRoutes(
	r *gin.Engine,
	drivers *controller.DriverController,
	teams *controller.TeamController,
	races *controller.RaceController,
) {
	g := r.Group("/api/v1")

	// Drivers
	g.GET("/drivers", drivers.GetAllDrivers)
	g.GET("/driver", drivers.GetDriverByPermanentNumber)
	g.GET("/drivers/lastname/:lastName", drivers.GetDriverByLastName)
	g.GET("/drivers/name/:name/career-history", drivers.GetDriverCareerHistory)
	g.GET("/drivers/season/:season", drivers.GetDriversBySeason)
	g.GET("/drivers/:id/race-results", drivers.GetDriverRaceResults)

	// Teams
	g.GET("/teams/:season", teams.GetAllTeamsForSeason)
	g.GET("/team/:name", teams.SearchTeamByName)

	// Races
	g.GET("/races/:season", races.GetAllRacesForSeason)
}

func registerProtectedRoutes(
	r *gin.Engine,
	profile *controller.ProfileController,
	predictions *controller.PredictionController,
) {
	g := r.Group("/api/v1")
	g.Use(RequireAuth())

	// Profile
	g.GET("/profile", profile.GetProfile)

	// Predictions and pools
	g.POST("/predictions", predictions.CreatePrediction)
	g.POST("/predictions/team", predictions.CreateTeamPrediction)
	g.GET("/predictions/status", predictions.GetPredictionStatus)
	g.GET("/predictions/team/personal", predictions.GetPersonalTeamPrediction)

	g.GET("/predictions/pools", predictions.GetUserPools)
	g.POST("/predictions/pools", predictions.CreatePool)
	g.POST("/predictions/pools/join", predictions.JoinPool)
	g.GET("/predictions/pools/:poolId", predictions.GetPoolDetails)
	g.GET("/predictions/pools/:poolId/team", predictions.GetPoolTeamPrediction)
	g.DELETE("/predictions/pools/:poolId", predictions.LeavePool)
	g.DELETE("/predictions/pools/:poolId/members/:memberId", predictions.KickMember)
}

// RequireAuth rejects requests that JwtFilter did not authenticate.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, exists := c.Get("userPrincipal"); !exists {
			controller.SendError(c, http.StatusUnauthorized, "Authentication required")
			c.Abort()
			return
		}
		c.Next()
	}
}
