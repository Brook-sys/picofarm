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

func (c *Client) ValidateAPIKey(ctx context.Context, apiKey string) (accountID, displayName string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/me", nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = resp.Status
		}
		return "", "", fmt.Errorf("olx me: %s", sanitizeError(message))
	}

	var acc accountResponse
	if err := json.Unmarshal(body, &acc); err != nil {
		return "", "", err
	}
	return acc.ID, acc.Name, nil
}

func sanitizeError(msg string) string {
	msg = strings.ReplaceAll(msg, "api_key=", "api_key=[REDACTED]")
	msg = strings.ReplaceAll(msg, "token=", "token=[REDACTED]")
	return msg
}
