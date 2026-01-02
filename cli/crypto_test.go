package main

import (
	"encoding/base64"
	"testing"
)

func TestEncryptMessage(t *testing.T) {
	tests := []struct {
		name      string
		plaintext string
		wantErr   bool
	}{
		{
			name:      "simple password",
			plaintext: "MyPassword123",
			wantErr:   false,
		},
		{
			name:      "empty string",
			plaintext: "",
			wantErr:   false, // Empty is allowed, encryption should work
		},
		{
			name:      "long text",
			plaintext: "This is a very long secret message that contains multiple sentences. It should still encrypt and decrypt correctly without any issues.",
			wantErr:   false,
		},
		{
			name:      "special characters",
			plaintext: "P@ssw0rd!#$%^&*(){}[]|\\:;\"'<>,.?/~`",
			wantErr:   false,
		},
		{
			name:      "unicode characters",
			plaintext: "こんにちは世界 🔒🔐🗝️",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := encryptMessage(tt.plaintext)
			if (err != nil) != tt.wantErr {
				t.Errorf("encryptMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return // Expected error
			}

			// Verify structure
			if encrypted.Ciphertext == "" {
				t.Error("Ciphertext is empty")
			}
			if encrypted.IV == "" {
				t.Error("IV is empty")
			}
			if encrypted.Key == "" {
				t.Error("Key is empty")
			}

			// Verify base64 encoding
			if _, err := base64.StdEncoding.DecodeString(encrypted.Ciphertext); err != nil {
				t.Errorf("Ciphertext is not valid base64: %v", err)
			}
			if _, err := base64.StdEncoding.DecodeString(encrypted.IV); err != nil {
				t.Errorf("IV is not valid base64: %v", err)
			}
			if _, err := base64.URLEncoding.DecodeString(encrypted.Key); err != nil {
				t.Errorf("Key is not valid base64url: %v", err)
			}

			// Verify key length (should be 32 bytes for AES-256)
			keyBytes, _ := base64.URLEncoding.DecodeString(encrypted.Key)
			if len(keyBytes) != 32 {
				t.Errorf("Key length = %d, want 32 bytes", len(keyBytes))
			}

			// Verify IV length (should be 12 bytes for GCM)
			ivBytes, _ := base64.StdEncoding.DecodeString(encrypted.IV)
			if len(ivBytes) != 12 {
				t.Errorf("IV length = %d, want 12 bytes", len(ivBytes))
			}
		})
	}
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	// Note: We'd need to implement decryptMessage for this test
	// Since the backend handles decryption, we'll skip this for now
	// But it's good practice to test encryption produces valid output

	plaintext := "Test secret message"
	encrypted, err := encryptMessage(plaintext)
	if err != nil {
		t.Fatalf("encryptMessage() failed: %v", err)
	}

	// Verify we can at least decode the base64
	cipherBytes, err := base64.StdEncoding.DecodeString(encrypted.Ciphertext)
	if err != nil {
		t.Errorf("Failed to decode ciphertext: %v", err)
	}

	// Ciphertext should be longer than plaintext (due to GCM tag)
	// GCM adds 16 bytes authentication tag
	if len(cipherBytes) < len(plaintext) {
		t.Errorf("Ciphertext length %d is less than plaintext length %d", len(cipherBytes), len(plaintext))
	}
}

func TestEncryptMessageUniqueness(t *testing.T) {
	plaintext := "Same message"

	// Encrypt same message twice
	encrypted1, err := encryptMessage(plaintext)
	if err != nil {
		t.Fatalf("First encryption failed: %v", err)
	}

	encrypted2, err := encryptMessage(plaintext)
	if err != nil {
		t.Fatalf("Second encryption failed: %v", err)
	}

	// Keys should be different (randomly generated)
	if encrypted1.Key == encrypted2.Key {
		t.Error("Keys should be unique for each encryption")
	}

	// IVs should be different (randomly generated)
	if encrypted1.IV == encrypted2.IV {
		t.Error("IVs should be unique for each encryption")
	}

	// Ciphertexts should be different (due to unique key and IV)
	if encrypted1.Ciphertext == encrypted2.Ciphertext {
		t.Error("Ciphertexts should be unique for each encryption")
	}
}

// Benchmark encryption performance
func BenchmarkEncryptMessage(b *testing.B) {
	plaintext := "This is a benchmark test message for encryption performance"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := encryptMessage(plaintext)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncryptMessageLong(b *testing.B) {
	// Test with a longer message (1KB)
	plaintext := ""
	for i := 0; i < 1024; i++ {
		plaintext += "x"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := encryptMessage(plaintext)
		if err != nil {
			b.Fatal(err)
		}
	}
}
