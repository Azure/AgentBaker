// Package nebraska provides a Go client for the Nebraska Publisher API.
// The publisher API is used to register COSI packages, create channels/groups,
// and manage update rollout.
package nebraska

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// TokenProvider is a function that returns a Bearer token for authenticating
// with the Nebraska Publisher API.
type TokenProvider func(ctx context.Context) (string, error)

// Client is an HTTP client for the Nebraska Publisher API.
type Client struct {
	baseURL       string
	tokenProvider TokenProvider
	httpClient    *http.Client
}

// NewClient creates a new Nebraska Publisher API client.
func NewClient(baseURL string, tokenProvider TokenProvider, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		baseURL:       strings.TrimRight(baseURL, "/"),
		tokenProvider: tokenProvider,
		httpClient:    httpClient,
	}
}

// GetApplication retrieves an application by ID.
func (c *Client) GetApplication(ctx context.Context, appID string) (*Application, error) {
	path := fmt.Sprintf("/api/apps/%s", appID)
	var app Application
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &app); err != nil {
		return nil, fmt.Errorf("get application %s: %w", appID, err)
	}
	return &app, nil
}

// CreatePackage registers a new package (version) for an application.
func (c *Client) CreatePackage(ctx context.Context, appID string, pkg Package) (*Package, error) {
	path := fmt.Sprintf("/api/apps/%s/packages", appID)
	var created Package
	if err := c.doJSON(ctx, http.MethodPost, path, pkg, &created); err != nil {
		return nil, fmt.Errorf("create package for app %s: %w", appID, err)
	}
	return &created, nil
}

// ListChannels lists all channels for an application.
func (c *Client) ListChannels(ctx context.Context, appID string) ([]Channel, error) {
	path := fmt.Sprintf("/api/apps/%s/channels", appID)
	var resp struct {
		Channels []Channel `json:"channels"`
	}
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("list channels for app %s: %w", appID, err)
	}
	return resp.Channels, nil
}

// CreateChannel creates a new channel for an application.
func (c *Client) CreateChannel(ctx context.Context, appID string, ch Channel) (*Channel, error) {
	path := fmt.Sprintf("/api/apps/%s/channels", appID)
	var created Channel
	if err := c.doJSON(ctx, http.MethodPost, path, ch, &created); err != nil {
		return nil, fmt.Errorf("create channel for app %s: %w", appID, err)
	}
	return &created, nil
}

// UpdateChannel updates an existing channel (e.g., to point to a new package).
func (c *Client) UpdateChannel(ctx context.Context, appID string, channelID string, ch Channel) (*Channel, error) {
	path := fmt.Sprintf("/api/apps/%s/channels/%s", appID, channelID)
	var updated Channel
	if err := c.doJSON(ctx, http.MethodPut, path, ch, &updated); err != nil {
		return nil, fmt.Errorf("update channel %s for app %s: %w", channelID, appID, err)
	}
	return &updated, nil
}

// CreateGroup creates a new group for an application.
func (c *Client) CreateGroup(ctx context.Context, appID string, g Group) (*Group, error) {
	path := fmt.Sprintf("/api/apps/%s/groups", appID)
	var created Group
	if err := c.doJSON(ctx, http.MethodPost, path, g, &created); err != nil {
		return nil, fmt.Errorf("create group for app %s: %w", appID, err)
	}
	return &created, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	if c.tokenProvider != nil {
		token, err := c.tokenProvider(ctx)
		if err != nil {
			return fmt.Errorf("get auth token: %w", err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
			Method:     method,
			Path:       path,
		}
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}

// APIError represents an error response from the Nebraska API.
type APIError struct {
	StatusCode int
	Body       string
	Method     string
	Path       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("nebraska API error: %s %s returned %d: %s", e.Method, e.Path, e.StatusCode, e.Body)
}
