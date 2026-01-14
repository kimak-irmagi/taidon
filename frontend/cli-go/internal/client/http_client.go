package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

func (c *Client) ListNames(ctx context.Context, filters ListFilters) ([]NameEntry, error) {
	var out []NameEntry
	query := url.Values{}
	addFilter(query, "name", filters.Name)
	addFilter(query, "instance", filters.Instance)
	addFilter(query, "state", filters.State)
	addFilter(query, "image", filters.Image)
	if err := c.doJSON(ctx, http.MethodGet, appendQuery("/v1/names", query), true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ListInstances(ctx context.Context, filters ListFilters) ([]InstanceEntry, error) {
	var out []InstanceEntry
	query := url.Values{}
	addFilter(query, "instance", filters.Instance)
	addFilter(query, "state", filters.State)
	addFilter(query, "name", filters.Name)
	addFilter(query, "image", filters.Image)
	if err := c.doJSON(ctx, http.MethodGet, appendQuery("/v1/instances", query), true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ListStates(ctx context.Context, filters ListFilters) ([]StateEntry, error) {
	var out []StateEntry
	query := url.Values{}
	addFilter(query, "state", filters.State)
	addFilter(query, "kind", filters.Kind)
	addFilter(query, "image", filters.Image)
	if err := c.doJSON(ctx, http.MethodGet, appendQuery("/v1/states", query), true, &out); err != nil {
		return nil, err
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

func addFilter(values url.Values, key, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	values.Set(key, value)
}

func appendQuery(path string, values url.Values) string {
	if len(values) == 0 {
		return path
	}
	return path + "?" + values.Encode()
}
