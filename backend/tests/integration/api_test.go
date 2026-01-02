package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/milkiss/vanish/backend/internal/api"
	"github.com/milkiss/vanish/backend/internal/config"
	"github.com/milkiss/vanish/backend/internal/models"
	"github.com/milkiss/vanish/backend/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter(t *testing.T) (*httptest.Server, func()) {
	// Load test config
	cfg := &config.Config{
		Server: config.ServerConfig{
			AllowedOrigins: []string{"*"},
		},
		JWT: config.JWTConfig{
			SecretKey: "test-secret-key-for-integration-tests",
		},
	}

	// Setup test storage
	store, err := storage.NewRedisStorage("localhost:6379", "", 1)
	require.NoError(t, err)

	// Create mock repositories (nil for integration tests as we're testing public endpoints)
	router := api.SetupRouter(cfg, store, nil, nil, nil, nil, nil, nil)
	server := httptest.NewServer(router)

	cleanup := func() {
		server.Close()
		store.Close()
	}

	return server, cleanup
}

func TestCreateAndRetrieveFlow(t *testing.T) {
	server, cleanup := setupTestRouter(t)
	defer cleanup()

	// Create a message
	createReq := models.CreateMessageRequest{
		Ciphertext: "dGVzdC1jaXBoZXJ0ZXh0", // base64 encoded
		IV:         "dGVzdC1pdg==",         // base64 encoded
	}

	body, _ := json.Marshal(createReq)
	resp, err := http.Post(server.URL+"/api/messages", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var createResp models.CreateMessageResponse
	err = json.NewDecoder(resp.Body).Decode(&createResp)
	require.NoError(t, err)
	assert.NotEmpty(t, createResp.ID)

	// Retrieve the message (burn)
	getResp, err := http.Get(server.URL + "/api/messages/" + createResp.ID)
	require.NoError(t, err)
	defer getResp.Body.Close()

	assert.Equal(t, http.StatusOK, getResp.StatusCode)

	var msgResp models.MessageResponse
	err = json.NewDecoder(getResp.Body).Decode(&msgResp)
	require.NoError(t, err)
	assert.Equal(t, createReq.Ciphertext, msgResp.Ciphertext)
	assert.Equal(t, createReq.IV, msgResp.IV)

	// Try to retrieve again (should be burned)
	getResp2, err := http.Get(server.URL + "/api/messages/" + createResp.ID)
	require.NoError(t, err)
	defer getResp2.Body.Close()

	assert.Equal(t, http.StatusNotFound, getResp2.StatusCode)
}

func TestHeadEndpoint(t *testing.T) {
	server, cleanup := setupTestRouter(t)
	defer cleanup()

	// Create a message
	createReq := models.CreateMessageRequest{
		Ciphertext: "dGVzdC1jaXBoZXJ0ZXh0",
		IV:         "dGVzdC1pdg==",
	}

	body, _ := json.Marshal(createReq)
	resp, err := http.Post(server.URL+"/api/messages", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	var createResp models.CreateMessageResponse
	json.NewDecoder(resp.Body).Decode(&createResp)

	// Check existence with HEAD (should not burn)
	req, _ := http.NewRequest("HEAD", server.URL+"/api/messages/"+createResp.ID, nil)
	headResp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer headResp.Body.Close()

	assert.Equal(t, http.StatusOK, headResp.StatusCode)

	// Message should still be retrievable with GET
	getResp, err := http.Get(server.URL + "/api/messages/" + createResp.ID)
	require.NoError(t, err)
	defer getResp.Body.Close()

	assert.Equal(t, http.StatusOK, getResp.StatusCode)
}

func TestInvalidInput(t *testing.T) {
	server, cleanup := setupTestRouter(t)
	defer cleanup()

	tests := []struct {
		name     string
		payload  interface{}
		expected int
	}{
		{
			name:     "missing ciphertext",
			payload:  map[string]string{"iv": "dGVzdC1pdg=="},
			expected: http.StatusBadRequest,
		},
		{
			name:     "missing iv",
			payload:  map[string]string{"ciphertext": "dGVzdC1jaXBoZXJ0ZXh0"},
			expected: http.StatusBadRequest,
		},
		{
			name:     "invalid base64",
			payload:  map[string]string{"ciphertext": "not-base64!!!", "iv": "dGVzdC1pdg=="},
			expected: http.StatusBadRequest,
		},
		{
			name: "invalid TTL (too short)",
			payload: map[string]interface{}{
				"ciphertext": "dGVzdC1jaXBoZXJ0ZXh0",
				"iv":         "dGVzdC1pdg==",
				"ttl":        100, // Less than MinTTL
			},
			expected: http.StatusBadRequest,
		},
		{
			name: "invalid TTL (too long)",
			payload: map[string]interface{}{
				"ciphertext": "dGVzdC1jaXBoZXJ0ZXh0",
				"iv":         "dGVzdC1pdg==",
				"ttl":        999999999, // More than MaxTTL
			},
			expected: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			resp, err := http.Post(server.URL+"/api/messages", "application/json", bytes.NewBuffer(body))
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expected, resp.StatusCode)
		})
	}
}

func TestHealthEndpoint(t *testing.T) {
	server, cleanup := setupTestRouter(t)
	defer cleanup()

	resp, err := http.Get(server.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var health map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&health)
	require.NoError(t, err)
	assert.Equal(t, "healthy", health["status"])
}

func TestCORS(t *testing.T) {
	server, cleanup := setupTestRouter(t)
	defer cleanup()

	req, _ := http.NewRequest("OPTIONS", server.URL+"/api/messages", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Check CORS headers
	assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Origin"))
	assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Methods"))
}

func TestSecurityHeaders(t *testing.T) {
	server, cleanup := setupTestRouter(t)
	defer cleanup()

	resp, err := http.Get(server.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify security headers are present
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", resp.Header.Get("X-XSS-Protection"))
	assert.NotEmpty(t, resp.Header.Get("Strict-Transport-Security"))
}
