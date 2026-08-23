package security

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"

	"github.com/gin-gonic/gin"
)

type OAuth2JwtService interface {
	GenerateToken(user interface{}) (string, error)
	GetExpiration() int64
}

type OAuth2AuthService interface {
	FindOrCreateUser(provider string, providerID string, attributes map[string]interface{}) (interface{}, error)
	BuildCookieHeader(token string, isProduction bool, maxAgeSeconds int) string
}

type OAuth2AuthenticationSuccessHandler struct {
	jwtService       OAuth2JwtService
	authService      OAuth2AuthService
	cookieRepository *HttpCookieOAuth2AuthorizationRequestRepository
}

func NewOAuth2AuthenticationSuccessHandler(
	jwtService OAuth2JwtService,
	authService OAuth2AuthService,
	cookieRepository *HttpCookieOAuth2AuthorizationRequestRepository,
) *OAuth2AuthenticationSuccessHandler {
	return &OAuth2AuthenticationSuccessHandler{
		jwtService:       jwtService,
		authService:      authService,
		cookieRepository: cookieRepository,
	}
}

func (h *OAuth2AuthenticationSuccessHandler) HandleSuccess(c *gin.Context, provider string, providerID string, attributes map[string]interface{}) {
	envFrontendURL := os.Getenv("FRONTEND_URL")
	baseURL := "http://localhost:4200"
	if envFrontendURL != "" {
		baseURL = envFrontendURL
	}

	cookieReturnURL, err := h.cookieRepository.GetCookie(c, RedirectURIParamCookieName)
	if err != nil || cookieReturnURL == "" {
		cookieReturnURL = "/"
	}

	re := regexp.MustCompile(`^/[^/].*`)
	finalReturnURL := "/"
	if re.MatchString(cookieReturnURL) || cookieReturnURL == "/" {
		finalReturnURL = cookieReturnURL
	}

	h.cookieRepository.DeleteCookie(c, RedirectURIParamCookieName)

	user, err := h.authService.FindOrCreateUser(provider, providerID, attributes)
	if err != nil || user == nil {
		c.Redirect(http.StatusFound, baseURL+"/login?error=user_creation_failed")
		return
	}

	token, err := h.jwtService.GenerateToken(user)
	if err != nil {
		c.Redirect(http.StatusFound, baseURL+"/login?error=token_generation_failed")
		return
	}

	isProduction := envFrontendURL != ""
	cookieHeader := h.authService.BuildCookieHeader(token, isProduction, int(h.jwtService.GetExpiration()/1000))
	c.Header("Set-Cookie", cookieHeader)

	targetURL := fmt.Sprintf("%s/login?returnUrl=%s", baseURL, url.QueryEscape(finalReturnURL))
	c.Redirect(http.StatusFound, targetURL)
}