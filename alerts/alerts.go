package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const resendEndpoint = "https://api.resend.com/emails"

type Config struct {
	APIKey string
	From   string
	To     []string
}

type Notifier struct {
	cfg        Config
	httpClient *http.Client
}

func NewNotifier(cfg Config) *Notifier {
	return &Notifier{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (n *Notifier) NotifyScraperFailure(ctx context.Context, scraperName string, seasonYear string, scrapeErr error) error {
	if scrapeErr == nil || n.cfg.APIKey == "" || n.cfg.From == "" || len(n.cfg.To) == 0 {
		return nil
	}

	subject := fmt.Sprintf("Scraper Failure: %s", scraperName)
	if seasonYear != "" {
		subject = fmt.Sprintf("Scraper Failure: %s (%s)", scraperName, seasonYear)
	}

	seasonLabel := seasonYear
	if seasonLabel == "" {
		seasonLabel = "(default)"
	}

	body := fmt.Sprintf(
		"Scraper: %s\nSeason: %s\nTime (UTC): %s\n\nError:\n%s\n",
		scraperName,
		seasonLabel,
		time.Now().UTC().Format(time.RFC3339),
		scrapeErr.Error(),
	)

	payload := struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text"`
	}{
		From:    n.cfg.From,
		To:      n.cfg.To,
		Subject: subject,
		Text:    body,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendEndpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+n.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		detail := strings.TrimSpace(string(errBody))
		if detail != "" {
			return fmt.Errorf("resend API returned status %d: %s", resp.StatusCode, detail)
		}
		return fmt.Errorf("resend API returned status %d", resp.StatusCode)
	}
	return nil
}
