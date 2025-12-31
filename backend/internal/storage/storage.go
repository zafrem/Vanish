package storage

import (
	"context"
	"time"

	"github.com/milkiss/vanish/backend/internal/models"
)

// Storage defines the interface for message storage operations
type Storage interface {
	// Store saves an encrypted message with a TTL and returns a unique ID
	Store(ctx context.Context, msg *models.Message, ttl time.Duration) (string, error)

	// GetAndDelete atomically retrieves and deletes a message (burn-on-read)
	GetAndDelete(ctx context.Context, id string) (*models.Message, error)

	// Exists checks if a message exists without burning it
	Exists(ctx context.Context, id string) (bool, error)

	// Close closes the storage connection
	Close() error

	// Ping checks if the storage is reachable
	Ping(ctx context.Context) error
}
