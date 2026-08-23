package security

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type TokenValidator interface {
	ValidateToken(token string) (any, bool)
}

func JwtFilter(jwtService TokenValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Origin", c.GetHeader("Origin"))
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With, Accept, Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.AbortWithStatus(http.StatusOK)
			return
		}

		var jwtToken string

		authHeader := c.GetHeader("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			jwtToken = strings.TrimPrefix(authHeader, "Bearer ")
		}

		if jwtToken == "" {
			if cookie, err := c.Cookie("jwt"); err == nil {
				jwtToken = cookie
			}
		}

		if jwtToken == "" {
			c.Next()
			return
		}

		claims, valid := jwtService.ValidateToken(jwtToken)
		if valid {
			c.Set("userPrincipal", claims)
		}

		c.Next()
	}
}