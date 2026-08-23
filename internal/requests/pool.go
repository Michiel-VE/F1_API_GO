package requests

type CreatePoolRequest struct {
	Name string `json:"name" binding:"required"`
}

type JoinPoolRequest struct {
	InviteCode string `json:"inviteCode" binding:"required"`
}