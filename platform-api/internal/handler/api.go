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
	"path/filepath"
	"strings"

	"github.com/pb33f/libopenapi"
	v2high "github.com/pb33f/libopenapi/datamodel/high/v2"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	openapivalidator "github.com/pb33f/libopenapi-validator"
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

	// Obtain spec content from the uploaded file.
	file, header, fileErr := r.FormFile("file")
	if fileErr != nil {
		return apperror.ValidationFailed.New("a spec file is required")
	}
	defer file.Close()
	req.File.InitFromMultipart(header)

	data, err := io.ReadAll(io.LimitReader(file, importOpenAPIMaxBytes+1))
	if err != nil {
		return apperror.ValidationFailed.New("failed to read uploaded spec file")
	}
	if int64(len(data)) > importOpenAPIMaxBytes {
		return apperror.ValidationFailed.New("spec file exceeds the maximum allowed size")
	}
	specFileName := normalizeSpecFileName(req.File.Filename())

	// Parse the spec once; extract operations from the parsed document.
	sd, loadErr := loadSpecDocument(data)
	if loadErr != nil {
		h.slogger.Error("Failed to parse OpenAPI spec", "error", loadErr)
		return apperror.ValidationFailed.New("invalid OpenAPI specification")
	}
	if result := validateSpec(sd); !result.IsValid {
		msg := "invalid OpenAPI specification"
		if len(result.Errors) > 0 {
			msg = result.Errors[0].Message
		}
		return apperror.ValidationFailed.New(msg)
	}
	operations := extractOperations(sd)

	specContent := string(sd.raw)
	importedContentType := "application/x-yaml"
	if sd.isJSON {
		importedContentType = "application/json"
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

	createdBy, err := resolveActorErr(r, h.identity, "import OpenAPI")
	if err != nil {
		return err
	}

	apiResponse, artifactUUID, err := h.apiService.CreateAPI(createReq, orgId, createdBy)
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

// normalizeSpecFileName strips the directory component from the uploaded filename.
func normalizeSpecFileName(name string) string {
	return filepath.Base(name)
}

// specDoc holds a libopenapi-parsed document with format metadata and a pre-built
// typed model so callers never build the model twice.
type specDoc struct {
	doc    libopenapi.Document
	raw    []byte // original input bytes, used for YAML re-encoding
	isJSON bool
	isV3   bool
	v3     *libopenapi.DocumentModel[v3high.Document]
	v2     *libopenapi.DocumentModel[v2high.Swagger]
	errs   []error // model-build / validation errors
}

// isJSONBytes returns true if data's first non-whitespace byte is '{'.
func isJSONBytes(data []byte) bool {
	for _, b := range data {
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			continue
		}
		return b == '{'
	}
	return false
}

// loadSpecDocument parses the spec with libopenapi, builds the typed model,
// and returns an error only for unparseable input or missing openapi/swagger key.
// Build/validation errors are stored in sd.errs.
func loadSpecDocument(data []byte) (*specDoc, error) {
	doc, err := libopenapi.NewDocument(data)
	if err != nil {
		return nil, fmt.Errorf("spec is neither valid JSON nor YAML, or is missing 'openapi'/'swagger' key: %w", err)
	}

	sd := &specDoc{
		doc:    doc,
		raw:    data,
		isJSON: isJSONBytes(data),
	}

	info := doc.GetSpecInfo()
	// SpecType is "openapi" for v3, "swagger" for v2.
	sd.isV3 = info != nil && info.SpecType == "openapi"

	if sd.isV3 {
		m, buildErrs := doc.BuildV3Model()
		if buildErrs != nil {
			sd.errs = append(sd.errs, buildErrs)
		}
		sd.v3 = m
	} else {
		m, buildErrs := doc.BuildV2Model()
		if buildErrs != nil {
			sd.errs = append(sd.errs, buildErrs)
		}
		sd.v2 = m
	}

	return sd, nil
}

// validateSpec validates the spec and returns an api.ValidateOpenAPIResponse with
// Info (title/version) populated when valid.
//
// For OpenAPI 3.x, the document is validated against the full OpenAPI JSON
// Meta-Schema via libopenapi-validator, which catches structural violations
// (unknown path item keys, paths without a leading "/", missing required
// fields, invalid $ref targets, etc.) that BuildV3Model errors alone miss.
//
// For Swagger 2.x, libopenapi-validator does not support v2, so build errors
// from BuildV2Model are used instead.
func validateSpec(sd *specDoc) api.ValidateOpenAPIResponse {
	if sd.isV3 {
		return validateSpecV3(sd)
	}
	return validateSpecV2(sd)
}

func validateSpecV3(sd *specDoc) api.ValidateOpenAPIResponse {
	// Surface any model-build errors first ($ref resolution failures, etc.).
	// When present, skip the JSON Schema validator — NewValidator would also fail.
	if len(sd.errs) > 0 {
		errs := make([]api.OpenAPIValidationError, 0, len(sd.errs))
		for _, e := range sd.errs {
			errs = append(errs, api.OpenAPIValidationError{Message: e.Error()})
		}
		return api.ValidateOpenAPIResponse{IsValid: false, Errors: errs}
	}

	// NewValidator internally calls BuildV3Model and — critically — sets the
	// document field that ValidateDocument requires. NewValidatorFromV3Model does
	// not set that field, causing the "Document is not set" error.
	v, vErrs := openapivalidator.NewValidator(sd.doc)
	if vErrs != nil {
		errs := make([]api.OpenAPIValidationError, 0, len(vErrs))
		for _, e := range vErrs {
			errs = append(errs, api.OpenAPIValidationError{Message: e.Error()})
		}
		return api.ValidateOpenAPIResponse{IsValid: false, Errors: errs}
	}

	valid, valErrs := v.ValidateDocument()
	if !valid {
		errs := make([]api.OpenAPIValidationError, 0, len(valErrs))
		for _, e := range valErrs {
			if len(e.SchemaValidationErrors) > 0 {
				for _, sve := range e.SchemaValidationErrors {
					var path *string
					if p := sve.FieldPath; p != "" {
						path = &p
					}
					errs = append(errs, api.OpenAPIValidationError{
						Message: sve.Reason,
						Path:    path,
					})
				}
			} else {
				var path *string
				if p := e.SpecPath; p != "" {
					path = &p
				}
				errs = append(errs, api.OpenAPIValidationError{
					Message: e.Message,
					Path:    path,
				})
			}
		}
		return api.ValidateOpenAPIResponse{IsValid: false, Errors: errs}
	}
	result := api.ValidateOpenAPIResponse{IsValid: true, Errors: []api.OpenAPIValidationError{}}
	if sd.v3.Model.Info != nil {
		title, version := sd.v3.Model.Info.Title, sd.v3.Model.Info.Version
		result.Info = &api.OpenAPISpecInfo{Title: &title, Version: &version}
	}
	return result
}

func validateSpecV2(sd *specDoc) api.ValidateOpenAPIResponse {
	if len(sd.errs) > 0 {
		errs := make([]api.OpenAPIValidationError, 0, len(sd.errs))
		for _, e := range sd.errs {
			errs = append(errs, api.OpenAPIValidationError{Message: e.Error()})
		}
		return api.ValidateOpenAPIResponse{IsValid: false, Errors: errs}
	}
	result := api.ValidateOpenAPIResponse{IsValid: true, Errors: []api.OpenAPIValidationError{}}
	if sd.v2 != nil && sd.v2.Model.Info != nil {
		title, version := sd.v2.Model.Info.Title, sd.v2.Model.Info.Version
		result.Info = &api.OpenAPISpecInfo{Title: &title, Version: &version}
	}
	return result
}

// extractOperations builds api.Operation entries from the spec's paths.
// Returns nil when paths are absent; the service layer creates a wildcard.
func extractOperations(sd *specDoc) []api.Operation {
	httpMethods := map[string]api.OperationRequestMethod{
		"get":     "GET",
		"post":    "POST",
		"put":     "PUT",
		"delete":  "DELETE",
		"patch":   "PATCH",
		"head":    "HEAD",
		"options": "OPTIONS",
		"trace":   "TRACE",
	}

	if sd.isV3 && sd.v3 != nil && sd.v3.Model.Paths != nil && sd.v3.Model.Paths.PathItems != nil {
		var ops []api.Operation
		for path, pathItem := range sd.v3.Model.Paths.PathItems.FromOldest() {
			type methodOp struct {
				method string
				op     *v3high.Operation
			}
			candidates := []methodOp{
				{"get", pathItem.Get},
				{"post", pathItem.Post},
				{"put", pathItem.Put},
				{"delete", pathItem.Delete},
				{"patch", pathItem.Patch},
				{"head", pathItem.Head},
				{"options", pathItem.Options},
				{"trace", pathItem.Trace},
			}
			for _, c := range candidates {
				if c.op == nil {
					continue
				}
				opMethod, supported := httpMethods[c.method]
				if !supported {
					continue
				}
				op := api.Operation{
					Request: api.OperationRequest{
						Method: opMethod,
						Path:   path,
					},
				}
				if c.op.OperationId != "" {
					name := c.op.OperationId
					op.Name = &name
				} else if c.op.Summary != "" {
					summary := c.op.Summary
					op.Name = &summary
				}
				if c.op.Description != "" {
					desc := c.op.Description
					op.Description = &desc
				}
				ops = append(ops, op)
			}
		}
		return ops
	}

	if !sd.isV3 && sd.v2 != nil && sd.v2.Model.Paths != nil && sd.v2.Model.Paths.PathItems != nil {
		var ops []api.Operation
		for path, pathItem := range sd.v2.Model.Paths.PathItems.FromOldest() {
			type methodOp struct {
				method string
				op     *v2high.Operation
			}
			candidates := []methodOp{
				{"get", pathItem.Get},
				{"post", pathItem.Post},
				{"put", pathItem.Put},
				{"delete", pathItem.Delete},
				{"patch", pathItem.Patch},
				{"head", pathItem.Head},
				{"options", pathItem.Options},
			}
			for _, c := range candidates {
				if c.op == nil {
					continue
				}
				opMethod, supported := httpMethods[c.method]
				if !supported {
					continue
				}
				op := api.Operation{
					Request: api.OperationRequest{
						Method: opMethod,
						Path:   path,
					},
				}
				if c.op.OperationId != "" {
					name := c.op.OperationId
					op.Name = &name
				} else if c.op.Summary != "" {
					summary := c.op.Summary
					op.Name = &summary
				}
				if c.op.Description != "" {
					desc := c.op.Description
					op.Description = &desc
				}
				ops = append(ops, op)
			}
		}
		return ops
	}

	return nil
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
	var specReq api.OpenAPISpecFileRequest
	specReq.File.InitFromMultipart(header)

	specContent, err := io.ReadAll(io.LimitReader(file, importOpenAPIMaxBytes+1))
	if err != nil {
		return apperror.ValidationFailed.New("failed to read spec file")
	}
	if int64(len(specContent)) > importOpenAPIMaxBytes {
		return apperror.PayloadTooLarge.New("spec file exceeds maximum allowed size (5 MiB)")
	}

	sd, loadErr := loadSpecDocument(specContent)
	if loadErr != nil {
		return apperror.ValidationFailed.New("uploaded file is not a valid OpenAPI/Swagger spec (must be JSON or YAML)")
	}

	if result := validateSpec(sd); !result.IsValid {
		msg := "uploaded file is not a valid OpenAPI specification"
		if len(result.Errors) > 0 {
			msg = result.Errors[0].Message
		}
		return apperror.ValidationFailed.New(msg)
	}

	importedContentType := "application/x-yaml"
	if sd.isJSON {
		importedContentType = "application/json"
	}

	updatedBy, err := resolveActorErr(r, h.identity, "update API openapi spec")
	if err != nil {
		return err
	}

	artifactUUID, err := h.apiService.GetArtifactUUID(restApiId, orgId)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to resolve API %s in org %s", restApiId, orgId))
	}

	specFileName := normalizeSpecFileName(specReq.File.Filename())

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
	specOps := extractOperations(sd)
	syncedAPI, syncedOps, err := h.computeSyncedOperations(restApiId, orgId, specOps)
	if err != nil {
		return err
	}

	// Operations are written before the spec document so that a failure here
	// leaves nothing persisted.
	// If the spec write subsequently fails the operations are updated but the
	// stored spec is stale; a re-PUT recovers that without data loss.
	if syncedAPI != nil {
		updatedAPI := *syncedAPI
		updatedAPI.Operations = &syncedOps
		if _, err := h.apiService.UpdateAPIByHandle(restApiId, &updatedAPI, orgId, updatedBy); err != nil {
			return serviceError(err, fmt.Sprintf("failed to update operations for API %s after spec change", restApiId))
		}
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

	content := string(specContent)
	httputil.WriteJSON(w, http.StatusOK, api.OpenAPIContent{Content: &content})
	return nil
}

// computeSyncedOperations fetches the API and pre-computes the merged operation
// list from the spec's paths and the existing policy attachments.
// Returns (nil, nil, nil) when the API is gateway-originated (read-only) —
// the caller must skip the operations write in that case.
func (h *APIHandler) computeSyncedOperations(restApiId, orgId string, specOps []api.Operation) (*api.RESTAPI, []api.Operation, error) {
	existingAPI, err := h.apiService.GetAPIByHandle(restApiId, orgId)
	if err != nil {
		return nil, nil, serviceError(err, fmt.Sprintf("failed to fetch API %s to sync operations from spec", restApiId))
	}
	// Gateway-originated APIs are read-only in the control plane; their operations
	// are managed by the data plane and must not be overwritten from the spec.
	if existingAPI.ReadOnly != nil && *existingAPI.ReadOnly {
		return nil, nil, nil
	}

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

	sd, loadErr := loadSpecDocument([]byte(strings.TrimSpace(string(data))))
	if loadErr != nil {
		httputil.WriteJSON(w, http.StatusOK, api.ValidateOpenAPIResponse{
			IsValid: false,
			Errors:  []api.OpenAPIValidationError{{Message: loadErr.Error()}},
		})
		return nil
	}

	result := validateSpec(sd)
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
