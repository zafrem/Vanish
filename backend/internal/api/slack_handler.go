package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/milkiss/vanish/backend/internal/integrations/slack"
	"github.com/milkiss/vanish/backend/internal/models"
	"github.com/milkiss/vanish/backend/internal/repository"
	"github.com/milkiss/vanish/backend/internal/storage"
)

// SlackHandler handles Slack slash commands and interactions
type SlackHandler struct {
	slackClient  *slack.Client
	storage      storage.Storage
	metadataRepo *repository.MetadataRepository
	userRepo     *repository.UserRepository
	signingSecret string
	baseURL      string
}

// NewSlackHandler creates a new Slack handler
func NewSlackHandler(
	slackClient *slack.Client,
	storage storage.Storage,
	metadataRepo *repository.MetadataRepository,
	userRepo *repository.UserRepository,
	signingSecret string,
	baseURL string,
) *SlackHandler {
	return &SlackHandler{
		slackClient:  slackClient,
		storage:      storage,
		metadataRepo: metadataRepo,
		userRepo:     userRepo,
		signingSecret: signingSecret,
		baseURL:      baseURL,
	}
}

// SlashCommandPayload represents the payload from Slack slash command
type SlashCommandPayload struct {
	Token       string `form:"token"`
	TeamID      string `form:"team_id"`
	TeamDomain  string `form:"team_domain"`
	ChannelID   string `form:"channel_id"`
	ChannelName string `form:"channel_name"`
	UserID      string `form:"user_id"`
	UserName    string `form:"user_name"`
	Command     string `form:"command"`
	Text        string `form:"text"`
	ResponseURL string `form:"response_url"`
	TriggerID   string `form:"trigger_id"`
}

// InteractionPayload represents a Slack interaction payload
type InteractionPayload struct {
	Type        string                 `json:"type"`
	User        InteractionUser        `json:"user"`
	TriggerID   string                 `json:"trigger_id"`
	Team        InteractionTeam        `json:"team"`
	View        *InteractionView       `json:"view,omitempty"`
	ResponseURL string                 `json:"response_url,omitempty"`
}

type InteractionUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

type InteractionTeam struct {
	ID     string `json:"id"`
	Domain string `json:"domain"`
}

type InteractionView struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	State         InteractionViewState   `json:"state"`
	PrivateMetadata string               `json:"private_metadata"`
}

type InteractionViewState struct {
	Values map[string]map[string]InteractionValue `json:"values"`
}

type InteractionValue struct {
	Type           string                  `json:"type"`
	Value          string                  `json:"value,omitempty"`
	SelectedUser   string                  `json:"selected_user,omitempty"`
	SelectedOption *InteractionOption      `json:"selected_option,omitempty"`
}

type InteractionOption struct {
	Text  InteractionText `json:"text"`
	Value string          `json:"value"`
}

type InteractionText struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Emoji bool   `json:"emoji,omitempty"`
}

// HandleSlashCommand handles the /vanishPW slash command
func (h *SlackHandler) HandleSlashCommand(c *gin.Context) {
	// Verify Slack request signature
	if !h.verifySlackRequest(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
		return
	}

	var payload SlashCommandPayload
	if err := c.ShouldBind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	// Open modal for password input
	modal := h.buildPasswordModal()

	if err := h.slackClient.OpenModal(c.Request.Context(), payload.TriggerID, modal); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"response_type": "ephemeral",
			"text": fmt.Sprintf("Failed to open modal: %v", err),
		})
		return
	}

	// Return 200 OK immediately (Slack requires quick response)
	c.Status(http.StatusOK)
}

// HandleInteraction handles Slack interactive components (modal submissions)
func (h *SlackHandler) HandleInteraction(c *gin.Context) {
	// Verify Slack request signature
	if !h.verifySlackRequest(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
		return
	}

	// Parse the payload from form data
	payloadStr := c.PostForm("payload")
	if payloadStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing payload"})
		return
	}

	var payload InteractionPayload
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	// Handle view submission (modal submit)
	if payload.Type == "view_submission" && payload.View != nil {
		h.handleModalSubmission(c, &payload)
		return
	}

	c.Status(http.StatusOK)
}

// handleModalSubmission processes the modal submission
func (h *SlackHandler) handleModalSubmission(c *gin.Context, payload *InteractionPayload) {
	ctx := c.Request.Context()

	// Extract values from modal
	values := payload.View.State.Values

	recipientEmail := values["recipient_block"]["recipient_input"].Value
	password := values["password_block"]["password_input"].Value
	ttlStr := values["ttl_block"]["ttl_input"].SelectedOption.Value

	// Parse TTL
	ttlSeconds, err := strconv.ParseInt(ttlStr, 10, 64)
	if err != nil {
		ttlSeconds = models.DefaultTTL
	}

	// Validate TTL
	if ttlSeconds < models.MinTTL || ttlSeconds > models.MaxTTL {
		c.JSON(http.StatusOK, gin.H{
			"response_action": "errors",
			"errors": gin.H{
				"ttl_block": "Invalid expiration time",
			},
		})
		return
	}

	// Get sender's Slack user info to find their email
	senderInfo, err := h.slackClient.GetUserInfo(ctx, payload.User.ID)
	if err != nil {
		h.sendEphemeralError(ctx, payload.User.ID, "Failed to get your user information")
		c.Status(http.StatusOK)
		return
	}

	// Find sender in database by email
	sender, err := h.userRepo.FindByEmail(ctx, senderInfo.Email)
	if err != nil {
		h.sendEphemeralError(ctx, payload.User.ID, "You must be registered in Vanish to send messages. Please register at "+h.baseURL)
		c.Status(http.StatusOK)
		return
	}

	// Find recipient in database
	recipient, err := h.userRepo.FindByEmail(ctx, recipientEmail)
	if err != nil {
		h.sendEphemeralError(ctx, payload.User.ID, "Recipient not found. They must be registered in Vanish first.")
		c.Status(http.StatusOK)
		return
	}

	// Encrypt the password using server-side encryption (similar to client-side AES-256-GCM)
	encryptedMsg, err := encryptMessage(password)
	if err != nil {
		h.sendEphemeralError(ctx, payload.User.ID, "Failed to encrypt message")
		c.Status(http.StatusOK)
		return
	}

	// Create message object
	msg := &models.Message{
		Ciphertext: encryptedMsg.Ciphertext,
		IV:         encryptedMsg.IV,
		CreatedAt:  time.Now().UTC(),
	}

	// Store encrypted message in Redis with TTL
	id, err := h.storage.Store(ctx, msg, time.Duration(ttlSeconds)*time.Second)
	if err != nil {
		h.sendEphemeralError(ctx, payload.User.ID, "Failed to store message")
		c.Status(http.StatusOK)
		return
	}

	// Calculate expiration time
	expiresAt := msg.CreatedAt.Add(time.Duration(ttlSeconds) * time.Second)

	// Store metadata in PostgreSQL
	metadata := &models.MessageMetadata{
		MessageID:     id,
		SenderID:      sender.ID,
		RecipientID:   recipient.ID,
		EncryptionKey: encryptedMsg.Key,
		Status:        models.StatusPending,
		CreatedAt:     msg.CreatedAt,
		ExpiresAt:     expiresAt,
	}

	if err := h.metadataRepo.Create(ctx, metadata); err != nil {
		h.sendEphemeralError(ctx, payload.User.ID, "Failed to store message metadata")
		c.Status(http.StatusOK)
		return
	}

	// Build the shareable URL with encryption key
	secretURL := fmt.Sprintf("%s/m/%s#%s", h.baseURL, id, encryptedMsg.Key)

	// Send DM to recipient with the URL
	err = h.slackClient.SendSecretNotification(ctx, recipient.Email, sender.Name, secretURL)
	if err != nil {
		// Log error but don't fail - sender can still share URL manually
		h.sendEphemeralError(ctx, payload.User.ID, fmt.Sprintf("Message created but failed to notify recipient via Slack. Share this URL manually: %s", secretURL))
		c.Status(http.StatusOK)
		return
	}

	// Send confirmation to sender
	confirmMsg := fmt.Sprintf("✅ Secure message sent to %s (%s)\n\nThey will receive a notification in Slack with a one-time access link.\n\nExpires: %s",
		recipient.Name,
		recipient.Email,
		expiresAt.Format(time.RFC1123),
	)
	h.slackClient.SendEphemeralMessage(ctx, payload.User.ID, confirmMsg)

	// Return success (closes modal)
	c.Status(http.StatusOK)
}

// buildPasswordModal creates the modal view for password input
func (h *SlackHandler) buildPasswordModal() map[string]interface{} {
	return map[string]interface{}{
		"type": "modal",
		"callback_id": "vanish_password_modal",
		"title": map[string]interface{}{
			"type": "plain_text",
			"text": "Send Secure Message",
		},
		"submit": map[string]interface{}{
			"type": "plain_text",
			"text": "Send",
		},
		"close": map[string]interface{}{
			"type": "plain_text",
			"text": "Cancel",
		},
		"blocks": []map[string]interface{}{
			{
				"type": "input",
				"block_id": "recipient_block",
				"element": map[string]interface{}{
					"type": "plain_text_input",
					"action_id": "recipient_input",
					"placeholder": map[string]interface{}{
						"type": "plain_text",
						"text": "recipient@example.com",
					},
				},
				"label": map[string]interface{}{
					"type": "plain_text",
					"text": "Recipient Email",
				},
			},
			{
				"type": "input",
				"block_id": "password_block",
				"element": map[string]interface{}{
					"type": "plain_text_input",
					"action_id": "password_input",
					"multiline": true,
					"placeholder": map[string]interface{}{
						"type": "plain_text",
						"text": "Enter the password or secret message",
					},
				},
				"label": map[string]interface{}{
					"type": "plain_text",
					"text": "Secret Message",
				},
			},
			{
				"type": "input",
				"block_id": "ttl_block",
				"element": map[string]interface{}{
					"type": "static_select",
					"action_id": "ttl_input",
					"placeholder": map[string]interface{}{
						"type": "plain_text",
						"text": "Select expiration time",
					},
					"initial_option": map[string]interface{}{
						"text": map[string]interface{}{
							"type": "plain_text",
							"text": "24 hours",
						},
						"value": "86400",
					},
					"options": []map[string]interface{}{
						{
							"text": map[string]interface{}{
								"type": "plain_text",
								"text": "1 hour",
							},
							"value": "3600",
						},
						{
							"text": map[string]interface{}{
								"type": "plain_text",
								"text": "24 hours",
							},
							"value": "86400",
						},
						{
							"text": map[string]interface{}{
								"type": "plain_text",
								"text": "3 days",
							},
							"value": "259200",
						},
						{
							"text": map[string]interface{}{
								"type": "plain_text",
								"text": "7 days",
							},
							"value": "604800",
						},
					},
				},
				"label": map[string]interface{}{
					"type": "plain_text",
					"text": "Expires In",
				},
			},
			{
				"type": "context",
				"elements": []map[string]interface{}{
					{
						"type": "mrkdwn",
						"text": "🔒 Your message will be encrypted and can only be read once. It will be permanently destroyed after the recipient views it or when it expires.",
					},
				},
			},
		},
	}
}

// verifySlackRequest verifies the Slack request signature
func (h *SlackHandler) verifySlackRequest(c *gin.Context) bool {
	// Skip verification if signing secret is not configured (dev mode)
	if h.signingSecret == "" {
		return true
	}

	timestamp := c.GetHeader("X-Slack-Request-Timestamp")
	signature := c.GetHeader("X-Slack-Signature")

	if timestamp == "" || signature == "" {
		return false
	}

	// Check timestamp (protect against replay attacks)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}

	if time.Now().Unix()-ts > 300 { // 5 minutes
		return false
	}

	// Read body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return false
	}

	// Restore body for later use
	c.Request.Body = io.NopCloser(strings.NewReader(string(body)))

	// Calculate signature
	baseString := fmt.Sprintf("v0:%s:%s", timestamp, body)
	hash := hmac.New(sha256.New, []byte(h.signingSecret))
	hash.Write([]byte(baseString))
	expectedSignature := "v0=" + hex.EncodeToString(hash.Sum(nil))

	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}

// sendEphemeralError sends an ephemeral error message to a user
func (h *SlackHandler) sendEphemeralError(ctx context.Context, userID, message string) {
	h.slackClient.SendEphemeralMessage(ctx, userID, "❌ "+message)
}
