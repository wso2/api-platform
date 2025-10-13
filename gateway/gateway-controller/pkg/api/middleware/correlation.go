package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// CorrelationIDHeader is the HTTP header name for correlation ID
	CorrelationIDHeader = "X-Correlation-ID"
	// CorrelationIDKey is the Gin context key for correlation ID
	CorrelationIDKey = "correlation_id"
	// LoggerKey is the Gin context key for the correlation-aware logger
	LoggerKey = "logger"
)

// CorrelationIDMiddleware creates a middleware that handles correlation ID tracking
// It checks for an existing X-Correlation-ID header (case-insensitive), generates a new
// UUID if not present, stores it in the Gin context, adds it to the response header,
// and creates a logger with the correlation ID for use in subsequent handlers.
//
// Header matching is case-insensitive per HTTP/1.1 spec, so 'x-correlation-id',
// 'X-Correlation-ID', and any case variation will work.
func CorrelationIDMiddleware(baseLogger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if correlation ID exists in request header (case-insensitive)
		correlationID := c.GetHeader(CorrelationIDHeader)

		// Generate new UUID if not present
		if correlationID == "" {
			correlationID = uuid.New().String()
		}

		// Store correlation ID in context
		c.Set(CorrelationIDKey, correlationID)

		// Create a logger with correlation ID field
		logger := baseLogger.With(zap.String("correlation_id", correlationID))
		c.Set(LoggerKey, logger)

		// Add correlation ID to response header
		c.Header(CorrelationIDHeader, correlationID)

		// Continue processing
		c.Next()
	}
}

// GetLogger retrieves the correlation-aware logger from the Gin context
// If not found, returns the provided fallback logger
func GetLogger(c *gin.Context, fallback *zap.Logger) *zap.Logger {
	if logger, exists := c.Get(LoggerKey); exists {
		if l, ok := logger.(*zap.Logger); ok {
			return l
		}
	}
	return fallback
}

// GetCorrelationID retrieves the correlation ID from the Gin context
// Returns empty string if not found
func GetCorrelationID(c *gin.Context) string {
	if correlationID, exists := c.Get(CorrelationIDKey); exists {
		if id, ok := correlationID.(string); ok {
			return id
		}
	}
	return ""
}
