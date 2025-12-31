package vault

import (
	"context"
	"fmt"
	"time"

	vault "github.com/hashicorp/vault/api"
)

// Config holds Vault configuration
type Config struct {
	Address   string
	Token     string
	Namespace string // Optional for Vault Enterprise
}

// Client wraps the Vault API client
type Client struct {
	client *vault.Client
}

// NewClient creates a new Vault client
func NewClient(config *Config) (*Client, error) {
	vaultConfig := vault.DefaultConfig()
	vaultConfig.Address = config.Address

	client, err := vault.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault client: %w", err)
	}

	client.SetToken(config.Token)

	if config.Namespace != "" {
		client.SetNamespace(config.Namespace)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.Sys().HealthWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to vault: %w", err)
	}

	return &Client{client: client}, nil
}

// GetSecret retrieves a secret from Vault KV v2
func (c *Client) GetSecret(ctx context.Context, path string) (map[string]interface{}, error) {
	secret, err := c.client.KVv2("secret").Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("secret not found at path: %s", path)
	}

	return secret.Data, nil
}

// GetSecretField retrieves a specific field from a Vault secret
func (c *Client) GetSecretField(ctx context.Context, path, field string) (string, error) {
	data, err := c.GetSecret(ctx, path)
	if err != nil {
		return "", err
	}

	value, ok := data[field].(string)
	if !ok {
		return "", fmt.Errorf("field %s not found or not a string", field)
	}

	return value, nil
}

// PutSecret stores a secret in Vault KV v2
func (c *Client) PutSecret(ctx context.Context, path string, data map[string]interface{}) error {
	_, err := c.client.KVv2("secret").Put(ctx, path, data)
	if err != nil {
		return fmt.Errorf("failed to write secret: %w", err)
	}
	return nil
}

// GetDatabaseCredentials retrieves dynamic database credentials
func (c *Client) GetDatabaseCredentials(ctx context.Context, role string) (*DatabaseCredentials, error) {
	secret, err := c.client.Logical().ReadWithContext(ctx, fmt.Sprintf("database/creds/%s", role))
	if err != nil {
		return nil, fmt.Errorf("failed to get database credentials: %w", err)
	}

	if secret == nil {
		return nil, fmt.Errorf("no credentials returned for role: %s", role)
	}

	username, _ := secret.Data["username"].(string)
	password, _ := secret.Data["password"].(string)

	return &DatabaseCredentials{
		Username:  username,
		Password:  password,
		LeaseID:   secret.LeaseID,
		LeaseDuration: secret.LeaseDuration,
	}, nil
}

// RenewLease renews a Vault lease
func (c *Client) RenewLease(ctx context.Context, leaseID string) error {
	_, err := c.client.Sys().RenewWithContext(ctx, leaseID, 0)
	return err
}

// DatabaseCredentials represents dynamic database credentials from Vault
type DatabaseCredentials struct {
	Username      string
	Password      string
	LeaseID       string
	LeaseDuration int
}

// GetJWTSecret retrieves JWT signing secret from Vault
func (c *Client) GetJWTSecret(ctx context.Context) (string, error) {
	return c.GetSecretField(ctx, "vanish/jwt", "secret")
}

// GetSlackToken retrieves Slack bot token from Vault
func (c *Client) GetSlackToken(ctx context.Context) (string, error) {
	return c.GetSecretField(ctx, "vanish/slack", "bot_token")
}

// GetSMTPCredentials retrieves SMTP credentials from Vault
func (c *Client) GetSMTPCredentials(ctx context.Context) (map[string]interface{}, error) {
	return c.GetSecret(ctx, "vanish/smtp")
}

// GetOktaConfig retrieves Okta configuration from Vault
func (c *Client) GetOktaConfig(ctx context.Context) (map[string]interface{}, error) {
	return c.GetSecret(ctx, "vanish/okta")
}
