package unit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/milkiss/vanish/backend/internal/api"
	"github.com/milkiss/vanish/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock storage implementation
type mockStorage struct {
	storeFunc      func(ctx context.Context, msg *models.Message, ttl time.Duration) (string, error)
	getDeleteFunc  func(ctx context.Context, id string) (*models.Message, error)
	existsFunc     func(ctx context.Context, id string) (bool, error)
	pingFunc       func(ctx context.Context) error
	closeFunc      func() error
}

func (m *mockStorage) Store(ctx context.Context, msg *models.Message, ttl time.Duration) (string, error) {
	if m.storeFunc != nil {
		return m.storeFunc(ctx, msg, ttl)
	}
	return "test-id-123", nil
}

func (m *mockStorage) GetAndDelete(ctx context.Context, id string) (*models.Message, error) {
	if m.getDeleteFunc != nil {
		return m.getDeleteFunc(ctx, id)
	}
	return &models.Message{
		Ciphertext: "test-ciphertext",
		IV:         "test-iv",
	}, nil
}

func (m *mockStorage) Exists(ctx context.Context, id string) (bool, error) {
	if m.existsFunc != nil {
		return m.existsFunc(ctx, id)
	}
	return true, nil
}

func (m *mockStorage) Ping(ctx context.Context) error {
	if m.pingFunc != nil {
		return m.pingFunc(ctx)
	}
	return nil
}

func (m *mockStorage) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockStore := &mockStorage{}
	handler := api.NewMessageHandler(mockStore, nil)

	router := gin.New()
	router.GET("/health", handler.Health)

	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])
}

func TestHealth_StorageError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockStore := &mockStorage{
		pingFunc: func(ctx context.Context) error {
			return errors.New("storage error")
		},
	}
	handler := api.NewMessageHandler(mockStore, nil)

	router := gin.New()
	router.GET("/health", handler.Health)

	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestCheckMessage_Exists(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockStore := &mockStorage{
		existsFunc: func(ctx context.Context, id string) (bool, error) {
			return true, nil
		},
	}
	handler := api.NewMessageHandler(mockStore, nil)

	router := gin.New()
	router.HEAD("/messages/:id", handler.CheckMessage)

	req, _ := http.NewRequest("HEAD", "/messages/test-id", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCheckMessage_NotExists(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockStore := &mockStorage{
		existsFunc: func(ctx context.Context, id string) (bool, error) {
			return false, nil
		},
	}
	handler := api.NewMessageHandler(mockStore, nil)

	router := gin.New()
	router.HEAD("/messages/:id", handler.CheckMessage)

	req, _ := http.NewRequest("HEAD", "/messages/test-id", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestCreateMessage_Success would require a real or mocked MetadataRepository
// Skipping this test to avoid complexity with concrete repository types

func TestCreateMessage_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockStore := &mockStorage{}
	handler := api.NewMessageHandler(mockStore, nil)

	router := gin.New()
	// No auth middleware - user_id not set
	router.POST("/messages", handler.CreateMessage)

	reqBody := models.CreateMessageRequest{
		Ciphertext:  "dGVzdC1jaXBoZXJ0ZXh0",
		IV:          "dGVzdC1pdg==",
		RecipientID: 2,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/messages", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateMessage_InvalidTTL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockStore := &mockStorage{}
	handler := api.NewMessageHandler(mockStore, nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", int64(1))
		c.Next()
	})
	router.POST("/messages", handler.CreateMessage)

	invalidTTL := int64(100) // Too short
	reqBody := models.CreateMessageRequest{
		Ciphertext:  "dGVzdC1jaXBoZXJ0ZXh0",
		IV:          "dGVzdC1pdg==",
		TTL:         &invalidTTL,
		RecipientID: 2,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/messages", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetMessage tests would require a real or mocked MetadataRepository
// Skipping these tests to avoid complexity with concrete repository types
