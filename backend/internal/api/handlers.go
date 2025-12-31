package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/milkiss/vanish/backend/internal/models"
	"github.com/milkiss/vanish/backend/internal/repository"
	"github.com/milkiss/vanish/backend/internal/storage"
)

// MessageHandler handles all message-related HTTP requests
type MessageHandler struct {
	storage    storage.Storage
	metadataRepo *repository.MetadataRepository
}

// NewMessageHandler creates a new message handler
func NewMessageHandler(storage storage.Storage, metadataRepo *repository.MetadataRepository) *MessageHandler {
	return &MessageHandler{
		storage:    storage,
		metadataRepo: metadataRepo,
	}
}

// CreateMessage handles POST /api/messages
// Stores an encrypted message and returns an ID
// Requires authentication - sender must be logged in
func (h *MessageHandler) CreateMessage(c *gin.Context) {
	// Get sender ID from auth middleware
	senderID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Unauthorized",
		})
		return
	}

	var req models.CreateMessageRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request: " + err.Error(),
		})
		return
	}

	// Validate TTL
	ttlSeconds, err := models.ValidateTTL(req.TTL)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: err.Error(),
		})
		return
	}

	// Create message object (encrypted content for Redis)
	msg := &models.Message{
		Ciphertext: req.Ciphertext,
		IV:         req.IV,
		CreatedAt:  time.Now().UTC(),
	}

	// Store encrypted message in Redis with TTL
	id, err := h.storage.Store(c.Request.Context(), msg, time.Duration(ttlSeconds)*time.Second)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to store message",
		})
		return
	}

	// Calculate expiration time
	expiresAt := msg.CreatedAt.Add(time.Duration(ttlSeconds) * time.Second)

	// Store metadata in PostgreSQL (sender, recipient, but NOT content)
	metadata := &models.MessageMetadata{
		MessageID:     id,
		SenderID:      senderID.(int64),
		RecipientID:   req.RecipientID,
		EncryptionKey: req.EncryptionKey, // Store key for recipient link generation
		Status:        models.StatusPending,
		CreatedAt:     msg.CreatedAt,
		ExpiresAt:     expiresAt,
	}

	err = h.metadataRepo.Create(c.Request.Context(), metadata)
	if err != nil {
		// If metadata creation fails, we should clean up the Redis message
		// But for now, we'll log and continue (message will expire anyway)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to store message metadata",
		})
		return
	}

	// Return response
	c.JSON(http.StatusCreated, models.CreateMessageResponse{
		ID:        id,
		ExpiresAt: expiresAt,
	})
}

// GetMessage handles GET /api/messages/:id
// Retrieves and burns (deletes) a message atomically
// CRITICAL: Verifies that the current user is the intended recipient
func (h *MessageHandler) GetMessage(c *gin.Context) {
	// Get current user ID from auth middleware
	currentUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Unauthorized",
		})
		return
	}

	id := c.Param("id")

	if id == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Message ID is required",
		})
		return
	}

	// Check metadata and verify recipient
	metadata, err := h.metadataRepo.FindByMessageID(c.Request.Context(), id)
	if err != nil {
		if err == models.ErrMessageNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error: "Message not found or already burned",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve message metadata",
		})
		return
	}

	// CRITICAL SECURITY CHECK: Verify the current user is the intended recipient
	if metadata.RecipientID != currentUserID.(int64) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error: "You are not the intended recipient of this message",
		})
		return
	}

	// Check if already read
	if metadata.Status == models.StatusRead {
		c.JSON(http.StatusGone, models.ErrorResponse{
			Error: "Message has already been read and burned",
		})
		return
	}

	// Atomically get and delete the message from Redis (burn-on-read)
	msg, err := h.storage.GetAndDelete(c.Request.Context(), id)
	if err != nil {
		if err == models.ErrMessageNotFound {
			// Message exists in metadata but not in Redis (expired or race condition)
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error: "Message not found or already burned",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve message",
		})
		return
	}

	// Mark as read in metadata
	err = h.metadataRepo.MarkAsRead(c.Request.Context(), id)
	if err != nil {
		// Message was burned from Redis, but we couldn't update metadata
		// Log this but still return the message to user
		// The metadata will be marked as expired by cleanup job
	}

	// Return the encrypted message
	c.JSON(http.StatusOK, models.MessageResponse{
		Ciphertext: msg.Ciphertext,
		IV:         msg.IV,
	})
}

// CheckMessage handles HEAD /api/messages/:id
// Checks if a message exists without burning it
func (h *MessageHandler) CheckMessage(c *gin.Context) {
	id := c.Param("id")

	if id == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	exists, err := h.storage.Exists(c.Request.Context(), id)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	if exists {
		c.Status(http.StatusOK)
	} else {
		c.Status(http.StatusNotFound)
	}
}

// Health handles GET /health
// Returns the health status of the service
func (h *MessageHandler) Health(c *gin.Context) {
	// Check Redis connection
	err := h.storage.Ping(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  "Redis connection failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}
