package flightctl

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Client wraps the Flightctl HTTP API.
type Client struct {
	httpClient   *http.Client
	baseURL      string
	tokenManager *tokenManager
}

// Config holds Flightctl client configuration.
type Config struct {
	APIURL       string
	ClientID     string
	ClientSecret string
	TokenURL     string
	InsecureTLS  bool
	Timeout      time.Duration
}

// tokenManager handles OAuth 2.0 token acquisition and refresh.
type tokenManager struct {
	clientID     string
	clientSecret string
	tokenURL     string
	httpClient   *http.Client

	mu          sync.RWMutex
	accessToken string
	expiresAt   time.Time
}

// tokenResponse represents the OAuth 2.0 token response.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// oauth2Transport wraps an http.RoundTripper and adds OAuth 2.0 bearer tokens.
type oauth2Transport struct {
	base         http.RoundTripper
	tokenManager *tokenManager
}

// RoundTrip implements http.RoundTripper interface.
func (t *oauth2Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Get a valid token
	token, err := t.tokenManager.getToken(req.Context())
	if err != nil {
		return nil, fmt.Errorf("getting access token: %w", err)
	}

	// Clone the request to avoid modifying the original
	reqClone := req.Clone(req.Context())
	reqClone.Header.Set("Authorization", "Bearer "+token)

	// Perform the request
	return t.base.RoundTrip(reqClone)
}

// getToken returns a valid access token, fetching a new one if necessary.
func (tm *tokenManager) getToken(ctx context.Context) (string, error) {
	tm.mu.RLock()
	if tm.accessToken != "" && time.Now().Before(tm.expiresAt) {
		token := tm.accessToken
		tm.mu.RUnlock()
		return token, nil
	}
	tm.mu.RUnlock()

	return tm.fetchToken(ctx)
}

// fetchToken obtains a new access token using client credentials flow.
func (tm *tokenManager) fetchToken(ctx context.Context) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Double-check: another goroutine might have fetched the token
	if tm.accessToken != "" && time.Now().Before(tm.expiresAt) {
		return tm.accessToken, nil
	}

	// Prepare the OAuth 2.0 client credentials request
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", tm.clientID)
	data.Set("client_secret", tm.clientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", tm.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("creating token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := tm.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request returned status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decoding token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access token in response")
	}

	// Store the token with a buffer (subtract 60 seconds for safety)
	expiresIn := time.Duration(tokenResp.ExpiresIn) * time.Second
	if expiresIn > 60*time.Second {
		expiresIn -= 60 * time.Second
	}

	tm.accessToken = tokenResp.AccessToken
	tm.expiresAt = time.Now().Add(expiresIn)

	return tm.accessToken, nil
}

// NewClient creates a new Flightctl API client.
func NewClient(cfg Config) (*Client, error) {
	if cfg.APIURL == "" {
		return nil, fmt.Errorf("Flightctl API URL is required")
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("Flightctl client ID is required")
	}
	if cfg.ClientSecret == "" {
		return nil, fmt.Errorf("Flightctl client secret is required")
	}
	if cfg.TokenURL == "" {
		return nil, fmt.Errorf("Flightctl token URL is required")
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	// Create base transport
	baseTransport := &http.Transport{}
	if cfg.InsecureTLS {
		baseTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	// Create HTTP client for token requests (without OAuth transport)
	tokenHTTPClient := &http.Client{
		Transport: baseTransport,
		Timeout:   cfg.Timeout,
	}

	// Create token manager
	tm := &tokenManager{
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		tokenURL:     cfg.TokenURL,
		httpClient:   tokenHTTPClient,
	}

	// Wrap transport with OAuth2 transport
	oauth2Trans := &oauth2Transport{
		base:         baseTransport,
		tokenManager: tm,
	}

	return &Client{
		httpClient: &http.Client{
			Transport: oauth2Trans,
			Timeout:   cfg.Timeout,
		},
		baseURL:      cfg.APIURL,
		tokenManager: tm,
	}, nil
}

// Ping checks if the Flightctl API is reachable.
func (c *Client) Ping(ctx context.Context) error {
	println("Ping " + c.baseURL + "/api/v1/fleets")
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/fleets", nil)
	if err != nil {
		return fmt.Errorf("creating ping request: %w", err)
	}

	// Authorization header is automatically added by oauth2Transport
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
