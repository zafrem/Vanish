package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/milkiss/vanish/backend/internal/models"
	"github.com/milkiss/vanish/backend/internal/repository"
)

// HistoryHandler handles message history endpoints
type HistoryHandler struct {
	metadataRepo *repository.MetadataRepository
}

// NewHistoryHandler creates a new history handler
func NewHistoryHandler(metadataRepo *repository.MetadataRepository) *HistoryHandler {
	return &HistoryHandler{
		metadataRepo: metadataRepo,
	}
}

// GetMyHistory returns the current user's message history (sent and received)
func (h *HistoryHandler) GetMyHistory(c *gin.Context) {
	// Get user ID from auth middleware
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Unauthorized",
		})
		return
	}

	// Get history limit from query param (default 50)
	limit := 50
	if limitParam := c.Query("limit"); limitParam != "" {
		// Parse limit (omitting error handling for brevity)
		// In production, add proper parsing with max limit validation
	}

	history, err := h.metadataRepo.GetUserHistory(c.Request.Context(), userID.(int64), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve history",
		})
		return
	}

	c.JSON(http.StatusOK, history)
}
