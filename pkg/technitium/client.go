package technitium

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client is an HTTP client for the Technitium DNS Server API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient creates a new Technitium API client.
func NewClient(baseURL, token string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// GetStats fetches dashboard statistics from the Technitium server.
func (c *Client) GetStats(ctx context.Context) (*StatsResponse, error) {
	endpoint := "/api/dashboard/stats/get"
	params := url.Values{}
	params.Set("token", c.token)
	params.Set("type", "LastHour")

	resp, err := c.doRequest(ctx, endpoint, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read stats response body: %w", err)
	}

	var statsResp StatsResponse
	if err := json.Unmarshal(body, &statsResp); err != nil {
		return nil, fmt.Errorf("failed to parse stats response: %w", err)
	}

	if statsResp.Status != "ok" {
		return nil, fmt.Errorf("API returned non-ok status: %s", statsResp.Status)
	}

	return &statsResp, nil
}

// GetSettings fetches server settings from the Technitium server.
func (c *Client) GetSettings(ctx context.Context) (*SettingsResponse, error) {
	endpoint := "/api/settings/get"
	params := url.Values{}
	params.Set("token", c.token)

	resp, err := c.doRequest(ctx, endpoint, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get settings: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read settings response body: %w", err)
	}

	var settingsResp SettingsResponse
	if err := json.Unmarshal(body, &settingsResp); err != nil {
		return nil, fmt.Errorf("failed to parse settings response: %w", err)
	}

	if settingsResp.Status != "ok" {
		return nil, fmt.Errorf("API returned non-ok status: %s", settingsResp.Status)
	}

	return &settingsResp, nil
}

// doRequest performs an HTTP GET request to the specified endpoint.
func (c *Client) doRequest(ctx context.Context, endpoint string, params url.Values) (*http.Response, error) {
	reqURL := c.baseURL + endpoint + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return resp, nil
}
