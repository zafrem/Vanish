package unit

import (
	"testing"
	"time"

	"github.com/milkiss/vanish/backend/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTManager_Generate(t *testing.T) {
	manager := auth.NewJWTManager("test-secret-key", 24*time.Hour)

	token, err := manager.Generate(123, "test@example.com")
	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestJWTManager_Verify_Success(t *testing.T) {
	manager := auth.NewJWTManager("test-secret-key", 24*time.Hour)

	// Generate a token
	token, err := manager.Generate(123, "test@example.com")
	require.NoError(t, err)

	// Verify the token
	claims, err := manager.Verify(token)
	require.NoError(t, err)
	assert.Equal(t, int64(123), claims.UserID)
	assert.Equal(t, "test@example.com", claims.Email)
}

func TestJWTManager_Verify_InvalidToken(t *testing.T) {
	manager := auth.NewJWTManager("test-secret-key", 24*time.Hour)

	// Try to verify an invalid token
	_, err := manager.Verify("invalid.token.here")
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
}

func TestJWTManager_Verify_WrongSecretKey(t *testing.T) {
	manager1 := auth.NewJWTManager("secret1", 24*time.Hour)
	manager2 := auth.NewJWTManager("secret2", 24*time.Hour)

	// Generate token with manager1
	token, err := manager1.Generate(123, "test@example.com")
	require.NoError(t, err)

	// Try to verify with manager2 (different secret key)
	_, err = manager2.Verify(token)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
}

func TestJWTManager_Verify_ExpiredToken(t *testing.T) {
	// Create manager with very short duration
	manager := auth.NewJWTManager("test-secret-key", 1*time.Millisecond)

	// Generate token
	token, err := manager.Generate(123, "test@example.com")
	require.NoError(t, err)

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Try to verify expired token
	_, err = manager.Verify(token)
	assert.ErrorIs(t, err, auth.ErrExpiredToken)
}

func TestJWTManager_Verify_PreservesUserData(t *testing.T) {
	manager := auth.NewJWTManager("test-secret-key", 24*time.Hour)

	testCases := []struct {
		userID int64
		email  string
	}{
		{1, "user1@example.com"},
		{999, "admin@example.com"},
		{12345, "test+tag@example.com"},
	}

	for _, tc := range testCases {
		token, err := manager.Generate(tc.userID, tc.email)
		require.NoError(t, err)

		claims, err := manager.Verify(token)
		require.NoError(t, err)
		assert.Equal(t, tc.userID, claims.UserID)
		assert.Equal(t, tc.email, claims.Email)
	}
}
