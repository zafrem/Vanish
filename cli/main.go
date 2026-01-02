package main

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config represents the CLI configuration
type Config struct {
	BaseURL string `json:"base_url"`
	Token   string `json:"token"`
}

// EncryptedMessage represents the encrypted data structure
type EncryptedMessage struct {
	Ciphertext string
	IV         string
	Key        string
}

// CreateMessageRequest matches the backend request
type CreateMessageRequest struct {
	Ciphertext    string `json:"ciphertext"`
	IV            string `json:"iv"`
	RecipientID   int64  `json:"recipient_id"`
	EncryptionKey string `json:"encryption_key"`
	TTL           int64  `json:"ttl"`
}

// CreateMessageResponse matches the backend response
type CreateMessageResponse struct {
	ID        string    `json:"id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// User represents a user from the API
type User struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func main() {
	configCmd := flag.NewFlagSet("config", flag.ExitOnError)
	sendCmd := flag.NewFlagSet("send", flag.ExitOnError)

	// Send flags
	ttl := sendCmd.Int64("ttl", 86400, "Time to live in seconds (default 24h)")

	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "config":
		configCmd.Parse(os.Args[2:])
		runConfig()
	case "send":
		sendCmd.Parse(os.Args[2:])
		runSend(sendCmd.Args(), *ttl)
	default:
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("Vanish CLI Tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  vanish config             Configure the CLI (interactive)")
	fmt.Println("  vanish send <email> [msg] Send a secret to a user")
	fmt.Println()
	fmt.Println("Flags for send:")
	fmt.Println("  -ttl <seconds>            Expiration time (default 86400)")
}

func getConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".vanish", "config.json"), nil
}

func loadConfig() (*Config, error) {
	path, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func saveConfig(cfg *Config) error {
	path, err := getConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func runConfig() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter Vanish Server URL [http://localhost:8080]: ")
	url, _ := reader.ReadString('\n')
	url = strings.TrimSpace(url)
	if url == "" {
		url = "http://localhost:8080"
	}

	fmt.Print("Enter API Token (get from local storage/login): ")
	token, _ := reader.ReadString('\n')
	token = strings.TrimSpace(token)

	cfg := &Config{
		BaseURL: url,
		Token:   token,
	}

	if err := saveConfig(cfg); err != nil {
		fmt.Printf("Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Configuration saved successfully!")
}

func runSend(args []string, ttl int64) {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\nRun 'vanish config' first.\n", err)
		os.Exit(1)
	}

	if len(args) < 1 {
		fmt.Println("Usage: vanish send <email> [message]")
		os.Exit(1)
	}

	recipientEmail := args[0]
	var secret string

	if len(args) > 1 {
		secret = strings.Join(args[1:], " ")
	} else {
		// Read from stdin
		info, err := os.Stdin.Stat()
		if err != nil {
			fmt.Printf("Error checking stdin: %v\n", err)
			os.Exit(1)
		}

		if (info.Mode() & os.ModeCharDevice) == 0 {
			// Data is being piped
			bytes, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Printf("Error reading stdin: %v\n", err)
				os.Exit(1)
			}
			secret = string(bytes)
		} else {
			// Prompt user
			fmt.Print("Enter secret: ")
			reader := bufio.NewReader(os.Stdin)
			secret, _ = reader.ReadString('\n')
		}
	}
	secret = strings.TrimSpace(secret)
	if secret == "" {
		fmt.Println("Error: Secret message cannot be empty")
		os.Exit(1)
	}

	// 1. Find User ID
	recipientID, err := findUserID(cfg, recipientEmail)
	if err != nil {
		fmt.Printf("Error finding user: %v\n", err)
		os.Exit(1)
	}

	// 2. Encrypt Message
	encrypted, err := encryptMessage(secret)
	if err != nil {
		fmt.Printf("Error encrypting message: %v\n", err)
		os.Exit(1)
	}

	// 3. Send to API
	url, err := sendToAPI(cfg, recipientID, encrypted, ttl)
	if err != nil {
		fmt.Printf("Error sending message: %v\n", err)
		os.Exit(1)
	}

	// 4. Notify (Automatic via API now if enabled, but let's just print the URL)
	fmt.Println("✓ Secret created successfully!")
	fmt.Printf("🔗 %s\n", url)
	
	// Try to notify via Slack automatically if available
	// The backend handles this if we call the notification endpoint, 
	// but the createMessage endpoint doesn't automatically notify yet (per previous plan).
	// We can add a call to the notification endpoint here if we want to be fancy.
	
	fmt.Println("\nAttempting to send Slack notification...")
	if err := sendSlackNotification(cfg, recipientID, url); err != nil {
		fmt.Printf("Could not auto-send Slack notification: %v\n", err)
	} else {
		fmt.Println("✓ Notification sent via Slack")
	}
}

func findUserID(cfg *Config, email string) (int64, error) {
	req, err := http.NewRequest("GET", cfg.BaseURL+"/api/users", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("api returned status: %s", resp.Status)
	}

	var users []User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return 0, err
	}

	for _, u := range users {
		if strings.EqualFold(u.Email, email) {
			return u.ID, nil
		}
	}

	return 0, fmt.Errorf("user not found: %s", email)
}

func encryptMessage(plaintext string) (*EncryptedMessage, error) {
	// Generate a random 256-bit encryption key
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
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

func sendToAPI(cfg *Config, recipientID int64, encrypted *EncryptedMessage, ttl int64) (string, error) {
	payload := CreateMessageRequest{
		Ciphertext:    encrypted.Ciphertext,
		IV:            encrypted.IV,
		RecipientID:   recipientID,
		EncryptionKey: encrypted.Key,
		TTL:           ttl,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", cfg.BaseURL+"/api/messages", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("api error: %s - %s", resp.Status, string(b))
	}

	var result CreateMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	// Construct shareable URL
	// We need to know the base frontend URL, which might differ from API URL.
	// For now, assume API URL is base URL or close to it.
	// Actually, usually cfg.BaseURL is the API URL (e.g. localhost:8080), 
	// while frontend is localhost:3000.
	// Ideally config should ask for frontend URL too, but let's infer or just print the API-based one 
	// and let the user replace port if needed, or simpler: just print it.
	// The backend routes.go uses "BASE_URL" env var. 
	
	// Let's assume the user configures the *Frontend* URL as BaseURL if they want pretty links,
	// but they need API URL for requests.
	// Let's update Config to have ApiURL and FrontendURL?
	// Or just assume standard ports if localhost.
	
	// Simplification: Just construct it using the configured BaseURL but replacing /api if present?
	// Or just use the BaseURL provided.
	
	baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
	// If the user pointed to API (e.g. http://localhost:8080), we might want to point to frontend (http://localhost:3000)
	// But let's just use what's configured for now.
	
	return fmt.Sprintf("%s/m/%s#%s", baseURL, result.ID, encrypted.Key), nil
}

func sendSlackNotification(cfg *Config, recipientID int64, messageURL string) error {
	payload := map[string]interface{}{
		"recipient_id": recipientID,
		"message_url":  messageURL,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", cfg.BaseURL+"/api/notifications/send-slack", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %s", resp.Status)
	}

	return nil
}
