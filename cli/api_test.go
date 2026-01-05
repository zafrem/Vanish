package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zafrem/vanish/shared/client"
	"github.com/zafrem/vanish/shared/config"
	"github.com/zafrem/vanish/shared/crypto"
	"github.com/zafrem/vanish/shared/models"
)

func TestFindUserID(t *testing.T) {
	tests := []struct {
		name        string
		email       string
		users       []models.User
		wantID      int64
		wantErr     bool
		statusCode  int
		errContains string
	}{
		{
			name:  "user found",
			email: "test@example.com",
			users: []models.User{
				{ID: 1, Name: "User 1", Email: "user1@example.com"},
				{ID: 2, Name: "Test User", Email: "test@example.com"},
				{ID: 3, Name: "User 3", Email: "user3@example.com"},
			},
			wantID:     2,
			wantErr:    false,
			statusCode: http.StatusOK,
		},
		{
			name:  "user not found",
			email: "notfound@example.com",
			users: []models.User{
				{ID: 1, Name: "User 1", Email: "user1@example.com"},
			},
			wantID:      0,
			wantErr:     true,
			statusCode:  http.StatusOK,
			errContains: "user not found",
		},
		{
			name:        "api error",
			email:       "test@example.com",
			users:       nil,
			wantID:      0,
			wantErr:     true,
			statusCode:  http.StatusInternalServerError,
			errContains: "",
		},
		{
			name:  "case insensitive email match",
			email: "TEST@EXAMPLE.COM",
			users: []models.User{
				{ID: 5, Name: "Test User", Email: "test@example.com"},
			},
			wantID:     5,
			wantErr:    false,
			statusCode: http.StatusOK,
		},
		{
			name:       "empty user list",
			email:      "test@example.com",
			users:      []models.User{},
			wantID:     0,
			wantErr:    true,
			statusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				if r.URL.Path != "/api/users" {
					t.Errorf("Request path = %s, want /api/users", r.URL.Path)
				}

				auth := r.Header.Get("Authorization")
				if auth != "Bearer test-token" {
					t.Errorf("Authorization header = %s, want Bearer test-token", auth)
				}

				// Send response
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					json.NewEncoder(w).Encode(tt.users)
				}
			}))
			defer server.Close()

			cfg := &config.Config{
				BaseURL: server.URL,
				Token:   "test-token",
			}

			// Create client and find user
			c := client.NewClient(cfg)
			id, err := c.FindUserByEmail(tt.email)

			if (err != nil) != tt.wantErr {
				t.Errorf("FindUserByEmail() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if id != tt.wantID {
				t.Errorf("FindUserByEmail() ID = %d, want %d", id, tt.wantID)
			}
		})
	}
}

func TestSendToAPI(t *testing.T) {
	// Create encrypted message for testing
	encrypted := &crypto.EncryptedMessage{
		Ciphertext: "encrypted-data",
		IV:         "initialization-vector",
		Key:        "encryption-key",
	}

	tests := []struct {
		name         string
		recipientID  int64
		ttl          int64
		statusCode   int
		wantErr      bool
		wantResponse *models.CreateMessageResponse
	}{
		{
			name:        "successful send",
			recipientID: 123,
			ttl:         86400,
			statusCode:  http.StatusCreated,
			wantErr:     false,
			wantResponse: &models.CreateMessageResponse{
				ID:        "test-message-id",
				ExpiresAt: mustParseTime("2026-01-05T10:00:00Z"),
			},
		},
		{
			name:        "server error",
			recipientID: 123,
			ttl:         86400,
			statusCode:  http.StatusInternalServerError,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				if r.URL.Path != "/api/messages" {
					t.Errorf("Request path = %s, want /api/messages", r.URL.Path)
				}

				if r.Method != "POST" {
					t.Errorf("Request method = %s, want POST", r.Method)
				}

				// Verify request body
				var req models.CreateMessageRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("Failed to decode request body: %v", err)
				}

				if req.RecipientID != tt.recipientID {
					t.Errorf("RecipientID = %d, want %d", req.RecipientID, tt.recipientID)
				}

				// Send response
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusCreated {
					json.NewEncoder(w).Encode(tt.wantResponse)
				}
			}))
			defer server.Close()

			cfg := &config.Config{
				BaseURL: server.URL,
				Token:   "test-token",
			}

			// Create client and send message
			c := client.NewClient(cfg)
			url, resp, err := c.SendMessage(tt.recipientID, encrypted, tt.ttl)

			if (err != nil) != tt.wantErr {
				t.Errorf("SendMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if url == "" {
					t.Error("SendMessage() returned empty URL")
				}
				if resp.ID != tt.wantResponse.ID {
					t.Errorf("Response ID = %s, want %s", resp.ID, tt.wantResponse.ID)
				}
			}
		})
	}
}

func TestSendSlackNotification(t *testing.T) {
	tests := []struct {
		name        string
		recipientID int64
		messageURL  string
		statusCode  int
		wantErr     bool
	}{
		{
			name:        "successful notification",
			recipientID: 123,
			messageURL:  "http://example.com/m/test",
			statusCode:  http.StatusOK,
			wantErr:     false,
		},
		{
			name:        "server error",
			recipientID: 123,
			messageURL:  "http://example.com/m/test",
			statusCode:  http.StatusInternalServerError,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				if r.URL.Path != "/api/notifications/send-slack" {
					t.Errorf("Request path = %s, want /api/notifications/send-slack", r.URL.Path)
				}

				if r.Method != "POST" {
					t.Errorf("Request method = %s, want POST", r.Method)
				}

				// Send response
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			cfg := &config.Config{
				BaseURL: server.URL,
				Token:   "test-token",
			}

			// Create client and send notification
			c := client.NewClient(cfg)
			err := c.SendSlackNotification(tt.recipientID, tt.messageURL)

			if (err != nil) != tt.wantErr {
				t.Errorf("SendSlackNotification() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Helper function to parse time
func mustParseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
