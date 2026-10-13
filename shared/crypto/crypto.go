package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
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

// errInvalidKey never includes the key itself, so it is safe to print.
var errInvalidKey = errors.New("invalid encryption key: expected 32 bytes of base64")

// DecodeKey decodes a link's encryption key. It accepts the URL-safe or
// standard base64 alphabet, padded or unpadded (the frontend accepts all of
// them too), and requires exactly 32 bytes. Call it before fetching a message
// so a mistyped key does not burn it.
func DecodeKey(s string) ([]byte, error) {
	for _, enc := range []*base64.Encoding{
		base64.URLEncoding, base64.RawURLEncoding, base64.StdEncoding, base64.RawStdEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil {
			if len(b) != keySize {
				return nil, errInvalidKey
			}
			return b, nil
		}
	}
	return nil, errInvalidKey
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

	keyBytes, err := DecodeKey(keyStr)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// gcm.Open panics on a wrong-length nonce; the IV comes from the server.
	if len(ivBytes) != gcm.NonceSize() {
		return nil, fmt.Errorf("invalid IV length %d", len(ivBytes))
	}

	plaintext, err := gcm.Open(nil, ivBytes, ciphertextBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}
