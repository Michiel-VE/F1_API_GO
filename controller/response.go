package controller

import (
	"errors"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
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

// SendServiceError turns a service error into an HTTP response.
//
// Known business errors (the three sentinels in the services package) are
// shown to the client with a fitting status code. Anything else is an
// unexpected failure (database error, bug, ...): it is logged on the server and
// the client only gets a generic message. Handlers used to send err.Error()
// for these, which exposed SQL and internal details.
func SendServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrNotFound):
		SendError(c, http.StatusNotFound, publicMessage(err, services.ErrNotFound))
	case errors.Is(err, services.ErrAccessDenied):
		SendError(c, http.StatusForbidden, publicMessage(err, services.ErrAccessDenied))
	case errors.Is(err, services.ErrInvalidState):
		SendError(c, http.StatusBadRequest, publicMessage(err, services.ErrInvalidState))
	default:
		log.Printf("ERROR %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
		SendError(c, http.StatusInternalServerError, "Internal server error")
	}
}

// publicMessage drops the "resource not found: " style prefix of a wrapped sentinel.
func publicMessage(err, sentinel error) string {
	msg := strings.TrimPrefix(err.Error(), sentinel.Error()+": ")
	if msg == "" {
		return sentinel.Error()
	}
	return msg
}

var seasonPattern = regexp.MustCompile(`^\d{4}$`)

// ValidSeason accepts a 4-digit year in a sane range. Seasons end up in Redis
// cache keys ("drivers::season:<value>"), so without this check anyone could
// create unlimited cache entries by requesting /drivers/season/<random text>.
func ValidSeason(season string) bool {
	if !seasonPattern.MatchString(season) {
		return false
	}
	year, err := strconv.Atoi(season)
	return err == nil && year >= 1950 && year <= time.Now().Year()+1
}

// RequireSeason sends a 400 and returns false when the season is invalid.
func RequireSeason(c *gin.Context, season string) bool {
	if !ValidSeason(season) {
		SendError(c, http.StatusBadRequest, "Invalid season: expected a 4-digit year")
		return false
	}
	return true
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
