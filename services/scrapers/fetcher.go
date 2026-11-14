package scrapers

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	maxFetchAttempts = 3
	maxBodyBytes     = 10 << 20 // 10 MB
	scraperUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
)

// pageFetcher is the single HTTP + HTML-parsing path used by every scraper.
// It always checks the status code (a 403/404 page used to be parsed as if it
// were real content and silently produced "0 results"), retries transient
// failures, and caps the body size.
type pageFetcher struct {
	client *http.Client
	logger *slog.Logger
}

func newPageFetcher(timeout time.Duration, logger *slog.Logger) *pageFetcher {
	return &pageFetcher{
		client: &http.Client{Timeout: timeout},
		logger: logger,
	}
}

// fetchDocument retries network errors, 429 and 5xx with a short backoff.
// Permanent failures (404, parse errors) return immediately.
func (f *pageFetcher) fetchDocument(ctx context.Context, url string) (*goquery.Document, error) {
	var lastErr error
	for attempt := 1; attempt <= maxFetchAttempts; attempt++ {
		doc, retryable, err := f.fetchOnce(ctx, url)
		if err == nil {
			return doc, nil
		}
		lastErr = err
		if !retryable || attempt == maxFetchAttempts {
			break
		}
		backoff := time.Duration(attempt) * 2 * time.Second
		f.logger.Warn("Fetch failed, retrying",
			slog.String("url", url),
			slog.Int("attempt", attempt),
			slog.Duration("backoff", backoff),
			slog.String("error", err.Error()))
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}
	return nil, lastErr
}

func (f *pageFetcher) fetchOnce(ctx context.Context, url string) (*goquery.Document, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", scraperUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, true, fmt.Errorf("request to %s failed: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return nil, retryable, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	doc, err := goquery.NewDocumentFromReader(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, false, fmt.Errorf("failed to parse html from %s: %w", url, err)
	}
	return doc, false, nil
}
