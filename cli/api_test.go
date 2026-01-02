package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFindUserID(t *testing.T) {
	tests := []struct {
		name        string
		email       string
		users       []User
		wantID      int64
		wantErr     bool
		statusCode  int
		errContains string
	}{
		{
			name:  "user found",
			email: "test@example.com",
			users: []User{
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
			users: []User{
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
			errContains: "api returned status",
		},
		{
			name:  "case insensitive email match",
			email: "TEST@EXAMPLE.COM",
			users: []User{
				{ID: 5, Name: "Test User", Email: "test@example.com"},
			},
			wantID:     5,
			wantErr:    false,
			statusCode: http.StatusOK,
		},
		{
			name:       "empty user list",
			email:      "test@example.com",
			users:      []User{},
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

			cfg := &Config{
				BaseURL: server.URL,
				Token:   "test-token",
			}

			id, err := findUserID(cfg, tt.email)

			if (err != nil) != tt.wantErr {
				t.Errorf("findUserID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want to contain %s", err, tt.errContains)
				}
				return
			}

			if id != tt.wantID {
				t.Errorf("findUserID() = %d, want %d", id, tt.wantID)
			}
		})
	}
}

func TestSendToAPI(t *testing.T) {
	tests := []struct {
		name        string
		recipientID int64
		ttl         int64
		statusCode  int
		wantErr     bool
		errContains string
	}{
		{
			name:        "successful send",
			recipientID: 5,
			ttl:         86400,
			statusCode:  http.StatusCreated,
			wantErr:     false,
		},
		{
			name:        "api error",
			recipientID: 5,
			ttl:         86400,
			statusCode:  http.StatusBadRequest,
			wantErr:     true,
			errContains: "api error",
		},
		{
			name:        "unauthorized",
			recipientID: 5,
			ttl:         86400,
			statusCode:  http.StatusUnauthorized,
			wantErr:     true,
			errContains: "api error",
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

				auth := r.Header.Get("Authorization")
				if auth != "Bearer test-token" {
					t.Errorf("Authorization header = %s, want Bearer test-token", auth)
				}

				contentType := r.Header.Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("Content-Type = %s, want application/json", contentType)
				}

				// Parse request body
				var req CreateMessageRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("Failed to decode request body: %v", err)
				}

				// Verify request data
				if req.RecipientID != tt.recipientID {
					t.Errorf("RecipientID = %d, want %d", req.RecipientID, tt.recipientID)
				}
				if req.TTL != tt.ttl {
					t.Errorf("TTL = %d, want %d", req.TTL, tt.ttl)
				}

				// Send response
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusCreated {
					resp := CreateMessageResponse{
						ID: "test-message-id",
					}
					json.NewEncoder(w).Encode(resp)
				}
			}))
			defer server.Close()

			cfg := &Config{
				BaseURL: server.URL,
				Token:   "test-token",
			}

			encrypted := &EncryptedMessage{
				Ciphertext: "dGVzdC1jaXBoZXJ0ZXh0",
				IV:         "dGVzdC1pdg==",
				Key:        "dGVzdC1rZXk=",
			}

			url, err := sendToAPI(cfg, tt.recipientID, encrypted, tt.ttl)

			if (err != nil) != tt.wantErr {
				t.Errorf("sendToAPI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want to contain %s", err, tt.errContains)
				}
				return
			}

			if !tt.wantErr {
				expectedURL := server.URL + "/m/test-message-id#" + encrypted.Key
				if url != expectedURL {
					t.Errorf("URL = %s, want %s", url, expectedURL)
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
			recipientID: 5,
			messageURL:  "http://test.com/m/abc#key",
			statusCode:  http.StatusOK,
			wantErr:     false,
		},
		{
			name:        "notification fails",
			recipientID: 5,
			messageURL:  "http://test.com/m/abc#key",
			statusCode:  http.StatusNotFound,
			wantErr:     true,
		},
		{
			name:        "server error",
			recipientID: 5,
			messageURL:  "http://test.com/m/abc#key",
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

				// Verify authorization
				auth := r.Header.Get("Authorization")
				if auth != "Bearer test-token" {
					t.Errorf("Authorization header = %s, want Bearer test-token", auth)
				}

				// Parse and verify body
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("Failed to decode request body: %v", err)
				}

				if int64(body["recipient_id"].(float64)) != tt.recipientID {
					t.Errorf("recipient_id = %v, want %d", body["recipient_id"], tt.recipientID)
				}

				if body["message_url"] != tt.messageURL {
					t.Errorf("message_url = %v, want %s", body["message_url"], tt.messageURL)
				}

				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			cfg := &Config{
				BaseURL: server.URL,
				Token:   "test-token",
			}

			err := sendSlackNotification(cfg, tt.recipientID, tt.messageURL)

			if (err != nil) != tt.wantErr {
				t.Errorf("sendSlackNotification() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
