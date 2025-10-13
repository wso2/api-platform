package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"go.uber.org/zap"
)

// ErrorHandlingMiddleware creates a Gin middleware for error recovery
func ErrorHandlingMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Get correlation-aware logger from context
				log := GetLogger(c, logger)

				log.Error("Panic recovered",
					zap.Any("error", err),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
				)

				c.JSON(http.StatusInternalServerError, api.ErrorResponse{
					Status:  "error",
					Message: "Internal server error",
				})

				c.Abort()
			}
		}()

		c.Next()
	}
}
