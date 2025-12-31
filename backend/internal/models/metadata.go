package models

import "time"

// MessageStatus represents the status of a message
type MessageStatus string

const (
	StatusPending MessageStatus = "pending" // Created but not yet read
	StatusRead    MessageStatus = "read"    // Message has been read and burned
	StatusExpired MessageStatus = "expired" // Message expired before being read
)

// MessageMetadata stores audit information about messages
// CRITICAL: This stores WHO sent to WHOM, but NEVER the actual content
// Content remains ephemeral and zero-knowledge in Redis
// The encryption key is stored to allow recipients to access their messages via the UI
type MessageMetadata struct {
	ID            int64         `json:"id" db:"id"`
	MessageID     string        `json:"message_id" db:"message_id"`       // Links to Redis key
	SenderID      int64         `json:"sender_id" db:"sender_id"`         // Who sent it
	RecipientID   int64         `json:"recipient_id" db:"recipient_id"`   // Who should receive it
	EncryptionKey string        `json:"-" db:"encryption_key"`            // Client-side encryption key (not exposed in API)
	Status        MessageStatus `json:"status" db:"status"`               // Current status
	CreatedAt     time.Time     `json:"created_at" db:"created_at"`       // When created
	ReadAt        *time.Time    `json:"read_at,omitempty" db:"read_at"`   // When read (if applicable)
	ExpiresAt     time.Time     `json:"expires_at" db:"expires_at"`       // When it expires
	SenderName    string        `json:"sender_name,omitempty" db:"-"`     // Populated via join
	RecipientName string        `json:"recipient_name,omitempty" db:"-"`  // Populated via join
}

// MessageHistoryResponse represents a message in the user's history
type MessageHistoryResponse struct {
	MessageID     string        `json:"message_id"`
	SenderName    string        `json:"sender_name"`
	RecipientName string        `json:"recipient_name"`
	Status        MessageStatus `json:"status"`
	CreatedAt     time.Time     `json:"created_at"`
	ReadAt        *time.Time    `json:"read_at,omitempty"`
	ExpiresAt     time.Time     `json:"expires_at"`
	IsSender      bool          `json:"is_sender"`        // True if current user is sender
	IsRecipient   bool          `json:"is_recipient"`     // True if current user is recipient
	EncryptionKey string        `json:"encryption_key,omitempty"` // Only included for recipients with pending messages
}
