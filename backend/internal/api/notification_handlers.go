package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/milkiss/vanish/backend/internal/integrations/email"
	"github.com/milkiss/vanish/backend/internal/integrations/slack"
	"github.com/milkiss/vanish/backend/internal/models"
	"github.com/milkiss/vanish/backend/internal/repository"
)

// NotificationHandler handles notification-related HTTP requests
type NotificationHandler struct {
	userRepo     *repository.UserRepository
	metadataRepo *repository.MetadataRepository
	emailClient  *email.Client
	slackClient  *slack.Client
}

// NewNotificationHandler creates a new notification handler
func NewNotificationHandler(
	userRepo *repository.UserRepository,
	metadataRepo *repository.MetadataRepository,
	emailClient *email.Client,
	slackClient *slack.Client,
) *NotificationHandler {
	return &NotificationHandler{
		userRepo:     userRepo,
		metadataRepo: metadataRepo,
		emailClient:  emailClient,
		slackClient:  slackClient,
	}
}

// SendNotificationRequest defines the request body for sending notifications
type SendNotificationRequest struct {
	RecipientID int64  `json:"recipient_id" binding:"required"`
	MessageURL  string `json:"message_url" binding:"required"`
}

// SendSlackNotification handles POST /api/notifications/send-slack
func (h *NotificationHandler) SendSlackNotification(c *gin.Context) {
	if h.slackClient == nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{
			Error: "Slack integration is not enabled",
		})
		return
	}

	// Get sender ID from auth middleware
	senderID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Unauthorized",
		})
		return
	}

	var req SendNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request: " + err.Error(),
		})
		return
	}

	// Verify sender
	sender, err := h.userRepo.FindByID(c.Request.Context(), senderID.(int64))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve sender information",
		})
		return
	}

	// Retrieve recipient
	recipient, err := h.userRepo.FindByID(c.Request.Context(), req.RecipientID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: "Recipient not found",
		})
		return
	}

	// Send notification
	err = h.slackClient.SendSecretNotification(
		c.Request.Context(),
		recipient.Email,
		sender.Name,
		req.MessageURL,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: fmt.Sprintf("Failed to send Slack notification: %v", err),
		})
		return
	}

	c.Status(http.StatusOK)
}

// SendEmailNotification handles POST /api/notifications/send-email
func (h *NotificationHandler) SendEmailNotification(c *gin.Context) {
	if h.emailClient == nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{
			Error: "Email integration is not enabled",
		})
		return
	}

	// Get sender ID from auth middleware
	senderID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Unauthorized",
		})
		return
	}

	var req SendNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request: " + err.Error(),
		})
		return
	}

	// Verify sender
	sender, err := h.userRepo.FindByID(c.Request.Context(), senderID.(int64))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve sender information",
		})
		return
	}

	// Retrieve recipient
	recipient, err := h.userRepo.FindByID(c.Request.Context(), req.RecipientID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: "Recipient not found",
		})
		return
	}

	// Send notification
	err = h.emailClient.SendSecretNotification(
		recipient.Email,
		recipient.Name,
		sender.Name,
		req.MessageURL,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: fmt.Sprintf("Failed to send Email notification: %v", err),
		})
		return
	}

	c.Status(http.StatusOK)
}
