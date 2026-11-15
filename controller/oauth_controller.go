package controller

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"f1-api-go/security"
	"f1-api-go/services"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

const (
	oauthStateCookie    = "oauth2_state"
	oauthRedirectCookie = "redirect_uri"
	oauthCookieMaxAge   = 600 // seconds
)

// OAuthController implements "Sign in with Google" using the same URLs the old
// Spring app used, so the frontend button and the Google console settings keep working:
//
//	GET /oauth2/authorization/google?redirect_uri=/   -> start
//	GET /login/oauth2/code/google?code=..&state=..    -> callback (registered redirect URI)
type OAuthController struct {
	oauthConfig  *oauth2.Config
	userInfoURI  string
	oauthService *services.OAuthService
	authService  *services.AuthService
}

func NewOAuthController(cfg *oauth2.Config, userInfoURI string, oauthService *services.OAuthService, authService *services.AuthService) *OAuthController {
	return &OAuthController{
		oauthConfig:  cfg,
		userInfoURI:  userInfoURI,
		oauthService: oauthService,
		authService:  authService,
	}
}

type googleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func frontendBaseURL() string {
	if u := strings.TrimRight(strings.TrimSpace(os.Getenv("FRONTEND_URL")), "/"); u != "" {
		return u
	}
	if strings.EqualFold(os.Getenv("ENV"), "prod") {
		return "https://f1.michielve.be"
	}
	return "http://localhost:4200"
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// setTempCookie stores short-lived values. SameSite=Lax (not Strict) is needed:
// Google sends the user back with a top-level GET, which Lax allows.
func (o *OAuthController) setTempCookie(c *gin.Context, name, value string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(name, value, maxAge, "/", "", os.Getenv("BASE_URL") != "", true)
}

func (o *OAuthController) redirectToLoginError(c *gin.Context, code string) {
	c.Redirect(http.StatusFound, frontendBaseURL()+"/login?error="+url.QueryEscape(code))
}

// Start redirects the browser to Google.
func (o *OAuthController) Start(c *gin.Context) {
	if o.oauthConfig == nil || o.oauthConfig.ClientID == "" || o.oauthConfig.ClientSecret == "" {
		log.Printf("ERROR: Google OAuth is not configured (missing google_client_id / google_client_secret)")
		o.redirectToLoginError(c, "oauth_not_configured")
		return
	}

	state, err := randomToken(32)
	if err != nil {
		log.Printf("ERROR: generating oauth state failed: %v", err)
		o.redirectToLoginError(c, "oauth_failed")
		return
	}

	returnPath := c.Query("redirect_uri")
	if !security.IsSafeReturnPath(returnPath) {
		returnPath = "/"
	}

	o.setTempCookie(c, oauthStateCookie, state, oauthCookieMaxAge)
	o.setTempCookie(c, oauthRedirectCookie, returnPath, oauthCookieMaxAge)

	c.Redirect(http.StatusFound, o.oauthConfig.AuthCodeURL(state))
}

// Callback finishes the login: checks state, exchanges the code, reads the
// Google profile, finds or creates the user and sets the session cookie.
func (o *OAuthController) Callback(c *gin.Context) {
	if o.oauthConfig == nil {
		o.redirectToLoginError(c, "oauth_not_configured")
		return
	}

	expectedState, stateErr := c.Cookie(oauthStateCookie)
	returnPath, _ := c.Cookie(oauthRedirectCookie)
	// Both cookies are single-use.
	o.setTempCookie(c, oauthStateCookie, "", -1)
	o.setTempCookie(c, oauthRedirectCookie, "", -1)

	if !security.IsSafeReturnPath(returnPath) {
		returnPath = "/"
	}

	if c.Query("error") != "" {
		log.Printf("INFO: google sign-in cancelled or denied: %s", c.Query("error"))
		o.redirectToLoginError(c, "oauth_denied")
		return
	}

	state := c.Query("state")
	if stateErr != nil || expectedState == "" || subtle.ConstantTimeCompare([]byte(state), []byte(expectedState)) != 1 {
		log.Printf("WARN: google sign-in rejected: state mismatch")
		o.redirectToLoginError(c, "invalid_state")
		return
	}

	code := c.Query("code")
	if code == "" {
		o.redirectToLoginError(c, "oauth_failed")
		return
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)

	token, err := o.oauthConfig.Exchange(ctx, code)
	if err != nil {
		log.Printf("ERROR: google code exchange failed: %v", err)
		o.redirectToLoginError(c, "oauth_failed")
		return
	}

	info, err := o.fetchUserInfo(ctx, token)
	if err != nil {
		log.Printf("ERROR: reading google profile failed: %v", err)
		o.redirectToLoginError(c, "oauth_failed")
		return
	}
	if info.Sub == "" || info.Email == "" || !info.EmailVerified {
		log.Printf("WARN: google profile rejected (missing subject/email or email not verified)")
		o.redirectToLoginError(c, "email_not_verified")
		return
	}

	user, err := o.oauthService.FindOrCreateGoogleUser(ctx, services.GoogleProfile{
		Subject: info.Sub,
		Email:   info.Email,
		Name:    info.Name,
		Picture: info.Picture,
	})
	if err != nil {
		if errors.Is(err, services.ErrAccountExists) {
			o.redirectToLoginError(c, "email_already_registered")
			return
		}
		log.Printf("ERROR: finding or creating user failed: %v", err)
		o.redirectToLoginError(c, "user_creation_failed")
		return
	}

	isProduction := os.Getenv("BASE_URL") != ""
	expirationMs := o.authService.TokenLifetime().Milliseconds()
	cookieHeader, err := o.authService.CookieHeaderForUser(user, isProduction, expirationMs)
	if err != nil {
		log.Printf("ERROR: creating session token failed: %v", err)
		o.redirectToLoginError(c, "token_generation_failed")
		return
	}

	// Add, not Set: the temporary cookies above were also set on this response.
	c.Writer.Header().Add("Set-Cookie", cookieHeader)
	c.Redirect(http.StatusFound, fmt.Sprintf("%s/login?returnUrl=%s", frontendBaseURL(), url.QueryEscape(returnPath)))
}

func (o *OAuthController) fetchUserInfo(ctx context.Context, token *oauth2.Token) (*googleUserInfo, error) {
	client := o.oauthConfig.Client(ctx, token)
	resp, err := client.Get(o.userInfoURI)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo returned status %d", resp.StatusCode)
	}

	var info googleUserInfo
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}
