/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/router"
	"github.com/wso2/api-platform/platform-api/internal/service"

	"github.com/wso2/api-platform/httpkit/httputil"
)
type APIHandler struct {
	apiService   *service.APIService
	identity     *service.IdentityService
	apiDocumentService *service.APIDocumentService
	slogger      *slog.Logger
	cfg          *config.Server
}

func NewAPIHandler(apiService *service.APIService, identity *service.IdentityService, apiDocumentService *service.APIDocumentService, slogger *slog.Logger,cfg *config.Server) *APIHandler {
	return &APIHandler{
		apiService:   apiService,
		identity:     identity,
		apiDocumentService: apiDocumentService,
		slogger:      slogger,
		cfg:          cfg,
	}
}

// getOpenAPISpecMaxBytes returns the configured max bytes, falling back to 5 MiB if unset.
func (h *APIHandler) getOpenAPISpecMaxBytes() int64 {
	if h.cfg.OpenAPISpecMaxFetchBytes <= 0 {
		return constants.DefaultOpenAPISpecMaxBytes
	}
	return h.cfg.OpenAPISpecMaxFetchBytes
}

// CreateAPI handles POST /api/v0.9/rest-apis and creates a new API
func (h *APIHandler) CreateAPI(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	var req api.CreateRESTAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.NewValidation(err)
	}

	// Validate required fields
	if req.DisplayName == "" {
		return apperror.ValidationFailed.New("API name is required")
	}
	if req.Context == "" {
		return apperror.ValidationFailed.New("API context is required")
	}
	if req.Version == "" {
		return apperror.ValidationFailed.New("API version is required")
	}
	if strings.TrimSpace(req.ProjectId) == "" {
		return apperror.ValidationFailed.New("Project ID is required")
	}
	if isEmptyUpstreamDefinition(req.Upstream.Main) && (req.Upstream.Sandbox == nil || isEmptyUpstreamDefinition(*req.Upstream.Sandbox)) {
		return apperror.ValidationFailed.New("At least one upstream endpoint (main or sandbox) is required").
			WithLogMessage(fmt.Sprintf("no upstream endpoints provided for org %s", orgId))
	}
	if err := validateUpstreamDefinitions(req.Upstream); err != nil {
		return err
	}

	createdBy, err := resolveActorErr(r, h.identity, "create API")
	if err != nil {
		return err
	}
	apiResponse, _, err := h.apiService.CreateAPI(&req, orgId, createdBy)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to create API in org %s", orgId))
	}

	setLocation(w, "rest-apis", strOrEmpty(apiResponse.Id))
	httputil.WriteJSON(w, http.StatusCreated, apiResponse)
	return nil
}

// GetAPI handles GET /api/v0.9/rest-apis/:apiId and retrieves an API by its handle
func (h *APIHandler) GetAPI(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("restApiId")
	if apiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	apiResponse, err := h.apiService.GetAPIByHandle(apiId, orgId)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to get API %s in org %s", apiId, orgId))
	}

	httputil.WriteJSON(w, http.StatusOK, apiResponse)
	return nil
}

// ListAPIs handles GET /api/v0.9/rest-apis and lists APIs for an organization filtered by project
func (h *APIHandler) ListAPIs(w http.ResponseWriter, r *http.Request) error {
	// Get organization from JWT token
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	projectId := strings.TrimSpace(r.URL.Query().Get("projectId"))
	if projectId == "" {
		return apperror.ValidationFailed.New("projectId query parameter is required")
	}

	opts := parseListOptions(r)

	apis, total, err := h.apiService.GetAPIsByOrganization(orgId, projectId, opts)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to get APIs for project %s in org %s", projectId, orgId))
	}

	response := api.RESTAPIListResponse{
		Count: len(apis),
		List:  apis,
		Pagination: api.Pagination{
			Total:  total,
			Offset: opts.Offset,
			Limit:  opts.Limit,
		},
	}

	httputil.WriteJSON(w, http.StatusOK, response)
	return nil
}

// UpdateAPI updates an existing API identified by handle
func (h *APIHandler) UpdateAPI(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("restApiId")
	if apiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	var req api.RESTAPI
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.NewValidation(err)
	}

	// Validate upstream configuration if provided
	if isEmptyUpstreamDefinition(req.Upstream.Main) && (req.Upstream.Sandbox == nil || isEmptyUpstreamDefinition(*req.Upstream.Sandbox)) {
		return apperror.ValidationFailed.New("At least one upstream endpoint (main or sandbox) is required")
	}
	if err := validateUpstreamDefinitions(req.Upstream); err != nil {
		return err
	}

	updatedBy, err := resolveActorErr(r, h.identity, "update API")
	if err != nil {
		return err
	}
	apiResponse, err := h.apiService.UpdateAPIByHandle(apiId, &req, orgId, updatedBy)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to update API %s in org %s", apiId, orgId))
	}

	httputil.WriteJSON(w, http.StatusOK, apiResponse)
	return nil
}

// DeleteAPI handles DELETE /api/v0.9/rest-apis/:apiId and deletes an API by its handle
func (h *APIHandler) DeleteAPI(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("restApiId")
	if apiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	deletedBy, err := resolveActorErr(r, h.identity, "delete API")
	if err != nil {
		return err
	}
	if err := h.apiService.DeleteAPIByHandle(apiId, orgId, deletedBy); err != nil {
		return serviceError(err, fmt.Sprintf("failed to delete API %s in org %s", apiId, orgId))
	}

	httputil.WriteJSON(w, http.StatusNoContent, nil)
	return nil
}

// AddGatewaysToAPI handles POST /api/v0.9/rest-apis/:apiId/gateways to associate gateways with an API
func (h *APIHandler) AddGatewaysToAPI(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("restApiId")
	if apiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	var req []api.AddGatewayToRESTAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.NewValidation(err)
	}

	if len(req) == 0 {
		return apperror.ValidationFailed.New("At least one gateway ID is required")
	}

	// Extract gateway IDs from request
	gatewayIds := make([]string, len(req))
	for i, gw := range req {
		gatewayIds[i] = gw.GatewayId
	}

	createdBy, err := resolveActorErr(r, h.identity, "associate gateways with API")
	if err != nil {
		return err
	}

	gatewaysResponse, err := h.apiService.AddGatewaysToAPIByHandle(apiId, gatewayIds, orgId, createdBy)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to associate gateways with API %s in org %s", apiId, orgId))
	}

	httputil.WriteJSON(w, http.StatusOK, gatewaysResponse)
	return nil
}

// GetAPIGateways handles GET /api/v0.9/rest-apis/:apiId/gateways to get gateways associated with an API including deployment details
func (h *APIHandler) GetAPIGateways(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("restApiId")
	if apiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	limit, offset := parsePagination(r)

	gatewaysResponse, err := h.apiService.GetAPIGatewaysByHandle(apiId, orgId, limit, offset)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to get gateways for API %s in org %s", apiId, orgId))
	}

	httputil.WriteJSON(w, http.StatusOK, gatewaysResponse)
	return nil
}

// ImportOpenAPI handles POST /api/v0.9/rest-apis/import-openapi.
// It accepts multipart/form-data with either a spec file upload or a URL, parses the
// OpenAPI 3.x spec to extract operations, creates the API, and persists the raw spec.
// Swagger 2.x specs are rejected.
func (h *APIHandler) ImportOpenAPI(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	importOpenAPIMaxBytes := h.getOpenAPISpecMaxBytes()
	r.Body = http.MaxBytesReader(w, r.Body, importOpenAPIMaxBytes)
	if err := r.ParseMultipartForm(importOpenAPIMaxBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return apperror.PayloadTooLarge.New("request body exceeds the maximum allowed size")
		}
		return apperror.ValidationFailed.New("invalid multipart form")
	}

	var req api.ImportOpenAPIRequest
	req.DisplayName = strings.TrimSpace(r.FormValue("displayName"))
	req.Version = strings.TrimSpace(r.FormValue("version"))
	req.Context = strings.TrimSpace(r.FormValue("context"))
	req.ProjectId = strings.TrimSpace(r.FormValue("projectId"))
	if id := strings.TrimSpace(r.FormValue("id")); id != "" {
		req.Id = &id
	}
	if desc := strings.TrimSpace(r.FormValue("description")); desc != "" {
		req.Description = &desc
	}
	if upstreamStr := r.FormValue("upstream"); upstreamStr != "" {
		if err := json.Unmarshal([]byte(upstreamStr), &req.Upstream); err != nil {
			return apperror.ValidationFailed.New("upstream must be a valid JSON object")
		}
	}

	if req.DisplayName == "" {
		return apperror.ValidationFailed.New("displayName is required")
	}
	if req.Version == "" {
		return apperror.ValidationFailed.New("version is required")
	}
	if req.Context == "" {
		return apperror.ValidationFailed.New("context is required")
	}
	if req.ProjectId == "" {
		return apperror.ValidationFailed.New("projectId is required")
	}
	if isEmptyUpstreamDefinition(req.Upstream.Main) && (req.Upstream.Sandbox == nil || isEmptyUpstreamDefinition(*req.Upstream.Sandbox)) {
		return apperror.ValidationFailed.New("At least one upstream endpoint (main or sandbox) is required")
	}
	if err := validateUpstreamDefinitions(req.Upstream); err != nil {
		return err
	}

	// Obtain spec content from the uploaded file
	file, header, fileErr := r.FormFile("file")
	if fileErr != nil {
		return apperror.ValidationFailed.New("a spec file is required")
	}
	defer file.Close()

	specContent, err := io.ReadAll(io.LimitReader(file, importOpenAPIMaxBytes+1))
	if err != nil {
		return apperror.ValidationFailed.New("failed to read uploaded spec file")
	}
	if int64(len(specContent)) > importOpenAPIMaxBytes {
		return apperror.PayloadTooLarge.New("spec file exceeds maximum allowed size")
	}

	specFileName := h.apiDocumentService.NormalizeSpecFileName(header.Filename)
 
	// Validate and extract operations from spec
	operations, err := h.apiDocumentService.ExtractOperationsFromSpec(specContent)
	if err != nil {
		return err
	}

	// Create the API with extracted operations
	createdBy, err := resolveActorErr(r, h.identity, "import OpenAPI")
	if err != nil {
		return err
	}

	createReq := &api.CreateRESTAPIRequest{
		Id:          req.Id,
		DisplayName: req.DisplayName,
		Version:     req.Version,
		Context:     req.Context,
		ProjectId:   req.ProjectId,
		Description: req.Description,
		Upstream:    req.Upstream,
		Operations:  &operations,
	}

	apiResponse, artifactUUID, err := h.apiService.CreateAPI(createReq, orgId, createdBy)
	if err != nil {
		return serviceError(err, "failed to create API in org "+orgId)
	}

	// Persist the spec document
	// If this fails, rollback the API creation
	docReq := &dto.CreateAPIDocumentRequest{
		Type:             constants.DocumentTypeDefinition,
		Handle:           constants.DocumentHandleDefinition,
		DisplayName:      constants.DocumentDisplayNameDefinition,
		FileName:         specFileName,
		Content:		  specContent,
	}

	_, docErr := h.apiDocumentService.CreateDocument(docReq, orgId, createdBy, artifactUUID)
	if docErr != nil {
		h.slogger.Error("Failed to persist OpenAPI spec document; rolling back API", 
			"apiId", artifactUUID, "error", docErr)
		if rollbackErr := h.apiService.DeleteAPI(artifactUUID, orgId, createdBy); rollbackErr != nil {
			h.slogger.Error("Rollback after document creation failure also failed", 
				"apiId", artifactUUID, "error", rollbackErr)
		}
		return apperror.Internal.Wrap(docErr).WithLogMessage("failed to persist API specification")
	}

	setLocation(w, "rest-apis", strOrEmpty(apiResponse.Id))
	httputil.WriteJSON(w, http.StatusCreated, apiResponse)
	return nil
}

// GetOpenAPISpec handles GET /rest-apis/{restApiId}/openapi.
func (h *APIHandler) GetOpenAPISpec(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	restApiId := r.PathValue("restApiId")
	if restApiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	// Resolve artifact UUID from API handle
	artifactUUID, err := h.apiService.GetArtifactUUID(restApiId, orgId)
	if err != nil {
		return serviceError(err, "failed to resolve API "+restApiId+" in org "+orgId)
	}

	// Retrieve document
	doc, err := h.apiDocumentService.GetDocument(artifactUUID, orgId)
	if err != nil {
		return serviceError(err, "failed to fetch openapi spec for API "+restApiId)
	}

	content := string(doc.Content)
	httputil.WriteJSON(w, http.StatusOK, api.OpenAPIContent{Content: &content})
	return nil
}

// PutOpenAPISpec handles PUT /rest-apis/{restApiId}/openapi.
// Replaces (or creates) the API definition spec for this API. Only OpenAPI 3.x
// specs are accepted; Swagger 2.x is rejected.
func (h *APIHandler) PutOpenAPISpec(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	restApiId := r.PathValue("restApiId")
	if restApiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	importOpenAPIMaxBytes := h.getOpenAPISpecMaxBytes()
	r.Body = http.MaxBytesReader(w, r.Body, importOpenAPIMaxBytes)
	if err := r.ParseMultipartForm(importOpenAPIMaxBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return apperror.PayloadTooLarge.New("request body exceeds the maximum allowed size")
		}
		return apperror.ValidationFailed.New("failed to parse multipart form: request too large or malformed")
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return apperror.ValidationFailed.New("spec file is required (field: file)")
	}
	defer file.Close()

	specContent, err := io.ReadAll(io.LimitReader(file, importOpenAPIMaxBytes+1))
	if err != nil {
		return apperror.ValidationFailed.New("failed to read spec file")
	}
	if int64(len(specContent)) > importOpenAPIMaxBytes {
		return apperror.PayloadTooLarge.New("spec file exceeds maximum allowed size")
	}

	specFileName := h.apiDocumentService.NormalizeSpecFileName(header.Filename)

	updatedBy, err := resolveActorErr(r, h.identity, "update API openapi spec")
	if err != nil {
		return err
	}

	// Fetch existing API before branching — needed to check read-only status and,
	// in the non-read-only path, to merge existing operation policies.
	existingAPI, err := h.apiService.GetAPIByHandle(restApiId, orgId)
	if err != nil {
		return serviceError(err, "failed to fetch API "+restApiId+" to sync operations from spec")
	}

	// Resolve artifact UUID
	artifactUUID, err := h.apiService.GetArtifactUUID(restApiId, orgId)
	if err != nil {
		return serviceError(err, "failed to resolve API "+restApiId+" in org "+orgId)
	}

	operationsUpdated := false
	if existingAPI.ReadOnly != nil && *existingAPI.ReadOnly {
		// Read-only API: validate the spec but do not update operations.
		if result := h.apiDocumentService.ValidateOpenAPISpec(specContent); !result.IsValid {
			msg := "invalid OpenAPI specification"
			if len(result.Errors) > 0 {
				msg = result.Errors[0].Message
			}
			return apperror.ValidationFailed.New(msg)
		}
	} else {
		// Non-read-only: validate, extract operations, merge with existing policies.
		syncedOps, err := h.apiDocumentService.ExtractAndMergeOperations(specContent, existingAPI.Operations)
		if err != nil {
			return err
		}
		if len(syncedOps) > 0 {
			updatedAPI := *existingAPI
			updatedAPI.Operations = &syncedOps
			_, err = h.apiService.UpdateAPIByHandle(restApiId, &updatedAPI, orgId, updatedBy)
			if err != nil {
				return serviceError(err, "failed to update operations for API "+restApiId+" after spec change")
			}
			operationsUpdated = true
		}
	}

	// Update document
	docReq := &dto.PutAPIDocumentRequest{
		Type:             constants.DocumentTypeDefinition,
		Handle:           constants.DocumentHandleDefinition,
		DisplayName:      constants.DocumentDisplayNameDefinition,
		FileName:         specFileName,
		Content:		  specContent,
	}

	if err := h.apiDocumentService.PutDocument(docReq, orgId, updatedBy, artifactUUID); err != nil {
		h.slogger.Error("Failed to persist spec", "api", restApiId, "error", err)
		if operationsUpdated {
			if _, rollbackErr := h.apiService.UpdateAPIByHandle(restApiId, existingAPI, orgId, updatedBy); rollbackErr != nil {
				h.slogger.Error("Failed to restore operations after spec persist failure",
					"api", restApiId, "error", rollbackErr)
			}
		}
		return serviceError(err, "failed to persist API specification for API "+restApiId)
	}

	content := string(specContent)
	httputil.WriteJSON(w, http.StatusOK, api.OpenAPIContent{Content: &content})
	return nil
}

// ValidateOpenAPI handles POST /api/v0.9/rest-apis/validate-openapi.
// Validates an OpenAPI 3.x spec without creating or modifying any resource.
// Swagger 2.x specs are rejected. Accepts multipart/form-data with a `file` field
// containing the raw spec (.json, .yaml, .yml).
func (h *APIHandler) ValidateOpenAPI(w http.ResponseWriter, r *http.Request) error {
	importOpenAPIMaxBytes := h.getOpenAPISpecMaxBytes()
	r.Body = http.MaxBytesReader(w, r.Body, importOpenAPIMaxBytes)
	if err := r.ParseMultipartForm(importOpenAPIMaxBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return apperror.PayloadTooLarge.New("request body exceeds the maximum allowed size")
		}
		return apperror.ValidationFailed.New("invalid multipart form")
	}

	file, _, fileErr := r.FormFile("file")
	if fileErr != nil {
		return apperror.ValidationFailed.New("a spec file is required")
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, importOpenAPIMaxBytes+1))
	if err != nil {
		return apperror.ValidationFailed.New("failed to read spec file")
	}
	if int64(len(data)) > importOpenAPIMaxBytes {
		return apperror.PayloadTooLarge.New("spec file exceeds maximum allowed size")
	}

	// Delegate to service for validation
	result := h.apiDocumentService.ValidateOpenAPISpec(data)
	httputil.WriteJSON(w, http.StatusOK, result)
	return nil
}

// RegisterRoutes registers all API routes
func (h *APIHandler) RegisterRoutes(mux router.Router) {
	h.slogger.Debug("Registering REST API routes")
	base := constants.APIBasePath + "/rest-apis"
	mux.HandleFunc("POST "+base+"/validate-openapi", middleware.MapErrors(h.slogger, h.ValidateOpenAPI))
	mux.HandleFunc("POST "+base+"/import-openapi", middleware.MapErrors(h.slogger, h.ImportOpenAPI))
	mux.HandleFunc("POST "+base, middleware.MapErrors(h.slogger, h.CreateAPI))
	mux.HandleFunc("GET "+base, middleware.MapErrors(h.slogger, h.ListAPIs))
	mux.HandleFunc("GET "+base+"/{restApiId}", middleware.MapErrors(h.slogger, h.GetAPI))
	mux.HandleFunc("PUT "+base+"/{restApiId}", middleware.MapErrors(h.slogger, h.UpdateAPI))
	mux.HandleFunc("DELETE "+base+"/{restApiId}", middleware.MapErrors(h.slogger, h.DeleteAPI))
	mux.HandleFunc("GET "+base+"/{restApiId}/gateways", middleware.MapErrors(h.slogger, h.GetAPIGateways))
	mux.HandleFunc("POST "+base+"/{restApiId}/gateways", middleware.MapErrors(h.slogger, h.AddGatewaysToAPI))
	mux.HandleFunc("GET "+base+"/{restApiId}/openapi", middleware.MapErrors(h.slogger, h.GetOpenAPISpec))
	mux.HandleFunc("PUT "+base+"/{restApiId}/openapi", middleware.MapErrors(h.slogger, h.PutOpenAPISpec))
}

func isEmptyUpstreamDefinition(definition api.UpstreamDefinition) bool {
	return !hasUpstreamURL(definition) && !hasUpstreamRef(definition)
}

func hasUpstreamURL(definition api.UpstreamDefinition) bool {
	return definition.Url != nil && strings.TrimSpace(*definition.Url) != ""
}

func hasUpstreamRef(definition api.UpstreamDefinition) bool {
	return definition.Ref != nil && strings.TrimSpace(*definition.Ref) != ""
}

// validateUpstreamDefinitions ensures every provided upstream definition specifies
// exactly one of url or ref, as required by the UpstreamDefinition schema.
func validateUpstreamDefinitions(upstream api.Upstream) error {
	if hasUpstreamURL(upstream.Main) && hasUpstreamRef(upstream.Main) {
		return apperror.ValidationFailed.New("The upstream main must specify either a url or a ref, not both.")
	}
	if upstream.Sandbox != nil && hasUpstreamURL(*upstream.Sandbox) && hasUpstreamRef(*upstream.Sandbox) {
		return apperror.ValidationFailed.New("The upstream sandbox must specify either a url or a ref, not both.")
	}
	return nil
}
