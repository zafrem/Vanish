package storage

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/milkiss/vanish/backend/internal/models"
	"github.com/redis/go-redis/v9"
)

// Lua script for atomic GET and DELETE operation
// This ensures the message can only be read once (burn-on-read)
const getAndDeleteScript = `
local key = KEYS[1]
local value = redis.call('GET', key)
if value then
    redis.call('DEL', key)
    return value
else
    return nil
end
`

// RedisStorage implements the Storage interface using Redis
type RedisStorage struct {
	client            *redis.Client
	getAndDeleteSHA   string
}

// NewRedisStorage creates a new Redis storage instance
func NewRedisStorage(address, password string, db int) (*RedisStorage, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
		DB:       db,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	storage := &RedisStorage{
		client: client,
	}

	// Load the Lua script and cache its SHA
	sha, err := client.ScriptLoad(ctx, getAndDeleteScript).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to load Lua script: %w", err)
	}
	storage.getAndDeleteSHA = sha

	return storage, nil
}

// Store saves an encrypted message with a TTL and returns a unique ID
func (r *RedisStorage) Store(ctx context.Context, msg *models.Message, ttl time.Duration) (string, error) {
	// Generate a cryptographically secure random ID
	id, err := generateID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %w", err)
	}

	// Serialize message to JSON
	data, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal message: %w", err)
	}

	// Store in Redis with TTL
	key := messageKey(id)
	err = r.client.Set(ctx, key, data, ttl).Err()
	if err != nil {
		return "", fmt.Errorf("failed to store message: %w", err)
	}

	return id, nil
}

// GetAndDelete atomically retrieves and deletes a message (burn-on-read)
// This uses a Lua script to ensure atomicity and prevent race conditions
func (r *RedisStorage) GetAndDelete(ctx context.Context, id string) (*models.Message, error) {
	key := messageKey(id)

	// Execute the Lua script using its cached SHA
	result, err := r.client.EvalSha(ctx, r.getAndDeleteSHA, []string{key}).Result()
	if err != nil {
		// If script not found (Redis restarted), reload it
		if err.Error() == "NOSCRIPT No matching script. Please use EVAL." {
			sha, loadErr := r.client.ScriptLoad(ctx, getAndDeleteScript).Result()
			if loadErr != nil {
				return nil, fmt.Errorf("failed to reload Lua script: %w", loadErr)
			}
			r.getAndDeleteSHA = sha

			// Retry the operation
			result, err = r.client.EvalSha(ctx, r.getAndDeleteSHA, []string{key}).Result()
			if err != nil {
				return nil, fmt.Errorf("failed to execute script after reload: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to execute script: %w", err)
		}
	}

	// If result is nil, message doesn't exist
	if result == nil {
		return nil, models.ErrMessageNotFound
	}

	// Unmarshal the message
	var msg models.Message
	err = json.Unmarshal([]byte(result.(string)), &msg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return &msg, nil
}

// Exists checks if a message exists without burning it
func (r *RedisStorage) Exists(ctx context.Context, id string) (bool, error) {
	key := messageKey(id)
	count, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check existence: %w", err)
	}
	return count > 0, nil
}

// Ping checks if Redis is reachable
func (r *RedisStorage) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close closes the Redis connection
func (r *RedisStorage) Close() error {
	return r.client.Close()
}

// messageKey generates the Redis key for a message ID
func messageKey(id string) string {
	return fmt.Sprintf("vanish:message:%s", id)
}

// generateID generates a cryptographically secure random ID
// Uses 16 bytes (128 bits) of entropy, base64 URL-encoded
func generateID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
