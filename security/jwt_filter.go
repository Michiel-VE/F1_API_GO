package security

import (
	"strings"

	"github.com/gin-gonic/gin"
)

type TokenValidator interface {
	ValidateToken(token string) (any, bool)
}

// JwtFilter authenticates a request when it carries a valid token (Bearer
// header or "jwt" cookie). It never rejects a request itself; routes that need
// a user are protected by RequireAuth / GetAuthUser.
//
// CORS is NOT handled here: the cors middleware in SecurityConfig runs first
// and answers every preflight, so the CORS code that used to live in this file
// never ran.
func JwtFilter(jwtService TokenValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
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
