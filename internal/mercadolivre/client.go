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

// get performs an authenticated GET request.
func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, BaseURL+path, nil)
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

// OrderItem represents a line item in a Mercado Libre order.
type OrderItem struct {
	Item struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		SKU   string `json:"seller_custom_field"`
	} `json:"item"`
	Quantity   int     `json:"quantity"`
	UnitPrice  float64 `json:"unit_price"`
	CurrencyID string  `json:"currency_id"`
}

// Order represents the minimal Mercado Libre order shape PicoFarm needs.
type Order struct {
	ID          int64   `json:"id"`
	DateCreated string  `json:"date_created"`
	LastUpdated string  `json:"last_updated"`
	Status      string  `json:"status"`
	TotalAmount float64 `json:"total_amount"`
	CurrencyID  string  `json:"currency_id"`
	Buyer       struct {
		ID        int64  `json:"id"`
		Nickname  string `json:"nickname"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Email     string `json:"email"`
	} `json:"buyer"`
	Items    []OrderItem `json:"order_items"`
	Shipping struct {
		ID int64 `json:"id"`
	} `json:"shipping"`
}

// GetCurrentUser fetches the authenticated user (used for connection validation).
func (c *Client) GetCurrentUser(ctx context.Context) (*User, error) {
	resp, err := c.get(ctx, "/users/me")
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

// GetOrder fetches a Mercado Libre order by provider order ID.
func (c *Client) GetOrder(ctx context.Context, externalOrderID string) (*Order, error) {
	resp, err := c.get(ctx, "/orders/"+externalOrderID)
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

	var order Order
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, fmt.Errorf("parsing order: %w", err)
	}
	return &order, nil
}

// OrdersSearchResponse is the minimal response returned by /orders/search.
type OrdersSearchResponse struct {
	Results []Order `json:"results"`
}

// ListOrders lists recent Mercado Livre orders for the authenticated seller.
func (c *Client) ListOrders(ctx context.Context) ([]*Order, error) {
	user, err := c.GetCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := c.get(ctx, fmt.Sprintf("/orders/search?seller=%d&sort=date_desc&limit=50", user.ID))
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

	var search OrdersSearchResponse
	if err := json.Unmarshal(body, &search); err != nil {
		return nil, fmt.Errorf("parsing orders search: %w", err)
	}
	orders := make([]*Order, 0, len(search.Results))
	for i := range search.Results {
		orders = append(orders, &search.Results[i])
	}
	return orders, nil
}
