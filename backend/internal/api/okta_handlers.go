package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/milkiss/vanish/backend/internal/auth"
	"github.com/milkiss/vanish/backend/internal/integrations/okta"
	"github.com/milkiss/vanish/backend/internal/models"
	"github.com/milkiss/vanish/backend/internal/repository"
)

// OktaHandler handles Okta OAuth authentication
type OktaHandler struct {
	oktaClient *okta.Client
	userRepo   *repository.UserRepository
	jwtManager *auth.JWTManager
	states     map[string]time.Time // CSRF state tracking (use Redis in production)
}

// NewOktaHandler creates a new Okta handler
func NewOktaHandler(oktaClient *okta.Client, userRepo *repository.UserRepository, jwtManager *auth.JWTManager) *OktaHandler {
	return &OktaHandler{
		oktaClient: oktaClient,
		userRepo:   userRepo,
		jwtManager: jwtManager,
		states:     make(map[string]time.Time),
	}
}

// InitiateLogin redirects to Okta for authentication
func (h *OktaHandler) InitiateLogin(c *gin.Context) {
	// Generate CSRF state token
	state, err := h.generateState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to generate state",
		})
		return
	}

	// Store state with expiration (5 minutes)
	h.states[state] = time.Now().Add(5 * time.Minute)

	// Get Okta authorization URL
	authURL := h.oktaClient.GetAuthURL(state)

	// Redirect to Okta
	c.Redirect(http.StatusFound, authURL)
}

// HandleCallback handles the OAuth callback from Okta
func (h *OktaHandler) HandleCallback(c *gin.Context) {
	// Verify state (CSRF protection)
	state := c.Query("state")
	if !h.validateState(state) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid state parameter",
		})
		return
	}

	// Clean up used state
	delete(h.states, state)

	// Check for error from Okta
	if errMsg := c.Query("error"); errMsg != "" {
		errorDesc := c.Query("error_description")
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: fmt.Sprintf("Okta error: %s - %s", errMsg, errorDesc),
		})
		return
	}

	// Get authorization code
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Missing authorization code",
		})
		return
	}

	// Exchange code for tokens
	ctx := c.Request.Context()
	token, err := h.oktaClient.ExchangeCode(ctx, code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to exchange code for token",
		})
		return
	}

	// Get user info from Okta
	userInfo, err := h.oktaClient.GetUserInfo(ctx, token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to get user info",
		})
		return
	}

	// Find or create user in our database
	user, err := h.findOrCreateUser(ctx, userInfo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to process user",
		})
		return
	}

	// Generate our own JWT token for API access
	jwtToken, err := h.jwtManager.Generate(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to generate token",
		})
		return
	}

	// Return token and user info
	c.JSON(http.StatusOK, models.AuthResponse{
		Token: jwtToken,
		User:  user.ToUserInfo(),
	})
}

// ValidateOktaToken validates an Okta access token (alternative to JWT)
func (h *OktaHandler) ValidateOktaToken(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Authorization header required",
		})
		return
	}

	// Extract token
	var token string
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	} else {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Invalid authorization header format",
		})
		return
	}

	// Validate with Okta
	ctx := c.Request.Context()
	userInfo, err := h.oktaClient.ValidateAccessToken(ctx, token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Invalid or expired token",
		})
		return
	}

	// Find user in our database
	user, err := h.userRepo.FindByEmail(ctx, userInfo.Email)
	if err != nil {
		// User exists in Okta but not in our DB - create them
		user, err = h.createUserFromOkta(ctx, userInfo)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error: "Failed to create user",
			})
			return
		}
	}

	c.JSON(http.StatusOK, user.ToUserInfo())
}

// Helper functions

func (h *OktaHandler) generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func (h *OktaHandler) validateState(state string) bool {
	expiration, exists := h.states[state]
	if !exists {
		return false
	}

	// Check if expired
	if time.Now().After(expiration) {
		delete(h.states, state)
		return false
	}

	return true
}

func (h *OktaHandler) findOrCreateUser(ctx context.Context, userInfo *okta.UserInfo) (*models.User, error) {
	// Try to find existing user
	user, err := h.userRepo.FindByEmail(ctx, userInfo.Email)
	if err == nil {
		return user, nil
	}

	// User doesn't exist, create new one
	return h.createUserFromOkta(ctx, userInfo)
}

func (h *OktaHandler) createUserFromOkta(ctx context.Context, userInfo *okta.UserInfo) (*models.User, error) {
	// Create user with Okta info
	user := &models.User{
		Email: userInfo.Email,
		Name:  userInfo.Name,
		// No password - Okta handles authentication
		Password: "", // Empty password for SSO users
	}

	err := h.userRepo.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// CleanupExpiredStates periodically removes expired CSRF states
// Should be called in a goroutine
func (h *OktaHandler) CleanupExpiredStates() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		for state, expiration := range h.states {
			if now.After(expiration) {
				delete(h.states, state)
			}
		}
	}
}
