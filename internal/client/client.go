package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	DefaultBaseURL = "https://platform.serverscamp.com/api/v1"
	SSHKeysEP      = "/ssh-keys"
	NetworksEP     = "/network"
	RoutersEP      = "/router"
)

// Client wraps HTTP communication with the SCAMP API.
type Client struct {
	BaseURL string
	Token   string
	http    *http.Client
}

// New creates a new SCAMP API client.
func New(baseURL, token string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// buildURL constructs full URL from endpoint and optional query params.
func (c *Client) buildURL(ep string, q url.Values) (string, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return "", err
	}
	u.Path = u.Path + ep
	if q != nil && len(q) > 0 {
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

// APIError represents error response from API.
type APIError struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

// doJSON performs HTTP request with JSON body and returns response.
func (c *Client) doJSON(ctx context.Context, method, fullURL string, payload any) ([]byte, int, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, err
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, 0, err
	}

	tflog.Debug(ctx, "HTTP request", map[string]any{"method": method, "url": fullURL})

	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	rb, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	tflog.Debug(ctx, "HTTP response", map[string]any{"status": resp.StatusCode, "url": fullURL})

	if resp.StatusCode >= 400 {
		var apiErr APIError
		_ = json.Unmarshal(rb, &apiErr)
		if apiErr.Message != "" {
			return nil, resp.StatusCode, fmt.Errorf("http %d: %s", resp.StatusCode, apiErr.Message)
		}
		if apiErr.Error != "" {
			return nil, resp.StatusCode, fmt.Errorf("http %d: %s", resp.StatusCode, apiErr.Error)
		}
		return nil, resp.StatusCode, fmt.Errorf("http %d: %s", resp.StatusCode, string(rb))
	}

	return rb, resp.StatusCode, nil
}

// GetJSON performs GET request and unmarshals response into out.
func (c *Client) GetJSON(ctx context.Context, ep string, q url.Values, out any) error {
	u, err := c.buildURL(ep, q)
	if err != nil {
		return err
	}
	b, _, err := c.doJSON(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// PostJSON performs POST request with payload and unmarshals response into out.
func (c *Client) PostJSON(ctx context.Context, ep string, payload any, out any) error {
	u, err := c.buildURL(ep, nil)
	if err != nil {
		return err
	}
	b, _, err := c.doJSON(ctx, http.MethodPost, u, payload)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(b, out)
}

// Delete performs DELETE request.
func (c *Client) Delete(ctx context.Context, ep string) error {
	u, err := c.buildURL(ep, nil)
	if err != nil {
		return err
	}
	_, _, err = c.doJSON(ctx, http.MethodDelete, u, nil)
	return err
}
