package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Config holds Slack configuration
type Config struct {
	BotToken      string
	WebhookURL    string
	SigningSecret string
}

// Client represents a Slack API client
type Client struct {
	config *Config
	client *http.Client
}

// NewClient creates a new Slack client
func NewClient(config *Config) *Client {
	return &Client{
		config: config,
		client: &http.Client{},
	}
}

// SendDirectMessage sends a DM to a user by email
func (c *Client) SendDirectMessage(ctx context.Context, userEmail, message string) error {
	// First, look up user by email
	userID, err := c.getUserIDByEmail(ctx, userEmail)
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}

	// Open a DM channel
	channelID, err := c.openDMChannel(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to open DM channel: %w", err)
	}

	// Send message
	return c.postMessage(ctx, channelID, message)
}

// SendSecretNotification sends a notification that a secret has been shared
func (c *Client) SendSecretNotification(ctx context.Context, recipientEmail, senderName, secretURL string) error {
	message := fmt.Sprintf(
		"🔒 *New Secure Message from %s*\n\n"+
			"You have received a secure, ephemeral message.\n\n"+
			"Click here to view (one-time access only):\n%s\n\n"+
			"⚠️ This message will be permanently destroyed after you read it.",
		senderName, secretURL,
	)

	return c.SendDirectMessage(ctx, recipientEmail, message)
}

func (c *Client) getUserIDByEmail(ctx context.Context, email string) (string, error) {
	url := "https://slack.com/api/users.lookupByEmail?email=" + email

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+c.config.BotToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		OK    bool   `json:"ok"`
		User  struct {
			ID string `json:"id"`
		} `json:"user"`
		Error string `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if !result.OK {
		return "", fmt.Errorf("slack API error: %s", result.Error)
	}

	return result.User.ID, nil
}

func (c *Client) openDMChannel(ctx context.Context, userID string) (string, error) {
	url := "https://slack.com/api/conversations.open"

	payload := map[string]interface{}{
		"users": userID,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+c.config.BotToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		OK      bool   `json:"ok"`
		Channel struct {
			ID string `json:"id"`
		} `json:"channel"`
		Error string `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if !result.OK {
		return "", fmt.Errorf("slack API error: %s", result.Error)
	}

	return result.Channel.ID, nil
}

func (c *Client) postMessage(ctx context.Context, channelID, message string) error {
	url := "https://slack.com/api/chat.postMessage"

	payload := map[string]interface{}{
		"channel": channelID,
		"text":    message,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.config.BotToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.OK {
		return fmt.Errorf("slack API error: %s", result.Error)
	}

	return nil
}

// OpenModal opens a modal dialog in Slack
func (c *Client) OpenModal(ctx context.Context, triggerID string, view map[string]interface{}) error {
	url := "https://slack.com/api/views.open"

	payload := map[string]interface{}{
		"trigger_id": triggerID,
		"view":       view,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.config.BotToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.OK {
		return fmt.Errorf("slack API error: %s", result.Error)
	}

	return nil
}

// UserInfo represents Slack user information
type UserInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// GetUserInfo gets information about a Slack user by ID
func (c *Client) GetUserInfo(ctx context.Context, userID string) (*UserInfo, error) {
	url := "https://slack.com/api/users.info?user=" + userID

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.config.BotToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		OK    bool   `json:"ok"`
		User  struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Profile struct {
				Email string `json:"email"`
			} `json:"profile"`
		} `json:"user"`
		Error string `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.OK {
		return nil, fmt.Errorf("slack API error: %s", result.Error)
	}

	return &UserInfo{
		ID:    result.User.ID,
		Name:  result.User.Name,
		Email: result.User.Profile.Email,
	}, nil
}

// SendEphemeralMessage sends an ephemeral message to a user
// Ephemeral messages are only visible to the specified user
func (c *Client) SendEphemeralMessage(ctx context.Context, userID, message string) error {
	// Open a DM channel first
	channelID, err := c.openDMChannel(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to open DM channel: %w", err)
	}

	url := "https://slack.com/api/chat.postEphemeral"

	payload := map[string]interface{}{
		"channel": channelID,
		"user":    userID,
		"text":    message,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.config.BotToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.OK {
		return fmt.Errorf("slack API error: %s", result.Error)
	}

	return nil
}
