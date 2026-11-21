package alerts

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/resend/resend-go/v4"
)

type Config struct {
	APIKey string
	From   string
	To     []string
}

type Notifier struct {
	cfg    Config
	client *resend.Client
}

func NewNotifier(cfg Config) *Notifier {
	var client *resend.Client
	if cfg.APIKey != "" {
		client = resend.NewClient(cfg.APIKey)
	}
	return &Notifier{
		cfg:    cfg,
		client: client,
	}
}

func (n *Notifier) NotifyScraperFailure(ctx context.Context, scraperName string, seasonYear string, scrapeErr error) error {
	if scrapeErr == nil {
		return nil
	}
	if n.client == nil || n.cfg.From == "" || len(n.cfg.To) == 0 {
		log.Printf(
			"WARN: Skipping Resend alert (hasAPIKey=%t, hasFrom=%t, toCount=%d)",
			n.client != nil,
			n.cfg.From != "",
			len(n.cfg.To),
		)
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

	params := &resend.SendEmailRequest{
		From:    n.cfg.From,
		To:      n.cfg.To,
		Subject: subject,
		Text:    body,
	}

	resp, err := n.client.Emails.SendWithContext(ctx, params)
	if err != nil {
		return err
	}
	if resp != nil {
		log.Printf("Sent Resend failure alert (id=%s)", resp.Id)
	}
	return nil
}
