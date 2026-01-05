package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// EncryptedMessage represents the encrypted data structure
type EncryptedMessage struct {
	Ciphertext string
	IV         string
	Key        string
}

// EncryptMessage encrypts a plaintext message using AES-256-GCM
// Returns the encrypted message with ciphertext, IV, and key
func EncryptMessage(plaintext string) (*EncryptedMessage, error) {
	// Generate a random 256-bit encryption key
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	return &EncryptedMessage{
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		IV:         base64.StdEncoding.EncodeToString(nonce),
		Key:        base64.URLEncoding.EncodeToString(key),
	}, nil
}

// DecryptMessage decrypts a message encrypted with EncryptMessage
// This is provided for completeness but may not be used by CLI/MCP
// (decryption typically happens in the frontend)
func DecryptMessage(ciphertext, iv, keyStr string) (string, error) {
	// Decode base64 strings
	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	ivBytes, err := base64.StdEncoding.DecodeString(iv)
	if err != nil {
		return "", fmt.Errorf("failed to decode IV: %w", err)
	}

	// Key might be base64url encoded (URL-safe)
	keyBytes, err := base64.URLEncoding.DecodeString(keyStr)
	if err != nil {
		// Try standard base64 if URL encoding fails
		keyBytes, err = base64.StdEncoding.DecodeString(keyStr)
		if err != nil {
			return "", fmt.Errorf("failed to decode key: %w", err)
		}
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, ivBytes, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}
