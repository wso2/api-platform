/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/httpkit/httputil"
)

// maxEndpointRequestBodyBytes caps Endpoint create/update request bodies. An
// Endpoint is a handful of URLs, labels and a tls block.
const maxEndpointRequestBodyBytes = 256 << 10

// SetEndpointService wires the gateway-wide Endpoint service. Until it is set
// the /endpoints operations answer 503.
func (s *APIServer) SetEndpointService(service *utils.EndpointService) {
	s.endpointService = service
}

// endpointServiceOr503 returns the Endpoint service, or writes a 503 and
// returns nil when this controller was started without one.
func (s *APIServer) endpointServiceOr503(w http.ResponseWriter) *utils.EndpointService {
	if s.endpointService == nil {
		httputil.WriteJSON(w, http.StatusServiceUnavailable, api.ErrorResponse{
			Status:  "error",
			Message: "Endpoint management is not available on this gateway",
		})
		return nil
	}
	return s.endpointService
}

// readEndpointBody reads a size-capped request body, writing the error
// response itself when the body cannot be used.
func (s *APIServer) readEndpointBody(w http.ResponseWriter, r *http.Request, log *slog.Logger) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxEndpointRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httputil.WriteJSON(w, http.StatusRequestEntityTooLarge, api.ErrorResponse{
				Status:  "error",
				Message: "Request body is too large",
			})
			return nil, false
		}
		log.Error("Failed to read request body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return nil, false
	}
	return body, true
}

// CreateEndpoint implements ServerInterface.CreateEndpoint
// (POST /endpoints)
func (s *APIServer) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
	log := middleware.GetLogger(r, s.logger)
	service := s.endpointServiceOr503(w)
	if service == nil {
		return
	}
	body, ok := s.readEndpointBody(w, r, log)
	if !ok {
		return
	}

	stored, err := service.Create(utils.EndpointParams{
		Spec:          body,
		ContentType:   r.Header.Get("Content-Type"),
		CorrelationID: middleware.GetCorrelationID(r),
		Logger:        log,
	})
	if err != nil {
		switch {
		case errors.Is(err, utils.ErrEndpointValidation):
			log.Warn("Endpoint configuration invalid", slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		case storage.IsConflictError(err):
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		default:
			log.Error("Failed to create endpoint", slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to create endpoint",
			})
		}
		return
	}

	log.Info("Endpoint created successfully",
		slog.String("uuid", stored.UUID), slog.String("endpoint", stored.GetHandle()))
	httputil.WriteJSON(w, http.StatusCreated, buildEndpointResourceResponse(stored))
}

// ListEndpoints implements ServerInterface.ListEndpoints
// (GET /endpoints)
func (s *APIServer) ListEndpoints(w http.ResponseWriter, r *http.Request) {
	log := middleware.GetLogger(r, s.logger)
	service := s.endpointServiceOr503(w)
	if service == nil {
		return
	}

	endpoints, err := service.List()
	if err != nil {
		log.Error("Failed to list endpoints", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to list endpoints",
		})
		return
	}

	items := make([]any, 0, len(endpoints))
	for _, endpoint := range endpoints {
		items = append(items, buildEndpointResourceResponse(endpoint))
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":    "success",
		"count":     len(items),
		"endpoints": items,
	})
}

// GetEndpointById implements ServerInterface.GetEndpointById
// (GET /endpoints/{id})
func (s *APIServer) GetEndpointById(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	service := s.endpointServiceOr503(w)
	if service == nil {
		return
	}

	endpoint, err := service.Get(id)
	if err != nil {
		if errors.Is(err, utils.ErrEndpointNotFound) {
			writeEndpointNotFound(w, id)
			return
		}
		log.Error("Failed to get endpoint", slog.String("endpoint", id), slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to get endpoint",
		})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, buildEndpointResourceResponse(endpoint))
}

// UpdateEndpoint implements ServerInterface.UpdateEndpoint
// (PUT /endpoints/{id})
func (s *APIServer) UpdateEndpoint(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	service := s.endpointServiceOr503(w)
	if service == nil {
		return
	}
	body, ok := s.readEndpointBody(w, r, log)
	if !ok {
		return
	}

	updated, err := service.Update(id, utils.EndpointParams{
		Spec:          body,
		ContentType:   r.Header.Get("Content-Type"),
		CorrelationID: middleware.GetCorrelationID(r),
		Logger:        log,
	})
	if err != nil {
		switch {
		case errors.Is(err, utils.ErrEndpointNotFound):
			writeEndpointNotFound(w, id)
		case errors.Is(err, utils.ErrEndpointValidation):
			log.Warn("Endpoint configuration invalid", slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		default:
			log.Error("Failed to update endpoint", slog.String("endpoint", id), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to update endpoint",
			})
		}
		return
	}

	log.Info("Endpoint updated successfully",
		slog.String("uuid", updated.UUID), slog.String("endpoint", updated.GetHandle()))
	httputil.WriteJSON(w, http.StatusOK, buildEndpointResourceResponse(updated))
}

// DeleteEndpoint implements ServerInterface.DeleteEndpoint
// (DELETE /endpoints/{id})
func (s *APIServer) DeleteEndpoint(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	service := s.endpointServiceOr503(w)
	if service == nil {
		return
	}

	deleted, err := service.Delete(id, middleware.GetCorrelationID(r), log)
	if err != nil {
		switch {
		case errors.Is(err, utils.ErrEndpointNotFound):
			writeEndpointNotFound(w, id)
		case errors.Is(err, utils.ErrEndpointInUse):
			// The message names the Endpoint and the APIs that hold it, which
			// is what the admin needs in order to release it.
			log.Warn("Endpoint delete refused: still a fixed target", slog.String("endpoint", id), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{
				Status:  "error",
				Message: endpointInUseMessage(err),
			})
		default:
			log.Error("Failed to delete endpoint", slog.String("endpoint", id), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to delete endpoint",
			})
		}
		return
	}

	log.Info("Endpoint deleted successfully",
		slog.String("uuid", deleted.UUID), slog.String("endpoint", deleted.GetHandle()))
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "Endpoint deleted successfully",
		"id":      deleted.GetHandle(),
	})
}

func writeEndpointNotFound(w http.ResponseWriter, id string) {
	httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
		Status:  "error",
		Message: fmt.Sprintf("Endpoint with id '%s' not found", id),
	})
}

// endpointInUseMessage strips the sentinel prefix from an ErrEndpointInUse
// error, leaving the sentence that names the Endpoint and its APIs.
func endpointInUseMessage(err error) string {
	prefix := utils.ErrEndpointInUse.Error() + ": "
	message := err.Error()
	if len(message) > len(prefix) && message[:len(prefix)] == prefix {
		return message[len(prefix):]
	}
	return message
}

// buildEndpointResourceResponse builds the response body for an Endpoint: its
// configuration plus the server-managed status. An Endpoint is not deployed or
// undeployed, so only the id and the created/updated timestamps are set.
func buildEndpointResourceResponse(endpoint *models.StoredEndpoint) any {
	if endpoint == nil {
		return nil
	}
	id := endpoint.GetHandle()
	createdAt := endpoint.CreatedAt.UTC()
	updatedAt := endpoint.UpdatedAt.UTC()
	cfg := endpoint.Configuration
	cfg.Status = &api.ResourceStatus{
		Id:        &id,
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	}
	return cfg
}
