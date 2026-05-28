// Package mailpitapi is a thin HTTP client for the running mailpit
// admin API (default 127.0.0.1:8025). Used by status + doctor.
package mailpitapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client talks to a single mailpit instance.
type Client struct {
	baseURL string
	http    *http.Client
}

// New builds a Client. If httpClient is nil a 2s-timeout default is used.
func New(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 2 * time.Second}
	}
	return &Client{baseURL: baseURL, http: httpClient}
}

// Info is the subset of GET /api/v1/info we care about. Mailpit returns
// many more fields; we deliberately ignore them so the struct stays
// stable as upstream evolves.
type Info struct {
	Version  string `json:"Version"`
	Messages int    `json:"Messages"`
}

// Info calls GET /api/v1/info.
func (c *Client) Info() (Info, error) {
	resp, err := c.http.Get(c.baseURL + "/api/v1/info")
	if err != nil {
		return Info{}, fmt.Errorf("mailpit info: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		return Info{}, fmt.Errorf("mailpit info: HTTP %d: %s", resp.StatusCode, body)
	}
	var info Info
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return Info{}, fmt.Errorf("mailpit info decode: %w", err)
	}
	return info, nil
}
