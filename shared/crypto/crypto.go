package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

const (
	keySize   = 32 // AES-256
	nonceSize = 12 // 96-bit IV
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
	return EncryptBytes([]byte(plaintext))
}

// EncryptBytes encrypts arbitrary bytes using AES-256-GCM with a fresh random
// key and IV. Ciphertext and IV are standard base64; the key is URL-safe base64.
func EncryptBytes(plaintext []byte) (*EncryptedMessage, error) {
	key := make([]byte, keySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	return seal(key, nonce, plaintext)
}

// seal encrypts with a caller-supplied key and nonce. Callers must never reuse
// a (key, nonce) pair; it exists so tests can build deterministic vectors.
func seal(key, nonce, plaintext []byte) (*EncryptedMessage, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("invalid nonce size %d", len(nonce))
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

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
	plaintext, err := DecryptBytes(ciphertext, iv, keyStr)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// DecryptBytes decrypts a message encrypted with EncryptBytes (or by the
// frontend) and returns the raw plaintext bytes. The key may be URL-safe or
// standard base64.
func DecryptBytes(ciphertext, iv, keyStr string) ([]byte, error) {
	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	ivBytes, err := base64.StdEncoding.DecodeString(iv)
	if err != nil {
		return nil, fmt.Errorf("failed to decode IV: %w", err)
	}

	// Key might be base64url encoded (URL-safe)
	keyBytes, err := base64.URLEncoding.DecodeString(keyStr)
	if err != nil {
		// Try standard base64 if URL encoding fails
		keyBytes, err = base64.StdEncoding.DecodeString(keyStr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode key: %w", err)
		}
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, ivBytes, ciphertextBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}
