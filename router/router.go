package router

import (
	"f1-api-go/controller"
	"f1-api-go/security"

	"github.com/gin-gonic/gin"
)

func SetupRouter(
	authController *controller.AuthController,
	profileController *controller.ProfileController,
	driverController *controller.DriverController,
	teamController *controller.TeamController,
	raceController *controller.RaceController,
	predictionController *controller.PredictionController,
	jwtService security.TokenValidator, // Pass your jwtService here
) *gin.Engine {
	r := gin.Default()

	api := r.Group("/api/v1")

	// Plug in your existing JwtFilter here so it evaluates incoming requests
	api.Use(security.JwtFilter(jwtService))

	{
		// Auth Routes
		api.POST("/auth/register", authController.Register)
		api.POST("/auth/login", authController.Login)
		api.POST("/auth/logout", authController.Logout)

		// Profile Routes
		api.GET("/profile", profileController.GetProfile)

		// Driver Routes
		api.GET("/drivers", driverController.GetAllDrivers)
		api.GET("/driver", driverController.GetDriverByPermanentNumber)
		api.GET("/drivers/lastname/:lastName", driverController.GetDriverByLastName)
		api.GET("/drivers/name/:name/career-history", driverController.GetDriverCareerHistory)
		api.GET("/drivers/:id/results", driverController.GetDriverRaceResults)
		api.GET("/drivers/season/:season", driverController.GetDriversBySeason)

		// Team Routes
		api.GET("/teams/:season", teamController.GetAllTeamsForSeason)
		api.GET("/team/:name", teamController.SearchTeamByName)

		// Race Routes
		api.GET("/races/:season", raceController.GetAllRacesForSeason)

		// Prediction & Pool Routes
		api.POST("/predictions", predictionController.CreatePrediction)
		api.POST("/predictions/team", predictionController.CreateTeamPrediction)
		api.GET("/predictions/pools", predictionController.GetUserPools)
		api.GET("/predictions/pools/:poolId", predictionController.GetPoolDetails)
		api.GET("/predictions/status", predictionController.GetPredictionStatus)
		api.GET("/predictions/team/personal", predictionController.GetPersonalTeamPrediction)
		api.GET("/predictions/pools/:poolId/team", predictionController.GetPoolTeamPrediction)
		api.POST("/predictions/pools", predictionController.CreatePool)
		api.POST("/predictions/pools/join", predictionController.JoinPool)

		api.DELETE("/predictions/pools/:poolId", predictionController.LeavePool)
		api.DELETE("/predictions/pools/:poolId/members/:memberId", predictionController.KickMember)
	}

	return r
}
