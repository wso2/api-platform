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
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/router"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"github.com/wso2/api-platform/httpkit/httputil"
	"gopkg.in/yaml.v3"
)

const importOpenAPIMaxBytes = 5 << 20 // 5 MiB

type APIHandler struct {
	apiService   *service.APIService
	identity     *service.IdentityService
	documentRepo repository.DocumentRepository
	slogger      *slog.Logger
}

func NewAPIHandler(apiService *service.APIService, identity *service.IdentityService, documentRepo repository.DocumentRepository, slogger *slog.Logger) *APIHandler {
	return &APIHandler{
		apiService:   apiService,
		identity:     identity,
		documentRepo: documentRepo,
		slogger:      slogger,
	}
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
// OpenAPI spec to extract operations, creates the API, and persists the raw spec.
func (h *APIHandler) ImportOpenAPI(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	if err := r.ParseMultipartForm(importOpenAPIMaxBytes); err != nil {
		return apperror.ValidationFailed.New("invalid multipart form")
	}

	name := strings.TrimSpace(r.FormValue("name"))
	version := strings.TrimSpace(r.FormValue("version"))
	context := strings.TrimSpace(r.FormValue("context"))
	projectId := strings.TrimSpace(r.FormValue("projectId"))
	description := strings.TrimSpace(r.FormValue("description"))
	upstreamURL := strings.TrimSpace(r.FormValue("upstream"))

	if name == "" {
		return apperror.ValidationFailed.New("name is required")
	}
	if version == "" {
		return apperror.ValidationFailed.New("version is required")
	}
	if context == "" {
		return apperror.ValidationFailed.New("context is required")
	}
	if projectId == "" {
		return apperror.ValidationFailed.New("projectId is required")
	}
	if upstreamURL == "" {
		return apperror.ValidationFailed.New("upstream is required")
	}

	// Obtain spec content from the uploaded file.
	file, header, fileErr := r.FormFile("file")
	if fileErr != nil {
		return apperror.ValidationFailed.New("a spec file is required")
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, importOpenAPIMaxBytes+1))
	if err != nil {
		return apperror.ValidationFailed.New("failed to read uploaded spec file")
	}
	if int64(len(data)) > importOpenAPIMaxBytes {
		return apperror.ValidationFailed.New("spec file exceeds the maximum allowed size")
	}
	specContent := string(data)
	specFileName := filepath.Base(header.Filename)

	// Parse the spec once; extract operations from the parsed root.
	specRoot, err := parseSpecRoot(specContent)
	if err != nil {
		h.slogger.Error("Failed to parse OpenAPI spec", "error", err)
		return apperror.ValidationFailed.New("invalid OpenAPI specification")
	}
	operations := extractOperationsFromRoot(specRoot)

	// Build upstream.
	upstreamConfig := api.Upstream{
		Main: api.UpstreamDefinition{Url: &upstreamURL},
	}

	// Build the create request.
	var descPtr *string
	if description != "" {
		descPtr = &description
	}
	req := &api.CreateRESTAPIRequest{
		DisplayName: name,
		Version:     version,
		Context:     context,
		ProjectId:   projectId,
		Description: descPtr,
		Upstream:    upstreamConfig,
		Operations:  &operations,
	}

	createdBy, err := resolveActorErr(r, h.identity, "import OpenAPI")
	if err != nil {
		return err
	}

	apiResponse, artifactUUID, err := h.apiService.CreateAPI(req, orgId, createdBy)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to create API from OpenAPI spec in org %s", orgId))
	}

	// Persist the raw spec as a DEFINITION document.
	if h.documentRepo != nil && artifactUUID != "" {
		handle, handleErr := utils.GenerateHandle("OpenAPI Definition", func(candidate string) bool {
			exists, _ := h.documentRepo.DocumentHandleExistsForArtifact(artifactUUID, candidate)
			return exists
		})
		if handleErr != nil {
			h.slogger.Error("Failed to generate document handle", "apiId", artifactUUID, "error", handleErr)
		} else {
			doc := &model.Document{
				ArtifactUUID:     artifactUUID,
				OrganizationUUID: orgId,
				Type:             model.DocumentTypeDefinition,
				Handle:           handle,
				DisplayName:      "OpenAPI Definition",
				FileName:         specFileName,
				Content:          []byte(specContent),
				DataVersion:      "1.0",
				CreatedBy:        createdBy,
			}
			if docErr := h.documentRepo.CreateDocument(doc); docErr != nil {
				h.slogger.Error("Failed to persist OpenAPI spec document", "apiId", artifactUUID, "error", docErr)
				// Non-fatal: the API was created successfully; log and continue.
			}
		}
	}

	setLocation(w, "rest-apis", strOrEmpty(apiResponse.Id))
	httputil.WriteJSON(w, http.StatusCreated, apiResponse)
	return nil
}

// parseSpecRoot deserialises a JSON or YAML OpenAPI spec string into a raw map.
func parseSpecRoot(specContent string) (map[string]interface{}, error) {
	var root map[string]interface{}
	if err := json.Unmarshal([]byte(specContent), &root); err != nil {
		if err2 := yaml.Unmarshal([]byte(specContent), &root); err2 != nil {
			return nil, fmt.Errorf("spec is neither valid JSON nor YAML")
		}
	}
	return root, nil
}

// extractOperationsFromRoot returns the list of HTTP operations declared in a
// parsed OpenAPI 3.x/Swagger 2.x spec root. Returns nil when no paths exist;
// the service layer then generates a default wildcard operation.
func extractOperationsFromRoot(root map[string]interface{}) []api.Operation {
	paths, _ := root["paths"].(map[string]interface{})
	if len(paths) == 0 {
		return nil
	}

	httpMethods := map[string]api.OperationRequestMethod{
		"get":     "GET",
		"post":    "POST",
		"put":     "PUT",
		"delete":  "DELETE",
		"patch":   "PATCH",
		"head":    "HEAD",
		"options": "OPTIONS",
	}

	var ops []api.Operation
	for path, pathItemRaw := range paths {
		pathItem, ok := pathItemRaw.(map[string]interface{})
		if !ok {
			continue
		}
		for method, opRaw := range pathItem {
			opMethod, supported := httpMethods[strings.ToLower(method)]
			if !supported {
				continue
			}
			op := api.Operation{
				Request: api.OperationRequest{
					Method: opMethod,
					Path:   path,
				},
			}
			if opMap, ok := opRaw.(map[string]interface{}); ok {
				if opId, ok := opMap["operationId"].(string); ok && opId != "" {
					op.Name = &opId
				} else if summary, ok := opMap["summary"].(string); ok && summary != "" {
					op.Name = &summary
				}
				if desc, ok := opMap["description"].(string); ok && desc != "" {
					op.Description = &desc
				}
			}
			ops = append(ops, op)
		}
	}
	return ops
}

// RegisterRoutes registers all API routes
func (h *APIHandler) RegisterRoutes(mux router.Router) {
	h.slogger.Debug("Registering REST API routes")
	base := constants.APIBasePath + "/rest-apis"
	mux.HandleFunc("POST "+base+"/import-openapi", middleware.MapErrors(h.slogger, h.ImportOpenAPI))
	mux.HandleFunc("POST "+base, middleware.MapErrors(h.slogger, h.CreateAPI))
	mux.HandleFunc("GET "+base, middleware.MapErrors(h.slogger, h.ListAPIs))
	mux.HandleFunc("GET "+base+"/{restApiId}", middleware.MapErrors(h.slogger, h.GetAPI))
	mux.HandleFunc("PUT "+base+"/{restApiId}", middleware.MapErrors(h.slogger, h.UpdateAPI))
	mux.HandleFunc("DELETE "+base+"/{restApiId}", middleware.MapErrors(h.slogger, h.DeleteAPI))
	mux.HandleFunc("GET "+base+"/{restApiId}/gateways", middleware.MapErrors(h.slogger, h.GetAPIGateways))
	mux.HandleFunc("POST "+base+"/{restApiId}/gateways", middleware.MapErrors(h.slogger, h.AddGatewaysToAPI))
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
