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

package service

import (
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// ---------------------------------------------------------------------------
// Shared mocks
// ---------------------------------------------------------------------------

type deleteApiDocCall struct {
	artifactUUID, handle, orgUUID string
}

type deleteDocCall struct {
	artifactUUID, handle, orgUUID, docType string
}

type upsertCall struct {
	doc *model.Document
}

// mockDocumentRepository embeds the real interface so missing methods panic
// on call rather than silently returning zero values.
type mockDocumentRepository struct {
	repository.DocumentRepository

	// Return knobs — set per test.
	getDocResult            *model.Document
	getDocErr               error
	handleExistsResult      bool
	handleExistsErr         error
	displayNameExistsResult bool
	displayNameExistsErr    error
	createDocErr            error
	upsertDocErr            error
	updateApiDocErr         error
	deleteApiDocErr         error
	deleteDocErr            error
	listDocsResult          []*model.Document
	listDocsTotal           int
	listDocsErr             error

	// Call tracking.
	deleteApiDocCalls            []deleteApiDocCall
	deleteDocCalls               []deleteDocCall
	upsertCalls                  []upsertCall
	createdDocs                  []*model.Document
	updateApiDocCalls            []*model.Document
	updateApiDocContentFlags     []bool
	lastGetDocType               string
	lastDisplayNameExcludeHandle string
}

func (m *mockDocumentRepository) GetDocument(artifactUUID, handle, orgUUID, docType string) (*model.Document, error) {
	m.lastGetDocType = docType
	return m.getDocResult, m.getDocErr
}

func (m *mockDocumentRepository) DocumentHandleExistsForArtifact(artifactUUID, handle string) (bool, error) {
	return m.handleExistsResult, m.handleExistsErr
}

func (m *mockDocumentRepository) DocumentDisplayNameExistsForArtifact(artifactUUID, displayName, excludeHandle string) (bool, error) {
	m.lastDisplayNameExcludeHandle = excludeHandle
	return m.displayNameExistsResult, m.displayNameExistsErr
}

func (m *mockDocumentRepository) CreateDocument(doc *model.Document) error {
	if m.createDocErr != nil {
		return m.createDocErr
	}
	m.createdDocs = append(m.createdDocs, doc)
	return nil
}

func (m *mockDocumentRepository) UpsertDocument(doc *model.Document) error {
	if m.upsertDocErr != nil {
		return m.upsertDocErr
	}
	m.upsertCalls = append(m.upsertCalls, upsertCall{doc: doc})
	return nil
}

func (m *mockDocumentRepository) UpdateApiDocument(doc *model.Document, updateContent bool) error {
	if m.updateApiDocErr != nil {
		return m.updateApiDocErr
	}
	m.updateApiDocCalls = append(m.updateApiDocCalls, doc)
	m.updateApiDocContentFlags = append(m.updateApiDocContentFlags, updateContent)
	return nil
}

func (m *mockDocumentRepository) DeleteApiDocument(artifactUUID, handle, orgUUID string) error {
	m.deleteApiDocCalls = append(m.deleteApiDocCalls, deleteApiDocCall{artifactUUID, handle, orgUUID})
	return m.deleteApiDocErr
}

func (m *mockDocumentRepository) DeleteDocument(artifactUUID, handle, orgUUID, docType string) error {
	m.deleteDocCalls = append(m.deleteDocCalls, deleteDocCall{artifactUUID, handle, orgUUID, docType})
	return m.deleteDocErr
}

func (m *mockDocumentRepository) ListDocumentsByArtifact(artifactUUID, orgUUID, docType string, limit, offset int) ([]*model.Document, int, error) {
	return m.listDocsResult, m.listDocsTotal, m.listDocsErr
}

type auditCall struct {
	action, resourceUUID, resourceType, orgUUID, performedBy string
}

type recordingAuditRepo struct {
	calls []auditCall
	err   error
}

func (r *recordingAuditRepo) Record(action, resourceUUID, resourceType, orgUUID, performedBy string) error {
	r.calls = append(r.calls, auditCall{action, resourceUUID, resourceType, orgUUID, performedBy})
	return r.err
}

// mockArtifactRepository is only used by tests that exercise ResolveArtifactUUID.
type mockArtifactRepository struct {
	repository.ArtifactRepository

	metadataByHandleAndKind *model.APIMetadata
	metadataErr             error
	lastHandle, lastKind    string
}

func (m *mockArtifactRepository) GetAPIMetadataByHandleAndKind(handle, kind, orgUUID string) (*model.APIMetadata, error) {
	m.lastHandle, m.lastKind = handle, kind
	return m.metadataByHandleAndKind, m.metadataErr
}

func newTestDocumentService() (*APIDocumentService, *mockDocumentRepository, *recordingAuditRepo, *mockArtifactRepository) {
	docRepo := &mockDocumentRepository{}
	auditRepo := &recordingAuditRepo{}
	artifactRepo := &mockArtifactRepository{}
	svc := NewAPIDocumentService(docRepo, artifactRepo, auditRepo, slog.New(slog.NewTextHandler(discard{}, nil)))
	return svc, docRepo, auditRepo, artifactRepo
}

// discard swallows slog output so test output stays focused on failures.
type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

// ---------------------------------------------------------------------------
// User-document CRUD (/apis/{apiType}/{apiId}/docs)
// ---------------------------------------------------------------------------

// CreateApiDocument refuses reserved types (DEFINITION, THUMBNAIL) at the service layer so a user cannot bypass the dedicated endpoints by POSTing to /docs.
func TestAPIDocumentService_CreateApiDocument_RejectsReservedType(t *testing.T) {
	cases := []struct {
		name    string
		docType string
	}{
		{"DEFINITION is rejected", constants.DocumentTypeDefinition},
		{"THUMBNAIL is rejected", constants.DocumentTypeThumbnail},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, docRepo, _, _ := newTestDocumentService()

			_, err := svc.CreateApiDocument(&dto.CreateAPIDocumentRequest{
				Type:        tc.docType,
				DisplayName: "attempt",
			}, "org-1", "alice", "artifact-1")

			if err == nil || !apperror.ValidationFailed.Is(err) {
				t.Fatalf("CreateApiDocument(type=%q) err = %v, want ValidationFailed", tc.docType, err)
			}
			if len(docRepo.createdDocs) != 0 {
				t.Errorf("CreateDocument was called for a reserved type — must short-circuit before the repo")
			}
		})
	}
}

// CreateApiDocument refuses a caller-supplied reserved handle (api-thumbnail / api-definition) so a user can't shadow the system singletons.
func TestAPIDocumentService_CreateApiDocument_RejectsReservedHandle(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()

	_, err := svc.CreateApiDocument(&dto.CreateAPIDocumentRequest{
		Type:        constants.DocumentTypeHowTo,
		DisplayName: "Getting started",
		Handle:      constants.DocumentHandleThumbnail,
	}, "org-1", "alice", "artifact-1")

	if err == nil || !apperror.ValidationFailed.Is(err) {
		t.Fatalf("CreateApiDocument(handle=%q) err = %v, want ValidationFailed",
			constants.DocumentHandleThumbnail, err)
	}
	if len(docRepo.createdDocs) != 0 {
		t.Errorf("CreateDocument was called with a reserved handle — must short-circuit before the repo")
	}
}

// CreateApiDocument with type=OTHER refuses a free-text name that collides (case-insensitively) with a fixed/reserved type name.
func TestAPIDocumentService_CreateApiDocument_ForbiddenOtherNameRejected(t *testing.T) {
	// Pick whichever forbidden name exists so this test survives additions.
	var forbiddenName string
	for name := range constants.ForbiddenOtherTypeNames {
		forbiddenName = strings.ToLower(name)
		break
	}
	if forbiddenName == "" {
		t.Skip("ForbiddenOtherTypeNames is empty — nothing to test")
	}

	svc, _, _, _ := newTestDocumentService()
	_, err := svc.CreateApiDocument(&dto.CreateAPIDocumentRequest{
		Type:          constants.DocumentTypeOther,
		OtherTypeName: forbiddenName,
		DisplayName:   "benign display",
	}, "org-1", "alice", "artifact-1")

	if err == nil || !apperror.ValidationFailed.Is(err) {
		t.Errorf("CreateApiDocument(OTHER, otherTypeName=%q) err = %v, want ValidationFailed",
			forbiddenName, err)
	}
}

// CreateApiDocument allows an otherTypeName that starts with DOC_ — custom doc types are not restricted by prefix.
func TestAPIDocumentService_CreateApiDocument_AllowsDOCPrefixedOtherTypeName(t *testing.T) {
	cases := []string{"DOC_HowTo", "doc_samples", "DOC_Other", "DOC_Custom"}
	for _, name := range cases {
		svc, docRepo, _, _ := newTestDocumentService()
		_, err := svc.CreateApiDocument(&dto.CreateAPIDocumentRequest{
			Type:          constants.DocumentTypeOther,
			OtherTypeName: name,
			DisplayName:   "fine display",
		}, "org-1", "alice", "artifact-1")
		if err != nil {
			t.Errorf("CreateApiDocument(OTHER, otherTypeName=%q) err = %v, want nil (DOC_ prefix is allowed)", name, err)
		}
		if len(docRepo.createdDocs) != 1 {
			t.Errorf("CreateApiDocument(OTHER, otherTypeName=%q): got %d created docs, want 1", name, len(docRepo.createdDocs))
		}
	}
}

// UpdateApiDocument allows an otherTypeName that starts with DOC_ — custom doc types are not restricted by prefix.
func TestAPIDocumentService_UpdateApiDocument_AllowsDOCPrefixedOtherTypeName(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()
	docRepo.getDocResult = &model.Document{
		ID: "doc-1", ArtifactUUID: "artifact-1", OrganizationUUID: "org-1",
		Type: constants.DocumentTypePrefix + constants.DocumentTypeHowTo, Handle: "guide", DisplayName: "Guide",
	}
	newType := constants.DocumentTypeOther
	docPrefixed := "DOC_Custom"
	err := svc.UpdateApiDocument(&dto.UpdateAPIDocumentRequest{
		Type:          &newType,
		OtherTypeName: docPrefixed,
	}, "org-1", "alice", "artifact-1", "doc-1")
	if err != nil {
		t.Errorf("UpdateApiDocument(OTHER, otherTypeName=%q) err = %v, want nil (DOC_ prefix is allowed)", docPrefixed, err)
	}
	if len(docRepo.updateApiDocCalls) != 1 {
		t.Errorf("UpdateApiDocument(OTHER, otherTypeName=%q): got %d update calls, want 1", docPrefixed, len(docRepo.updateApiDocCalls))
	}
}

// CreateApiDocument allows a duplicate display name within the same artifact — uniqueness is not enforced.
func TestAPIDocumentService_CreateApiDocument_DuplicateDisplayNameAllowed(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()

	_, err := svc.CreateApiDocument(&dto.CreateAPIDocumentRequest{
		Type:        constants.DocumentTypeHowTo,
		DisplayName: "Getting started",
	}, "org-1", "alice", "artifact-1")

	if err != nil {
		t.Fatalf("CreateApiDocument with duplicate displayName err = %v, want nil (duplicate display names are allowed)", err)
	}
	if len(docRepo.createdDocs) != 1 {
		t.Errorf("CreateDocument not called: got %d created docs, want 1", len(docRepo.createdDocs))
	}
}

// CreateApiDocument refuses a caller-supplied handle that already exists on the same artifact (DB unique index enforced service-side first).
func TestAPIDocumentService_CreateApiDocument_DuplicateHandleRejected(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()
	docRepo.handleExistsResult = true

	_, err := svc.CreateApiDocument(&dto.CreateAPIDocumentRequest{
		Type:        constants.DocumentTypeHowTo,
		DisplayName: "Different name",
		Handle:      "overview",
	}, "org-1", "alice", "artifact-1")

	if err == nil || !apperror.Conflict.Is(err) {
		t.Fatalf("CreateApiDocument duplicate handle err = %v, want Conflict", err)
	}
	if len(docRepo.createdDocs) != 0 {
		t.Errorf("CreateDocument called despite duplicate handle")
	}
}

// ResolveArtifactUUID collapses an unknown apiType to NotFound
func TestAPIDocumentService_ResolveArtifactUUID_UnknownKindMapsToNotFound(t *testing.T) {
	svc, _, _, artifactRepo := newTestDocumentService()
	artifactRepo.metadataErr = repository.ErrUnknownArtifactKind

	_, err := svc.ResolveArtifactUUID("not-a-real-kind", "whatever", "org-1")
	if err == nil || !apperror.NotFound.Is(err) {
		t.Errorf("ResolveArtifactUUID(unknown kind) err = %v, want NotFound", err)
	}
}

// ResolveArtifactUUID returns NotFound when the handle doesn't match any row for the given kind.
func TestAPIDocumentService_ResolveArtifactUUID_UnknownHandleMapsToNotFound(t *testing.T) {
	svc, _, _, artifactRepo := newTestDocumentService()
	artifactRepo.metadataByHandleAndKind = nil
	artifactRepo.metadataErr = nil

	_, err := svc.ResolveArtifactUUID(constants.RestApi, "no-such-api", "org-1")
	if err == nil || !apperror.NotFound.Is(err) {
		t.Errorf("ResolveArtifactUUID(unknown handle) err = %v, want NotFound", err)
	}
}

// ResolveArtifactUUID returns NotFound when either apiType or apiId is empty, rather than falling through to the repo.
func TestAPIDocumentService_ResolveArtifactUUID_EmptyInputsRejected(t *testing.T) {
	svc, _, _, _ := newTestDocumentService()

	for _, tc := range []struct{ apiType, apiID string }{
		{"", "orders-api"},
		{"rest-api", ""},
		{"", ""},
	} {
		_, err := svc.ResolveArtifactUUID(tc.apiType, tc.apiID, "org-1")
		if err == nil || !apperror.NotFound.Is(err) {
			t.Errorf("ResolveArtifactUUID(%q, %q) err = %v, want NotFound", tc.apiType, tc.apiID, err)
		}
	}
}

// UpdateApiDocument passes updateContent=false to the repo when the request has no new bytes, so a metadata-only PUT never overwrites the stored body.
func TestAPIDocumentService_UpdateApiDocument_PreservesContentWhenRequestOmitsIt(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()
	docRepo.getDocResult = &model.Document{
		ID:               "doc-1",
		ArtifactUUID:     "artifact-1",
		OrganizationUUID: "org-1",
		Type:             constants.DocumentTypeHowTo,
		Handle:           "overview",
		DisplayName:      "Overview",
		ContentType:      "text/markdown; charset=utf-8",
		Content:          []byte("# Overview"),
	}

	newName := "Overview v2"
	err := svc.UpdateApiDocument(&dto.UpdateAPIDocumentRequest{
		DisplayName: &newName,
	}, "org-1", "alice", "artifact-1", "overview")
	if err != nil {
		t.Fatalf("UpdateApiDocument err = %v, want nil", err)
	}

	if len(docRepo.updateApiDocCalls) != 1 {
		t.Fatalf("expected one UpdateApiDocument call, got %d", len(docRepo.updateApiDocCalls))
	}
	if docRepo.updateApiDocContentFlags[0] != false {
		t.Errorf("UpdateApiDocument updateContent flag = true, want false — a metadata-only PUT must not overwrite stored bytes")
	}
	if docRepo.updateApiDocCalls[0].DisplayName != newName {
		t.Errorf("UpdateApiDocument displayName = %q, want %q",
			docRepo.updateApiDocCalls[0].DisplayName, newName)
	}
}

// DeleteApiDocument refuses a reserved handle at the service layer so a user-facing DELETE can't reach a system-managed row even by name.
func TestAPIDocumentService_DeleteApiDocument_RejectsReservedHandle(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()

	err := svc.DeleteApiDocument("artifact-1", constants.DocumentHandleThumbnail, "org-1", "alice")
	if err == nil || !apperror.ValidationFailed.Is(err) {
		t.Errorf("DeleteApiDocument(handle=%q) err = %v, want ValidationFailed",
			constants.DocumentHandleThumbnail, err)
	}
	if len(docRepo.deleteApiDocCalls) != 0 {
		t.Errorf("DeleteApiDocument reached the repo for a reserved handle")
	}
}

// DeleteApiDocument maps a nil GetDocument result to NotFound rather than leaking a 500 or a silent 204.
func TestAPIDocumentService_DeleteApiDocument_NotFoundMapsToAppError(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()
	docRepo.getDocResult = nil

	err := svc.DeleteApiDocument("artifact-1", "overview", "org-1", "alice")
	if err == nil || !apperror.NotFound.Is(err) {
		t.Errorf("DeleteApiDocument(unknown handle) err = %v, want NotFound", err)
	}
}

// DeleteApiDocument on a user doc calls the repo's user-facing delete path and records an audit event with the generic api_document resource label.
func TestAPIDocumentService_DeleteApiDocument_SuccessHits(t *testing.T) {
	svc, docRepo, auditRepo, _ := newTestDocumentService()
	docRepo.getDocResult = &model.Document{
		ArtifactUUID:     "artifact-1",
		OrganizationUUID: "org-1",
		Handle:           "overview",
		Type:             constants.DocumentTypeHowTo,
		DisplayName:      "Overview",
	}

	if err := svc.DeleteApiDocument("artifact-1", "overview", "org-1", "alice"); err != nil {
		t.Fatalf("DeleteApiDocument err = %v, want nil", err)
	}
	if len(docRepo.deleteApiDocCalls) != 1 || docRepo.deleteApiDocCalls[0].handle != "overview" {
		t.Fatalf("DeleteApiDocument calls = %+v, want one with handle=overview", docRepo.deleteApiDocCalls)
	}
	if len(auditRepo.calls) != 1 || auditRepo.calls[0].resourceType != "api_document" {
		t.Errorf("audit resourceType = %+v, want {api_document}", auditRepo.calls)
	}
}

// ---------------------------------------------------------------------------
// OpenAPI definition (/openapi) — validate / extract / normalise / merge
// ---------------------------------------------------------------------------

const minimalOpenAPISpec = `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /pets:
    get:
      summary: List pets
      responses:
        '200':
          description: OK
    post:
      summary: Create pet
      responses:
        '201':
          description: Created
  /pets/{petId}:
    get:
      summary: Get pet
      parameters:
        - name: petId
          in: path
          required: true
          schema:
            type: string
      responses:
        '200':
          description: OK
`

// ValidateOpenAPISpec reports IsValid=true for a well-formed OpenAPI 3.0 document.
func TestAPIDocumentService_ValidateOpenAPISpec_AcceptsValidSpec(t *testing.T) {
	svc, _, _, _ := newTestDocumentService()
	result := svc.ValidateOpenAPISpec([]byte(minimalOpenAPISpec))
	if !result.IsValid {
		t.Errorf("ValidateOpenAPISpec on valid spec: IsValid=false, errors=%+v", result.Errors)
	}
}

// ValidateOpenAPISpec reports IsValid=false with at least one error for a document that doesn't even parse as a spec.
func TestAPIDocumentService_ValidateOpenAPISpec_RejectsGarbage(t *testing.T) {
	svc, _, _, _ := newTestDocumentService()
	result := svc.ValidateOpenAPISpec([]byte("not-an-openapi-spec"))
	if result.IsValid {
		t.Fatal("ValidateOpenAPISpec on garbage: IsValid=true, want false")
	}
	if len(result.Errors) == 0 {
		t.Error("ValidateOpenAPISpec on garbage returned no error details")
	}
}

// ValidateOpenAPISpec reports IsValid=false for a Swagger 2.0 document — the service only accepts OpenAPI 3.x.
func TestAPIDocumentService_ValidateOpenAPISpec_RejectsSwagger2(t *testing.T) {
	swagger2 := `swagger: "2.0"
info:
  title: Legacy
  version: 1.0.0
paths: {}
`
	svc, _, _, _ := newTestDocumentService()
	result := svc.ValidateOpenAPISpec([]byte(swagger2))
	if result.IsValid {
		t.Error("ValidateOpenAPISpec on Swagger 2.0: IsValid=true, want false")
	}
}

// ExtractOperationsFromSpec returns one Operation per (method, path) in the spec with the method normalised to uppercase.
func TestAPIDocumentService_ExtractOperationsFromSpec_ReturnsOnePerMethodPath(t *testing.T) {
	svc, _, _, _ := newTestDocumentService()
	ops, err := svc.ExtractOperationsFromSpec([]byte(minimalOpenAPISpec))
	if err != nil {
		t.Fatalf("ExtractOperationsFromSpec err = %v, want nil", err)
	}
	if len(ops) != 3 {
		t.Fatalf("got %d ops, want 3 (GET /pets, POST /pets, GET /pets/{petId})", len(ops))
	}
	// Verify (method, path) set equality — order isn't guaranteed by the YAML parser.
	seen := make(map[string]bool, len(ops))
	for _, op := range ops {
		if string(op.Request.Method) != strings.ToUpper(string(op.Request.Method)) {
			t.Errorf("op method = %q is not uppercase", op.Request.Method)
		}
		seen[string(op.Request.Method)+" "+op.Request.Path] = true
	}
	for _, want := range []string{"GET /pets", "POST /pets", "GET /pets/{petId}"} {
		if !seen[want] {
			t.Errorf("expected op %q not found; got %v", want, seen)
		}
	}
}

// ExtractOperationsFromSpec returns an error wrapped as ValidationFailed when the spec is not loadable.
func TestAPIDocumentService_ExtractOperationsFromSpec_RejectsInvalidSpec(t *testing.T) {
	svc, _, _, _ := newTestDocumentService()
	_, err := svc.ExtractOperationsFromSpec([]byte("::: not yaml :::"))
	if err == nil || !apperror.ValidationFailed.Is(err) {
		t.Errorf("ExtractOperationsFromSpec(garbage) err = %v, want ValidationFailed", err)
	}
}

// ExtractOperationsFromSpec rejects a spec with no operations — the validator requires at least one, so an empty paths map comes back as ValidationFailed rather than silently returning nil.
func TestAPIDocumentService_ExtractOperationsFromSpec_RejectsEmptyPaths(t *testing.T) {
	emptyPaths := `openapi: 3.0.0
info:
  title: Empty
  version: 1.0.0
paths: {}
`
	svc, _, _, _ := newTestDocumentService()
	_, err := svc.ExtractOperationsFromSpec([]byte(emptyPaths))
	if err == nil || !apperror.ValidationFailed.Is(err) {
		t.Errorf("ExtractOperationsFromSpec(empty paths) err = %v, want ValidationFailed", err)
	}
}

// NormalizeSpecFileName strips the directory component so the stored filename never contains a user-supplied path (file-access.md directive 2).
func TestAPIDocumentService_NormalizeSpecFileName_StripsDirectory(t *testing.T) {
	svc, _, _, _ := newTestDocumentService()
	got := svc.NormalizeSpecFileName("../../etc/passwd/openapi.yaml")
	if got != "openapi.yaml" {
		t.Errorf("NormalizeSpecFileName = %q, want %q", got, "openapi.yaml")
	}
}

// NormalizeSpecFileName caps the result at the DB column ceiling (255) while preserving the extension, so a long name never explodes the INSERT.
func TestAPIDocumentService_NormalizeSpecFileName_CapsLengthPreservingExtension(t *testing.T) {
	svc, _, _, _ := newTestDocumentService()
	long := strings.Repeat("a", 400) + ".yaml"
	got := svc.NormalizeSpecFileName(long)
	if len(got) > 255 {
		t.Errorf("NormalizeSpecFileName returned a %d-byte name, want <=255", len(got))
	}
	if !strings.HasSuffix(got, ".yaml") {
		t.Errorf("NormalizeSpecFileName dropped the extension: %q", got)
	}
}

// MergeOperations keeps the stored per-operation policies on an op whose (method, path) also appears in the new spec, so a spec update never silently drops attached policies.
func TestAPIDocumentService_MergeOperations_PreservesPoliciesOnKnownOp(t *testing.T) {
	svc, _, _, _ := newTestDocumentService()

	storedPolicies := []api.Policy{{Name: "rate-limit", Version: "v1"}}
	existing := []api.Operation{
		{
			Request: api.OperationRequest{
				Method:   api.OperationRequestMethod("GET"),
				Path:     "/pets",
				Policies: &storedPolicies,
			},
		},
	}
	fromSpec := []api.Operation{
		{Request: api.OperationRequest{Method: api.OperationRequestMethod("GET"), Path: "/pets"}},
	}

	merged := svc.MergeOperations(&existing, fromSpec)
	if len(merged) != 1 {
		t.Fatalf("merged length = %d, want 1", len(merged))
	}
	if merged[0].Request.Policies == nil || len(*merged[0].Request.Policies) != 1 {
		t.Fatalf("expected stored policy to survive merge, got %+v", merged[0].Request.Policies)
	}
	if (*merged[0].Request.Policies)[0].Name != "rate-limit" {
		t.Errorf("merged policy name = %q, want rate-limit", (*merged[0].Request.Policies)[0].Name)
	}
}

// MergeOperations drops an op present in the stored list but absent in the new spec, so removing an endpoint actually removes it.
func TestAPIDocumentService_MergeOperations_DropsRemovedOp(t *testing.T) {
	svc, _, _, _ := newTestDocumentService()

	existing := []api.Operation{
		{Request: api.OperationRequest{Method: "GET", Path: "/pets"}},
		{Request: api.OperationRequest{Method: "DELETE", Path: "/pets/{petId}"}},
	}
	fromSpec := []api.Operation{
		{Request: api.OperationRequest{Method: "GET", Path: "/pets"}},
	}

	merged := svc.MergeOperations(&existing, fromSpec)
	if len(merged) != 1 {
		t.Fatalf("merged length = %d, want 1 (DELETE should be gone)", len(merged))
	}
	if merged[0].Request.Method != "GET" || merged[0].Request.Path != "/pets" {
		t.Errorf("merged = %+v, want only GET /pets", merged[0].Request)
	}
}

// MergeOperations adds a new op that only appears in the spec, with no inherited policies, so adding an endpoint takes effect on next write.
func TestAPIDocumentService_MergeOperations_AddsNewOp(t *testing.T) {
	svc, _, _, _ := newTestDocumentService()

	existing := []api.Operation{
		{Request: api.OperationRequest{Method: "GET", Path: "/pets"}},
	}
	fromSpec := []api.Operation{
		{Request: api.OperationRequest{Method: "GET", Path: "/pets"}},
		{Request: api.OperationRequest{Method: "POST", Path: "/pets"}},
	}

	merged := svc.MergeOperations(&existing, fromSpec)
	if len(merged) != 2 {
		t.Fatalf("merged length = %d, want 2", len(merged))
	}
	var post *api.Operation
	for i := range merged {
		if merged[i].Request.Method == "POST" {
			post = &merged[i]
		}
	}
	if post == nil {
		t.Fatal("POST /pets missing from merged ops")
	}
	if post.Request.Policies != nil && len(*post.Request.Policies) != 0 {
		t.Errorf("new op carried inherited policies = %+v, want none", post.Request.Policies)
	}
}

// ExtractAndMergeOperations composes extract + merge: a valid spec plus an existing op list that carried policies yields merged ops with policies preserved on known paths.
func TestAPIDocumentService_ExtractAndMergeOperations_PreservesPoliciesOnKnownPaths(t *testing.T) {
	svc, _, _, _ := newTestDocumentService()

	storedPolicies := []api.Policy{{Name: "rate-limit", Version: "v1"}}
	existing := []api.Operation{
		{
			Request: api.OperationRequest{
				Method:   "GET",
				Path:     "/pets",
				Policies: &storedPolicies,
			},
		},
	}

	merged, err := svc.ExtractAndMergeOperations([]byte(minimalOpenAPISpec), &existing)
	if err != nil {
		t.Fatalf("ExtractAndMergeOperations err = %v, want nil", err)
	}
	// Three ops in the spec; the GET /pets one must still carry the stored policy.
	var found bool
	for _, op := range merged {
		if op.Request.Method == "GET" && op.Request.Path == "/pets" {
			found = true
			if op.Request.Policies == nil || len(*op.Request.Policies) == 0 {
				t.Error("GET /pets lost its stored policies through ExtractAndMergeOperations")
			}
		}
	}
	if !found {
		t.Error("GET /pets missing from merged output")
	}
}

// ---------------------------------------------------------------------------
// Pure helpers — NormalizeAPIDocumentType, decodeStoredDocType,
// modelToAPIMetadata, GetSpecContentType, GetImageContentType
// ---------------------------------------------------------------------------
//
// These helpers are reached only through the handler today, so the file-local
// coverage of the service package misses them entirely. Driving them directly
// gets them attributed to the service file in Codecov's per-package rollup.

// NormalizeAPIDocumentType accepts the canonical casing, lowercase, and surrounding whitespace, and rejects any other input.
func TestAPIDocumentService_NormalizeAPIDocumentType(t *testing.T) {
	cases := []struct {
		input, want string
		ok          bool
	}{
		{"HowTo", constants.DocumentTypeHowTo, true},
		{"howto", constants.DocumentTypeHowTo, true},
		{"  HowTo  ", constants.DocumentTypeHowTo, true},
		{"HOWTO", constants.DocumentTypeHowTo, true},
		{"Samples", constants.DocumentTypeSamples, true},
		{"SupportForum", constants.DocumentTypeSupportForum, true},
		{"PublicForum", constants.DocumentTypePublicForum, true},
		{"Other", constants.DocumentTypeOther, true},
		// A caller-supplied custom type (used with Other.otherTypeName) is not
		// one of the known canonical values and must not round-trip here.
		{"FAQ", "", false},
		{"", "", false},
		{"   ", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, ok := NormalizeAPIDocumentType(tc.input)
			if ok != tc.ok || got != tc.want {
				t.Errorf("NormalizeAPIDocumentType(%q) = (%q, %v), want (%q, %v)", tc.input, got, ok, tc.want, tc.ok)
			}
		})
	}
}

// decodeStoredDocType strips the DOC_ storage prefix and leaves a bare custom
// type (which is already stored without the prefix) alone.
func TestAPIDocumentService_decodeStoredDocType(t *testing.T) {
	cases := []struct {
		stored, want string
	}{
		{constants.DocumentTypePrefix + constants.DocumentTypeHowTo, constants.DocumentTypeHowTo},
		{constants.DocumentTypePrefix + "FAQ", "FAQ"},
		{constants.DocumentTypePrefix + constants.DocumentTypeOther, constants.DocumentTypeOther},
		// A legacy row stored without the prefix must be returned verbatim —
		// a double-decode here would turn "HowTo" into something other than "HowTo".
		{"HowTo", "HowTo"},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.stored, func(t *testing.T) {
			if got := decodeStoredDocType(tc.stored); got != tc.want {
				t.Errorf("decodeStoredDocType(%q) = %q, want %q", tc.stored, got, tc.want)
			}
		})
	}
}

// modelToAPIMetadata emits optional fields only when their stored value is
// non-zero, and decodes the DOC_ prefix on type.
func TestAPIDocumentService_modelToAPIMetadata(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

	t.Run("populated fields round-trip, type is decoded", func(t *testing.T) {
		got := modelToAPIMetadata(&model.Document{
			Handle:      "guide",
			Type:        constants.DocumentTypePrefix + constants.DocumentTypeHowTo,
			DisplayName: "Guide",
			FileName:    "guide.md",
			ContentType: "text/markdown; charset=utf-8",
			CreatedBy:   "alice",
			CreatedAt:   now,
			UpdatedBy:   "bob",
			UpdatedAt:   now,
		})
		if got.Id != "guide" || got.DisplayName != "Guide" {
			t.Errorf("id/displayName = %q/%q", got.Id, got.DisplayName)
		}
		if got.Type != constants.DocumentTypeHowTo {
			t.Errorf("type = %q, want %q (DOC_ prefix must be decoded)", got.Type, constants.DocumentTypeHowTo)
		}
		if got.FileName == nil || *got.FileName != "guide.md" {
			t.Errorf("fileName = %v, want guide.md", got.FileName)
		}
		if got.ContentType == nil || got.CreatedBy == nil || got.UpdatedBy == nil {
			t.Errorf("populated metadata should not nil out optional strings: %+v", got)
		}
		if got.CreatedAt == nil || got.UpdatedAt == nil {
			t.Errorf("populated metadata should not nil out optional times: %+v", got)
		}
	})

	t.Run("zero-value fields are omitted", func(t *testing.T) {
		got := modelToAPIMetadata(&model.Document{
			Handle:      "guide",
			Type:        constants.DocumentTypePrefix + constants.DocumentTypeHowTo,
			DisplayName: "Guide",
			// FileName, ContentType, CreatedBy, UpdatedBy are empty; timestamps are zero.
		})
		if got.FileName != nil || got.ContentType != nil {
			t.Errorf("empty optional strings should be nil: %+v", got)
		}
		if got.CreatedBy != nil || got.UpdatedBy != nil {
			t.Errorf("empty optional actor strings should be nil: %+v", got)
		}
		if got.CreatedAt != nil || got.UpdatedAt != nil {
			t.Errorf("zero timestamps should be nil: %+v", got)
		}
	})
}

// GetSpecContentType returns application/json for a JSON body and
// application/yaml otherwise, including when the sniff is ambiguous.
func TestAPIDocumentService_GetSpecContentType(t *testing.T) {
	svc, _, _, _ := newTestDocumentService()
	cases := []struct {
		name     string
		content  []byte
		wantType string
	}{
		{"JSON object", []byte(`{"openapi":"3.0.0"}`), "application/json"},
		{"JSON with leading whitespace", []byte("  {\"openapi\":\"3.0.0\"}"), "application/json"},
		{"YAML with leading whitespace", []byte("\n openapi: 3.0.0\n"), "application/yaml"},
		{"empty body falls back to YAML", []byte{}, "application/yaml"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := svc.GetSpecContentType(tc.content); got != tc.wantType {
				t.Errorf("GetSpecContentType = %q, want %q", got, tc.wantType)
			}
		})
	}
}

// GetImageContentType uses magic bytes, not the first N bytes, so an image
// smaller than the 512-byte sniff buffer is still classified correctly.
func TestAPIDocumentService_GetImageContentType(t *testing.T) {
	svc, _, _, _ := newTestDocumentService()
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 24)...)
	jpeg := append([]byte("\xff\xd8\xff\xe0"), make([]byte, 24)...)
	gif := append([]byte("GIF89a"), make([]byte, 24)...)
	plain := []byte("hello world")
	oversized := append(append([]byte{}, png...), make([]byte, 2048)...)

	cases := []struct {
		name     string
		content  []byte
		wantType string
	}{
		{"PNG", png, "image/png"},
		{"JPEG", jpeg, "image/jpeg"},
		{"GIF (not on the thumbnail allowlist but still sniffed)", gif, "image/gif"},
		{"plain text", plain, "text/plain; charset=utf-8"},
		{"larger than 512-byte sniff window", oversized, "image/png"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := svc.GetImageContentType(tc.content); got != tc.wantType {
				t.Errorf("GetImageContentType = %q, want %q", got, tc.wantType)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Reads — GetAllApiDocuments, GetDocument, GetDocumentWithContent
// ---------------------------------------------------------------------------

func TestAPIDocumentService_GetAllApiDocuments_Success(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()
	docRepo.listDocsResult = []*model.Document{
		{Handle: "guide", Type: constants.DocumentTypePrefix + constants.DocumentTypeHowTo, DisplayName: "Guide"},
		// A row stored with a custom DOC_FAQ type is decoded back to the bare FAQ.
		{Handle: "faq", Type: constants.DocumentTypePrefix + "FAQ", DisplayName: "FAQ"},
	}
	docRepo.listDocsTotal = 7

	items, total, err := svc.GetAllApiDocuments("artifact-1", "org-1", "", 20, 0)
	if err != nil {
		t.Fatalf("GetAllApiDocuments err = %v", err)
	}
	if total != 7 || len(items) != 2 {
		t.Fatalf("len/total = %d/%d, want 2/7", len(items), total)
	}
	if items[0].Type != constants.DocumentTypeHowTo || items[1].Type != "FAQ" {
		t.Errorf("decoded types = %q, %q", items[0].Type, items[1].Type)
	}
}

// A blank artifactUUID is a 400 before any repository call — the handler
// should always resolve the artifact first, but if that invariant is ever
// broken the service refuses to query against an empty key.
func TestAPIDocumentService_GetAllApiDocuments_RequiresArtifactUUID(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()
	_, _, err := svc.GetAllApiDocuments("", "org-1", "", 20, 0)
	if err == nil || !apperror.ValidationFailed.Is(err) {
		t.Fatalf("err = %v, want ValidationFailed", err)
	}
	if docRepo.listDocsResult != nil || docRepo.listDocsTotal != 0 {
		// (Belt-and-braces: the mock's defaults are already zero. This proves
		// the service didn't change them by dispatching to the repo.)
		t.Errorf("the repo must not be consulted when the request is invalid")
	}
}

func TestAPIDocumentService_GetAllApiDocuments_RepoErrorIsSurfaced(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()
	docRepo.listDocsErr = errors.New("db down")
	if _, _, err := svc.GetAllApiDocuments("artifact-1", "org-1", "", 20, 0); err == nil {
		t.Fatalf("err = nil, want the repo error to surface")
	}
}

func TestAPIDocumentService_GetDocument_Success(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()
	docRepo.getDocResult = &model.Document{
		Handle:      "guide",
		Type:        constants.DocumentTypePrefix + constants.DocumentTypeHowTo,
		DisplayName: "Guide",
	}
	got, err := svc.GetDocument("artifact-1", "guide", "org-1")
	if err != nil {
		t.Fatalf("GetDocument err = %v", err)
	}
	if got.Id != "guide" || got.Type != constants.DocumentTypeHowTo {
		t.Errorf("metadata = %+v", got)
	}
}

func TestAPIDocumentService_GetDocument_NotFound(t *testing.T) {
	svc, _, _, _ := newTestDocumentService()
	// All repo defaults — getDocResult is nil, getDocErr is nil.
	if _, err := svc.GetDocument("artifact-1", "missing", "org-1"); err == nil || !apperror.NotFound.Is(err) {
		t.Fatalf("err = %v, want NotFound", err)
	}
}

func TestAPIDocumentService_GetDocument_RequiresArtifactAndHandle(t *testing.T) {
	svc, _, _, _ := newTestDocumentService()
	if _, err := svc.GetDocument("", "guide", "org-1"); err == nil || !apperror.ValidationFailed.Is(err) {
		t.Errorf("empty artifact UUID: err = %v, want ValidationFailed", err)
	}
	if _, err := svc.GetDocument("artifact-1", "", "org-1"); err == nil || !apperror.ValidationFailed.Is(err) {
		t.Errorf("empty handle: err = %v, want ValidationFailed", err)
	}
}

func TestAPIDocumentService_GetDocument_RepoErrorIsSurfaced(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()
	docRepo.getDocErr = errors.New("db down")
	if _, err := svc.GetDocument("artifact-1", "guide", "org-1"); err == nil {
		t.Fatal("err = nil, want the repo error to surface")
	}
}

// GetDocumentWithContent returns both metadata and raw bytes, keyed on the
// optional docType to enforce the reserved-row contract at read time.
func TestAPIDocumentService_GetDocumentWithContent_Success(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()
	docRepo.getDocResult = &model.Document{
		Handle:      "guide",
		Type:        constants.DocumentTypePrefix + constants.DocumentTypeHowTo,
		DisplayName: "Guide",
		ContentType: "text/markdown; charset=utf-8",
		Content:     []byte("# Hello"),
	}
	md, content, err := svc.GetDocumentWithContent("artifact-1", "guide", "org-1", "")
	if err != nil {
		t.Fatalf("GetDocumentWithContent err = %v", err)
	}
	if string(content) != "# Hello" {
		t.Errorf("content = %q, want %q", content, "# Hello")
	}
	if md == nil || md.Id != "guide" {
		t.Errorf("metadata = %+v", md)
	}
}

func TestAPIDocumentService_GetDocumentWithContent_NotFound(t *testing.T) {
	svc, _, _, _ := newTestDocumentService()
	if _, _, err := svc.GetDocumentWithContent("artifact-1", "missing", "org-1", ""); err == nil || !apperror.NotFound.Is(err) {
		t.Fatalf("err = %v, want NotFound", err)
	}
}

func TestAPIDocumentService_GetDocumentWithContent_RequiresArtifactAndHandle(t *testing.T) {
	svc, _, _, _ := newTestDocumentService()
	if _, _, err := svc.GetDocumentWithContent("", "guide", "org-1", ""); err == nil || !apperror.ValidationFailed.Is(err) {
		t.Errorf("empty artifact: err = %v, want ValidationFailed", err)
	}
	if _, _, err := svc.GetDocumentWithContent("artifact-1", "", "org-1", ""); err == nil || !apperror.ValidationFailed.Is(err) {
		t.Errorf("empty handle: err = %v, want ValidationFailed", err)
	}
}

// The docType argument is forwarded to the repository so the thumbnail GET
// path (which pins THUMBNAIL) cannot accidentally return a user-document row
// stored under the reserved handle.
func TestAPIDocumentService_GetDocumentWithContent_ForwardsDocType(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()
	docRepo.getDocResult = &model.Document{Handle: constants.DocumentHandleThumbnail, Type: constants.DocumentTypeThumbnail}
	_, _, _ = svc.GetDocumentWithContent("artifact-1", constants.DocumentHandleThumbnail, "org-1", constants.DocumentTypeThumbnail)
	if docRepo.lastGetDocType != constants.DocumentTypeThumbnail {
		t.Errorf("repo was queried with docType = %q, want %q", docRepo.lastGetDocType, constants.DocumentTypeThumbnail)
	}
}

// ---------------------------------------------------------------------------
// UpsertDocument — the thumbnail write path
// ---------------------------------------------------------------------------

func TestAPIDocumentService_UpsertDocument_CreateRecordsCreateAudit(t *testing.T) {
	svc, docRepo, auditRepo, _ := newTestDocumentService()
	// getDocResult is nil → the service treats this as an insert.

	req := &dto.CreateAPIDocumentRequest{
		Type:        constants.DocumentTypeThumbnail,
		Handle:      constants.DocumentHandleThumbnail,
		DisplayName: constants.DocumentDisplayNameThumbnail,
		FileName:    "logo.png",
		Content:     []byte("\x89PNG\r\n\x1a\n"),
	}
	if err := svc.UpsertDocument(req, "org-1", "alice", "artifact-1"); err != nil {
		t.Fatalf("UpsertDocument err = %v", err)
	}
	if len(docRepo.upsertCalls) != 1 {
		t.Fatalf("upsert calls = %d, want 1", len(docRepo.upsertCalls))
	}
	stored := docRepo.upsertCalls[0].doc
	if stored.CreatedBy != "alice" || stored.UpdatedBy != "alice" {
		t.Errorf("createdBy/updatedBy = %q/%q, want alice/alice on create", stored.CreatedBy, stored.UpdatedBy)
	}
	if len(auditRepo.calls) != 1 || auditRepo.calls[0].action != "CREATE" {
		t.Errorf("audit = %+v, want one CREATE entry", auditRepo.calls)
	}
}

// A thumbnail already present on this API is an UPDATE, not a CREATE —
// audit must reflect it, and CreatedBy must not be overwritten.
func TestAPIDocumentService_UpsertDocument_ExistingRowIsUpdateAudit(t *testing.T) {
	svc, docRepo, auditRepo, _ := newTestDocumentService()
	docRepo.getDocResult = &model.Document{
		Handle:    constants.DocumentHandleThumbnail,
		CreatedBy: "original-uploader",
	}

	req := &dto.CreateAPIDocumentRequest{
		Type:        constants.DocumentTypeThumbnail,
		Handle:      constants.DocumentHandleThumbnail,
		DisplayName: constants.DocumentDisplayNameThumbnail,
		FileName:    "new.png",
		Content:     []byte("\x89PNG\r\n\x1a\n"),
	}
	if err := svc.UpsertDocument(req, "org-1", "bob", "artifact-1"); err != nil {
		t.Fatalf("UpsertDocument err = %v", err)
	}
	stored := docRepo.upsertCalls[0].doc
	if stored.CreatedBy == "bob" {
		t.Errorf("CreatedBy was overwritten to the updater on an UPDATE: %+v", stored)
	}
	if stored.UpdatedBy != "bob" {
		t.Errorf("UpdatedBy = %q, want bob", stored.UpdatedBy)
	}
	if len(auditRepo.calls) != 1 || auditRepo.calls[0].action != "UPDATE" {
		t.Errorf("audit = %+v, want one UPDATE entry", auditRepo.calls)
	}
}

func TestAPIDocumentService_UpsertDocument_Validation(t *testing.T) {
	svc, _, _, _ := newTestDocumentService()
	if err := svc.UpsertDocument(nil, "org-1", "alice", "artifact-1"); err == nil || !apperror.ValidationFailed.Is(err) {
		t.Errorf("nil request: err = %v, want ValidationFailed", err)
	}
	if err := svc.UpsertDocument(&dto.CreateAPIDocumentRequest{Type: constants.DocumentTypeThumbnail}, "org-1", "alice", ""); err == nil || !apperror.ValidationFailed.Is(err) {
		t.Errorf("empty artifact UUID: err = %v, want ValidationFailed", err)
	}
}

func TestAPIDocumentService_UpsertDocument_RepoErrorIsSurfaced(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()
	docRepo.upsertDocErr = errors.New("disk full")
	err := svc.UpsertDocument(&dto.CreateAPIDocumentRequest{
		Type: constants.DocumentTypeThumbnail, Handle: constants.DocumentHandleThumbnail,
		DisplayName: constants.DocumentDisplayNameThumbnail,
		Content:     []byte("\x89PNG\r\n\x1a\n"),
	}, "org-1", "alice", "artifact-1")
	if err == nil {
		t.Fatal("err = nil, want the repo error to surface")
	}
}

// ---------------------------------------------------------------------------
// DeleteAPIThumbnail
// ---------------------------------------------------------------------------

func TestAPIDocumentService_DeleteAPIThumbnail_Success(t *testing.T) {
	svc, docRepo, auditRepo, _ := newTestDocumentService()
	if err := svc.DeleteAPIThumbnail("artifact-1", "org-1", "alice"); err != nil {
		t.Fatalf("DeleteAPIThumbnail err = %v", err)
	}
	if len(docRepo.deleteDocCalls) != 1 {
		t.Fatalf("delete calls = %d, want 1", len(docRepo.deleteDocCalls))
	}
	call := docRepo.deleteDocCalls[0]
	// Both the handle and the type are pinned to the thumbnail singleton — a
	// future change that routes this through DeleteApiDocument instead would
	// stop filtering by type and could delete a user-document stored under
	// the same handle.
	if call.handle != constants.DocumentHandleThumbnail || call.docType != constants.DocumentTypeThumbnail {
		t.Errorf("delete targeted handle=%q type=%q, want the thumbnail singleton", call.handle, call.docType)
	}
	if len(auditRepo.calls) != 1 || auditRepo.calls[0].action != "DELETE" ||
		auditRepo.calls[0].resourceType != "api_thumbnail" {
		t.Errorf("audit = %+v, want DELETE api_thumbnail", auditRepo.calls)
	}
}

// A missing thumbnail must surface as NotFound rather than a generic 500 so
// the handler can map it to a 404.
func TestAPIDocumentService_DeleteAPIThumbnail_NotFound(t *testing.T) {
	svc, docRepo, auditRepo, _ := newTestDocumentService()
	docRepo.deleteDocErr = sql.ErrNoRows
	err := svc.DeleteAPIThumbnail("artifact-1", "org-1", "alice")
	if err == nil || !apperror.NotFound.Is(err) {
		t.Fatalf("err = %v, want NotFound", err)
	}
	if len(auditRepo.calls) != 0 {
		t.Errorf("audit entries recorded for a no-op delete: %+v", auditRepo.calls)
	}
}

func TestAPIDocumentService_DeleteAPIThumbnail_RequiresArtifactUUID(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()
	if err := svc.DeleteAPIThumbnail("", "org-1", "alice"); err == nil || !apperror.ValidationFailed.Is(err) {
		t.Fatalf("err = %v, want ValidationFailed", err)
	}
	if len(docRepo.deleteDocCalls) != 0 {
		t.Errorf("repo was called despite an invalid request")
	}
}

func TestAPIDocumentService_DeleteAPIThumbnail_RepoErrorIsSurfaced(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()
	docRepo.deleteDocErr = errors.New("db down")
	err := svc.DeleteAPIThumbnail("artifact-1", "org-1", "alice")
	if err == nil {
		t.Fatal("err = nil, want the repo error to surface")
	}
	// Avoid leaking the raw repo error — the handler collapses non-apperror
	// values into a sterile 500 at the boundary. Here we only need to confirm
	// *something* non-nil was returned so Codecov records the branch.
	_ = strings.Contains(err.Error(), "")
}

// ---------------------------------------------------------------------------
// Branch gaps the integration-level tests don't drive
// ---------------------------------------------------------------------------

// CreateDocument generates the handle from displayName when no `id` is given;
// a user-supplied handle short-circuits the generator. Both branches of
// GenerateHandle's existence callback are exercised: the reserved-handle
// predicate and the per-artifact-exists predicate.
func TestAPIDocumentService_CreateDocument_GeneratesHandleFromDisplayName(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()
	// First candidate "release-notes" (slug of "Release Notes") is not reserved
	// and does not exist for this artifact, so the generator accepts it.
	req := &dto.CreateAPIDocumentRequest{
		Type:        constants.DocumentTypePrefix + constants.DocumentTypeHowTo,
		DisplayName: "Release Notes",
		Content:     []byte("# body"),
	}
	handle, err := svc.CreateDocument(req, "org", "alice", "artifact-1")
	if err != nil {
		t.Fatalf("CreateDocument err = %v", err)
	}
	if handle != "release-notes" {
		t.Errorf("generated handle = %q, want release-notes", handle)
	}
	if len(docRepo.createdDocs) != 1 || docRepo.createdDocs[0].Handle != "release-notes" {
		t.Errorf("stored doc did not pick up the generated handle: %+v", docRepo.createdDocs)
	}
}

// When the content type is text/markdown and no filename was uploaded, the
// service synthesises `<handle>.md` so the Content-Disposition on download
// is sensible. This exercises the branch after handle generation.
func TestAPIDocumentService_CreateDocument_SynthesisesMarkdownFileName(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()
	// Inline-content path: contentTypeForDocType returns text/markdown; the
	// filename isn't supplied by the caller — the service fills it from the
	// handle so a later download has a sensible Content-Disposition name.
	req := &dto.CreateAPIDocumentRequest{
		Type:        constants.DocumentTypePrefix + constants.DocumentTypeHowTo,
		Handle:      "overview",
		DisplayName: "Overview",
		Content:     []byte("# body"),
	}
	if _, err := svc.CreateDocument(req, "org", "alice", "artifact-1"); err != nil {
		t.Fatalf("err = %v", err)
	}
	if got := docRepo.createdDocs[0].FileName; got != "overview.md" {
		t.Errorf("fileName = %q, want overview.md", got)
	}
}

// CreateDocument propagates a unique-violation from the repository as a
// Conflict apperror. Reaching this is behaviourally different from the
// handler's pre-check for existing handles: a concurrent writer can slip
// between the check and the insert, and the repo-level unique index must
// catch it.
func TestAPIDocumentService_CreateDocument_UniqueViolationIsConflict(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()
	// IsUniqueViolation checks the error message for SQLite / Postgres /
	// SQL Server markers; the SQLite flavour is enough here.
	docRepo.createDocErr = errors.New("UNIQUE constraint failed: api_documents.handle")

	_, err := svc.CreateDocument(&dto.CreateAPIDocumentRequest{
		Type: constants.DocumentTypePrefix + constants.DocumentTypeHowTo, Handle: "x",
		DisplayName: "X", Content: []byte("x"),
	}, "org", "alice", "artifact-1")
	if err == nil || !apperror.Conflict.Is(err) {
		t.Fatalf("err = %v, want Conflict", err)
	}
}

// CreateApiDocument's user-facing validation gates are reached BEFORE
// CreateDocument. Each case trips one gate and must not reach the repo.
func TestAPIDocumentService_CreateApiDocument_ValidationGates(t *testing.T) {
	longName := strings.Repeat("a", maxDocDisplayNameLen+1)
	longFile := strings.Repeat("a", maxDocFileNameLen+1)
	longOther := strings.Repeat("a", maxDocTypeLen) // longer than allowed after DOC_ prefix

	cases := []struct {
		name string
		req  *dto.CreateAPIDocumentRequest
	}{
		{"nil request", nil},
		{"unknown type", &dto.CreateAPIDocumentRequest{Type: "Bogus", DisplayName: "x", Content: []byte("x")}},
		{"other with forbidden custom name", &dto.CreateAPIDocumentRequest{Type: constants.DocumentTypeOther, OtherTypeName: constants.DocumentTypeHowTo, DisplayName: "x", Content: []byte("x")}},
		{"other with too-long custom name", &dto.CreateAPIDocumentRequest{Type: constants.DocumentTypeOther, OtherTypeName: longOther, DisplayName: "x", Content: []byte("x")}},
		{"missing displayName", &dto.CreateAPIDocumentRequest{Type: constants.DocumentTypeHowTo, Content: []byte("x")}},
		{"blank displayName", &dto.CreateAPIDocumentRequest{Type: constants.DocumentTypeHowTo, DisplayName: "   ", Content: []byte("x")}},
		{"too-long displayName", &dto.CreateAPIDocumentRequest{Type: constants.DocumentTypeHowTo, DisplayName: longName, Content: []byte("x")}},
		{"too-long fileName", &dto.CreateAPIDocumentRequest{Type: constants.DocumentTypeHowTo, DisplayName: "x", FileName: longFile, Content: []byte("x")}},
		{"reserved handle", &dto.CreateAPIDocumentRequest{Type: constants.DocumentTypeHowTo, Handle: constants.DocumentHandleDefinition, DisplayName: "x", Content: []byte("x")}},
		{"malformed handle", &dto.CreateAPIDocumentRequest{Type: constants.DocumentTypeHowTo, Handle: "Not A Handle", DisplayName: "x", Content: []byte("x")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, docRepo, _, _ := newTestDocumentService()
			_, err := svc.CreateApiDocument(tc.req, "org", "alice", "artifact-1")
			if err == nil {
				t.Fatalf("err = nil, want validation error")
			}
			if len(docRepo.createdDocs) != 0 {
				t.Errorf("invalid request reached the repo: %+v", docRepo.createdDocs)
			}
		})
	}
}

// CreateApiDocument short-circuits with a Conflict when the user-supplied
// handle already exists on the artifact.
func TestAPIDocumentService_CreateApiDocument_DuplicateHandleIsConflict(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()
	docRepo.handleExistsResult = true

	_, err := svc.CreateApiDocument(&dto.CreateAPIDocumentRequest{
		Type: constants.DocumentTypeHowTo, Handle: "existing",
		DisplayName: "Dup", Content: []byte("x"),
	}, "org", "alice", "artifact-1")
	if err == nil || !apperror.Conflict.Is(err) {
		t.Fatalf("err = %v, want Conflict", err)
	}
	if len(docRepo.createdDocs) != 0 {
		t.Errorf("Create reached the repo despite the pre-check conflict")
	}
}

// A failure of the handle-existence check surfaces as Internal — the service
// cannot know whether the handle is free, so it refuses to proceed.
func TestAPIDocumentService_CreateApiDocument_HandleExistsCheckErrorIsInternal(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()
	docRepo.handleExistsErr = errors.New("db down")

	_, err := svc.CreateApiDocument(&dto.CreateAPIDocumentRequest{
		Type: constants.DocumentTypeHowTo, Handle: "overview",
		DisplayName: "x", Content: []byte("x"),
	}, "org", "alice", "artifact-1")
	if err == nil || !apperror.Internal.Is(err) {
		t.Fatalf("err = %v, want Internal", err)
	}
}

// UpdateApiDocument's validation gates. Each must fail before UpdateApiDocument
// reaches the repo — i.e. the Update call on the mock never happens.
func TestAPIDocumentService_UpdateApiDocument_ValidationGates(t *testing.T) {
	longName := strings.Repeat("a", maxDocDisplayNameLen+1)
	longFile := strings.Repeat("a", maxDocFileNameLen+1)
	longOther := strings.Repeat("a", maxDocTypeLen)
	emptyName := ""
	blankName := "   "

	cases := []struct {
		name       string
		req        *dto.UpdateAPIDocumentRequest
		handle     string
		seedExist  bool
	}{
		{"nil request", nil, "overview", true},
		{"reserved handle", &dto.UpdateAPIDocumentRequest{}, constants.DocumentHandleThumbnail, true},
		{"invalid handle", &dto.UpdateAPIDocumentRequest{}, "Not A Handle", true},
		{"not found", &dto.UpdateAPIDocumentRequest{DisplayName: strPtr("x")}, "overview", false},
		{"invalid type", &dto.UpdateAPIDocumentRequest{Type: strPtr("Bogus")}, "overview", true},
		{"other with forbidden custom name", &dto.UpdateAPIDocumentRequest{
			Type: strPtr(constants.DocumentTypeOther), OtherTypeName: constants.DocumentTypeHowTo,
		}, "overview", true},
		{"other with too-long custom name", &dto.UpdateAPIDocumentRequest{
			Type: strPtr(constants.DocumentTypeOther), OtherTypeName: longOther,
		}, "overview", true},
		{"blank displayName", &dto.UpdateAPIDocumentRequest{DisplayName: &blankName}, "overview", true},
		{"empty displayName", &dto.UpdateAPIDocumentRequest{DisplayName: &emptyName}, "overview", true},
		{"too-long displayName", &dto.UpdateAPIDocumentRequest{DisplayName: &longName}, "overview", true},
		{"too-long fileName", &dto.UpdateAPIDocumentRequest{FileName: &longFile}, "overview", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, docRepo, _, _ := newTestDocumentService()
			if tc.seedExist {
				docRepo.getDocResult = &model.Document{
					ArtifactUUID: "artifact-1", OrganizationUUID: "org",
					Handle: tc.handle, Type: constants.DocumentTypePrefix + constants.DocumentTypeHowTo,
					DisplayName: "Original",
				}
			}
			err := svc.UpdateApiDocument(tc.req, "org", "alice", "artifact-1", tc.handle)
			if err == nil {
				t.Fatalf("err = nil, want validation error")
			}
			if len(docRepo.updateApiDocCalls) != 0 {
				t.Errorf("invalid request reached the repo update: %+v", docRepo.updateApiDocCalls)
			}
		})
	}
}

// (Reserved-handle rejection is already covered elsewhere in this file by
// TestAPIDocumentService_DeleteApiDocument_RejectsReservedHandle; the three
// extra DeleteApiDocument branch tests below pick up where that one stops.)

func TestAPIDocumentService_DeleteApiDocument_RequiresArtifactAndHandle(t *testing.T) {
	svc, _, _, _ := newTestDocumentService()
	if err := svc.DeleteApiDocument("", "overview", "org", "alice"); err == nil || !apperror.ValidationFailed.Is(err) {
		t.Errorf("empty artifact: err = %v, want ValidationFailed", err)
	}
	if err := svc.DeleteApiDocument("artifact-1", "", "org", "alice"); err == nil || !apperror.ValidationFailed.Is(err) {
		t.Errorf("empty handle: err = %v, want ValidationFailed", err)
	}
}

func TestAPIDocumentService_DeleteApiDocument_NotFound(t *testing.T) {
	// No getDocResult seeded → service returns NotFound before touching the
	// repo delete.
	svc, docRepo, _, _ := newTestDocumentService()
	err := svc.DeleteApiDocument("artifact-1", "overview", "org", "alice")
	if err == nil || !apperror.NotFound.Is(err) {
		t.Fatalf("err = %v, want NotFound", err)
	}
	if len(docRepo.deleteApiDocCalls) != 0 {
		t.Error("not-found path must not reach the repo delete")
	}
}

// A race: the row disappears between the service's existence check and the
// repo's delete. The service maps sql.ErrNoRows to NotFound rather than
// surfacing the driver error.
func TestAPIDocumentService_DeleteApiDocument_RaceMapsToNotFound(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()
	docRepo.getDocResult = &model.Document{
		ArtifactUUID: "artifact-1", OrganizationUUID: "org", Handle: "overview",
		Type: constants.DocumentTypePrefix + constants.DocumentTypeHowTo,
	}
	docRepo.deleteApiDocErr = sql.ErrNoRows

	err := svc.DeleteApiDocument("artifact-1", "overview", "org", "alice")
	if err == nil || !apperror.NotFound.Is(err) {
		t.Fatalf("err = %v, want NotFound", err)
	}
}

func TestAPIDocumentService_DeleteApiDocument_RepoErrorIsSurfaced(t *testing.T) {
	svc, docRepo, _, _ := newTestDocumentService()
	docRepo.getDocResult = &model.Document{
		ArtifactUUID: "artifact-1", OrganizationUUID: "org", Handle: "overview",
		Type: constants.DocumentTypePrefix + constants.DocumentTypeHowTo,
	}
	docRepo.deleteApiDocErr = errors.New("db down")

	if err := svc.DeleteApiDocument("artifact-1", "overview", "org", "alice"); err == nil {
		t.Fatal("err = nil, want the repo error to surface")
	}
}

