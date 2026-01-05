package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/zafrem/vanish/shared/models"
)

// ListUsers retrieves all users from the Vanish system
func (c *Client) ListUsers() ([]models.User, error) {
	resp, err := c.doRequest("GET", "/api/users", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, handleError(resp)
	}

	var users []models.User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, fmt.Errorf("failed to decode users response: %w", err)
	}

	return users, nil
}

// FindUserByEmail finds a user by their email address
// Returns the user ID if found, error otherwise
func (c *Client) FindUserByEmail(email string) (int64, error) {
	resp, err := c.doRequest("GET", "/api/users", nil)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch users: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, handleError(resp)
	}

	var users []models.User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return 0, fmt.Errorf("failed to decode users response: %w", err)
	}

	// Search for user with matching email (case-insensitive)
	for _, u := range users {
		if strings.EqualFold(u.Email, email) {
			return u.ID, nil
		}
	}

	return 0, fmt.Errorf("user not found: %s", email)
}

// GetUserByID retrieves a user by their ID
func (c *Client) GetUserByID(id int64) (*models.User, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/api/users/%d", id), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, handleError(resp)
	}

	var user models.User
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("failed to decode user response: %w", err)
	}

	return &user, nil
}
