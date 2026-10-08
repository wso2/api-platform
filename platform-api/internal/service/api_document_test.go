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
	"log/slog"
	"strings"
	"testing"

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
