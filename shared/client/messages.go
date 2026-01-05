package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/zafrem/vanish/shared/crypto"
	"github.com/zafrem/vanish/shared/models"
)

// SendMessage sends an encrypted message to the API
// Returns the message URL and response
func (c *Client) SendMessage(recipientID int64, encrypted *crypto.EncryptedMessage, ttl int64) (string, *models.CreateMessageResponse, error) {
	payload := models.CreateMessageRequest{
		Ciphertext:    encrypted.Ciphertext,
		IV:            encrypted.IV,
		RecipientID:   recipientID,
		EncryptionKey: encrypted.Key,
		TTL:           ttl,
	}

	resp, err := c.doRequest("POST", "/api/messages", payload)
	if err != nil {
		return "", nil, fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", nil, handleError(resp)
	}

	var result models.CreateMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Construct shareable URL
	// Format: {baseURL}/m/{messageID}#{encryptionKey}
	url := fmt.Sprintf("%s/m/%s#%s", c.config.BaseURL, result.ID, encrypted.Key)

	return url, &result, nil
}

// CheckMessageStatus checks if a message exists (pending) or has been burned (read/expired)
// Uses HEAD request to minimize data transfer
func (c *Client) CheckMessageStatus(messageID string) (models.MessageStatus, error) {
	resp, err := c.doRequest("HEAD", fmt.Sprintf("/api/messages/%s", messageID), nil)
	if err != nil {
		return "", fmt.Errorf("failed to check message status: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// Message exists and is pending
		return models.StatusPending, nil
	case http.StatusNotFound:
		// Message has been burned or expired
		return models.StatusRead, nil
	default:
		return "", handleError(resp)
	}
}

// GetMessage retrieves a message by ID
// Note: This burns the message (one-time read)
func (c *Client) GetMessage(messageID string) (*models.MessageResponse, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/api/messages/%s", messageID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("message not found or already burned")
		}
		return nil, handleError(resp)
	}

	var message models.MessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&message); err != nil {
		return nil, fmt.Errorf("failed to decode message response: %w", err)
	}

	return &message, nil
}

// SendSlackNotification sends a Slack notification to the recipient
// This is a best-effort operation - errors are non-fatal
func (c *Client) SendSlackNotification(recipientID int64, messageURL string) error {
	payload := map[string]interface{}{
		"recipient_id": recipientID,
		"message_url":  messageURL,
	}

	resp, err := c.doRequest("POST", "/api/notifications/send-slack", payload)
	if err != nil {
		return fmt.Errorf("failed to send Slack notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack notification failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}
