package database

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"github.com/milkiss/vanish/backend/internal/models"
	"github.com/milkiss/vanish/backend/internal/repository"
)

const (
	defaultAdminEmail = "admin@vanish.local"
	defaultAdminName  = "Admin"
	passwordLength    = 16 // Characters in generated password
)

// generateRandomPassword creates a cryptographically secure random password
func generateRandomPassword(length int) (string, error) {
	// Generate random bytes
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random password: %w", err)
	}

	// Encode to base64 and trim to desired length
	password := base64.URLEncoding.EncodeToString(bytes)
	if len(password) > length {
		password = password[:length]
	}

	return password, nil
}

// CreateDefaultAdmin creates a default admin account on first run
// Returns true if admin was created, false if already exists
func CreateDefaultAdmin(db *sql.DB, userRepo *repository.UserRepository) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if admin already exists
	existingAdmin, err := userRepo.FindByEmail(ctx, defaultAdminEmail)
	if err != nil && err != models.ErrInvalidCredentials {
		return false, fmt.Errorf("failed to check for existing admin: %w", err)
	}

	// Admin already exists
	if existingAdmin != nil {
		return false, nil
	}

	// Generate random password
	password, err := generateRandomPassword(passwordLength)
	if err != nil {
		return false, err
	}

	// Hash the password
	hashedPassword, err := models.HashPassword(password)
	if err != nil {
		return false, fmt.Errorf("failed to hash admin password: %w", err)
	}

	// Create admin user
	admin := &models.User{
		Email:    defaultAdminEmail,
		Name:     defaultAdminName,
		Password: hashedPassword,
		IsAdmin:  true, // Mark as admin
	}

	if err := userRepo.Create(ctx, admin); err != nil {
		return false, fmt.Errorf("failed to create admin user: %w", err)
	}

	// Print credentials to console (only time password is shown)
	log.Println("═══════════════════════════════════════════════════════════")
	log.Println("🔐 DEFAULT ADMIN ACCOUNT CREATED")
	log.Println("═══════════════════════════════════════════════════════════")
	log.Printf("Email:    %s", defaultAdminEmail)
	log.Printf("Password: %s", password)
	log.Println("═══════════════════════════════════════════════════════════")
	log.Println("⚠️  IMPORTANT: Save these credentials immediately!")
	log.Println("⚠️  The password will NOT be displayed again.")
	log.Println("⚠️  Change the password after first login.")
	log.Println("═══════════════════════════════════════════════════════════")

	return true, nil
}
