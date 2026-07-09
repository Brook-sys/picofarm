package mercadolivre

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// BaseURL is the Mercado Libre REST API base (production).
// Tests may override it with an httptest.Server URL.
var BaseURL = "https://api.mercadolibre.com"

// Client is a Mercado Livre/Mercado Libre HTTP client.
// It is intentionally minimal for ML-03; live methods will be added in follow-up cards.
type Client struct {
	accessToken string
	httpClient  *http.Client
}

// NewClient creates a new Mercado Libre client.
func NewClient(accessToken string) *Client {
	return &Client{
		accessToken: accessToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// WithHTTPClient replaces the underlying HTTP client (useful for testing with RoundTripper).
func (c *Client) WithHTTPClient(hc *http.Client) *Client {
	if hc != nil {
		c.httpClient = hc
	}
	return c
}

// do performs an authenticated GET request.
func (c *Client) do(ctx context.Context, method, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// User represents a minimal Mercado Libre user.
type User struct {
	ID        int64  `json:"id"`
	Nickname  string `json:"nickname"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	SiteID    string `json:"site_id"`
}

// GetCurrentUser fetches the authenticated user (used for connection validation).
func (c *Client) GetCurrentUser(ctx context.Context) (*User, error) {
	resp, err := c.do(ctx, http.MethodGet, "/users/me")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var u User
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, fmt.Errorf("parsing user: %w", err)
	}
	return &u, nil
}
