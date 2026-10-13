package api

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/milkiss/vanish/backend/internal/models"
)

// NoBodyLoggingMiddleware prevents request bodies from being logged
// This is critical for security (NFR-02) - we must never log encrypted payloads
func NoBodyLoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Disable Gin's default logging for this request's body
		// The actual prevention happens at the Gin setup level
		c.Next()
	}
}

// CORSMiddleware configures CORS for the allowed origins
func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Origin", "Authorization"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	})
}

// SecurityHeadersMiddleware adds security headers to all responses
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking
		c.Header("X-Frame-Options", "DENY")

		// Enable XSS protection
		c.Header("X-XSS-Protection", "1; mode=block")

		// HSTS header (only if using HTTPS in production)
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		// Content Security Policy
		c.Header("Content-Security-Policy", "default-src 'self'")

		// Referrer policy
		c.Header("Referrer-Policy", "no-referrer")

		c.Next()
	}
}

// SetupGinWithNoLogging configures Gin to not log request bodies
func SetupGinWithNoLogging() *gin.Engine {
	// Disable debug mode in production
	gin.SetMode(gin.ReleaseMode)

	// Disable default logging output (we'll use custom logger that doesn't log bodies)
	gin.DefaultWriter = io.Discard

	// Create engine with custom logger that only logs metadata
	router := gin.New()

	// Recovery middleware
	router.Use(gin.Recovery())

	// Custom logger that excludes request bodies
	router.Use(customLogger())

	return router
}

// customLogger creates a custom logging middleware that doesn't log request bodies
func customLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path

		// Process request
		c.Next()

		// Log only metadata (no body content)
		latency := time.Since(start)
		statusCode := c.Writer.Status()
		method := c.Request.Method

		// Only log to stdout if needed (can be disabled entirely)
		if gin.Mode() == gin.DebugMode {
			gin.DefaultWriter = io.Discard // Even in debug, don't log
		}

		// Custom logging can be added here that only logs:
		// - Method
		// - Path
		// - Status code
		// - Latency
		// - Client IP (optional)
		// But NEVER the request or response body

		_ = latency
		_ = statusCode
		_ = method
		_ = path
	}
}

// MaxMessageBodyBytes caps the create-message request body. A 5 MiB file
// becomes ~7 MB after AES-GCM and base64, so 8 MiB leaves headroom.
const MaxMessageBodyBytes = 8 << 20

// BodyLimit rejects requests whose body exceeds max bytes. It checks the
// declared Content-Length up front and also caps bodies of unknown length.
// It never reads or logs the body.
func BodyLimit(max int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > max {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, models.ErrorResponse{
				Error: "Message too large",
			})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
		c.Next()
	}
}

// IsBodyTooLarge reports whether err came from exceeding a BodyLimit.
func IsBodyTooLarge(err error) bool {
	var tooLarge *http.MaxBytesError
	return errors.As(err, &tooLarge)
}
