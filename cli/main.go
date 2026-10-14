package main

import (
	"bufio"
	"errors"
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
	filePath := sendCmd.String("file", "", "Send this file (max 5 MiB) instead of a text message")

	getCmd := flag.NewFlagSet("get", flag.ExitOnError)
	getOut := getCmd.String("o", "", "Write a received file to this path")
	getForce := getCmd.Bool("force", false, "Overwrite the output file if it exists")

	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "config":
		if err := configCmd.Parse(os.Args[2:]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(2)
		}
		runConfig()
	case "send":
		args, err := parseInterspersed(sendCmd, os.Args[2:], true)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(2)
		}
		runSend(args, *ttl, *filePath)
	case "get":
		args, err := parseInterspersed(getCmd, os.Args[2:], false)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(2)
		}
		runGetCommand(args, *getOut, *getForce)
	default:
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("Vanish CLI Tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  vanish config                         Configure the CLI (interactive)")
	fmt.Println("  vanish send <email> [msg]             Send a secret to a user")
	fmt.Println("  vanish send <email> -file <path>      Send a file (max 5 MiB)")
	fmt.Println("  vanish get <link>                     Read a secret (text to stdout, files saved)")
	fmt.Println("  vanish get -o <path> [-force] <link>  Save a received file to <path>")
	fmt.Println()
	fmt.Println("Flags for send:")
	fmt.Println("  -ttl <seconds>                        Expiration time (default 86400)")
	fmt.Println("  -file <path>                          File to send instead of a message")
}

// readLine reads one line. A final line without a trailing newline (EOF) is
// returned as-is; any other read error is reported.
func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return line, nil
}

func runConfig() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter Vanish Server URL [http://localhost:8080]: ")
	url, err := readLine(reader)
	if err != nil {
		fmt.Printf("Error reading input: %v\n", err)
		os.Exit(1)
	}
	url = strings.TrimSpace(url)
	if url == "" {
		url = "http://localhost:8080"
	}

	fmt.Print("Enter API Token (get from local storage/login): ")
	token, err := readLine(reader)
	if err != nil {
		fmt.Printf("Error reading input: %v\n", err)
		os.Exit(1)
	}
	token = strings.TrimSpace(token)

	cfg := &config.Config{
		BaseURL: url,
		Token:   token,
	}

	if err = config.SaveConfig(cfg); err != nil {
		fmt.Printf("Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Configuration saved successfully!")
}

func runSend(args []string, ttl int64, filePath string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\nRun 'vanish config' first.\n", err)
		os.Exit(1)
	}

	if len(args) < 1 {
		fmt.Println("Usage: vanish send <email> [message]  |  vanish send <email> -file <path>")
		os.Exit(1)
	}

	recipientEmail := args[0]
	var payload []byte

	if filePath != "" {
		if len(args) > 1 {
			fmt.Println("Error: -file cannot be combined with an inline message")
			os.Exit(1)
		}
		payload, err = loadFileEnvelope(filePath)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	} else {
		var secret string
		if len(args) > 1 {
			secret = strings.Join(args[1:], " ")
		} else {
			// Read from stdin
			var info os.FileInfo
			info, err = os.Stdin.Stat()
			if err != nil {
				fmt.Printf("Error checking stdin: %v\n", err)
				os.Exit(1)
			}

			if (info.Mode() & os.ModeCharDevice) == 0 {
				// Data is being piped
				var data []byte
				data, err = io.ReadAll(os.Stdin)
				if err != nil {
					fmt.Printf("Error reading stdin: %v\n", err)
					os.Exit(1)
				}
				secret = string(data)
			} else {
				// Prompt user
				fmt.Print("Enter secret: ")
				secret, err = readLine(bufio.NewReader(os.Stdin))
				if err != nil {
					fmt.Printf("Error reading input: %v\n", err)
					os.Exit(1)
				}
			}
		}
		secret = strings.TrimSpace(secret)
		if secret == "" {
			fmt.Println("Error: Secret message cannot be empty")
			os.Exit(1)
		}
		payload = []byte(secret)
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
	encrypted, err := crypto.EncryptBytes(payload)
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

func runGetCommand(args []string, out string, force bool) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\nRun 'vanish config' first.\n", err)
		os.Exit(1)
	}
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: vanish get [-o path] [-force] <link>")
		os.Exit(1)
	}

	saved, err := runGet(client.NewClient(cfg), getOptions{URL: args[0], Out: out, Force: force}, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if saved != "" {
		fmt.Fprintf(os.Stderr, "✓ Saved %s (the message has been destroyed)\n", saved)
	}
}
