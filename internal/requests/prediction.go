package requests

import "github.com/google/uuid"

type CreatePredictionRequest struct {
	RaceID           uuid.UUID   `json:"raceId" binding:"required"`
	PredictedDrivers []uuid.UUID `json:"predictedDrivers" binding:"required,len=10"`
}

type CreateSeasonPredictionRequest struct {
	PredictedTeams []uuid.UUID `json:"predictedTeams" binding:"required,min=1"`
	PoolID         *uuid.UUID  `json:"poolId,omitempty"`
}