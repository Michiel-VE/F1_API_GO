package response

import "github.com/google/uuid"

type MemberPredictionDTO struct {
	UserID           uuid.UUID `json:"userId"`
	Username         string    `json:"username"`
	Picture          string    `json:"picture"`
	PredictedTeamIDs []string  `json:"predictedTeamIds"`
}

type PoolSummaryResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	MemberCount int       `json:"memberCount"`
	CreatorID   uuid.UUID `json:"creatorId"`
	InviteCode  string    `json:"inviteCode"`
}

type PoolDetailsResponse struct {
	PoolID      uuid.UUID             `json:"poolId"`
	PoolName    string                `json:"poolName"`
	CreatorID   uuid.UUID             `json:"creatorId"`
	LeaderBoard []MemberPredictionDTO `json:"leaderBoard"`
}