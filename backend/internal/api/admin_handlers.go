package api

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/milkiss/vanish/backend/internal/models"
	"github.com/milkiss/vanish/backend/internal/repository"
)

// AdminHandler handles admin-only operations
type AdminHandler struct {
	userRepo     *repository.UserRepository
	metadataRepo *repository.MetadataRepository
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(userRepo *repository.UserRepository, metadataRepo *repository.MetadataRepository) *AdminHandler {
	return &AdminHandler{
		userRepo:     userRepo,
		metadataRepo: metadataRepo,
	}
}

// CreateUser handles POST /api/admin/users
// Admin creates a new user
func (h *AdminHandler) CreateUser(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Name     string `json:"name" binding:"required,min=2,max=100"`
		Password string `json:"password" binding:"required,min=8"`
		IsAdmin  bool   `json:"is_admin"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request: " + err.Error(),
		})
		return
	}

	// Hash password
	hashedPassword, err := models.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to hash password",
		})
		return
	}

	// Create user
	user := &models.User{
		Email:    req.Email,
		Name:     req.Name,
		Password: hashedPassword,
		IsAdmin:  req.IsAdmin,
	}

	if err := h.userRepo.Create(c.Request.Context(), user); err != nil {
		if err == models.ErrUserExists {
			c.JSON(http.StatusConflict, models.ErrorResponse{
				Error: "User with this email already exists",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to create user",
		})
		return
	}

	c.JSON(http.StatusCreated, user.ToUserInfo())
}

// UpdateUser handles PUT /api/admin/users/:id
// Admin updates a user
func (h *AdminHandler) UpdateUser(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid user ID",
		})
		return
	}

	var req struct {
		Email    *string `json:"email" binding:"omitempty,email"`
		Name     *string `json:"name" binding:"omitempty,min=2,max=100"`
		Password *string `json:"password" binding:"omitempty,min=8"`
		IsAdmin  *bool   `json:"is_admin"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request: " + err.Error(),
		})
		return
	}

	// Get existing user
	user, err := h.userRepo.FindByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: "User not found",
		})
		return
	}

	// Update fields if provided
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.Password != nil {
		hashedPassword, err := models.HashPassword(*req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error: "Failed to hash password",
			})
			return
		}
		user.Password = hashedPassword
	}
	if req.IsAdmin != nil {
		user.IsAdmin = *req.IsAdmin
	}

	// Update user
	if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to update user",
		})
		return
	}

	c.JSON(http.StatusOK, user.ToUserInfo())
}

// DeleteUser handles DELETE /api/admin/users/:id
// Admin deletes a user
func (h *AdminHandler) DeleteUser(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid user ID",
		})
		return
	}

	// Prevent deleting yourself
	currentUserID, _ := c.Get("user_id")
	if currentUserID.(int64) == userID {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Cannot delete your own account",
		})
		return
	}

	if err := h.userRepo.Delete(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to delete user",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

// ImportUsersCSV handles POST /api/admin/users/import
// Import users from CSV file
func (h *AdminHandler) ImportUsersCSV(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "No file uploaded",
		})
		return
	}

	// Open the file
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to open file",
		})
		return
	}
	defer f.Close()

	// Parse CSV
	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid CSV file",
		})
		return
	}

	if len(records) < 2 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "CSV file is empty",
		})
		return
	}

	// Validate header
	header := records[0]
	if len(header) < 3 || header[0] != "email" || header[1] != "name" || header[2] != "password" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid CSV format. Expected: email,name,password[,is_admin]",
		})
		return
	}

	var created, failed int
	var errors []string

	// Process each row
	for i, record := range records[1:] {
		if len(record) < 3 {
			errors = append(errors, fmt.Sprintf("Row %d: insufficient columns", i+2))
			failed++
			continue
		}

		email := strings.TrimSpace(record[0])
		name := strings.TrimSpace(record[1])
		password := strings.TrimSpace(record[2])
		isAdmin := false
		if len(record) > 3 && strings.ToLower(strings.TrimSpace(record[3])) == "true" {
			isAdmin = true
		}

		// Hash password
		hashedPassword, err := models.HashPassword(password)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Row %d: failed to hash password", i+2))
			failed++
			continue
		}

		// Create user
		user := &models.User{
			Email:    email,
			Name:     name,
			Password: hashedPassword,
			IsAdmin:  isAdmin,
		}

		if err := h.userRepo.Create(c.Request.Context(), user); err != nil {
			errors = append(errors, fmt.Sprintf("Row %d (%s): %v", i+2, email, err))
			failed++
			continue
		}

		created++
	}

	c.JSON(http.StatusOK, gin.H{
		"created": created,
		"failed":  failed,
		"errors":  errors,
	})
}

// GetStatistics handles GET /api/admin/statistics
// Get system statistics
func (h *AdminHandler) GetStatistics(c *gin.Context) {
	// Get user count
	users, err := h.userRepo.ListAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to get statistics",
		})
		return
	}

	adminCount := 0
	for _, user := range users {
		if user.IsAdmin {
			adminCount++
		}
	}

	// Get message history for statistics
	// Note: This is a simplified version - you might want to add dedicated queries
	history, err := h.metadataRepo.GetUserHistory(c.Request.Context(), 0, 10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to get message statistics",
		})
		return
	}

	pendingCount := 0
	readCount := 0
	expiredCount := 0

	for _, msg := range history {
		switch msg.Status {
		case models.StatusPending:
			pendingCount++
		case models.StatusRead:
			readCount++
		case models.StatusExpired:
			expiredCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"users": gin.H{
			"total":  len(users),
			"admins": adminCount,
			"regular": len(users) - adminCount,
		},
		"messages": gin.H{
			"total":   len(history),
			"pending": pendingCount,
			"read":    readCount,
			"expired": expiredCount,
		},
	})
}

// CleanupExpired handles POST /api/admin/cleanup
// Manually trigger cleanup of expired messages
func (h *AdminHandler) CleanupExpired(c *gin.Context) {
	count, err := h.metadataRepo.CleanupExpired(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to cleanup expired messages",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Cleanup completed",
		"expired_count": count,
	})
}
