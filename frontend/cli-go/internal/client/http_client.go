package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	client := &http.Client{Timeout: timeout}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) == 0 {
			return nil
		}
		prev := via[len(via)-1]
		if auth := prev.Header.Get("Authorization"); auth != "" {
			req.Header.Set("Authorization", auth)
		}
		if ua := prev.Header.Get("User-Agent"); ua != "" {
			req.Header.Set("User-Agent", ua)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return &Client{
		baseURL:   normalizeBaseURL(baseURL),
		http:      client,
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
	addFilter(query, "instance", filters.Instance)
	addFilter(query, "state", filters.State)
	addFilter(query, "image", filters.Image)
	if err := c.doJSON(ctx, http.MethodGet, appendQuery("/v1/names", query), true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetName(ctx context.Context, name string) (NameEntry, bool, error) {
	path := "/v1/names/" + url.PathEscape(strings.TrimSpace(name))
	var out NameEntry
	found, err := c.doJSONOptional(ctx, http.MethodGet, path, true, &out)
	return out, found, err
}

func (c *Client) ListInstances(ctx context.Context, filters ListFilters) ([]InstanceEntry, error) {
	var out []InstanceEntry
	query := url.Values{}
	addFilter(query, "state", filters.State)
	addFilter(query, "image", filters.Image)
	addFilter(query, "id_prefix", filters.IDPrefix)
	if err := c.doJSON(ctx, http.MethodGet, appendQuery("/v1/instances", query), true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetInstance(ctx context.Context, idOrName string) (InstanceEntry, bool, error) {
	path := "/v1/instances/" + url.PathEscape(strings.TrimSpace(idOrName))
	var out InstanceEntry
	found, err := c.doJSONOptional(ctx, http.MethodGet, path, true, &out)
	return out, found, err
}

func (c *Client) ListStates(ctx context.Context, filters ListFilters) ([]StateEntry, error) {
	var out []StateEntry
	query := url.Values{}
	addFilter(query, "kind", filters.Kind)
	addFilter(query, "image", filters.Image)
	addFilter(query, "id_prefix", filters.IDPrefix)
	if err := c.doJSON(ctx, http.MethodGet, appendQuery("/v1/states", query), true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetState(ctx context.Context, stateID string) (StateEntry, bool, error) {
	path := "/v1/states/" + url.PathEscape(strings.TrimSpace(stateID))
	var out StateEntry
	found, err := c.doJSONOptional(ctx, http.MethodGet, path, true, &out)
	return out, found, err
}

func (c *Client) DeleteInstance(ctx context.Context, instanceID string, opts DeleteOptions) (DeleteResult, int, error) {
	return c.deleteWithOptions(ctx, "/v1/instances/"+url.PathEscape(strings.TrimSpace(instanceID)), opts, false)
}

func (c *Client) DeleteState(ctx context.Context, stateID string, opts DeleteOptions) (DeleteResult, int, error) {
	return c.deleteWithOptions(ctx, "/v1/states/"+url.PathEscape(strings.TrimSpace(stateID)), opts, true)
}

func (c *Client) CreatePrepareJob(ctx context.Context, req PrepareJobRequest) (PrepareJobAccepted, error) {
	var out PrepareJobAccepted
	body, err := json.Marshal(req)
	if err != nil {
		return out, err
	}
	resp, err := c.doRequestWithBody(ctx, http.MethodPost, "/v1/prepare-jobs", true, bytes.NewReader(body), "application/json")
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		return out, parseErrorResponse(resp)
	}
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&out); err != nil {
		return out, err
	}
	return out, nil
}

func (c *Client) GetPrepareJob(ctx context.Context, jobID string) (PrepareJobStatus, bool, error) {
	path := "/v1/prepare-jobs/" + url.PathEscape(strings.TrimSpace(jobID))
	var out PrepareJobStatus
	found, err := c.doJSONOptional(ctx, http.MethodGet, path, true, &out)
	return out, found, err
}

func (c *Client) ListPrepareJobs(ctx context.Context, jobID string) ([]PrepareJobEntry, error) {
	var out []PrepareJobEntry
	query := url.Values{}
	addFilter(query, "job", jobID)
	if err := c.doJSON(ctx, http.MethodGet, appendQuery("/v1/prepare-jobs", query), true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) DeletePrepareJob(ctx context.Context, jobID string, opts DeleteOptions) (DeleteResult, int, error) {
	return c.deleteWithOptions(ctx, "/v1/prepare-jobs/"+url.PathEscape(strings.TrimSpace(jobID)), opts, false)
}

func (c *Client) ListTasks(ctx context.Context, jobID string) ([]TaskEntry, error) {
	var out []TaskEntry
	query := url.Values{}
	addFilter(query, "job", jobID)
	if err := c.doJSON(ctx, http.MethodGet, appendQuery("/v1/tasks", query), true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) RunCommand(ctx context.Context, req RunRequest) (io.ReadCloser, error) {
	var body []byte
	if req.Args == nil {
		req.Args = []string{}
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	body = data
	resp, err := c.doRequestWithBody(ctx, http.MethodPost, "/v1/runs", true, bytes.NewReader(body), "application/json")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, parseErrorResponse(resp)
	}
	return resp.Body, nil
}

func (c *Client) GetConfig(ctx context.Context, path string, effective bool) (any, error) {
	query := url.Values{}
	addFilter(query, "path", path)
	if effective {
		query.Set("effective", "true")
	}
	if strings.TrimSpace(path) == "" {
		var out map[string]any
		if err := c.doJSON(ctx, http.MethodGet, appendQuery("/v1/config", query), true, &out); err != nil {
			return nil, err
		}
		return out, nil
	}
	var out ConfigValue
	if err := c.doJSON(ctx, http.MethodGet, appendQuery("/v1/config", query), true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) SetConfig(ctx context.Context, req ConfigValue) (ConfigValue, error) {
	var out ConfigValue
	body, err := json.Marshal(req)
	if err != nil {
		return out, err
	}
	resp, err := c.doRequestWithBody(ctx, http.MethodPatch, "/v1/config", true, bytes.NewReader(body), "application/json")
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out, parseErrorResponse(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	return out, nil
}

func (c *Client) RemoveConfig(ctx context.Context, path string) (ConfigValue, error) {
	var out ConfigValue
	query := url.Values{}
	addFilter(query, "path", path)
	resp, err := c.doRequest(ctx, http.MethodDelete, appendQuery("/v1/config", query), true)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out, parseErrorResponse(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	return out, nil
}

func (c *Client) GetConfigSchema(ctx context.Context) (any, error) {
	var out map[string]any
	if err := c.doJSON(ctx, http.MethodGet, "/v1/config/schema", true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, useAuth bool, out any) error {
	resp, err := c.doRequest(ctx, method, path, useAuth)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPStatusError{StatusCode: resp.StatusCode, Status: resp.Status}
	}

	decoder := json.NewDecoder(resp.Body)
	return decoder.Decode(out)
}

func (c *Client) doJSONOptional(ctx context.Context, method, path string, useAuth bool, out any) (bool, error) {
	resp, err := c.doRequest(ctx, method, path, useAuth)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, &HTTPStatusError{StatusCode: resp.StatusCode, Status: resp.Status}
	}

	decoder := json.NewDecoder(resp.Body)
	return true, decoder.Decode(out)
}

func (c *Client) deleteWithOptions(ctx context.Context, path string, opts DeleteOptions, allowRecurse bool) (DeleteResult, int, error) {
	var out DeleteResult
	query := url.Values{}
	if opts.Force {
		query.Set("force", "true")
	}
	if opts.DryRun {
		query.Set("dry_run", "true")
	}
	if allowRecurse && opts.Recurse {
		query.Set("recurse", "true")
	}
	resp, err := c.doRequest(ctx, http.MethodDelete, appendQuery(path, query), true)
	if err != nil {
		return out, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		return out, resp.StatusCode, parseErrorResponse(resp)
	}
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&out); err != nil {
		return out, resp.StatusCode, err
	}
	return out, resp.StatusCode, nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, useAuth bool) (*http.Response, error) {
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if useAuth && c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}
	return c.http.Do(req)
}

func (c *Client) doRequestWithBody(ctx context.Context, method, path string, useAuth bool, body io.Reader, contentType string) (*http.Response, error) {
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if useAuth && c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}
	return c.http.Do(req)
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

type HTTPStatusError struct {
	StatusCode int
	Status     string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("unexpected status: %s", e.Status)
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

func parseErrorResponse(resp *http.Response) error {
	if resp == nil {
		return &HTTPStatusError{StatusCode: 0, Status: "missing response"}
	}
	var errResp ErrorResponse
	data, _ := io.ReadAll(resp.Body)
	if len(data) > 0 {
		if json.Unmarshal(data, &errResp) == nil && errResp.Message != "" {
			if errResp.Details != "" {
				return fmt.Errorf("%s: %s", errResp.Message, errResp.Details)
			}
			return fmt.Errorf("%s", errResp.Message)
		}
	}
	return &HTTPStatusError{StatusCode: resp.StatusCode, Status: resp.Status}
}
