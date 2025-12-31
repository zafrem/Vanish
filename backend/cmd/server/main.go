package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/milkiss/vanish/backend/internal/api"
	"github.com/milkiss/vanish/backend/internal/auth"
	"github.com/milkiss/vanish/backend/internal/config"
	"github.com/milkiss/vanish/backend/internal/database"
	"github.com/milkiss/vanish/backend/internal/integrations/okta"
	"github.com/milkiss/vanish/backend/internal/repository"
	"github.com/milkiss/vanish/backend/internal/storage"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize PostgreSQL database
	db, err := database.NewPostgresDB(database.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		SSLMode:  cfg.Database.SSLMode,
	})
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	log.Println("Successfully connected to PostgreSQL")

	// Initialize database schema
	if err := database.InitSchema(db); err != nil {
		log.Fatalf("Failed to initialize database schema: %v", err)
	}

	log.Println("Database schema initialized")

	// Initialize repositories (needed for admin creation)
	userRepo := repository.NewUserRepository(db)

	// Create default admin account on first run
	adminCreated, err := database.CreateDefaultAdmin(db, userRepo)
	if err != nil {
		log.Printf("Warning: Failed to create default admin: %v", err)
	} else if adminCreated {
		log.Println("Default admin account created successfully")
	}

	// Initialize Redis storage
	store, err := storage.NewRedisStorage(
		cfg.Redis.Address,
		cfg.Redis.Password,
		cfg.Redis.DB,
	)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer store.Close()

	log.Println("Successfully connected to Redis")

	// Initialize metadata repository
	metadataRepo := repository.NewMetadataRepository(db)

	// Initialize JWT manager
	jwtManager := auth.NewJWTManager(
		cfg.JWT.SecretKey,
		time.Duration(cfg.JWT.TokenDuration)*time.Hour,
	)

	// Initialize Okta client (if enabled)
	var oktaClient interface{}
	if cfg.Okta.Enabled {
		client, err := okta.NewClient(context.Background(), &okta.Config{
			Domain:       cfg.Okta.Domain,
			ClientID:     cfg.Okta.ClientID,
			ClientSecret: cfg.Okta.ClientSecret,
			RedirectURL:  cfg.Okta.RedirectURL,
		})
		if err != nil {
			log.Printf("Warning: Failed to initialize Okta client: %v", err)
		} else {
			oktaClient = client
			log.Println("Okta SSO enabled")
		}
	}

	// Setup router
	router := api.SetupRouter(cfg, store, userRepo, metadataRepo, jwtManager, oktaClient)

	// Create HTTP server
	addr := cfg.Address()
	server := &http.Server{
		Addr:           addr,
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
