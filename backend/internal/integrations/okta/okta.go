package okta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Config holds Okta configuration
type Config struct {
	Domain       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// Client represents an Okta OIDC client
type Client struct {
	config   *Config
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth2Config oauth2.Config
}

// UserInfo represents user information from Okta
type UserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
}

// NewClient creates a new Okta OIDC client
func NewClient(ctx context.Context, config *Config) (*Client, error) {
	issuerURL := fmt.Sprintf("https://%s", config.Domain)

	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	oauth2Config := oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: config.ClientID})

	return &Client{
		config:       config,
		provider:     provider,
		verifier:     verifier,
		oauth2Config: oauth2Config,
	}, nil
}

// GetAuthURL returns the OAuth2 authorization URL
func (c *Client) GetAuthURL(state string) string {
	return c.oauth2Config.AuthCodeURL(state)
}

// ExchangeCode exchanges an authorization code for tokens
func (c *Client) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := c.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	return token, nil
}

// VerifyIDToken verifies and extracts claims from an ID token
func (c *Client) VerifyIDToken(ctx context.Context, rawIDToken string) (*oidc.IDToken, error) {
	idToken, err := c.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}
	return idToken, nil
}

// GetUserInfo fetches user information from Okta
func (c *Client) GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}

	idToken, err := c.VerifyIDToken(ctx, rawIDToken)
	if err != nil {
		return nil, err
	}

	var userInfo UserInfo
	if err := idToken.Claims(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	return &userInfo, nil
}

// ValidateAccessToken validates an Okta access token
func (c *Client) ValidateAccessToken(ctx context.Context, accessToken string) (*UserInfo, error) {
	// Call Okta's introspection endpoint
	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("https://%s/oauth2/default/v1/introspect", c.config.Domain),
		strings.NewReader(fmt.Sprintf("token=%s&token_type_hint=access_token", accessToken)))

	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(c.config.ClientID, c.config.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Active bool   `json:"active"`
		Sub    string `json:"sub"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.Active {
		return nil, fmt.Errorf("token is not active")
	}

	// Fetch full user info
	userInfoReq, _ := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("https://%s/oauth2/default/v1/userinfo", c.config.Domain), nil)
	userInfoReq.Header.Set("Authorization", "Bearer "+accessToken)

	userInfoResp, err := http.DefaultClient.Do(userInfoReq)
	if err != nil {
		return nil, err
	}
	defer userInfoResp.Body.Close()

	var userInfo UserInfo
	if err := json.NewDecoder(userInfoResp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}
