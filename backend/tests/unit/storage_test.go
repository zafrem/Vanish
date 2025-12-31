package unit

import (
	"context"
	"testing"
	"time"

	"github.com/milkiss/vanish/backend/internal/models"
	"github.com/milkiss/vanish/backend/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: These tests require a running Redis instance
// For unit testing, consider using miniredis or test containers

func setupTestStorage(t *testing.T) storage.Storage {
	// Connect to test Redis instance
	store, err := storage.NewRedisStorage("localhost:6379", "", 1) // Use DB 1 for testing
	require.NoError(t, err, "Failed to connect to test Redis")
	return store
}

func TestStoreAndRetrieve(t *testing.T) {
	store := setupTestStorage(t)
	defer store.Close()

	ctx := context.Background()

	msg := &models.Message{
		Ciphertext: "encrypted-data",
		IV:         "initialization-vector",
		CreatedAt:  time.Now().UTC(),
	}

	// Store message
	id, err := store.Store(ctx, msg, 1*time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// Retrieve message (should burn it)
	retrieved, err := store.GetAndDelete(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, msg.Ciphertext, retrieved.Ciphertext)
	assert.Equal(t, msg.IV, retrieved.IV)

	// Try to retrieve again (should fail - already burned)
	_, err = store.GetAndDelete(ctx, id)
	assert.Equal(t, models.ErrMessageNotFound, err)
}

func TestAtomicGetAndDelete(t *testing.T) {
	store := setupTestStorage(t)
	defer store.Close()

	ctx := context.Background()

	msg := &models.Message{
		Ciphertext: "test-data",
		IV:         "test-iv",
		CreatedAt:  time.Now().UTC(),
	}

	// Store message
	id, err := store.Store(ctx, msg, 1*time.Hour)
	require.NoError(t, err)

	// Simulate concurrent access
	results := make(chan error, 5)

	for i := 0; i < 5; i++ {
		go func() {
			_, err := store.GetAndDelete(ctx, id)
			results <- err
		}()
	}

	// Collect results
	successCount := 0
	notFoundCount := 0

	for i := 0; i < 5; i++ {
		err := <-results
		if err == nil {
			successCount++
		} else if err == models.ErrMessageNotFound {
			notFoundCount++
		}
	}

	// Only one goroutine should succeed
	assert.Equal(t, 1, successCount, "Exactly one read should succeed")
	assert.Equal(t, 4, notFoundCount, "Four reads should fail with not found")
}

func TestMessageExpiry(t *testing.T) {
	store := setupTestStorage(t)
	defer store.Close()

	ctx := context.Background()

	msg := &models.Message{
		Ciphertext: "expiring-data",
		IV:         "expiring-iv",
		CreatedAt:  time.Now().UTC(),
	}

	// Store with very short TTL
	id, err := store.Store(ctx, msg, 2*time.Second)
	require.NoError(t, err)

	// Message should exist initially
	exists, err := store.Exists(ctx, id)
	require.NoError(t, err)
	assert.True(t, exists)

	// Wait for expiration
	time.Sleep(3 * time.Second)

	// Message should be gone
	exists, err = store.Exists(ctx, id)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestExists(t *testing.T) {
	store := setupTestStorage(t)
	defer store.Close()

	ctx := context.Background()

	msg := &models.Message{
		Ciphertext: "test-data",
		IV:         "test-iv",
		CreatedAt:  time.Now().UTC(),
	}

	// Store message
	id, err := store.Store(ctx, msg, 1*time.Hour)
	require.NoError(t, err)

	// Check existence (should not burn)
	exists, err := store.Exists(ctx, id)
	require.NoError(t, err)
	assert.True(t, exists)

	// Message should still be retrievable
	retrieved, err := store.GetAndDelete(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, msg.Ciphertext, retrieved.Ciphertext)

	// Now it should not exist
	exists, err = store.Exists(ctx, id)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestMessageNotFound(t *testing.T) {
	store := setupTestStorage(t)
	defer store.Close()

	ctx := context.Background()

	// Try to retrieve non-existent message
	_, err := store.GetAndDelete(ctx, "non-existent-id")
	assert.Equal(t, models.ErrMessageNotFound, err)
}

func TestPing(t *testing.T) {
	store := setupTestStorage(t)
	defer store.Close()

	ctx := context.Background()

	err := store.Ping(ctx)
	assert.NoError(t, err)
}
