package controller

import (
	"net/http"

	"f1-api-go/internal/requests"
	"f1-api-go/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PredictionController struct {
	predictionService *services.PredictionService
}

func NewPredictionController(predictionService *services.PredictionService) *PredictionController {
	return &PredictionController{
		predictionService: predictionService,
	}
}

func (p *PredictionController) CreatePrediction(c *gin.Context) {
	user, ok := GetAuthUser(c)
	if !ok {
		return
	}

	var req requests.CreatePredictionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := p.predictionService.CreatePrediction(c.Request.Context(), user.ID, &req); err != nil {
		SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	SendSuccess(c, http.StatusCreated, "Prediction Created")
}

func (p *PredictionController) CreateTeamPrediction(c *gin.Context) {
	user, ok := GetAuthUser(c)
	if !ok {
		return
	}

	var req requests.CreateSeasonPredictionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := p.predictionService.CreateTeamPrediction(c.Request.Context(), user.ID, &req); err != nil {
		SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	SendSuccess(c, http.StatusCreated, "Team Prediction Created")
}

func (p *PredictionController) GetUserPools(c *gin.Context) {
	user, ok := GetAuthUser(c)
	if !ok {
		return
	}

	pools, err := p.predictionService.GetUserPools(c.Request.Context(), user.ID)
	if err != nil {
		SendError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, pools)
}

func (p *PredictionController) GetPoolDetails(c *gin.Context) {
	user, ok := GetAuthUser(c)
	if !ok {
		return
	}

	poolID, err := uuid.Parse(c.Param("poolId"))
	if err != nil {
		SendError(c, http.StatusBadRequest, "Invalid pool ID")
		return
	}

	details, err := p.predictionService.GetPoolDetails(c.Request.Context(), user.ID, poolID)
	if err != nil {
		SendError(c, http.StatusNotFound, err.Error())
		return
	}

	c.JSON(http.StatusOK, details)
}

func (p *PredictionController) GetPredictionStatus(c *gin.Context) {
	user, ok := GetAuthUser(c)
	if !ok {
		return
	}

	status, err := p.predictionService.GetPredictionStatus(c.Request.Context(), user.ID)
	if err != nil {
		SendError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, status)
}

func (p *PredictionController) GetPersonalTeamPrediction(c *gin.Context) {
	user, ok := GetAuthUser(c)
	if !ok {
		return
	}

	prediction, err := p.predictionService.GetPersonalTeamPrediction(c.Request.Context(), user.ID)
	if err != nil {
		SendError(c, http.StatusNotFound, err.Error())
		return
	}
	c.JSON(http.StatusOK, prediction)
}

func (p *PredictionController) GetPoolTeamPrediction(c *gin.Context) {
	user, ok := GetAuthUser(c)
	if !ok {
		return
	}

	poolID, err := uuid.Parse(c.Param("poolId"))
	if err != nil {
		SendError(c, http.StatusBadRequest, "Invalid pool ID")
		return
	}

	prediction, err := p.predictionService.GetPoolTeamPrediction(c.Request.Context(), user.ID, poolID)
	if err != nil {
		SendError(c, http.StatusNotFound, err.Error())
		return
	}
	c.JSON(http.StatusOK, prediction)
}

func (p *PredictionController) CreatePool(c *gin.Context) {
	user, ok := GetAuthUser(c)
	if !ok {
		return
	}

	var req requests.CreatePoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	pool, err := p.predictionService.CreatePool(c.Request.Context(), user.ID, &req)
	if err != nil {
		SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Pool Created Successfully",
		"poolId":     pool.ID,
		"inviteCode": pool.InviteCode,
	})
}

func (p *PredictionController) JoinPool(c *gin.Context) {
	user, ok := GetAuthUser(c)
	if !ok {
		return
	}

	var req requests.JoinPoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := p.predictionService.JoinPool(c.Request.Context(), user.ID, &req); err != nil {
		SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	SendSuccess(c, http.StatusOK, "Successfully joined the pool")
}

func (p *PredictionController) LeavePool(c *gin.Context) {
	user, ok := GetAuthUser(c)
	if !ok {
		return
	}

	poolID, err := uuid.Parse(c.Param("poolId"))
	if err != nil {
		SendError(c, http.StatusBadRequest, "Invalid pool ID")
		return
	}

	if err := p.predictionService.LeaveOrDeletePool(c.Request.Context(), user.ID, poolID); err != nil {
		SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	SendSuccess(c, http.StatusOK, "Successfully left or dissolved the pool")
}

func (p *PredictionController) KickMember(c *gin.Context) {
	user, ok := GetAuthUser(c)
	if !ok {
		return
	}

	poolID, err := uuid.Parse(c.Param("poolId"))
	if err != nil {
		SendError(c, http.StatusBadRequest, "Invalid pool ID")
		return
	}

	memberID, err := uuid.Parse(c.Param("memberId"))
	if err != nil {
		SendError(c, http.StatusBadRequest, "Invalid member ID")
		return
	}

	if err := p.predictionService.KickMemberOrDeletePool(c.Request.Context(), user.ID, poolID, memberID); err != nil {
		SendError(c, http.StatusForbidden, err.Error())
		return
	}

	SendSuccess(c, http.StatusOK, "Member removed or pool dissolved successfully")
}