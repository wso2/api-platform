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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
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

	r.Body = http.MaxBytesReader(w, r.Body, importOpenAPIMaxBytes)
	if err := r.ParseMultipartForm(importOpenAPIMaxBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return apperror.PayloadTooLarge.New("request body exceeds the maximum allowed size")
		}
		return apperror.ValidationFailed.New("invalid multipart form")
	}

	displayName := strings.TrimSpace(r.FormValue("displayName"))
	apiId := strings.TrimSpace(r.FormValue("id"))
	version := strings.TrimSpace(r.FormValue("version"))
	context := strings.TrimSpace(r.FormValue("context"))
	projectId := strings.TrimSpace(r.FormValue("projectId"))
	description := strings.TrimSpace(r.FormValue("description"))
	upstreamURL := strings.TrimSpace(r.FormValue("upstream"))

	if displayName == "" {
		return apperror.ValidationFailed.New("displayName is required")
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
	specFileName := normalizeSpecFileName(header.Filename)

	// Parse the spec once; extract operations from the parsed root.
	specRoot, isJSON, err := parseSpecRoot(string(data))
	if err != nil {
		h.slogger.Error("Failed to parse OpenAPI spec", "error", err)
		return apperror.ValidationFailed.New("invalid OpenAPI specification")
	}
	if result := validateOpenAPIContent(string(data), specRoot); !result.IsValid {
		msg := "invalid OpenAPI specification"
		if len(result.Errors) > 0 {
			msg = result.Errors[0].Message
		}
		return apperror.ValidationFailed.New(msg)
	}
	operations := extractOperationsFromRoot(specRoot)

	// parseSpecRoot already determined the format; no re-parse needed.
	importedContentType := specContentType(isJSON)

	// Always persist as YAML regardless of the uploaded format.
	yamlBytes, err := yaml.Marshal(specRoot)
	if err != nil {
		return serviceError(err, "failed to re-encode spec as YAML")
	}
	specContent := string(yamlBytes)

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
		DisplayName: displayName,
		Version:     version,
		Context:     context,
		ProjectId:   projectId,
		Description: descPtr,
		Upstream:    upstreamConfig,
		Operations:  &operations,
	}
	if apiId != "" {
		req.Id = &apiId
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
	// Any failure here rolls back the API so the caller never receives a 201
	// for an API whose spec was not stored.
	if h.documentRepo != nil && artifactUUID != "" {
		// Freshly created artifact has no documents yet, so no collision is possible.
		handle, handleErr := utils.GenerateHandle("OpenAPI Definition", nil)
		if handleErr != nil {
			h.slogger.Error("Failed to generate document handle; rolling back API", "apiId", artifactUUID, "error", handleErr)
			if rollbackErr := h.apiService.DeleteAPI(artifactUUID, orgId, createdBy); rollbackErr != nil {
				h.slogger.Error("Rollback after document handle failure also failed", "apiId", artifactUUID, "error", rollbackErr)
			}
			return apperror.Internal.New()
		}
		doc := &model.Document{
			ArtifactUUID:     artifactUUID,
			OrganizationUUID: orgId,
			Type:             model.DocumentTypeDefinition,
			Handle:           handle,
			DisplayName:      "OpenAPI Definition",
			FileName:         specFileName,
			ContentType:      importedContentType,
			Content:          []byte(specContent),
			CreatedBy:        createdBy,
		}
		if docErr := h.documentRepo.CreateDocument(doc); docErr != nil {
			h.slogger.Error("Failed to persist OpenAPI spec document; rolling back API", "apiId", artifactUUID, "error", docErr)
			if rollbackErr := h.apiService.DeleteAPI(artifactUUID, orgId, createdBy); rollbackErr != nil {
				h.slogger.Error("Rollback after document persist failure also failed", "apiId", artifactUUID, "error", rollbackErr)
			}
			return apperror.Internal.New()
		}
	}

	setLocation(w, "rest-apis", strOrEmpty(apiResponse.Id))
	httputil.WriteJSON(w, http.StatusCreated, apiResponse)
	return nil
}

// normalizeSpecFileName ensures the stored filename always carries a .yaml extension.
func normalizeSpecFileName(name string) string {
	base := filepath.Base(name)
	if strings.EqualFold(filepath.Ext(base), ".json") {
		return strings.TrimSuffix(base, filepath.Ext(base)) + ".yaml"
	}
	return base
}

// specContentType maps the isJSON result from parseSpecRoot to a MIME type string.
func specContentType(isJSON bool) string {
	if isJSON {
		return "application/json"
	}
	return "application/x-yaml"
}

// parseSpecRoot deserialises a JSON or YAML OpenAPI spec string into a raw map
// and verifies that the document declares a top-level 'openapi' (3.x) or
// 'swagger' (2.x) key so non-OpenAPI payloads are rejected early.
// isJSON is true when the input was valid JSON; false means it was YAML.
// This lets callers record the original format without re-parsing.
func parseSpecRoot(specContent string) (root map[string]interface{}, isJSON bool, err error) {
	if jsonErr := json.Unmarshal([]byte(specContent), &root); jsonErr != nil {
		if yamlErr := yaml.Unmarshal([]byte(specContent), &root); yamlErr != nil {
			return nil, false, fmt.Errorf("spec is neither valid JSON nor YAML")
		}
		isJSON = false
	} else {
		isJSON = true
	}
	_, hasOpenAPI := root["openapi"]
	_, hasSwagger := root["swagger"]
	if !hasOpenAPI && !hasSwagger {
		return nil, false, fmt.Errorf("spec must declare 'openapi' (3.x) or 'swagger' (2.x) at the root")
	}
	return root, isJSON, nil
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

// GetOpenAPISpec handles GET /rest-apis/{restApiId}/openapi.
// Returns the raw API definition spec stored for this API, or 404 if none has been uploaded.
func (h *APIHandler) GetOpenAPISpec(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	restApiId := r.PathValue("restApiId")
	if restApiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	artifactUUID, err := h.apiService.GetArtifactUUID(restApiId, orgId)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to resolve API %s in org %s", restApiId, orgId))
	}

	doc, err := h.documentRepo.GetDocumentByArtifactAndType(artifactUUID, model.DocumentTypeDefinition, orgId)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to fetch openapi spec for API %s", restApiId))
	}
	if doc == nil {
		return apperror.NotFound.New("API definition not found")
	}

	content := string(doc.Content)
	httputil.WriteJSON(w, http.StatusOK, api.OpenAPIContent{Content: &content})
	return nil
}

// PutOpenAPISpec handles PUT /rest-apis/{restApiId}/openapi.
// Replaces (or creates) the API definition spec for this API.
func (h *APIHandler) PutOpenAPISpec(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	restApiId := r.PathValue("restApiId")
	if restApiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

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
		return apperror.PayloadTooLarge.New("spec file exceeds maximum allowed size (5 MiB)")
	}

	specRoot, isJSON, err := parseSpecRoot(string(specContent))
	if err != nil || specRoot == nil {
		return apperror.ValidationFailed.New("uploaded file is not a valid OpenAPI/Swagger spec (must be JSON or YAML)")
	}

	if result := validateOpenAPIContent(string(specContent), specRoot); !result.IsValid {
		msg := "uploaded file is not a valid OpenAPI specification"
		if len(result.Errors) > 0 {
			msg = result.Errors[0].Message
		}
		return apperror.ValidationFailed.New(msg)
	}

	// parseSpecRoot already determined the format; no re-parse needed.
	importedContentType := specContentType(isJSON)

	// Always persist as YAML regardless of the uploaded format.
	yamlBytes, err := yaml.Marshal(specRoot)
	if err != nil {
		return serviceError(err, "failed to re-encode spec as YAML")
	}
	specContent = yamlBytes

	updatedBy, err := resolveActorErr(r, h.identity, "update API openapi spec")
	if err != nil {
		return err
	}

	artifactUUID, err := h.apiService.GetArtifactUUID(restApiId, orgId)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to resolve API %s in org %s", restApiId, orgId))
	}

	specFileName := normalizeSpecFileName(header.Filename)

	// Re-use the existing document's handle so we update in place rather than
	// creating a second DEFINITION document. If no document exists yet, generate
	// a fresh handle the same way ImportOpenAPI does.
	existing, err := h.documentRepo.GetDocumentByArtifactAndType(artifactUUID, model.DocumentTypeDefinition, orgId)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to check existing openapi spec for API %s", restApiId))
	}
	var docHandle string
	if existing != nil {
		docHandle = existing.Handle
	} else {
		docHandle, err = utils.GenerateHandle("OpenAPI Definition", func(candidate string) bool {
			exists, _ := h.documentRepo.DocumentHandleExistsForArtifact(artifactUUID, candidate)
			return exists
		})
		if err != nil {
			return serviceError(err, fmt.Sprintf("failed to generate document handle for API %s", restApiId))
		}
	}

	// Pre-compute the synced operation list before any writes so that a read
	// failure (API not found, read-only) stops us before the document is persisted.
	syncedAPI, syncedOps, err := h.computeSyncedOperations(restApiId, orgId, specRoot)
	if err != nil {
		return err
	}

	doc := &model.Document{
		ArtifactUUID:     artifactUUID,
		OrganizationUUID: orgId,
		Type:             model.DocumentTypeDefinition,
		Handle:           docHandle,
		DisplayName:      "OpenAPI Definition",
		FileName:         specFileName,
		ContentType:      importedContentType,
		Content:          specContent,
		CreatedBy:        updatedBy,
		UpdatedBy:        updatedBy,
	}
	if err := h.documentRepo.UpsertDocument(doc); err != nil {
		return serviceError(err, fmt.Sprintf("failed to upsert openapi spec for API %s", restApiId))
	}

	// Write operations only for control-plane-managed APIs; gateway-originated
	// (read-only) APIs have their operations managed by the data plane.
	if syncedAPI != nil {
		updatedAPI := *syncedAPI
		updatedAPI.Operations = &syncedOps
		if _, err := h.apiService.UpdateAPIByHandle(restApiId, &updatedAPI, orgId, updatedBy); err != nil {
			return serviceError(err, fmt.Sprintf("failed to update operations for API %s after spec change", restApiId))
		}
	}

	content := string(specContent)
	httputil.WriteJSON(w, http.StatusOK, api.OpenAPIContent{Content: &content})
	return nil
}

// computeSyncedOperations fetches the API and pre-computes the merged operation
// list from the spec's paths and the existing policy attachments.
// Returns (nil, nil, nil) when the API is gateway-originated (read-only) —
// the caller must skip the operations write in that case.
func (h *APIHandler) computeSyncedOperations(restApiId, orgId string, specRoot map[string]interface{}) (*api.RESTAPI, []api.Operation, error) {
	existingAPI, err := h.apiService.GetAPIByHandle(restApiId, orgId)
	if err != nil {
		return nil, nil, serviceError(err, fmt.Sprintf("failed to fetch API %s to sync operations from spec", restApiId))
	}
	// Gateway-originated APIs are read-only in the control plane; their operations
	// are managed by the data plane and must not be overwritten from the spec.
	if existingAPI.ReadOnly != nil && *existingAPI.ReadOnly {
		return nil, nil, nil
	}

	specOps := extractOperationsFromRoot(specRoot)
	existingByKey := make(map[string]api.Operation)
	if existingAPI.Operations != nil {
		for _, op := range *existingAPI.Operations {
			key := strings.ToUpper(string(op.Request.Method)) + ":" + op.Request.Path
			existingByKey[key] = op
		}
	}
	synced := make([]api.Operation, 0, len(specOps))
	for _, op := range specOps {
		key := strings.ToUpper(string(op.Request.Method)) + ":" + op.Request.Path
		if prev, ok := existingByKey[key]; ok && prev.Request.Policies != nil && len(*prev.Request.Policies) > 0 {
			op.Request.Policies = prev.Request.Policies
		}
		synced = append(synced, op)
	}
	return existingAPI, synced, nil
}


type validateOpenAPIError struct {
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

type validateOpenAPIInfo struct {
	Title   string `json:"title,omitempty"`
	Version string `json:"version,omitempty"`
}

type validateOpenAPIResult struct {
	IsValid bool                  `json:"isValid"`
	Errors  []validateOpenAPIError `json:"errors"`
	Info    *validateOpenAPIInfo   `json:"info,omitempty"`
}

// validateOpenAPIContent validates an OpenAPI spec string using kin-openapi.
// specRoot is the already-parsed root map from parseSpecRoot — the caller owns
// that parse and passes it in so this function never re-parses the bytes.
// External $refs are not resolved (loader.IsExternalRefsAllowed = false),
// which prevents SSRF via spec $ref URLs.
// Swagger 2.x specs are passed through without kin-openapi structural
// validation because kin-openapi only understands OpenAPI 3.x; parseSpecRoot
// already confirmed the spec is parseable JSON/YAML with a valid root key.
func validateOpenAPIContent(specContent string, specRoot map[string]interface{}) validateOpenAPIResult {
	if _, isSwagger := specRoot["swagger"]; isSwagger {
		return validateOpenAPIResult{IsValid: true, Errors: []validateOpenAPIError{}}
	}

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false

	doc, err := loader.LoadFromData([]byte(specContent))
	if err != nil {
		return validateOpenAPIResult{
			IsValid: false,
			Errors:  []validateOpenAPIError{{Message: err.Error()}},
		}
	}

	if err := doc.Validate(context.Background(), openapi3.DisableExamplesValidation()); err != nil {
		return validateOpenAPIResult{
			IsValid: false,
			Errors:  extractOpenAPIValidationErrors(err),
		}
	}

	result := validateOpenAPIResult{
		IsValid: true,
		Errors:  []validateOpenAPIError{},
	}
	if doc.Info != nil {
		result.Info = &validateOpenAPIInfo{
			Title:   doc.Info.Title,
			Version: doc.Info.Version,
		}
	}
	return result
}

// extractOpenAPIValidationErrors flattens kin-openapi MultiError into a flat
// list of message strings. Each entry in a MultiError may itself be a
// MultiError, so the extraction is recursive.
func extractOpenAPIValidationErrors(err error) []validateOpenAPIError {
	var multi openapi3.MultiError
	if errors.As(err, &multi) {
		out := make([]validateOpenAPIError, 0, len(multi))
		for _, e := range multi {
			out = append(out, extractOpenAPIValidationErrors(e)...)
		}
		return out
	}
	return []validateOpenAPIError{{Message: err.Error()}}
}

// ValidateOpenAPI handles POST /api/v0.9/rest-apis/validate-openapi.
// Validates an OpenAPI 3.x or Swagger 2.x spec without creating or modifying
// any resource. Accepts multipart/form-data with a `file` field containing the
// raw spec (.json, .yaml, .yml).
func (h *APIHandler) ValidateOpenAPI(w http.ResponseWriter, r *http.Request) error {
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
		return apperror.PayloadTooLarge.New("spec file exceeds maximum allowed size (5 MiB)")
	}

	specContent := strings.TrimSpace(string(data))

	specRoot, _, err := parseSpecRoot(specContent)
	if err != nil {
		httputil.WriteJSON(w, http.StatusOK, validateOpenAPIResult{
			IsValid: false,
			Errors:  []validateOpenAPIError{{Message: err.Error()}},
		})
		return nil
	}

	result := validateOpenAPIContent(specContent, specRoot)
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
