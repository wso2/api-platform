/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

// Package handler contains the HTTP handlers for the platform API.
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/utils"

	"github.com/google/uuid"
	"github.com/wso2/go-httpkit/httputil"
)

// MockResource is an in-memory resource used for security-testing scenarios.
type MockResource struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"orgId"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Data        string    `json:"data,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// mockResourceRequest is the body accepted by Create and Update.
type mockResourceRequest struct {
	OrgID       string `json:"orgId"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Data        string `json:"data,omitempty"`
}

// MockResourceListResponse is the envelope returned by ListMockResources.
type MockResourceListResponse struct {
	Count int            `json:"count"`
	List  []MockResource `json:"list"`
}

// MockResourceHandler holds the in-memory store and exposes CRUD endpoints.
type MockResourceHandler struct {
	// store maps resource UUID → *MockResource.
	store   sync.Map
	slogger *slog.Logger
}

// NewMockResourceHandler constructs a MockResourceHandler.
func NewMockResourceHandler(slogger *slog.Logger) *MockResourceHandler {
	return &MockResourceHandler{slogger: slogger}
}

// CreateMockResource handles POST /api/v0.9/mock-resources.
func (h *MockResourceHandler) CreateMockResource(w http.ResponseWriter, r *http.Request) {
	var req mockResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	if req.Name == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "name is required"))
		return
	}
	if req.OrgID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "orgId is required"))
		return
	}

	now := time.Now().UTC()
	resource := MockResource{
		ID:          uuid.New().String(),
		OrgID:       req.OrgID,
		Name:        req.Name,
		Description: req.Description,
		Data:        req.Data,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	h.store.Store(resource.ID, resource)

	h.slogger.Info("MockResource created", "id", resource.ID, "orgId", resource.OrgID)
	httputil.WriteJSON(w, http.StatusCreated, resource)
}

func (h *MockResourceHandler) ListMockResources(w http.ResponseWriter, r *http.Request) {
	filterOrg := r.URL.Query().Get("orgId")

	var list []MockResource
	h.store.Range(func(_, val any) bool {
		res, ok := val.(MockResource)
		if !ok {
			return true
		}
		if filterOrg == "" || res.OrgID == filterOrg {
			list = append(list, res)
		}
		return true
	})

	if list == nil {
		list = []MockResource{}
	}
	httputil.WriteJSON(w, http.StatusOK, MockResourceListResponse{Count: len(list), List: list})
}

func (h *MockResourceHandler) GetMockResource(w http.ResponseWriter, r *http.Request) {
	resourceID := r.PathValue("resourceId")
	if resourceID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "resourceId path parameter is required"))
		return
	}

	val, ok := h.store.Load(resourceID)
	if !ok {
		httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Mock resource not found"))
		return
	}

	resource, ok := val.(MockResource)
	if !ok {
		h.slogger.Error("MockResource store type assertion failed", "id", resourceID)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to retrieve resource"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resource)
}

func (h *MockResourceHandler) UpdateMockResource(w http.ResponseWriter, r *http.Request) {
	resourceID := r.PathValue("resourceId")
	if resourceID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "resourceId path parameter is required"))
		return
	}

	val, ok := h.store.Load(resourceID)
	if !ok {
		httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Mock resource not found"))
		return
	}

	existing, ok := val.(MockResource)
	if !ok {
		h.slogger.Error("MockResource store type assertion failed", "id", resourceID)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to retrieve resource"))
		return
	}

	var req mockResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	if req.OrgID != "" {
		existing.OrgID = req.OrgID
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	existing.Description = req.Description
	existing.Data = req.Data
	existing.UpdatedAt = time.Now().UTC()

	h.store.Store(resourceID, existing)
	h.slogger.Info("MockResource updated", "id", resourceID, "orgId", existing.OrgID)
	httputil.WriteJSON(w, http.StatusOK, existing)
}

func (h *MockResourceHandler) DeleteMockResource(w http.ResponseWriter, r *http.Request) {
	resourceID := r.PathValue("resourceId")
	if resourceID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "resourceId path parameter is required"))
		return
	}

	_, ok := h.store.Load(resourceID)
	if !ok {
		httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Mock resource not found"))
		return
	}

	h.store.Delete(resourceID)
	h.slogger.Info("MockResource deleted", "id", resourceID)
	httputil.WriteJSON(w, http.StatusNoContent, nil)
}

// RegisterRoutes mounts all MockResource endpoints onto mux.
func (h *MockResourceHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/mock-resources", h.CreateMockResource)
	mux.HandleFunc("GET "+constants.APIBasePath+"/mock-resources", h.ListMockResources)
	mux.HandleFunc("GET "+constants.APIBasePath+"/mock-resources/{resourceId}", h.GetMockResource)
	mux.HandleFunc("PUT "+constants.APIBasePath+"/mock-resources/{resourceId}", h.UpdateMockResource)
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/mock-resources/{resourceId}", h.DeleteMockResource)
}
