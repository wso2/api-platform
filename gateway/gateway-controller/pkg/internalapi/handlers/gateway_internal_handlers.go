/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package handlers

import (
	"fmt"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	internalapi "github.com/wso2/api-platform/gateway/gateway-controller/pkg/internalapi/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/internalapi/services"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"go.uber.org/zap"
)

// InternalAPIServer implements the internal API ServerInterface
type InternalAPIServer struct {
	store        *storage.ConfigStore
	db           storage.Storage
	logger       *zap.Logger
	keyValidator *services.APIKeyValidator
}

// NewInternalAPIServer creates a new instance of InternalAPIServer
func NewInternalAPIServer(
	store *storage.ConfigStore,
	db storage.Storage,
	logger *zap.Logger,
) *InternalAPIServer {
	return &InternalAPIServer{
		store:        store,
		db:           db,
		logger:       logger,
		keyValidator: services.NewAPIKeyValidator(store, db, logger),
	}
}

// ValidateApiKey validates whether the provided API key is valid for accessing the specified API name and version
func (s *InternalAPIServer) ValidateApiKey(c *gin.Context, name string, version string, apikey string) {
	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)

	log.Info("Validating API key",
		zap.String("apiName", name),
		zap.String("apiVersion", version),
		zap.String("apiKey", maskApiKey(apikey)),
	)

	// Validate input parameters
	validationErrors := s.validateInputParams(name, version, apikey)
	if len(validationErrors) > 0 {
		log.Warn("Invalid input parameters", zap.Int("errorCount", len(validationErrors)))
		c.JSON(http.StatusBadRequest, internalapi.ErrorResponse{
			Status:  "error",
			Message: "Input validation failed",
			Errors:  &validationErrors,
		})
		return
	}

	// Check if the API exists
	_, err := s.store.GetByNameVersion(name, version)
	if err != nil {
		log.Warn("API configuration not found",
			zap.String("name", name),
			zap.String("version", version))
		c.JSON(http.StatusNotFound, internalapi.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("API configuration with name '%s' and version '%s' not found", name, version),
		})
		return
	}

	// Validate the API key using the new validation service
	isValid, err := s.keyValidator.ValidateAPIKey(name, version, apikey)
	if err != nil {
		log.Error("Error validating API key", zap.Error(err))
		c.JSON(http.StatusInternalServerError, internalapi.ErrorResponse{
			Status:  "error",
			Message: "Failed to validate API key",
		})
		return
	}

	log.Info("API key validation completed",
		zap.String("apiName", name),
		zap.String("apiVersion", version),
		zap.Bool("isValid", isValid),
	)

	c.JSON(http.StatusOK, internalapi.ApiKeyValidationResponse{
		IsValid: isValid,
	})
}

// validateInputParams validates the input parameters and returns structured validation errors
func (s *InternalAPIServer) validateInputParams(name, version, apikey string) []internalapi.ValidationError {
	var validationErrors []internalapi.ValidationError

	if name == "" {
		validationErrors = append(validationErrors, internalapi.ValidationError{
			Field:   stringPtr("name"),
			Message: stringPtr("API name cannot be empty"),
		})
	}

	if version == "" {
		validationErrors = append(validationErrors, internalapi.ValidationError{
			Field:   stringPtr("version"),
			Message: stringPtr("API version cannot be empty"),
		})
	}

	if apikey == "" {
		validationErrors = append(validationErrors, internalapi.ValidationError{
			Field:   stringPtr("apikey"),
			Message: stringPtr("API key cannot be empty"),
		})
	}

	return validationErrors
}

// maskApiKey masks the API key for logging purposes
func maskApiKey(apikey string) string {
	if len(apikey) <= 8 {
		return strings.Repeat("*", len(apikey))
	}
	return apikey[:4] + strings.Repeat("*", len(apikey)-8) + apikey[len(apikey)-4:]
}

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}
