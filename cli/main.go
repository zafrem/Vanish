package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/zafrem/vanish/shared/client"
	"github.com/zafrem/vanish/shared/config"
	"github.com/zafrem/vanish/shared/crypto"
)

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

	cfg := &config.Config{
		BaseURL: url,
		Token:   token,
	}

	if err := config.SaveConfig(cfg); err != nil {
		fmt.Printf("Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Configuration saved successfully!")
}

func runSend(args []string, ttl int64) {
	cfg, err := config.LoadConfig()
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

	// Create API client
	apiClient := client.NewClient(cfg)

	// 1. Find User ID
	recipientID, err := apiClient.FindUserByEmail(recipientEmail)
	if err != nil {
		fmt.Printf("Error finding user: %v\n", err)
		os.Exit(1)
	}

	// 2. Encrypt Message
	encrypted, err := crypto.EncryptMessage(secret)
	if err != nil {
		fmt.Printf("Error encrypting message: %v\n", err)
		os.Exit(1)
	}

	// 3. Send to API
	url, _, err := apiClient.SendMessage(recipientID, encrypted, ttl)
	if err != nil {
		fmt.Printf("Error sending message: %v\n", err)
		os.Exit(1)
	}

	// 4. Notify
	fmt.Println("✓ Secret created successfully!")
	fmt.Printf("🔗 %s\n", url)

	fmt.Println("\nAttempting to send Slack notification...")
	if err := apiClient.SendSlackNotification(recipientID, url); err != nil {
		fmt.Printf("Could not auto-send Slack notification: %v\n", err)
	} else {
		fmt.Println("✓ Notification sent via Slack")
	}
}
