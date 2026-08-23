package response

type ErrorResponse struct {
	Message   string `json:"message"`
	Status    int    `json:"status"`
	Timestamp string `json:"timestamp"`
}