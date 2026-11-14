package security

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

const defaultProductionOrigin = "https://f1.michielve.be"

type TokenValidator interface {
	ValidateToken(token string) (any, bool)
}

// loadAllowedOrigins builds the CORS allow-list from ALLOWED_ORIGINS and
// FRONTEND_URL (comma separated), plus http://localhost:4200 outside production.
//
// The preflight handler used to echo back whatever Origin the request sent
// together with Access-Control-Allow-Credentials: true, so any website was
// "allowed" to make credentialed calls to the API.
func loadAllowedOrigins() map[string]struct{} {
	origins := make(map[string]struct{})
	add := func(raw string) {
		for _, o := range strings.Split(raw, ",") {
			o = strings.TrimRight(strings.TrimSpace(o), "/")
			if o != "" {
				origins[o] = struct{}{}
			}
		}
	}
	add(os.Getenv("ALLOWED_ORIGINS"))
	add(os.Getenv("FRONTEND_URL"))
	if os.Getenv("BASE_URL") == "" { // not production
		origins["http://localhost:4200"] = struct{}{}
	}
	if len(origins) == 0 {
		// Same origin the API Gateway error responses already allow (infra/modules/api_gateway).
		origins[defaultProductionOrigin] = struct{}{}
		log.Printf("WARN: FRONTEND_URL / ALLOWED_ORIGINS not set, falling back to %s", defaultProductionOrigin)
	}
	return origins
}

func JwtFilter(jwtService TokenValidator) gin.HandlerFunc {
	allowedOrigins := loadAllowedOrigins()

	return func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			origin := c.GetHeader("Origin")
			c.Header("Vary", "Origin")
			if _, ok := allowedOrigins[origin]; ok {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With, Accept, Origin")
				c.Header("Access-Control-Allow-Credentials", "true")
				c.Header("Access-Control-Max-Age", "600")
			}
			c.AbortWithStatus(http.StatusNoContent)
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
