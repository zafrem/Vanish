package api

import (
	"github.com/gin-gonic/gin"
	"github.com/milkiss/vanish/backend/internal/auth"
	"github.com/milkiss/vanish/backend/internal/config"
	"github.com/milkiss/vanish/backend/internal/integrations/okta"
	"github.com/milkiss/vanish/backend/internal/repository"
	"github.com/milkiss/vanish/backend/internal/storage"
)

// SetupRouter creates and configures the Gin router with all routes
func SetupRouter(
	cfg *config.Config,
	store storage.Storage,
	userRepo *repository.UserRepository,
	metadataRepo *repository.MetadataRepository,
	jwtManager *auth.JWTManager,
	oktaClient interface{}, // *okta.Client or nil if Okta disabled
) *gin.Engine {
	// Create router with no default logging (security requirement)
	router := SetupGinWithNoLogging()

	// Apply middleware
	router.Use(SecurityHeadersMiddleware())
	router.Use(CORSMiddleware(cfg.Server.AllowedOrigins))

	// Create handlers
	authHandler := NewAuthHandler(userRepo, jwtManager)
	messageHandler := NewMessageHandler(store, metadataRepo)
	historyHandler := NewHistoryHandler(metadataRepo)
	adminHandler := NewAdminHandler(userRepo, metadataRepo)
	profileHandler := NewProfileHandler(userRepo)

	// Health check endpoint (public)
	router.GET("/health", messageHandler.Health)

	// API routes
	api := router.Group("/api")
	{
		// Public auth endpoints
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
		}

		// Okta OAuth endpoints (if enabled)
		if cfg.Okta.Enabled && oktaClient != nil {
			oktaHandler := NewOktaHandler(oktaClient.(*okta.Client), userRepo, jwtManager)

			// Start cleanup goroutine for CSRF states
			go oktaHandler.CleanupExpiredStates()

			auth.GET("/okta/login", oktaHandler.InitiateLogin)
			auth.GET("/okta/callback", oktaHandler.HandleCallback)
			auth.POST("/okta/validate", oktaHandler.ValidateOktaToken)
		}

		// Protected endpoints (require authentication)
		protected := api.Group("")
		protected.Use(AuthMiddleware(jwtManager))
		{
			// User endpoints
			protected.GET("/auth/me", authHandler.Me)
			protected.GET("/users", authHandler.ListUsers)

			// Message endpoints (all now require auth)
			messages := protected.Group("/messages")
			{
				messages.POST("", messageHandler.CreateMessage)
				messages.GET("/:id", messageHandler.GetMessage)
				messages.HEAD("/:id", messageHandler.CheckMessage)
			}

			// History endpoints
			protected.GET("/history", historyHandler.GetMyHistory)

			// User profile management
			profile := protected.Group("/profile")
			{
				profile.PUT("", profileHandler.UpdateProfile)
				profile.POST("/password", profileHandler.ChangePassword)
				profile.DELETE("", profileHandler.DeleteAccount)
			}

			// Admin endpoints (require admin role)
			admin := protected.Group("/admin")
			admin.Use(AdminMiddleware(userRepo))
			{
				// User management
				admin.POST("/users", adminHandler.CreateUser)
				admin.PUT("/users/:id", adminHandler.UpdateUser)
				admin.DELETE("/users/:id", adminHandler.DeleteUser)
				admin.POST("/users/import", adminHandler.ImportUsersCSV)

				// System management
				admin.GET("/statistics", adminHandler.GetStatistics)
				admin.POST("/cleanup", adminHandler.CleanupExpired)
			}
		}
	}

	return router
}
