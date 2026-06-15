// Package walmart is the library behind the walmart command line: an HTTP client
// for Walmart's public web pages and typeahead host, an optional Affiliate API
// backend, and the typed records every command emits.
//
// Walmart renders its public pages server-side and ships the data as a
// __NEXT_DATA__ JSON island, so this client GETs pages and reads that island;
// the typeahead host answers plain JSON. Walmart fronts its estate with the
// PerimeterX (HUMAN) bot manager, which walls almost everything from datacenter
// IPs: only the typeahead host and the homepage answer anonymously, while the
// product page (/ip/), keyword search (/search), the category pages (/cp/,
// /browse), and the store pages (/store/) return the "Robot or human?" challenge.
// So the client is two-layered: a reliable core over the open surfaces, and a
// best-effort layer over the walled ones that returns ErrBlocked when the wall is
// up and, if the operator has set Affiliate API credentials, falls back to that
// documented backend. Each surface lives in its own file (search.go, product.go,
// category.go, store.go, deals.go, suggest.go) with its parsing and record
// mapping; this file holds the shared web client.
package walmart

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Client talks to Walmart's public web pages. It paces requests, retries the
// transient failures, detects the bot wall, and caches response bodies on disk
// keyed by the request URL.
type Client struct {
	HTTP      *http.Client
	BaseURL   string
	UserAgent string
	Delay     time.Duration
	Retries   int

	// SuggestURL is the typeahead endpoint. It defaults to the public host; tests
	// point it at an httptest server.
	SuggestURL string

	// api is the optional Affiliate API backend, non-nil only when affiliate
	// credentials are configured. The walled surfaces fall back to it.
	api *apiClient

	cache   *cache
	refresh bool

	mu   sync.Mutex
	last time.Time
}

// NewClient builds a client from cfg.
func NewClient(cfg Config) *Client {
	c := &Client{
		HTTP:       &http.Client{Timeout: cfg.Timeout},
		BaseURL:    cfg.BaseURL,
		UserAgent:  cfg.UserAgent,
		Delay:      cfg.Delay,
		Retries:    cfg.Retries,
		SuggestURL: typeaheadURL,
		refresh:    cfg.Refresh,
	}
	if c.BaseURL == "" {
		c.BaseURL = BaseURL
	}
	if c.UserAgent == "" {
		c.UserAgent = DefaultUserAgent
	}
	// --refresh keeps the cache (so it is rewritten) but skips reads. --no-cache
	// drops it entirely.
	if !cfg.NoCache {
		c.cache = newCache(cfg.CacheDir, cfg.CacheTTL)
	}
	if cfg.ConsumerID != "" && (cfg.PrivateKey != "" || cfg.PrivateKeyFile != "") {
		c.api = newAPIClient(cfg)
	}
	return c
}

// get fetches url and returns the response body: paced, retried, cached, and
// wall-checked.
func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	if !c.refresh {
		if b, ok := c.cache.get(url); ok {
			return b, nil
		}
	}
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url, nil)
		if err == nil {
			c.cache.put(url, body)
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, lastErr
}

// do performs one GET and returns the body. retry reports whether the failure is
// worth another attempt. header, when non-nil, is applied to the request (the
// typeahead path sets its own).
func (c *Client) do(ctx context.Context, url string, header http.Header) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Set(k, v)
		}
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		// A connection reset mid-handshake is how the wall sometimes drops a
		// datacenter request; treat a transport error as retryable.
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusOK:
		// fall through to read and check the body
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, true, ErrRateLimited
	case resp.StatusCode == http.StatusForbidden,
		resp.StatusCode == http.StatusPreconditionFailed:
		// 403 and 412 are the two statuses PerimeterX serves its challenge with.
		return nil, false, ErrBlocked
	case resp.StatusCode == http.StatusNotFound, resp.StatusCode == http.StatusGone:
		return nil, false, ErrNotFound
	case resp.StatusCode >= 500:
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	default:
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	if isInterstitial(b) {
		return nil, false, ErrBlocked
	}
	return b, false, nil
}

// isInterstitial reports whether a 200 body is in fact the PerimeterX bot wall,
// which Walmart serves with a 200 status as often as a 403 or 412. The markers
// are the challenge page title and the captcha widget and script paths the wall
// injects.
func isInterstitial(b []byte) bool {
	return bytes.Contains(b, []byte("Robot or human?")) ||
		bytes.Contains(b, []byte("px-captcha")) ||
		bytes.Contains(b, []byte(`"redirectUrl":"/blocked`)) ||
		bytes.Contains(b, []byte(`jsClientSrc":"/px/`)) ||
		bytes.Contains(b, []byte(`appId":"PX`))
}

// pace blocks until at least Delay has passed since the previous request.
func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Delay <= 0 {
		c.last = time.Now()
		return
	}
	if wait := c.Delay - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// ClearCache removes the on-disk cache.
func (c *Client) ClearCache() error { return c.cache.clear() }
