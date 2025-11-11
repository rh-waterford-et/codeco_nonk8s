package flightctl

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"
)

// Client wraps the Flightctl HTTP API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	authToken  string
}

// Config holds Flightctl client configuration.
type Config struct {
	APIURL      string
	AuthToken   string
	InsecureTLS bool
	Timeout     time.Duration
}

// NewClient creates a new Flightctl API client.
func NewClient(cfg Config) (*Client, error) {
	if cfg.APIURL == "" {
		return nil, fmt.Errorf("Flightctl API URL is required")
	}
	if cfg.AuthToken == "" {
		return nil, fmt.Errorf("Flightctl auth token is required")
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	transport := &http.Transport{}
	if cfg.InsecureTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   cfg.Timeout,
		},
		baseURL:   cfg.APIURL,
		authToken: cfg.AuthToken,
	}, nil
}

// Ping checks if the Flightctl API is reachable.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/healthz", nil)
	if err != nil {
		return fmt.Errorf("creating ping request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ping request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ping returned status %d", resp.StatusCode)
	}

	return nil
}
