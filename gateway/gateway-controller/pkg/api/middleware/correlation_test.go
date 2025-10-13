package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestCorrelationIDMiddleware_ExistingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger, _ := zap.NewDevelopment()

	router := gin.New()
	router.Use(CorrelationIDMiddleware(logger))

	router.GET("/test", func(c *gin.Context) {
		correlationID := GetCorrelationID(c)
		if correlationID == "" {
			t.Error("Correlation ID should not be empty")
		}
		if correlationID != "test-correlation-id-123" {
			t.Errorf("Expected correlation ID 'test-correlation-id-123', got '%s'", correlationID)
		}
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(CorrelationIDHeader, "test-correlation-id-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	responseCorrelationID := w.Header().Get(CorrelationIDHeader)
	if responseCorrelationID != "test-correlation-id-123" {
		t.Errorf("Expected response header to contain 'test-correlation-id-123', got '%s'", responseCorrelationID)
	}
}

func TestCorrelationIDMiddleware_GenerateNew(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger, _ := zap.NewDevelopment()

	router := gin.New()
	router.Use(CorrelationIDMiddleware(logger))

	router.GET("/test", func(c *gin.Context) {
		correlationID := GetCorrelationID(c)
		if correlationID == "" {
			t.Error("Correlation ID should be auto-generated when not provided")
		}
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	responseCorrelationID := w.Header().Get(CorrelationIDHeader)
	if responseCorrelationID == "" {
		t.Error("Response header should contain auto-generated correlation ID")
	}
}

func TestGetLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	baseLogger, _ := zap.NewDevelopment()

	router := gin.New()
	router.Use(CorrelationIDMiddleware(baseLogger))

	router.GET("/test", func(c *gin.Context) {
		logger := GetLogger(c, baseLogger)
		if logger == nil {
			t.Error("Logger should not be nil")
		}
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetLogger_Fallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fallbackLogger, _ := zap.NewDevelopment()

	// Create a context without the middleware
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		logger := GetLogger(c, fallbackLogger)
		if logger != fallbackLogger {
			t.Error("Should return fallback logger when no logger in context")
		}
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestCorrelationIDMiddleware_LowercaseHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger, _ := zap.NewDevelopment()

	router := gin.New()
	router.Use(CorrelationIDMiddleware(logger))

	router.GET("/test", func(c *gin.Context) {
		correlationID := GetCorrelationID(c)
		if correlationID == "" {
			t.Error("Correlation ID should not be empty")
		}
		if correlationID != "lowercase-correlation-id-456" {
			t.Errorf("Expected correlation ID 'lowercase-correlation-id-456', got '%s'", correlationID)
		}
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	// Test with lowercase header name
	req.Header.Set("x-correlation-id", "lowercase-correlation-id-456")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	responseCorrelationID := w.Header().Get(CorrelationIDHeader)
	if responseCorrelationID != "lowercase-correlation-id-456" {
		t.Errorf("Expected response header to contain 'lowercase-correlation-id-456', got '%s'", responseCorrelationID)
	}
}

func TestCorrelationIDMiddleware_MixedCaseHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger, _ := zap.NewDevelopment()

	router := gin.New()
	router.Use(CorrelationIDMiddleware(logger))

	router.GET("/test", func(c *gin.Context) {
		correlationID := GetCorrelationID(c)
		if correlationID == "" {
			t.Error("Correlation ID should not be empty")
		}
		if correlationID != "mixed-case-id-789" {
			t.Errorf("Expected correlation ID 'mixed-case-id-789', got '%s'", correlationID)
		}
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	// Test with mixed case header name
	req.Header.Set("X-CoRrElAtIoN-Id", "mixed-case-id-789")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	responseCorrelationID := w.Header().Get(CorrelationIDHeader)
	if responseCorrelationID != "mixed-case-id-789" {
		t.Errorf("Expected response header to contain 'mixed-case-id-789', got '%s'", responseCorrelationID)
	}
}
