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
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
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
	artifactRepo repository.ArtifactRepository
	auditRepo    repository.AuditRepository
	slogger      *slog.Logger
}

// NewAPIDocumentService creates a new API document service. artifactRepo is
// required for the /apis/{apiType}/{apiId}/docs endpoints to resolve a
// kind-aware handle to an artifact UUID; nil is acceptable only in tests that
// never call ResolveArtifactUUID.
func NewAPIDocumentService(documentRepo repository.DocumentRepository, artifactRepo repository.ArtifactRepository, auditRepo repository.AuditRepository, slogger *slog.Logger) *APIDocumentService {
	return &APIDocumentService{
		documentRepo: documentRepo,
		artifactRepo: artifactRepo,
		auditRepo:    auditRepo,
		slogger:      slogger,
	}
}

// ResolveArtifactUUID resolves (apiType, apiId) to the artifact's internal UUID, scoped to orgID.
// An unrecognised apiType and an unknown apiId both collapse to the same NotFound.
func (s *APIDocumentService) ResolveArtifactUUID(apiType, apiID, orgID string) (string, error) {
	if apiType == "" || apiID == "" {
		return "", apperror.NotFound.New()
	}
	metadata, err := s.artifactRepo.GetAPIMetadataByHandleAndKind(apiID, apiType, orgID)
	if errors.Is(err, repository.ErrUnknownArtifactKind) {
		return "", apperror.NotFound.New()
	}
	if err != nil {
		return "", fmt.Errorf("failed to resolve artifact by handle and kind: %w", err)
	}
	if metadata == nil {
		return "", apperror.NotFound.New()
	}
	return metadata.ID, nil
}

// CreateDocument creates a document for a reserved type
// (DEFINITION / THUMBNAIL) that is managed via its own dedicated endpoints.
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
		ContentType:      s.contentTypeForDocType(req.Type, req.Content),
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

// resolveStoredDocType computes the value to persist in the type column.
// For OTHER it stores the otherTypeName value directly (e.g. "FAQ"), so the
// OTHER_ prefix never appears in the database. Fixed types are stored as-is.
func resolveStoredDocType(docType, otherTypeName string) string {
	if docType != constants.DocumentTypeOther {
		return docType
	}
	return strings.TrimSpace(otherTypeName)
}

// CreateApiDocument creates a user-authored document attached to an artifact.
func (s *APIDocumentService) CreateApiDocument(req *dto.CreateAPIDocumentRequest, orgID, userID, artifactUUID string) (string, error) {
	if req == nil {
		return "", apperror.ValidationFailed.New("document request is required")
	}
	if artifactUUID == "" {
		return "", apperror.ValidationFailed.New("artifact UUID is required")
	}
	if !constants.ValidAPIDocumentUserTypes[req.Type] {
		return "", apperror.ValidationFailed.New("invalid document type")
	}
	if req.Type == constants.DocumentTypeOther {
		trimmed := strings.TrimSpace(req.OtherTypeName)
		if trimmed == "" {
			return "", apperror.ValidationFailed.New("otherTypeName is required when type is OTHER")
		}
		if constants.ForbiddenOtherTypeNames[strings.ToUpper(trimmed)] {
			return "", apperror.ValidationFailed.New("otherTypeName cannot be a reserved or fixed document type name")
		}
	}
	req.Type = resolveStoredDocType(req.Type, req.OtherTypeName)
	if strings.TrimSpace(req.DisplayName) == "" {
		return "", apperror.ValidationFailed.New("displayName is required")
	}
	nameExists, nameErr := s.documentRepo.DocumentDisplayNameExistsForArtifact(artifactUUID, req.DisplayName, "")
	if nameErr != nil {
		s.slogger.Error("Failed to check document display name existence", "artifactUUID", artifactUUID, "error", nameErr)
		return "", apperror.Internal.Wrap(nameErr).WithLogMessage("failed to validate document display name")
	}
	if nameExists {
		return "", apperror.Conflict.New().WithLogMessage("document display name already exists for artifact")
	}
	if req.Handle != "" {
		exists, existsErr := s.documentRepo.DocumentHandleExistsForArtifact(artifactUUID, req.Handle)
		if existsErr != nil {
			s.slogger.Error("Failed to check document handle existence", "artifactUUID", artifactUUID, "handle", req.Handle, "error", existsErr)
			return "", apperror.Internal.Wrap(existsErr).WithLogMessage("failed to validate document handle")
		}
		if exists {
			return "", apperror.Conflict.New().WithLogMessage("document handle already exists for artifact")
		}
	}

	return s.CreateDocument(req, orgID, userID, artifactUUID)
}

// UpsertDocument updates or creates a document for an artifact.
// If a document of the same type already exists, it is updated in-place.
// If no document exists, a new one is created.
func (s *APIDocumentService) UpsertDocument(req *dto.PutAPIDocumentRequest, orgId string, userId string, artifactUUID string) error {
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
		ContentType:      s.contentTypeForDocType(req.Type, req.Content),
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

// DeleteUserDocument deletes a user-authored document identified by handle.
// Refuses to delete a DEFINITION/THUMBNAIL document
func (s *APIDocumentService) DeleteApiDocument(artifactUUID, handle, orgID, userID string) error {
	if artifactUUID == "" {
		return apperror.ValidationFailed.New("artifact UUID is required")
	}
	if handle == "" {
		return apperror.ValidationFailed.New("document handle is required")
	}

	// docType="" excludes reserved types at the repo layer, so a DELETE of
	// the DEFINITION/THUMBNAIL handle via this surface finds no row and 404s.
	existing, err := s.documentRepo.GetDocument(artifactUUID, handle, orgID, "")
	if err != nil {
		s.slogger.Error("Failed to load document for delete", "artifactUUID", artifactUUID, "handle", handle, "error", err)
		return err
	}
	if existing == nil {
		return apperror.NotFound.New()
	}

	if err := s.documentRepo.DeleteDocument(artifactUUID, handle, orgID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperror.NotFound.New()
		}
		s.slogger.Error("Failed to delete document", "artifactUUID", artifactUUID, "handle", handle, "error", err)
		return err
	}
	if err := s.auditRepo.Record("DELETE", artifactUUID, "api_definition", orgID, userID); err != nil {
		s.slogger.Error("Failed to record audit entry for document delete", "artifactUUID", artifactUUID, "error", err)
	}
	return nil
}

// GetAllApiDocuments returns a page of user-facing documents attached to
// artifactUUID, optionally filtered by docType. Reserved types (DEFINITION,
// THUMBNAIL) are excluded by the repository at the SQL layer.
func (s *APIDocumentService) GetAllApiDocuments(artifactUUID, orgID, docType string, limit, offset int) ([]*model.Document, int, error) {
	if artifactUUID == "" {
		return nil, 0, apperror.ValidationFailed.New("artifact UUID is required")
	}

	docs, total, err := s.documentRepo.ListDocumentsByArtifact(artifactUUID, orgID, docType, limit, offset)
	if err != nil {
		s.slogger.Error("Failed to list documents", "artifactUUID", artifactUUID, "error", err)
		return nil, 0, err
	}
	return docs, total, nil
}

// GetDocument retrieves a document (metadata + content) by handle, scoped
// to artifactUUID + orgID. docType is optional:
//
//   - docType != "": strict match on type too. Pass the reserved type
//     (e.g. constants.DocumentTypeDefinition) when fetching the OpenAPI spec
//     or thumbnail.
//   - docType == "": the request came from the user-facing /docs/{docId}
//     path. The repository excludes reserved types at the SQL layer, so a
//     caller cannot fetch the OpenAPI spec or thumbnail by guessing the
//     handle on this endpoint.
func (s *APIDocumentService) GetDocument(artifactUUID, handle, orgID, docType string) (*model.Document, error) {
	if artifactUUID == "" {
		return nil, apperror.ValidationFailed.New("artifact UUID is required")
	}
	if handle == "" {
		return nil, apperror.ValidationFailed.New("document handle is required")
	}

	doc, err := s.documentRepo.GetDocument(artifactUUID, handle, orgID, docType)
	if err != nil {
		s.slogger.Error("Failed to get document", "artifactUUID", artifactUUID, "handle", handle, "error", err)
		return nil, err
	}
	if doc == nil {
		return nil, apperror.NotFound.New()
	}
	return doc, nil
}

// UpdateUserDocument applies a partial update to a user-authored document.
// Each non-nil pointer field in req replaces the stored value; req.Content
// (non-nil) replaces the stored bytes along with ContentType and FileName.
// Type is validated against ValidAPIDocumentUserTypes so a PUT cannot morph
// a user doc into the singleton DEFINITION type.
func (s *APIDocumentService) UpdateApiDocument(req *dto.UpdateAPIDocumentRequest, orgID, userID, artifactUUID, handle string) error {
	if req == nil {
		return apperror.ValidationFailed.New("document request is required")
	}
	if artifactUUID == "" {
		return apperror.ValidationFailed.New("artifact UUID is required")
	}
	if handle == "" {
		return apperror.ValidationFailed.New("document handle is required")
	}

	// docType="" excludes reserved types at the repo layer, so a PUT against
	// the DEFINITION/THUMBNAIL handle via this surface finds no row
	existing, err := s.documentRepo.GetDocument(artifactUUID, handle, orgID, "")
	if err != nil {
		s.slogger.Error("Failed to load document for update", "artifactUUID", artifactUUID, "handle", handle, "error", err)
		return err
	}
	if existing == nil {
		return apperror.NotFound.New()
	}

	updatedDocument := *existing
	updatedDocument.UpdatedBy = userID
	if req.DisplayName != nil {
		trimmed := strings.TrimSpace(*req.DisplayName)
		if trimmed == "" {
			return apperror.ValidationFailed.New("displayName must not be empty")
		}
		nameExists, nameErr := s.documentRepo.DocumentDisplayNameExistsForArtifact(artifactUUID, trimmed, handle)
		if nameErr != nil {
			s.slogger.Error("Failed to check document display name existence", "artifactUUID", artifactUUID, "error", nameErr)
			return apperror.Internal.Wrap(nameErr).WithLogMessage("failed to validate document display name")
		}
		if nameExists {
			return apperror.Conflict.New().WithLogMessage("document display name already exists for artifact")
		}
		updatedDocument.DisplayName = trimmed
	}
	if req.FileName != nil {
		updatedDocument.FileName = *req.FileName
	}
	updateContent := req.Content != nil
	if updateContent {
		updatedDocument.Content = req.Content
		if req.ContentType != nil {
			updatedDocument.ContentType = *req.ContentType
		}
	}

	if err := s.documentRepo.UpdateDocument(&updatedDocument, updateContent); err != nil {
		s.slogger.Error("Failed to update document", "artifactUUID", artifactUUID, "handle", handle, "error", err)
		return err
	}

	if err := s.auditRepo.Record("UPDATE", artifactUUID, "api_definition", orgID, userID); err != nil {
		s.slogger.Error("Failed to record audit entry for document update", "artifactUUID", artifactUUID, "error", err)
	}
	return nil
}

// ValidateOpenAPISpec validates an OpenAPI 3.x spec without creating or modifying any resource.
// Swagger 2.x specs are rejected.
// Returns the validation result with any errors and spec info (title, version).
func (s *APIDocumentService) ValidateOpenAPISpec(specContent []byte) api.ValidateOpenAPIResponse {
	sd, loadErr := utils.LoadSpecDocument([]byte(strings.TrimSpace(string(specContent))))
	if loadErr != nil {
		return api.ValidateOpenAPIResponse{
			IsValid: false,
			Errors:  []api.OpenAPIValidationError{{Message: loadErr.Error()}},
		}
	}

	return utils.ValidateSpec(sd)
}

// ExtractOperationsFromSpec extracts operations from an OpenAPI spec.
// Returns nil if the spec has no paths; the caller creates a wildcard operation.
func (s *APIDocumentService) ExtractOperationsFromSpec(specContent []byte) ([]api.Operation, error) {
	sd, loadErr := utils.LoadSpecDocument(specContent)
	if loadErr != nil {
		s.slogger.Error("Failed to parse OpenAPI spec for operation extraction", "error", loadErr)
		return nil, apperror.ValidationFailed.New("invalid OpenAPI specification")
	}

	if result := utils.ValidateSpec(sd); !result.IsValid {
		msg := "invalid OpenAPI specification"
		if len(result.Errors) > 0 {
			msg = result.Errors[0].Message
		}
		return nil, apperror.ValidationFailed.New(msg)
	}

	return extractOperations(sd), nil
}

// maxSpecFileNameLen is the DB column ceiling (file_name VARCHAR(255)).
const maxSpecFileNameLen = 255

// NormalizeSpecFileName strips the directory component from an uploaded filename
// and caps the result to the DB column ceiling, preserving the extension and
// UTF-8 boundaries so a cap never splits a multi-byte character.
func (s *APIDocumentService) NormalizeSpecFileName(name string) string {
	base := filepath.Base(name)
	if len(base) <= maxSpecFileNameLen {
		return base
	}
	ext := filepath.Ext(base)
	stem := base[:len(base)-len(ext)]
	if maxStem := maxSpecFileNameLen - len(ext); maxStem > 0 {
		return utils.TruncateAtRuneBoundary(stem, maxStem) + ext
	}
	return utils.TruncateAtRuneBoundary(base, maxSpecFileNameLen)
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
	if utils.IsJSONBytes(specContent) {
		return "application/json"
	}
	return "application/yaml"
}

func (s *APIDocumentService) GetImageContentType(content []byte) string {
	head := content
	if len(head) > 512 {
		head = head[:512]
	}
	return http.DetectContentType(head)
}

// contentTypeForDocType chooses the stored MIME type for a document based on its type
func (s *APIDocumentService) contentTypeForDocType(docType string, content []byte) string {
	switch docType {
	case constants.DocumentTypeDefinition:
		return s.GetSpecContentType(content)
	case constants.DocumentTypeThumbnail:
		return s.GetImageContentType(content)
	default:
		return "text/markdown; charset=utf-8"
	}
}

// extractOperations builds api.Operation entries from the OpenAPI 3.x spec's paths.
// Returns nil when paths are absent; the service layer creates a wildcard.
func extractOperations(sd *utils.SpecDocument) []api.Operation {
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

	if sd.V3 != nil && sd.V3.Model.Paths != nil && sd.V3.Model.Paths.PathItems != nil {
		var ops []api.Operation
		for path, pathItem := range sd.V3.Model.Paths.PathItems.FromOldest() {
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
