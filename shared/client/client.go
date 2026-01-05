package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/zafrem/vanish/shared/config"
)

// Client provides HTTP client functionality for Vanish API
type Client struct {
	config     *config.Config
	httpClient *http.Client
}

// NewClient creates a new API client with the given configuration
func NewClient(cfg *config.Config) *Client {
	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// doRequest performs an HTTP request with authentication
func (c *Client) doRequest(method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	// Normalize path
	path = strings.TrimPrefix(path, "/")
	url := fmt.Sprintf("%s/%s", c.config.BaseURL, path)

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.config.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// handleError processes error responses from the API
func handleError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("authentication failed: invalid or expired token. Run 'vanish config' to update credentials")
	case http.StatusForbidden:
		return fmt.Errorf("access denied: you don't have permission for this resource")
	case http.StatusNotFound:
		return fmt.Errorf("resource not found: %s", string(body))
	case http.StatusBadRequest:
		return fmt.Errorf("invalid request: %s", string(body))
	case http.StatusInternalServerError:
		return fmt.Errorf("server error: please try again later")
	default:
		return fmt.Errorf("unexpected error (status %d): %s", resp.StatusCode, string(body))
	}
}
