package client

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/zafrem/vanish/shared/models"
)

// GetMessageHistory retrieves the message history for the authenticated user
// limit: maximum number of messages to return (default: 50)
func (c *Client) GetMessageHistory(limit int) ([]models.MessageHistoryResponse, error) {
	if limit <= 0 {
		limit = 50
	}

	path := fmt.Sprintf("/api/history?limit=%d", limit)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get message history: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, handleError(resp)
	}

	var history models.HistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
		return nil, fmt.Errorf("failed to decode history response: %w", err)
	}

	return history.Messages, nil
}
