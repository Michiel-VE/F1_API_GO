package alerts

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func newTestNotifier(cfg Config, server *httptest.Server) *Notifier {
	n := NewNotifier(cfg)
	if n.client != nil {
		u, err := url.Parse(server.URL)
		if err == nil {
			n.client.BaseURL = u
		}
	}
	return n
}

func TestNotifyScraperFailure_SkipsWhenMissingInputOrDisabled(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"email_123"}`))
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"email_123"}`))
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

func TestNotifyScraperFailure_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"message":"upstream error"}`))
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
}
