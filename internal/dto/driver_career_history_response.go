package dto

type DriverCareerHistoryResponse struct {
	SeasonName string `json:"seasonName"`
	TeamName   string `json:"teamName"`
	Position   string `json:"position"`
	Points     string `json:"points"`
}