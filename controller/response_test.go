package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"f1-api-go/services"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestContext(method, target string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, target, nil)
	return c, w
}

func TestSendErrorAndSendSuccess(t *testing.T) {
	c, w := newTestContext(http.MethodGet, "/test")
	SendError(c, http.StatusBadRequest, "bad input")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var errBody ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("failed to decode error JSON: %v", err)
	}
	if errBody.Message != "bad input" || errBody.StatusCode != http.StatusBadRequest || errBody.Timestamp == "" {
		t.Errorf("unexpected error response: %+v", errBody)
	}

	c2, w2 := newTestContext(http.MethodPost, "/ok")
	SendSuccess(c2, http.StatusCreated, "created")

	if w2.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", w2.Code, http.StatusCreated)
	}
	var okBody map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &okBody); err != nil {
		t.Fatalf("failed to decode success JSON: %v", err)
	}
	if okBody["message"] != "created" {
		t.Errorf("message = %v, want created", okBody["message"])
	}
}

func TestSendServiceError(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "not found wrapped",
			err:        fmt.Errorf("%w: driver not found", services.ErrNotFound),
			wantStatus: http.StatusNotFound,
			wantMsg:    "driver not found",
		},
		{
			name:       "access denied wrapped",
			err:        fmt.Errorf("%w: not pool owner", services.ErrAccessDenied),
			wantStatus: http.StatusForbidden,
			wantMsg:    "not pool owner",
		},
		{
			name:       "invalid state wrapped",
			err:        fmt.Errorf("%w: season locked", services.ErrInvalidState),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "season locked",
		},
		{
			name:       "unexpected internal error",
			err:        errors.New("sql: connection refused"),
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "Internal server error",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := newTestContext(http.MethodGet, "/items")
			SendServiceError(c, tc.err)

			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tc.wantStatus)
			}

			var body ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to decode JSON: %v", err)
			}
			if body.Message != tc.wantMsg {
				t.Errorf("message = %q, want %q", body.Message, tc.wantMsg)
			}
		})
	}
}

func TestValidSeasonAndRequireSeason(t *testing.T) {
	nextYear := fmt.Sprintf("%d", time.Now().Year()+1)
	tooFarYear := fmt.Sprintf("%d", time.Now().Year()+2)

	if !ValidSeason("2026") {
		t.Error("expected 2026 to be valid")
	}
	if !ValidSeason("1950") {
		t.Error("expected 1950 to be valid")
	}
	if !ValidSeason(nextYear) {
		t.Errorf("expected %s to be valid", nextYear)
	}
	if ValidSeason("1949") || ValidSeason(tooFarYear) || ValidSeason("26") || ValidSeason("abcd") {
		t.Error("expected invalid season strings to return false")
	}

	c, w := newTestContext(http.MethodGet, "/seasons/abc")
	if RequireSeason(c, "abc") {
		t.Fatal("expected RequireSeason to return false for invalid season")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSendListResponse(t *testing.T) {
	cEmpty, wEmpty := newTestContext(http.MethodGet, "/list")
	SendListResponse(cEmpty, []string{})
	if wEmpty.Code != http.StatusNoContent {
		t.Errorf("empty list status = %d, want %d", wEmpty.Code, http.StatusNoContent)
	}

	cItems, wItems := newTestContext(http.MethodGet, "/list")
	SendListResponse(cItems, []string{"VER", "LEC"})
	if wItems.Code != http.StatusOK {
		t.Errorf("non-empty list status = %d, want %d", wItems.Code, http.StatusOK)
	}
}

func TestGetAuthUser(t *testing.T) {
	cMissing, wMissing := newTestContext(http.MethodGet, "/me")
	if _, ok := GetAuthUser(cMissing); ok {
		t.Fatal("expected ok=false when userPrincipal is missing")
	}
	if wMissing.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", wMissing.Code, http.StatusUnauthorized)
	}

	cWrongType, wWrongType := newTestContext(http.MethodGet, "/me")
	cWrongType.Set("userPrincipal", "not-a-principal")
	if _, ok := GetAuthUser(cWrongType); ok {
		t.Fatal("expected ok=false when userPrincipal has wrong type")
	}
	if wWrongType.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", wWrongType.Code, http.StatusInternalServerError)
	}

	cValid, _ := newTestContext(http.MethodGet, "/me")
	expected := &services.CustomUserPrincipal{Username: "driver@example.com"}
	cValid.Set("userPrincipal", expected)
	got, ok := GetAuthUser(cValid)
	if !ok || got != expected {
		t.Errorf("GetAuthUser = (%+v, %v), want (%+v, true)", got, ok, expected)
	}
}
