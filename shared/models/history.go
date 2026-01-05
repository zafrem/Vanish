package models

import "time"

// MessageStatus represents the status of a message
type MessageStatus string

const (
	// StatusPending indicates the message was created but not yet read
	StatusPending MessageStatus = "pending"
	// StatusRead indicates the message has been read and burned
	StatusRead MessageStatus = "read"
	// StatusExpired indicates the message expired before being read
	StatusExpired MessageStatus = "expired"
)

// MessageHistoryResponse represents a message in the user's history
// Returned by GET /api/history endpoint
type MessageHistoryResponse struct {
	MessageID     string        `json:"message_id"`
	SenderName    string        `json:"sender_name"`
	RecipientName string        `json:"recipient_name"`
	Status        MessageStatus `json:"status"`
	CreatedAt     time.Time     `json:"created_at"`
	ReadAt        *time.Time    `json:"read_at,omitempty"`
	ExpiresAt     time.Time     `json:"expires_at"`
	IsSender      bool          `json:"is_sender"`                    // True if current user is sender
	IsRecipient   bool          `json:"is_recipient"`                 // True if current user is recipient
	EncryptionKey string        `json:"encryption_key,omitempty"`     // Only included for recipients with pending messages
}

// HistoryResponse represents the full history response
type HistoryResponse struct {
	Messages []MessageHistoryResponse `json:"messages"`
}
