package security

import (
	"encoding/base64"
	"encoding/json"
	"os"

	"github.com/gin-gonic/gin"
)

const (
	OAuth2AuthorizationRequestCookieName = "oauth2_auth_request"
	RedirectURIParamCookieName           = "redirect_uri"
	CookieExpireSeconds                  = 180
)

type OAuth2AuthorizationRequest struct {
	ClientID             string                 `json:"clientId"`
	AuthorizationURI     string                 `json:"authorizationUri"`
	RedirectURI          string                 `json:"redirectUri"`
	Scopes               []string               `json:"scopes"`
	State                string                 `json:"state"`
	AdditionalParameters map[string]interface{} `json:"additionalParameters"`
	Attributes           map[string]interface{} `json:"attributes"`
}

type OAuth2CookieRepository struct{}

// Type alias so oauth2_authentication_success_handler.go compiles without edits
type HttpCookieOAuth2AuthorizationRequestRepository = OAuth2CookieRepository

func NewOAuth2CookieRepository() *OAuth2CookieRepository {
	return &OAuth2CookieRepository{}
}

func (r *OAuth2CookieRepository) GetCookie(c *gin.Context, name string) (string, error) {
	return c.Cookie(name)
}

func (r *OAuth2CookieRepository) LoadAuthorizationRequest(c *gin.Context) (*OAuth2AuthorizationRequest, error) {
	cookie, err := c.Cookie(OAuth2AuthorizationRequestCookieName)
	if err != nil || cookie == "" {
		return nil, nil
	}
	return r.deserialize(cookie)
}

func (r *OAuth2CookieRepository) SaveAuthorizationRequest(c *gin.Context, authRequest *OAuth2AuthorizationRequest) error {
	if authRequest == nil {
		r.DeleteCookie(c, OAuth2AuthorizationRequestCookieName)
		r.DeleteCookie(c, RedirectURIParamCookieName)
		return nil
	}

	isProduction := os.Getenv("BASE_URL") != ""
	serialized, err := r.serialize(authRequest)
	if err != nil {
		return err
	}

	r.addCookie(c, OAuth2AuthorizationRequestCookieName, serialized, CookieExpireSeconds, isProduction)

	redirectURI := c.Query(RedirectURIParamCookieName)
	if redirectURI != "" {
		r.addCookie(c, RedirectURIParamCookieName, redirectURI, CookieExpireSeconds, isProduction)
	}

	return nil
}

func (r *OAuth2CookieRepository) RemoveAuthorizationRequest(c *gin.Context) (*OAuth2AuthorizationRequest, error) {
	req, err := r.LoadAuthorizationRequest(c)
	if err != nil {
		return nil, err
	}
	r.DeleteCookie(c, OAuth2AuthorizationRequestCookieName)
	r.DeleteCookie(c, RedirectURIParamCookieName)
	return req, nil
}

func (r *OAuth2CookieRepository) DeleteCookie(c *gin.Context, name string) {
	isProduction := os.Getenv("BASE_URL") != ""
	r.addCookie(c, name, "", -1, isProduction)
}

func (r *OAuth2CookieRepository) addCookie(c *gin.Context, name, value string, maxAge int, isProduction bool) {
	c.SetCookie(
		name,
		value,
		maxAge,
		"/",
		"",
		isProduction,
		true, // HttpOnly
	)
}

func (r *OAuth2CookieRepository) serialize(authRequest *OAuth2AuthorizationRequest) (string, error) {
	bytes, err := json.Marshal(authRequest)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

func (r *OAuth2CookieRepository) deserialize(cookieValue string) (*OAuth2AuthorizationRequest, error) {
	bytes, err := base64.URLEncoding.DecodeString(cookieValue)
	if err != nil {
		return nil, err
	}

	var authReq OAuth2AuthorizationRequest
	if err := json.Unmarshal(bytes, &authReq); err != nil {
		return nil, err
	}

	return &authReq, nil
}