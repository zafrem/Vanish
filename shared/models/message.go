package models

import "time"

// CreateMessageRequest represents the request body for creating a message
// This is sent from CLI/MCP to the backend API
type CreateMessageRequest struct {
	Ciphertext    string `json:"ciphertext" binding:"required,base64"`
	IV            string `json:"iv" binding:"required,base64"`
	TTL           int64  `json:"ttl,omitempty"`                    // Time to live in seconds
	RecipientID   int64  `json:"recipient_id" binding:"required"`  // Who can read this message
	EncryptionKey string `json:"encryption_key" binding:"required"` // Client-side encryption key
}

// CreateMessageResponse represents the response after creating a message
type CreateMessageResponse struct {
	ID        string    `json:"id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// MessageResponse represents the response when retrieving a message
// Not typically used by CLI/MCP but included for completeness
type MessageResponse struct {
	Ciphertext string `json:"ciphertext"`
	IV         string `json:"iv"`
}
