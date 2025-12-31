package unit

import (
	"testing"

	"github.com/milkiss/vanish/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateTTL_Default(t *testing.T) {
	ttl, err := models.ValidateTTL(nil)
	require.NoError(t, err)
	assert.Equal(t, int64(models.DefaultTTL), ttl)
}

func TestValidateTTL_Valid(t *testing.T) {
	validTTL := int64(7200) // 2 hours
	ttl, err := models.ValidateTTL(&validTTL)
	require.NoError(t, err)
	assert.Equal(t, validTTL, ttl)
}

func TestValidateTTL_TooShort(t *testing.T) {
	tooShort := int64(1800) // 30 minutes (less than MinTTL)
	_, err := models.ValidateTTL(&tooShort)
	assert.ErrorIs(t, err, models.ErrInvalidTTL)
}

func TestValidateTTL_TooLong(t *testing.T) {
	tooLong := int64(1000000) // More than 7 days
	_, err := models.ValidateTTL(&tooLong)
	assert.ErrorIs(t, err, models.ErrInvalidTTL)
}

func TestValidateTTL_BoundaryValues(t *testing.T) {
	// Test minimum boundary
	minTTL := int64(models.MinTTL)
	ttl, err := models.ValidateTTL(&minTTL)
	require.NoError(t, err)
	assert.Equal(t, minTTL, ttl)

	// Test maximum boundary
	maxTTL := int64(models.MaxTTL)
	ttl, err = models.ValidateTTL(&maxTTL)
	require.NoError(t, err)
	assert.Equal(t, maxTTL, ttl)
}

func TestHashPassword(t *testing.T) {
	password := "mySecurePassword123"
	hash, err := models.HashPassword(password)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, password, hash) // Hash should be different from plaintext
}

func TestHashPassword_DifferentHashesForSamePassword(t *testing.T) {
	password := "mySecurePassword123"
	hash1, err := models.HashPassword(password)
	require.NoError(t, err)

	hash2, err := models.HashPassword(password)
	require.NoError(t, err)

	// Bcrypt generates different salts, so hashes should be different
	assert.NotEqual(t, hash1, hash2)
}

func TestUser_CheckPassword_Success(t *testing.T) {
	password := "mySecurePassword123"
	hash, err := models.HashPassword(password)
	require.NoError(t, err)

	user := &models.User{
		Email:    "test@example.com",
		Password: hash,
	}

	assert.True(t, user.CheckPassword(password))
}

func TestUser_CheckPassword_Failure(t *testing.T) {
	password := "mySecurePassword123"
	wrongPassword := "wrongPassword456"
	hash, err := models.HashPassword(password)
	require.NoError(t, err)

	user := &models.User{
		Email:    "test@example.com",
		Password: hash,
	}

	assert.False(t, user.CheckPassword(wrongPassword))
}

func TestUser_ToUserInfo(t *testing.T) {
	user := &models.User{
		ID:       123,
		Email:    "test@example.com",
		Name:     "Test User",
		Password: "hashed-password",
	}

	userInfo := user.ToUserInfo()
	assert.Equal(t, user.ID, userInfo.ID)
	assert.Equal(t, user.Email, userInfo.Email)
	assert.Equal(t, user.Name, userInfo.Name)
}

func TestUser_ToUserInfo_NoPasswordLeak(t *testing.T) {
	user := &models.User{
		ID:       123,
		Email:    "test@example.com",
		Name:     "Test User",
		Password: "sensitive-password-hash",
	}

	userInfo := user.ToUserInfo()
	// UserInfo struct doesn't have a Password field, so this ensures no password leak
	assert.NotNil(t, userInfo)
	assert.Equal(t, user.ID, userInfo.ID)
}
