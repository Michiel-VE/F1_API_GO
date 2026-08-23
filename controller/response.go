package controller

import (
	"net/http"
	"time"

	"f1-api-go/services"

	"github.com/gin-gonic/gin"
)

type ErrorResponse struct {
	Message    string `json:"message"`
	StatusCode int    `json:"statusCode"`
	Timestamp  string `json:"timestamp"`
}

func SendError(c *gin.Context, status int, message string) {
	c.JSON(status, ErrorResponse{
		Message:    message,
		StatusCode: status,
		Timestamp:  time.Now().Format(time.RFC3339),
	})
}

func SendSuccess(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{
		"message":    message,
		"statusCode": status,
		"timestamp":  time.Now().Format(time.RFC3339),
	})
}

func SendListResponse[T any](c *gin.Context, items []T) {
	if len(items) == 0 {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, items)
}

func GetAuthUser(c *gin.Context) (*services.CustomUserPrincipal, bool) {
	val, exists := c.Get("userPrincipal")
	if !exists {
		SendError(c, http.StatusUnauthorized, "Authentication required")
		return nil, false
	}
	user, ok := val.(*services.CustomUserPrincipal)
	if !ok {
		SendError(c, http.StatusInternalServerError, "Invalid user context")
		return nil, false
	}
	return user, true
}
