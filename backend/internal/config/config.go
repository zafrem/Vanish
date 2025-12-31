package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration
type Config struct {
	Server   ServerConfig
	Redis    RedisConfig
	Database DatabaseConfig
	JWT      JWTConfig
	Message  MessageConfig
	Okta     OktaConfig
	Vault    VaultConfig
	Slack    SlackConfig
	Email    EmailConfig
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port           string
	Host           string
	AllowedOrigins []string
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Address  string
	Password string
	DB       int
}

// DatabaseConfig holds PostgreSQL configuration
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	SecretKey     string
	TokenDuration int64 // in hours
}

// MessageConfig holds message-related configuration
type MessageConfig struct {
	DefaultTTL int64
	MaxTTL     int64
	MinTTL     int64
}

// OktaConfig holds Okta OIDC configuration
type OktaConfig struct {
	Enabled      bool
	Domain       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// VaultConfig holds HashiCorp Vault configuration
type VaultConfig struct {
	Enabled   bool
	Address   string
	Token     string
	Namespace string
}

// SlackConfig holds Slack integration configuration
type SlackConfig struct {
	Enabled       bool
	BotToken      string
	WebhookURL    string
	SigningSecret string
}

// EmailConfig holds SMTP email configuration
type EmailConfig struct {
	Enabled      bool
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	FromAddress  string
	FromName     string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	config := &Config{
		Server: ServerConfig{
			Port:           getEnv("SERVER_PORT", "8080"),
			Host:           getEnv("SERVER_HOST", "0.0.0.0"),
			AllowedOrigins: getEnvAsSlice("ALLOWED_ORIGINS", []string{"http://localhost:5173", "http://localhost:3000"}),
		},
		Redis: RedisConfig{
			Address:  getEnv("REDIS_ADDRESS", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvAsInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "vanish"),
			Password: getEnv("DB_PASSWORD", "vanish"),
			DBName:   getEnv("DB_NAME", "vanish"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		JWT: JWTConfig{
			SecretKey:     getEnv("JWT_SECRET", "change-me-in-production"),
			TokenDuration: getEnvAsInt64("JWT_DURATION", 24), // 24 hours
		},
		Message: MessageConfig{
			DefaultTTL: getEnvAsInt64("DEFAULT_TTL", 86400),  // 24 hours
			MaxTTL:     getEnvAsInt64("MAX_TTL", 604800),     // 7 days
			MinTTL:     getEnvAsInt64("MIN_TTL", 3600),       // 1 hour
		},
		Okta: OktaConfig{
			Enabled:      getEnvAsBool("OKTA_ENABLED", false),
			Domain:       getEnv("OKTA_DOMAIN", ""),
			ClientID:     getEnv("OKTA_CLIENT_ID", ""),
			ClientSecret: getEnv("OKTA_CLIENT_SECRET", ""),
			RedirectURL:  getEnv("OKTA_REDIRECT_URL", ""),
		},
		Vault: VaultConfig{
			Enabled:   getEnvAsBool("VAULT_ENABLED", false),
			Address:   getEnv("VAULT_ADDR", "http://localhost:8200"),
			Token:     getEnv("VAULT_TOKEN", ""),
			Namespace: getEnv("VAULT_NAMESPACE", ""),
		},
		Slack: SlackConfig{
			Enabled:       getEnvAsBool("SLACK_ENABLED", false),
			BotToken:      getEnv("SLACK_BOT_TOKEN", ""),
			WebhookURL:    getEnv("SLACK_WEBHOOK_URL", ""),
			SigningSecret: getEnv("SLACK_SIGNING_SECRET", ""),
		},
		Email: EmailConfig{
			Enabled:      getEnvAsBool("EMAIL_ENABLED", false),
			SMTPHost:     getEnv("SMTP_HOST", ""),
			SMTPPort:     getEnvAsInt("SMTP_PORT", 587),
			SMTPUser:     getEnv("SMTP_USER", ""),
			SMTPPassword: getEnv("SMTP_PASSWORD", ""),
			FromAddress:  getEnv("EMAIL_FROM_ADDRESS", "noreply@vanish.local"),
			FromName:     getEnv("EMAIL_FROM_NAME", "Vanish"),
		},
	}

	return config, nil
}

// getEnvAsBool gets an environment variable as boolean
func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	return valueStr == "true" || valueStr == "1" || valueStr == "yes"
}

// getEnv gets an environment variable with a fallback default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt gets an environment variable as int with a fallback
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}

	return value
}

// getEnvAsInt64 gets an environment variable as int64 with a fallback
func getEnvAsInt64(key string, defaultValue int64) int64 {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.ParseInt(valueStr, 10, 64)
	if err != nil {
		return defaultValue
	}

	return value
}

// getEnvAsSlice gets an environment variable as a slice (comma-separated)
func getEnvAsSlice(key string, defaultValue []string) []string {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	return strings.Split(valueStr, ",")
}

// Address returns the full server address
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%s", c.Server.Host, c.Server.Port)
}
