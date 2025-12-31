package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/milkiss/vanish/backend/internal/auth"
	"github.com/milkiss/vanish/backend/internal/models"
	"github.com/milkiss/vanish/backend/internal/repository"
)

// AuthMiddleware creates a middleware that validates JWT tokens
func AuthMiddleware(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error: "Authorization header required",
			})
			c.Abort()
			return
		}

		// Check Bearer scheme
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error: "Invalid authorization header format",
			})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Verify token
		claims, err := jwtManager.Verify(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error: "Invalid or expired token",
			})
			c.Abort()
			return
		}

		// Set user info in context for handlers to use
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)

		c.Next()
	}
}

// AdminMiddleware ensures the user is an admin
func AdminMiddleware(userRepo *repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error: "Unauthorized",
			})
			c.Abort()
			return
		}

		// Get user from database to check admin status
		user, err := userRepo.FindByID(c.Request.Context(), userID.(int64))
		if err != nil {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error: "User not found",
			})
			c.Abort()
			return
		}

		if !user.IsAdmin {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error: "Admin access required",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
