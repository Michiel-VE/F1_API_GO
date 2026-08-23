package exceptions

import (
	"fmt"
	"net/http"
	"time"

	"f1-api-go/internal/dto"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

func GlobalExceptionHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err

			if validationErrs, ok := err.(validator.ValidationErrors); ok {
				out := make(map[string]string)
				for _, fe := range validationErrs {
					out[fe.Field()] = fmt.Sprintf("Failed on validation tag: %s", fe.Tag())
				}
				c.JSON(http.StatusBadRequest, out)
				return
			}

			errorResponse := dto.ErrorResponse{
				Message:   err.Error(),
				Status:    http.StatusBadRequest,
				Timestamp: time.Now().Format("2006-01-02T15:04:05"),
			}
			c.JSON(http.StatusBadRequest, errorResponse)
		}
	}
}