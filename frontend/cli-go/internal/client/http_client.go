package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Options struct {
	Timeout   time.Duration
	AuthToken string
	UserAgent string
}

type Client struct {
	baseURL   string
	http      *http.Client
	authToken string
	userAgent string
}

func New(baseURL string, opts Options) *Client {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		baseURL:   normalizeBaseURL(baseURL),
		http:      &http.Client{Timeout: timeout},
		authToken: opts.AuthToken,
		userAgent: opts.UserAgent,
	}
}

func (c *Client) Health(ctx context.Context) (HealthResponse, error) {
	var out HealthResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v1/health", false, &out); err != nil {
		return out, err
	}
	return out, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, useAuth bool, out any) error {
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return err
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if useAuth && c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	decoder := json.NewDecoder(resp.Body)
	return decoder.Decode(out)
}

func normalizeBaseURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return value
	}
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		value = "http://" + value
	}
	return strings.TrimRight(value, "/")
}
