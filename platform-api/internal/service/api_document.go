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

package service

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/pb33f/libopenapi"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	openapivalidator "github.com/pb33f/libopenapi-validator"
	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// APIDocumentService handles API document operations, including OpenAPI specifications.
// It manages document CRUD operations and OpenAPI spec validation.
type APIDocumentService struct {
	documentRepo repository.DocumentRepository
	auditRepo    repository.AuditRepository
	slogger      *slog.Logger
}

// NewAPIDocumentService creates a new API document service
func NewAPIDocumentService(documentRepo repository.DocumentRepository, auditRepo repository.AuditRepository, slogger *slog.Logger) *APIDocumentService {
	return &APIDocumentService{
		documentRepo: documentRepo,
		auditRepo:    auditRepo,
		slogger:      slogger,
	}
}

// CreateDocument creates a new OpenAPI spec document for an artifact.
func (s *APIDocumentService) CreateDocument(req *dto.CreateAPIDocumentRequest, orgId string, userId string, artifactUUID string) (string, error) {
	if req == nil {
		return "", apperror.ValidationFailed.New("document request is required")
	}
	if artifactUUID == "" {
		return "", apperror.ValidationFailed.New("artifact UUID is required")
	}

	doc := &model.Document{
		ArtifactUUID:     artifactUUID,
		OrganizationUUID: orgId,
		Type:             req.Type,
		Handle:           req.Handle,
		DisplayName:      req.DisplayName,
		FileName:         req.FileName,
		ContentType:      s.GetSpecContentType(req.Content),
		Content:          req.Content,
		CreatedBy:        userId,
	}

	if doc.Handle == "" {
		handle, handleErr := utils.GenerateHandle(doc.DisplayName, func(candidate string) bool {
			exists, err := s.documentRepo.DocumentHandleExistsForArtifact(doc.ArtifactUUID, candidate)
			if err != nil {
				return true
			}
			return exists
		})
		if handleErr != nil {
			s.slogger.Error("Failed to generate document handle", "displayName", doc.DisplayName, "error", handleErr)
			return "", apperror.Internal.Wrap(handleErr).WithLogMessage("failed to generate document handle")
		}
		doc.Handle = handle
	}

	if err := s.documentRepo.CreateDocument(doc); err != nil {
		s.slogger.Error("Failed to create document", "artifactUUID", doc.ArtifactUUID, "error", err)
		return "", err
	}

	if err := s.auditRepo.Record("CREATE", doc.ArtifactUUID, "api_definition", doc.OrganizationUUID, doc.CreatedBy); err != nil {
		s.slogger.Error("Failed to record audit entry for document create", "artifactUUID", doc.ArtifactUUID, "error", err)
	}
	return doc.Handle, nil
}

// GetDocument retrieves an OpenAPI spec document by artifact UUID and org.
func (s *APIDocumentService) GetDocument(artifactUUID, orgId string) (*dto.APIDocumentContent, error) {
	if artifactUUID == "" {
		return nil, apperror.ValidationFailed.New("artifact UUID is required")
	}

	doc, err := s.documentRepo.GetDocumentByArtifactAndType(artifactUUID, constants.DocumentTypeDefinition, orgId)
	if err != nil {
		s.slogger.Error("Failed to get document", "artifactUUID", artifactUUID, "error", err)
		return nil, err
	}

	if doc == nil {
		return nil, apperror.NotFound.New()
	}

	return &dto.APIDocumentContent{
		Content:     doc.Content,
		ContentType: doc.ContentType,
	}, nil
}

// PutDocument updates or creates an OpenAPI spec document for an artifact.
// If a document of the same type already exists, it is updated in-place.
// If no document exists, a new one is created.
func (s *APIDocumentService) PutDocument(req *dto.PutAPIDocumentRequest, orgId string, userId string, artifactUUID string) error {
	if req == nil {
		return apperror.ValidationFailed.New("document request is required")
	}
	if artifactUUID == "" {
		return apperror.ValidationFailed.New("artifact UUID is required")
	}

	doc := &model.Document{
		ArtifactUUID:     artifactUUID,
		OrganizationUUID: orgId,
		Type:             req.Type,
		Handle:           req.Handle,
		DisplayName:      req.DisplayName,
		FileName:         req.FileName,
		ContentType:      s.GetSpecContentType(req.Content),
		Content:          req.Content,
		UpdatedBy:        userId,
	}

	existing, err := s.documentRepo.GetDocumentByArtifactAndType(doc.ArtifactUUID, doc.Type, doc.OrganizationUUID)
	if err != nil {
		s.slogger.Error("Failed to check existing document", "artifactUUID", doc.ArtifactUUID, "error", err)
		return err
	}

	isUpdate := existing != nil
	if isUpdate {
		doc.Handle = existing.Handle
	} else {
		doc.CreatedBy = userId
		if doc.Handle == "" {
			handle, handleErr := utils.GenerateHandle(doc.DisplayName, func(candidate string) bool {
				exists, err := s.documentRepo.DocumentHandleExistsForArtifact(doc.ArtifactUUID, candidate)
				if err != nil {
					return true
				}
				return exists
			})
			if handleErr != nil {
				s.slogger.Error("Failed to generate document handle", "displayName", doc.DisplayName, "error", handleErr)
				return apperror.Internal.Wrap(handleErr).WithLogMessage("failed to generate document handle")
			}
			doc.Handle = handle
		}
	}

	if err := s.documentRepo.UpsertDocument(doc); err != nil {
		s.slogger.Error("Failed to upsert document", "artifactUUID", doc.ArtifactUUID, "error", err)
		return err
	}

	action := "CREATE"
	if isUpdate {
		action = "UPDATE"
	}
	if err := s.auditRepo.Record(action, doc.ArtifactUUID, "api_definition", doc.OrganizationUUID, doc.UpdatedBy); err != nil {
		s.slogger.Error("Failed to record audit entry for document upsert", "artifactUUID", doc.ArtifactUUID, "error", err)
	}
	return nil
}

// DeleteDocument deletes a document for an artifact identified by its handle.
func (s *APIDocumentService) DeleteDocument(artifactUUID, handle, orgId string) error {
	if artifactUUID == "" {
		return apperror.ValidationFailed.New("artifact UUID is required")
	}
	if handle == "" {
		return apperror.ValidationFailed.New("document handle is required")
	}

	if err := s.documentRepo.DeleteDocument(artifactUUID, handle, orgId); err != nil {
		s.slogger.Error("Failed to delete document", "artifactUUID", artifactUUID, "handle", handle, "error", err)
		return err
	}
	return nil
}

// ValidateOpenAPISpec validates an OpenAPI 3.x spec without creating or modifying any resource.
// Swagger 2.x specs are rejected.
// Returns the validation result with any errors and spec info (title, version).
func (s *APIDocumentService) ValidateOpenAPISpec(specContent []byte) api.ValidateOpenAPIResponse {
	sd, loadErr := loadSpecDocument([]byte(strings.TrimSpace(string(specContent))))
	if loadErr != nil {
		return api.ValidateOpenAPIResponse{
			IsValid: false,
			Errors:  []api.OpenAPIValidationError{{Message: loadErr.Error()}},
		}
	}

	return validateSpec(sd)
}

// ExtractOperationsFromSpec extracts operations from an OpenAPI spec.
// Returns nil if the spec has no paths; the caller creates a wildcard operation.
func (s *APIDocumentService) ExtractOperationsFromSpec(specContent []byte) ([]api.Operation, error) {
	sd, loadErr := loadSpecDocument(specContent)
	if loadErr != nil {
		s.slogger.Error("Failed to parse OpenAPI spec for operation extraction", "error", loadErr)
		return nil, apperror.ValidationFailed.New("invalid OpenAPI specification")
	}

	if result := validateSpec(sd); !result.IsValid {
		msg := "invalid OpenAPI specification"
		if len(result.Errors) > 0 {
			msg = result.Errors[0].Message
		}
		return nil, apperror.ValidationFailed.New(msg)
	}

	return extractOperations(sd), nil
}

// NormalizeSpecFileName strips the directory component from an uploaded filename,
// storing only the bare name (file-access rule: filename only in storage).
func (s *APIDocumentService) NormalizeSpecFileName(name string) string {
	return filepath.Base(name)
}

// ExtractAndMergeOperations validates the spec, extracts its operations, and
// merges them with existing ones, preserving per-operation policies.
// Use this in the non-read-only spec update path where all three steps are needed.
func (s *APIDocumentService) ExtractAndMergeOperations(specContent []byte, existing *[]api.Operation) ([]api.Operation, error) {
	specOps, err := s.ExtractOperationsFromSpec(specContent)
	if err != nil {
		return nil, err
	}
	return s.MergeOperations(existing, specOps), nil
}

// MergeOperations merges spec-derived operations with existing ones, preserving
// per-operation policies so a spec update doesn't silently drop attached policies.
func (s *APIDocumentService) MergeOperations(existing *[]api.Operation, specOps []api.Operation) []api.Operation {
	if existing == nil {
		return specOps
	}
	existingByKey := make(map[string]api.Operation)
	for _, op := range *existing {
		key := strings.ToUpper(string(op.Request.Method)) + ":" + op.Request.Path
		existingByKey[key] = op
	}
	synced := make([]api.Operation, 0, len(specOps))
	for _, op := range specOps {
		key := strings.ToUpper(string(op.Request.Method)) + ":" + op.Request.Path
		if prev, ok := existingByKey[key]; ok && prev.Request.Policies != nil && len(*prev.Request.Policies) > 0 {
			op.Request.Policies = prev.Request.Policies
		}
		synced = append(synced, op)
	}
	return synced
}

// GetSpecContentType determines the content type (JSON or YAML) for spec content.
func (s *APIDocumentService) GetSpecContentType(specContent []byte) string {
	if isJSONBytes(specContent) {
		return "application/json"
	}
	return "application/yaml"
}

// specDoc holds a libopenapi-parsed OpenAPI 3.x document with format metadata
// and a pre-built typed model.
type specDoc struct {
	doc    libopenapi.Document
	raw    []byte // original input bytes, used for re-encoding
	isJSON bool
	v3     *libopenapi.DocumentModel[v3high.Document]
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
// and returns an error if the spec is not OpenAPI 3.x.
// Swagger 2.x specs are rejected with a validation error.
// Build/validation errors are stored in sd.errs.
func loadSpecDocument(data []byte) (*specDoc, error) {
	doc, err := libopenapi.NewDocument(data)
	if err != nil {
		return nil, fmt.Errorf("spec is neither valid JSON nor YAML, or is missing 'openapi' key: %w", err)
	}

	info := doc.GetSpecInfo()
	// SpecType is "openapi" for v3, "swagger" for v2.
	if info == nil || info.SpecType != "openapi" {
		return nil, fmt.Errorf("only OpenAPI 3.x specifications are supported; Swagger 2.x is not allowed")
	}

	sd := &specDoc{
		doc:    doc,
		raw:    data,
		isJSON: isJSONBytes(data),
	}

	m, buildErrs := doc.BuildV3Model()
	if buildErrs != nil {
		sd.errs = append(sd.errs, buildErrs)
	}
	sd.v3 = m

	return sd, nil
}

// validateSpec validates the OpenAPI 3.x spec against the full OpenAPI JSON
// Meta-Schema via libopenapi-validator, which catches structural violations
// (unknown path item keys, paths without a leading "/", missing required
// fields, invalid $ref targets, etc.). Returns an api.ValidateOpenAPIResponse
// with Info (title/version) populated when valid.
func validateSpec(sd *specDoc) api.ValidateOpenAPIResponse {
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
	// Validate that at least one API path is defined.
    if sd.v3.Model.Paths != nil && sd.v3.Model.Paths.PathItems != nil && sd.v3.Model.Paths.PathItems.Len() == 0 {
        return api.ValidateOpenAPIResponse{
            IsValid: false,
            Errors: []api.OpenAPIValidationError{
                {
                    Message: "API contract must contain at least one operation.",
                },
            },
        }
    }
	result := api.ValidateOpenAPIResponse{IsValid: true, Errors: []api.OpenAPIValidationError{}}
	if sd.v3 != nil && sd.v3.Model.Info != nil {
		title, version := sd.v3.Model.Info.Title, sd.v3.Model.Info.Version
		result.Info = &api.OpenAPISpecInfo{Title: &title, Version: &version}
	}
	return result
}

// extractOperations builds api.Operation entries from the OpenAPI 3.x spec's paths.
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

	if sd.v3 != nil && sd.v3.Model.Paths != nil && sd.v3.Model.Paths.PathItems != nil {
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

	return nil
}
