package olx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

var BaseURL = "https://api.olx.com.br"

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		baseURL:    BaseURL,
		httpClient: &http.Client{},
	}
}

func (c *Client) WithBaseURL(u string) *Client {
	c.baseURL = u
	return c
}

func (c *Client) WithHTTPClient(hc *http.Client) *Client {
	c.httpClient = hc
	return c
}

type accountResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Ad is the normalized OLX Brasil classified listing returned by the official ads API.
type Ad struct {
	ID          string
	Title       string
	Description string
	URL         string
	Status      string
	PriceCents  int
	Currency    string
	Visible     bool
}

type adsResponse struct {
	Ads []adResponse `json:"ads"`
}

type adResponse struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	URL         string  `json:"url"`
	Status      string  `json:"status"`
	Price       float64 `json:"price"`
	Currency    string  `json:"currency"`
	Visible     bool    `json:"visible"`
}

func (c *Client) ValidateAPIKey(ctx context.Context, apiKey string) (accountID, displayName string, err error) {
	body, err := c.get(ctx, apiKey, "/me")
	if err != nil {
		return "", "", fmt.Errorf("olx me: %w", err)
	}

	var acc accountResponse
	if err := json.Unmarshal(body, &acc); err != nil {
		return "", "", err
	}
	return acc.ID, acc.Name, nil
}

// ListAds fetches OLX Brasil ads/listings visible to the configured integrator account.
func (c *Client) ListAds(ctx context.Context, apiKey string) ([]Ad, error) {
	body, err := c.get(ctx, apiKey, "/ads?page=1")
	if err != nil {
		return nil, fmt.Errorf("olx ads: %w", err)
	}
	var response adsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	ads := make([]Ad, 0, len(response.Ads))
	for _, ad := range response.Ads {
		ads = append(ads, Ad{
			ID:          ad.ID,
			Title:       ad.Title,
			Description: ad.Description,
			URL:         ad.URL,
			Status:      ad.Status,
			PriceCents:  int(ad.Price*100 + 0.5),
			Currency:    defaultCurrency(ad.Currency),
			Visible:     ad.Visible,
		})
	}
	return ads, nil
}

func (c *Client) get(ctx context.Context, apiKey, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = resp.Status
		}
		return nil, fmt.Errorf("%s", sanitizeError(message))
	}
	return body, nil
}

func defaultCurrency(currency string) string {
	if currency == "" {
		return "BRL"
	}
	return currency
}

func sanitizeError(msg string) string {
	msg = strings.ReplaceAll(msg, "api_key=", "api_key=[REDACTED]")
	msg = strings.ReplaceAll(msg, "token=", "token=[REDACTED]")
	return msg
}
