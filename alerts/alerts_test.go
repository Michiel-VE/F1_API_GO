package alerts

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type rewriteTransport struct {
	target *httptest.Server
	base   http.RoundTripper
}

// Redirects requests to the test server so tests do not hit the live Resend API.
func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	targetURL := t.target.URL
	if strings.HasPrefix(targetURL, "http://") {
		cloned.URL.Scheme = "http"
		cloned.URL.Host = strings.TrimPrefix(targetURL, "http://")
	}
	return t.base.RoundTrip(cloned)
}

func newTestNotifier(cfg Config, server *httptest.Server) *Notifier {
	n := NewNotifier(cfg)
	n.httpClient.Transport = &rewriteTransport{
		target: server,
		base:   http.DefaultTransport,
	}
	return n
}

func TestNotifyScraperFailure_SkipsWhenMissingInputOrDisabled(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	validCfg := Config{
		APIKey: "re_test_key",
		From:   "alerts@example.com",
		To:     []string{"ops@example.com"},
	}

	cases := []struct {
		name string
		cfg  Config
		err  error
	}{
		{"nil error", validCfg, nil},
		{"missing api key", Config{From: "a@b.com", To: []string{"c@d.com"}}, errors.New("boom")},
		{"missing from", Config{APIKey: "k", To: []string{"c@d.com"}}, errors.New("boom")},
		{"missing to", Config{APIKey: "k", From: "a@b.com"}, errors.New("boom")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called = false
			n := newTestNotifier(tc.cfg, server)
			if err := n.NotifyScraperFailure(context.Background(), "qualifying", "2026", tc.err); err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if called {
				t.Fatal("expected HTTP request not to be sent")
			}
		})
	}
}

func TestNotifyScraperFailure_SendsExpectedPayload(t *testing.T) {
	var gotAuth string
	var gotContentType string
	var gotPayload struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed reading request body: %v", err)
		}
		if err := json.Unmarshal(body, &gotPayload); err != nil {
			t.Errorf("failed decoding json payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := newTestNotifier(Config{
		APIKey: "re_secret_123",
		From:   "bot@example.com",
		To:     []string{"dev1@example.com", "dev2@example.com"},
	}, server)

	err := n.NotifyScraperFailure(context.Background(), "race_results", "", errors.New("timeout"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer re_secret_123" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer re_secret_123")
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", gotContentType, "application/json")
	}
	if gotPayload.Subject != "Scraper Failure: race_results" {
		t.Errorf("Subject = %q, want %q", gotPayload.Subject, "Scraper Failure: race_results")
	}
	if !strings.Contains(gotPayload.Text, "Season: (default)") {
		t.Errorf("expected default season label in body, got %q", gotPayload.Text)
	}
	if !strings.Contains(gotPayload.Text, "timeout") {
		t.Errorf("expected error message in body, got %q", gotPayload.Text)
	}
}

func TestNotifyScraperFailure_Non2xxResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream error"))
	}))
	defer server.Close()

	n := newTestNotifier(Config{
		APIKey: "re_secret_123",
		From:   "bot@example.com",
		To:     []string{"dev@example.com"},
	}, server)

	err := n.NotifyScraperFailure(context.Background(), "drivers", "2026", errors.New("parse failed"))
	if err == nil {
		t.Fatal("expected error for 502 response, got nil")
	}
	if !strings.Contains(err.Error(), "502") || !strings.Contains(err.Error(), "upstream error") {
		t.Errorf("unexpected error message: %v", err)
	}
}
