package response

type SavedPredictionResponse struct {
	PredictedTeams []string `json:"predictedTeams"`
}

type PredictionStatusResponse struct {
	HasPools              bool `json:"hasPools"`
	HasPersonalPrediction bool `json:"hasPersonalPrediction"`
}