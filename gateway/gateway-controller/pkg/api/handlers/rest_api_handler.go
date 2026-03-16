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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/restapi"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// RestAPIHandler handles HTTP requests for REST API CRUD operations.
type RestAPIHandler struct {
	service *restapi.RestAPIService
	logger  *slog.Logger
}

// NewRestAPIHandler creates a new RestAPIHandler.
func NewRestAPIHandler(service *restapi.RestAPIService, logger *slog.Logger) *RestAPIHandler {
	return &RestAPIHandler{
		service: service,
		logger:  logger,
	}
}

// CreateRestAPI implements ServerInterface.CreateRestAPI
// (POST /rest-apis)
func (h *RestAPIHandler) CreateRestAPI(c *gin.Context) {
	startTime := time.Now()
	operation := "create"

	log := middleware.GetLogger(c, h.logger)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		metrics.APIOperationsTotal.WithLabelValues(operation, "error", "rest_api").Inc()
		metrics.ValidationErrorsTotal.WithLabelValues(operation, "read_body_failed").Inc()
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	correlationID := middleware.GetCorrelationID(c)

	result, err := h.service.Create(restapi.CreateParams{
		Body:          body,
		ContentType:   c.GetHeader("Content-Type"),
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to deploy API configuration", slog.Any("error", err))
		metrics.APIOperationsTotal.WithLabelValues(operation, "error", "rest_api").Inc()
		h.mapCreateError(c, err)
		return
	}

	metrics.APIOperationsTotal.WithLabelValues(operation, "success", "rest_api").Inc()
	metrics.APIOperationDurationSeconds.WithLabelValues(operation, "rest_api").Observe(time.Since(startTime).Seconds())
	metrics.APIsTotal.WithLabelValues("rest_api", "active").Inc()

	c.JSON(http.StatusCreated, api.RestAPICreateResponse{
		Status:    stringPtr("success"),
		Message:   stringPtr("RestAPI created successfully"),
		Id:        stringPtr(result.StoredConfig.Handle),
		CreatedAt: timePtr(result.StoredConfig.CreatedAt),
	})
}

// ListRestAPIs implements ServerInterface.ListRestAPIs
// (GET /rest-apis)
func (h *RestAPIHandler) ListRestAPIs(c *gin.Context, params api.ListRestAPIsParams) {
	result := h.service.List(params)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"count":  len(result.Items),
		"apis":   result.Items,
	})
}

// GetRestAPIById implements ServerInterface.GetRestAPIById
// (GET /rest-apis/{id})
func (h *RestAPIHandler) GetRestAPIById(c *gin.Context, id string) {
	log := middleware.GetLogger(c, h.logger)

	result, err := h.service.GetByHandle(id)
	if err != nil {
		h.mapGetError(c, log, id, err)
		return
	}

	cfg := result.Config
	apiDetail := gin.H{
		"id":            cfg.Handle,
		"configuration": cfg.Configuration,
		"metadata": gin.H{
			"status":    string(cfg.Status),
			"createdAt": cfg.CreatedAt.Format(time.RFC3339),
			"updatedAt": cfg.UpdatedAt.Format(time.RFC3339),
		},
	}
	if cfg.DeployedAt != nil {
		apiDetail["metadata"].(gin.H)["deployedAt"] = cfg.DeployedAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"api":    apiDetail,
	})
}

// UpdateRestAPI implements ServerInterface.UpdateRestAPI
// (PUT /rest-apis/{id})
func (h *RestAPIHandler) UpdateRestAPI(c *gin.Context, id string) {
	startTime := time.Now()
	operation := "update"

	log := middleware.GetLogger(c, h.logger)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		metrics.APIOperationsTotal.WithLabelValues(operation, "error", "rest_api").Inc()
		metrics.ValidationErrorsTotal.WithLabelValues(operation, "read_body_failed").Inc()
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	correlationID := middleware.GetCorrelationID(c)

	result, err := h.service.Update(restapi.UpdateParams{
		Handle:        id,
		Body:          body,
		ContentType:   c.GetHeader("Content-Type"),
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to update API configuration", slog.Any("error", err))
		metrics.APIOperationsTotal.WithLabelValues(operation, "error", "rest_api").Inc()
		h.mapUpdateError(c, id, err)
		return
	}

	metrics.APIOperationsTotal.WithLabelValues(operation, "success", "rest_api").Inc()
	metrics.APIOperationDurationSeconds.WithLabelValues(operation, "rest_api").Observe(time.Since(startTime).Seconds())

	c.JSON(http.StatusOK, api.RestAPIUpdateResponse{
		Status:    stringPtr("success"),
		Message:   stringPtr("RestAPI updated successfully"),
		Id:        stringPtr(result.Config.Handle),
		UpdatedAt: timePtr(result.Config.UpdatedAt),
	})
}

// DeleteRestAPI implements ServerInterface.DeleteRestAPI
// (DELETE /rest-apis/{id})
func (h *RestAPIHandler) DeleteRestAPI(c *gin.Context, id string) {
	startTime := time.Now()
	operation := "delete"

	log := middleware.GetLogger(c, h.logger)

	correlationID := middleware.GetCorrelationID(c)

	_, err := h.service.Delete(restapi.DeleteParams{
		Handle:        id,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		metrics.APIOperationsTotal.WithLabelValues(operation, "error", "rest_api").Inc()
		h.mapDeleteError(c, log, id, err)
		return
	}

	metrics.APIOperationsTotal.WithLabelValues(operation, "success", "rest_api").Inc()
	metrics.APIOperationDurationSeconds.WithLabelValues(operation, "rest_api").Observe(time.Since(startTime).Seconds())
	metrics.APIsTotal.WithLabelValues("rest_api", "active").Dec()

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "RestAPI deleted successfully",
		"id":      id,
	})
}

// mapCreateError maps service errors to HTTP responses for Create.
func (h *RestAPIHandler) mapCreateError(c *gin.Context, err error) {
	if storage.IsConflictError(err) {
		c.JSON(http.StatusConflict, api.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	var validationErr *utils.ValidationErrorListError
	if errors.As(err, &validationErr) {
		apiErrors := make([]api.ValidationError, len(validationErr.Errors))
		for i, e := range validationErr.Errors {
			apiErrors[i] = api.ValidationError{
				Field:   stringPtr(e.Field),
				Message: stringPtr(e.Message),
			}
		}
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Configuration validation failed",
			Errors:  &apiErrors,
		})
		return
	}

	c.JSON(http.StatusBadRequest, api.ErrorResponse{
		Status:  "error",
		Message: err.Error(),
	})
}

// mapGetError maps service errors to HTTP responses for GetByHandle.
func (h *RestAPIHandler) mapGetError(c *gin.Context, log *slog.Logger, handle string, err error) {
	if errors.Is(err, restapi.ErrDatabaseUnavailable) {
		c.JSON(http.StatusServiceUnavailable, api.ErrorResponse{
			Status:  "error",
			Message: "Database storage not available",
		})
		return
	}

	log.Warn("API configuration not found", slog.String("handle", handle))
	c.JSON(http.StatusNotFound, api.ErrorResponse{
		Status:  "error",
		Message: fmt.Sprintf("RestAPI with handle '%s' not found", handle),
	})
}

// mapUpdateError maps service errors to HTTP responses for Update.
func (h *RestAPIHandler) mapUpdateError(c *gin.Context, handle string, err error) {
	var parseErr *restapi.ParseError
	if errors.As(err, &parseErr) {
		metrics.ValidationErrorsTotal.WithLabelValues("update", "parse_failed").Inc()
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Failed to parse configuration: %v", parseErr.Cause),
		})
		return
	}

	var handleErr *restapi.HandleMismatchError
	if errors.As(err, &handleErr) {
		metrics.ValidationErrorsTotal.WithLabelValues("update", "handle_mismatch").Inc()
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: handleErr.Error(),
		})
		return
	}

	var validationErr *restapi.ValidationError
	if errors.As(err, &validationErr) {
		metrics.ValidationErrorsTotal.WithLabelValues("update", "validation_failed").Add(float64(len(validationErr.Errors)))
		apiErrors := make([]api.ValidationError, len(validationErr.Errors))
		for i, e := range validationErr.Errors {
			apiErrors[i] = api.ValidationError{
				Field:   stringPtr(e.Field),
				Message: stringPtr(e.Message),
			}
		}
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Configuration validation failed",
			Errors:  &apiErrors,
		})
		return
	}

	if errors.Is(err, restapi.ErrDatabaseUnavailable) {
		c.JSON(http.StatusServiceUnavailable, api.ErrorResponse{
			Status:  "error",
			Message: "Database storage not available",
		})
		return
	}

	if errors.Is(err, restapi.ErrNotFound) {
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("RestAPI with handle '%s' not found", handle),
		})
		return
	}

	if storage.IsConflictError(err) {
		c.JSON(http.StatusConflict, api.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusInternalServerError, api.ErrorResponse{
		Status:  "error",
		Message: "Failed to update configuration",
	})
}

// mapDeleteError maps service errors to HTTP responses for Delete.
func (h *RestAPIHandler) mapDeleteError(c *gin.Context, log *slog.Logger, handle string, err error) {
	if errors.Is(err, restapi.ErrDatabaseUnavailable) {
		log.Error("Database storage not available")
		c.JSON(http.StatusServiceUnavailable, api.ErrorResponse{
			Status:  "error",
			Message: "Database storage not available",
		})
		return
	}

	if errors.Is(err, restapi.ErrNotFound) {
		log.Warn("API configuration not found", slog.String("handle", handle))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("RestAPI with handle '%s' not found", handle),
		})
		return
	}

	// Topic lifecycle or internal errors
	c.JSON(http.StatusInternalServerError, api.ErrorResponse{
		Status:  "error",
		Message: err.Error(),
	})
}
