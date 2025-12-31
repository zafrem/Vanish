package models

import (
	"errors"
	"time"
)

var (
	// ErrMessageNotFound is returned when a message doesn't exist or has been burned
	ErrMessageNotFound = errors.New("message not found or already burned")
	// ErrInvalidTTL is returned when TTL is out of acceptable range
	ErrInvalidTTL = errors.New("TTL must be between 1 hour and 7 days")
	// ErrInvalidInput is returned for validation failures
	ErrInvalidInput = errors.New("invalid input data")
)

// Message represents the encrypted message stored in Redis
type Message struct {
	Ciphertext string    `json:"ciphertext"`
	IV         string    `json:"iv"`
	CreatedAt  time.Time `json:"created_at"`
}

// CreateMessageRequest represents the request body for creating a message
type CreateMessageRequest struct {
	Ciphertext    string `json:"ciphertext" binding:"required,base64"`
	IV            string `json:"iv" binding:"required,base64"`
	TTL           *int64 `json:"ttl,omitempty"`                       // in seconds, optional
	RecipientID   int64  `json:"recipient_id" binding:"required"`     // Who can read this message
	EncryptionKey string `json:"encryption_key" binding:"required"`   // Client-side encryption key for recipient access
}

// CreateMessageResponse represents the response after creating a message
type CreateMessageResponse struct {
	ID        string    `json:"id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// MessageResponse represents the response when retrieving a message
type MessageResponse struct {
	Ciphertext string `json:"ciphertext"`
	IV         string `json:"iv"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// Constants for TTL limits
const (
	MinTTL     = 3600       // 1 hour in seconds
	MaxTTL     = 604800     // 7 days in seconds
	DefaultTTL = 86400      // 24 hours in seconds
)

// ValidateTTL validates and returns the TTL to use
func ValidateTTL(ttl *int64) (int64, error) {
	if ttl == nil {
		return DefaultTTL, nil
	}

	if *ttl < MinTTL || *ttl > MaxTTL {
		return 0, ErrInvalidTTL
	}

	return *ttl, nil
}
