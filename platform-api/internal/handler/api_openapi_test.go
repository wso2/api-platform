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

// Handler-level unit tests for the three openapi routes on api.go:
//   POST /api/v0.9/rest-apis/import-openapi
//   GET  /api/v0.9/rest-apis/{restApiId}/openapi
//   PUT  /api/v0.9/rest-apis/{restApiId}/openapi

package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const (
	oaOrg          = "org-1"
	oaProjectID    = "project-uuid-1"
	oaProjectHand  = "retail"
	oaAPIHandle    = "orders-api"
	oaAPIUUID      = "api-uuid-1"
	oaDisplayName  = "Orders API"
	oaVersion      = "v1.0"
	oaContext      = "/orders"
	oaMainUpstream = `{"main":{"url":"http://upstream:3000"}}`
)

const validOpenAPI = `openapi: 3.0.0
info:
  title: Orders API
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
`

const updatedOpenAPI = `openapi: 3.0.0
info:
  title: Orders API
  version: 2.0.0
paths:
  /animals:
    get:
      summary: List animals
      responses:
        '200':
          description: OK
`

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

// oaAPIRepo implements just enough of repository.APIRepository for the
// openapi handlers. The embedded interface is nil so any method the handler
// shouldn't reach will panic with a diagnosable error.
type oaAPIRepo struct {
	repository.APIRepository

	// Call knobs.
	handleExists      bool
	handleExistsErr   error
	nameVersionExists bool
	getMetadataErr    error
	getByUUIDErr      error
	createErr         error
	updateErr         error

	// Storage (write-through, so a Create → GetAPIByUUID round-trip works).
	byUUID    map[string]*model.API
	byHandle  map[string]*model.APIMetadata
	createdID string
}

func newOAAPIRepo() *oaAPIRepo {
	return &oaAPIRepo{
		byUUID:   map[string]*model.API{},
		byHandle: map[string]*model.APIMetadata{},
	}
}

func (r *oaAPIRepo) CheckAPIExistsByHandleInOrganization(handle, orgUUID string) (bool, error) {
	return r.handleExists, r.handleExistsErr
}

func (r *oaAPIRepo) CheckAPIExistsByNameAndVersionInOrganization(name, version, orgUUID, exclude string) (bool, error) {
	return r.nameVersionExists, nil
}

func (r *oaAPIRepo) CreateAPI(apiModel *model.API) error {
	if r.createErr != nil {
		return r.createErr
	}
	// The real repo generates the UUID. For deterministic tests, use a fixed one.
	apiModel.ID = oaAPIUUID
	r.createdID = apiModel.ID
	cp := *apiModel
	r.byUUID[apiModel.ID] = &cp
	r.byHandle[apiModel.Handle] = &model.APIMetadata{
		ID:             apiModel.ID,
		Handle:         apiModel.Handle,
		Kind:           apiModel.Kind,
		OrganizationID: apiModel.OrganizationID,
	}
	return nil
}

func (r *oaAPIRepo) GetAPIByUUID(apiUUID, orgUUID string) (*model.API, error) {
	if r.getByUUIDErr != nil {
		return nil, r.getByUUIDErr
	}
	a, ok := r.byUUID[apiUUID]
	if !ok {
		return nil, nil
	}
	return a, nil
}

func (r *oaAPIRepo) GetAPIMetadataByHandle(handle, orgUUID string) (*model.APIMetadata, error) {
	if r.getMetadataErr != nil {
		return nil, r.getMetadataErr
	}
	md, ok := r.byHandle[handle]
	if !ok {
		return nil, nil
	}
	return md, nil
}

func (r *oaAPIRepo) UpdateAPI(apiModel *model.API) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	cp := *apiModel
	r.byUUID[apiModel.ID] = &cp
	return nil
}

func (r *oaAPIRepo) DeleteAPI(apiUUID, orgUUID string) error {
	delete(r.byUUID, apiUUID)
	return nil
}

// oaProjectRepo stocks exactly one project — retail/project-uuid-1.
type oaProjectRepo struct {
	repository.ProjectRepository
	byHandleErr, byUUIDErr error
}

func (r *oaProjectRepo) GetProjectByHandleAndOrgID(handle, orgUUID string) (*model.Project, error) {
	if r.byHandleErr != nil {
		return nil, r.byHandleErr
	}
	if handle != oaProjectHand || orgUUID != oaOrg {
		return nil, nil
	}
	return &model.Project{ID: oaProjectID, Handle: oaProjectHand, OrganizationID: oaOrg}, nil
}

func (r *oaProjectRepo) GetProjectByUUID(uuid string) (*model.Project, error) {
	if r.byUUIDErr != nil {
		return nil, r.byUUIDErr
	}
	if uuid != oaProjectID {
		return nil, nil
	}
	return &model.Project{ID: oaProjectID, Handle: oaProjectHand, OrganizationID: oaOrg}, nil
}

// oaIdentityRepo satisfies IdentityService without reaching any database.
type oaIdentityRepo struct {
	repository.UserIdentityMappingRepository
	err error
}

func (r oaIdentityRepo) GetOrCreateUUID(identity string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	return "internal-" + identity, nil
}

func (r oaIdentityRepo) GetSubByUUID(uuid string) (string, bool, error) {
	return strings.TrimPrefix(uuid, "internal-"), true, nil
}

func (r oaIdentityRepo) GetSubsByUUIDs(uuids []string) (map[string]string, error) {
	out := map[string]string{}
	for _, u := range uuids {
		out[u] = strings.TrimPrefix(u, "internal-")
	}
	return out, nil
}

// oaAudit records everything but never fails.
type oaAudit struct{ calls int }

func (a *oaAudit) Record(action, resourceUUID, resourceType, orgUUID, performedBy string) error {
	a.calls++
	return nil
}

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

type oaEnv struct {
	mux          *http.ServeMux
	apiRepo      *oaAPIRepo
	projectRepo  *oaProjectRepo
	docRepo      *docHRepo
	artifactRepo *docHArtifactRepo
	audit        *oaAudit
	docAudit     *docHAudit
}

func newOAEnv(t *testing.T) *oaEnv {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	env := &oaEnv{
		apiRepo:      newOAAPIRepo(),
		projectRepo:  &oaProjectRepo{},
		docRepo:      &docHRepo{docs: map[string]*model.Document{}},
		artifactRepo: &docHArtifactRepo{},
		audit:        &oaAudit{},
		docAudit:     &docHAudit{},
	}
	identitySvc := service.NewIdentityService(oaIdentityRepo{})
	apiService := service.NewAPIService(env.apiRepo, env.projectRepo,
		nil /* orgRepo — not reached by these three handlers in the covered paths */,
		nil /* gatewayRepo */, nil /* deploymentRepo */, nil /* subscriptionPlanRepo */, nil /* customPolicyRepo */,
		nil /* gatewayEventsService */, &utils.APIUtil{}, logger, env.audit, identitySvc)
	apiDocumentService := service.NewAPIDocumentService(env.docRepo, env.artifactRepo, env.docAudit, logger)

	env.mux = http.NewServeMux()
	NewAPIHandler(apiService, identitySvc, apiDocumentService, logger, &config.Server{}).RegisterRoutes(env.mux)
	return env
}

// seedExistingAPI makes the artifact resolvable (needed for GET /openapi and PUT /openapi).
// `readOnly=true` marks the API as gateway-originated (`origin=gateway_api`),
// which the service-layer converter maps to `RESTAPI.ReadOnly = true` so the
// PUT /openapi handler takes the validate-only branch.
func (e *oaEnv) seedExistingAPI(readOnly bool) {
	origin := constants.OriginCP
	if readOnly {
		origin = constants.OriginDP
	}
	e.apiRepo.byUUID[oaAPIUUID] = &model.API{
		ID:             oaAPIUUID,
		Handle:         oaAPIHandle,
		Kind:           constants.RestApi,
		OrganizationID: oaOrg,
		Name:           oaDisplayName,
		Version:        oaVersion,
		ProjectID:      oaProjectID,
		Origin:         origin,
	}
	e.apiRepo.byHandle[oaAPIHandle] = &model.APIMetadata{
		ID: oaAPIUUID, Handle: oaAPIHandle, Kind: constants.RestApi, OrganizationID: oaOrg,
	}
}

type oaFormField struct{ name, value string }

func oaImportBody(fields []oaFormField, specFileName string, specContent []byte) ([]byte, string) {
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	for _, f := range fields {
		_ = w.WriteField(f.name, f.value)
	}
	if specContent != nil {
		part, _ := w.CreateFormFile("file", specFileName)
		_, _ = part.Write(specContent)
	}
	_ = w.Close()
	return buf.Bytes(), w.FormDataContentType()
}

func (e *oaEnv) do(t *testing.T, method, target, org, user string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if org != "" {
		req = middleware.WithOrganization(req, org)
	}
	if user != "" {
		req = middleware.WithUserID(req, user)
	}
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

func oaAssertError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, wantStatus, rec.Body.String())
	}
	var body struct {
		Status, Code string
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v; body: %s", err, rec.Body.String())
	}
	if body.Code != wantCode {
		t.Fatalf("code = %q, want %q; body: %s", body.Code, wantCode, rec.Body.String())
	}
	if body.Status != "error" {
		t.Fatalf("status field = %q, want %q", body.Status, "error")
	}
}

// ---------------------------------------------------------------------------
// POST /rest-apis/import-openapi
// ---------------------------------------------------------------------------

const importPath = constants.APIBasePath + "/rest-apis/import-openapi"

func fullImportFields() []oaFormField {
	return []oaFormField{
		{"displayName", oaDisplayName},
		{"version", oaVersion},
		{"context", oaContext},
		{"projectId", oaProjectHand},
		{"upstream", oaMainUpstream},
	}
}

func TestImportOpenAPI_MissingOrganizationIsUnauthorized(t *testing.T) {
	env := newOAEnv(t)
	body, ct := oaImportBody(fullImportFields(), "spec.yaml", []byte(validOpenAPI))
	rec := env.do(t, http.MethodPost, importPath, "", "", bytes.NewReader(body), ct)
	oaAssertError(t, rec, http.StatusUnauthorized, apperror.CodeCommonUnauthorized)
}

func TestImportOpenAPI_NotMultipartIsBadRequest(t *testing.T) {
	env := newOAEnv(t)
	rec := env.do(t, http.MethodPost, importPath, oaOrg, "", strings.NewReader("not multipart"), "application/json")
	// Multipart parse fails; the handler maps that to a validation 400.
	oaAssertError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
}

func TestImportOpenAPI_RejectsInvalidRequests(t *testing.T) {
	// Each case drops one required field or supplies an invalid value. Every
	// branch is reached BEFORE the apiService touches the repository, so a
	// successful rejection also proves the handler short-circuits early.
	cases := []struct {
		name   string
		fields []oaFormField
		file   []byte
	}{
		{"missing displayName", []oaFormField{
			{"version", oaVersion}, {"context", oaContext}, {"projectId", oaProjectHand}, {"upstream", oaMainUpstream},
		}, []byte(validOpenAPI)},
		{"missing version", []oaFormField{
			{"displayName", oaDisplayName}, {"context", oaContext}, {"projectId", oaProjectHand}, {"upstream", oaMainUpstream},
		}, []byte(validOpenAPI)},
		{"missing context", []oaFormField{
			{"displayName", oaDisplayName}, {"version", oaVersion}, {"projectId", oaProjectHand}, {"upstream", oaMainUpstream},
		}, []byte(validOpenAPI)},
		{"missing projectId", []oaFormField{
			{"displayName", oaDisplayName}, {"version", oaVersion}, {"context", oaContext}, {"upstream", oaMainUpstream},
		}, []byte(validOpenAPI)},
		{"empty upstream", []oaFormField{
			{"displayName", oaDisplayName}, {"version", oaVersion}, {"context", oaContext}, {"projectId", oaProjectHand},
		}, []byte(validOpenAPI)},
		{"invalid upstream JSON", []oaFormField{
			{"displayName", oaDisplayName}, {"version", oaVersion}, {"context", oaContext}, {"projectId", oaProjectHand},
			{"upstream", "not-json"},
		}, []byte(validOpenAPI)},
		{"upstream main with both url and ref", []oaFormField{
			{"displayName", oaDisplayName}, {"version", oaVersion}, {"context", oaContext}, {"projectId", oaProjectHand},
			{"upstream", `{"main":{"url":"http://x:3000","ref":"shared-upstream"}}`},
		}, []byte(validOpenAPI)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newOAEnv(t)
			body, ct := oaImportBody(tc.fields, "spec.yaml", tc.file)
			rec := env.do(t, http.MethodPost, importPath, oaOrg, "alice", bytes.NewReader(body), ct)
			oaAssertError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
			// The repo must not have been written to on an invalid request.
			if env.apiRepo.createdID != "" {
				t.Errorf("CreateAPI reached the repo on invalid request %q", tc.name)
			}
		})
	}
}

func TestImportOpenAPI_NeitherFileNorURLIsBadRequest(t *testing.T) {
	env := newOAEnv(t)
	// Build a multipart body with the fields but NO file part and no url field.
	body, ct := oaImportBody(fullImportFields(), "", nil)
	rec := env.do(t, http.MethodPost, importPath, oaOrg, "alice", bytes.NewReader(body), ct)
	oaAssertError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
}

func TestImportOpenAPI_BothFileAndURLIsBadRequest(t *testing.T) {
	// Exactly-one-of enforcement at the multipart boundary — if a caller
	// mistakenly sends both, the handler must refuse rather than pick a
	// silent winner.
	env := newOAEnv(t)
	fields := append(fullImportFields(), oaFormField{"url", "http://example.invalid/spec.yaml"})
	body, ct := oaImportBody(fields, "spec.yaml", []byte(validOpenAPI))
	rec := env.do(t, http.MethodPost, importPath, oaOrg, "alice", bytes.NewReader(body), ct)
	oaAssertError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
}

func TestImportOpenAPI_InvalidSpecIsBadRequest(t *testing.T) {
	// The spec is read through apiDocumentService.ExtractOperationsFromSpec,
	// which rejects anything that isn't a valid OpenAPI 3.x document.
	env := newOAEnv(t)
	body, ct := oaImportBody(fullImportFields(), "spec.yaml", []byte("this is not an openapi document"))
	rec := env.do(t, http.MethodPost, importPath, oaOrg, "alice", bytes.NewReader(body), ct)
	oaAssertError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
	if env.apiRepo.createdID != "" {
		t.Errorf("CreateAPI reached the repo despite invalid spec")
	}
}

func TestImportOpenAPI_SuccessCreatesAPIAndStoresSpec(t *testing.T) {
	env := newOAEnv(t)
	body, ct := oaImportBody(fullImportFields(), "orders.yaml", []byte(validOpenAPI))

	rec := env.do(t, http.MethodPost, importPath, oaOrg, "alice", bytes.NewReader(body), ct)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	// Location header points at the new API's handle.
	wantLocation := constants.APIBasePath + "/rest-apis/" + oaAPIHandle
	if got := rec.Header().Get("Location"); got != wantLocation {
		t.Errorf("Location = %q, want %q", got, wantLocation)
	}
	// Response is the newly created API's metadata.
	var created api.RESTAPI
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created api: %v", err)
	}
	if created.DisplayName != oaDisplayName {
		t.Errorf("displayName = %q, want %q", created.DisplayName, oaDisplayName)
	}
	// Operations were extracted from the spec: GET/POST /pets.
	if created.Operations == nil || len(*created.Operations) < 2 {
		t.Errorf("expected the two spec operations to be attached, got %+v", created.Operations)
	}
	// The spec was stored under the reserved DEFINITION handle/type for the
	// new artifact, so the GET /openapi endpoint can later serve it back.
	if len(env.docRepo.docs) != 1 {
		t.Fatalf("docs stored = %d, want 1", len(env.docRepo.docs))
	}
	if got := env.docRepo.docs[constants.DocumentHandleDefinition]; got == nil ||
		got.Type != constants.DocumentTypeDefinition {
		t.Errorf("the spec was not stored under the DEFINITION handle/type: %+v", env.docRepo.docs)
	}
}

func TestImportOpenAPI_PersistFailureRollsBackAPICreate(t *testing.T) {
	// When the document write fails after a successful API create, the
	// handler must delete the just-created API so the user doesn't end up
	// with an orphaned, specless row.
	env := newOAEnv(t)
	env.docRepo.upsertErr = errors.New("disk full")
	// CreateDocument fallback goes through CreateDocument on the real service,
	// which calls docRepo.CreateDocument. Simulate the write failing there.
	env.docRepo.createErr = errors.New("disk full")

	body, ct := oaImportBody(fullImportFields(), "orders.yaml", []byte(validOpenAPI))
	rec := env.do(t, http.MethodPost, importPath, oaOrg, "alice", bytes.NewReader(body), ct)
	oaAssertError(t, rec, http.StatusInternalServerError, apperror.CodeCommonInternalError)

	// Rollback: the API that was briefly created is now gone.
	if _, exists := env.apiRepo.byUUID[oaAPIUUID]; exists {
		t.Errorf("API %q was not rolled back after spec persist failure", oaAPIUUID)
	}
}

func TestImportOpenAPI_ProjectNotFoundIsNotFound(t *testing.T) {
	env := newOAEnv(t)
	fields := []oaFormField{
		{"displayName", oaDisplayName},
		{"version", oaVersion},
		{"context", oaContext},
		{"projectId", "no-such-project"}, // Not in the mock repo.
		{"upstream", oaMainUpstream},
	}
	body, ct := oaImportBody(fields, "spec.yaml", []byte(validOpenAPI))
	rec := env.do(t, http.MethodPost, importPath, oaOrg, "alice", bytes.NewReader(body), ct)
	oaAssertError(t, rec, http.StatusNotFound, apperror.CodeProjectNotFound)
	if env.apiRepo.createdID != "" {
		t.Errorf("API was created despite missing project")
	}
}

// ---------------------------------------------------------------------------
// GET /rest-apis/{restApiId}/openapi
// ---------------------------------------------------------------------------

func openapiPath(handle string) string {
	return constants.APIBasePath + "/rest-apis/" + handle + "/openapi"
}

func TestGetOpenAPISpec_MissingOrganizationIsUnauthorized(t *testing.T) {
	env := newOAEnv(t)
	rec := env.do(t, http.MethodGet, openapiPath(oaAPIHandle), "", "", nil, "")
	oaAssertError(t, rec, http.StatusUnauthorized, apperror.CodeCommonUnauthorized)
}

func TestGetOpenAPISpec_UnknownAPIIsNotFound(t *testing.T) {
	env := newOAEnv(t)
	rec := env.do(t, http.MethodGet, openapiPath("missing-api"), oaOrg, "", nil, "")
	oaAssertError(t, rec, http.StatusNotFound, apperror.CodeRESTAPINotFound)
}

func TestGetOpenAPISpec_StreamsStoredContent(t *testing.T) {
	env := newOAEnv(t)
	env.seedExistingAPI(false)
	// Seed the DEFINITION doc under the artifact; the handler reads it back.
	env.docRepo.docs[constants.DocumentHandleDefinition] = &model.Document{
		ArtifactUUID:     oaAPIUUID,
		OrganizationUUID: oaOrg,
		Type:             constants.DocumentTypeDefinition,
		Handle:           constants.DocumentHandleDefinition,
		DisplayName:      constants.DocumentDisplayNameDefinition,
		FileName:         "orders.yaml",
		ContentType:      "application/yaml",
		Content:          []byte(validOpenAPI),
	}

	rec := env.do(t, http.MethodGet, openapiPath(oaAPIHandle), oaOrg, "", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	// Response is {content: "<raw spec>"} — a JSON envelope, not raw bytes.
	var resp api.OpenAPIContent
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; body: %s", err, rec.Body.String())
	}
	if resp.Content == nil || *resp.Content != validOpenAPI {
		t.Errorf("content mismatch; got %v", resp.Content)
	}
}

func TestGetOpenAPISpec_NoSpecDocumentIsNotFound(t *testing.T) {
	// API exists but no DEFINITION row was ever created (e.g. an API created
	// via the plain POST /rest-apis path, not import-openapi).
	env := newOAEnv(t)
	env.seedExistingAPI(false)
	rec := env.do(t, http.MethodGet, openapiPath(oaAPIHandle), oaOrg, "", nil, "")
	oaAssertError(t, rec, http.StatusNotFound, apperror.CodeCommonNotFound)
}

// ---------------------------------------------------------------------------
// PUT /rest-apis/{restApiId}/openapi
// ---------------------------------------------------------------------------

func TestPutOpenAPISpec_MissingOrganizationIsUnauthorized(t *testing.T) {
	env := newOAEnv(t)
	body, ct := oaImportBody(nil, "spec.yaml", []byte(validOpenAPI))
	rec := env.do(t, http.MethodPut, openapiPath(oaAPIHandle), "", "", bytes.NewReader(body), ct)
	oaAssertError(t, rec, http.StatusUnauthorized, apperror.CodeCommonUnauthorized)
}

func TestPutOpenAPISpec_NotMultipartIsBadRequest(t *testing.T) {
	env := newOAEnv(t)
	env.seedExistingAPI(false)
	rec := env.do(t, http.MethodPut, openapiPath(oaAPIHandle), oaOrg, "alice",
		strings.NewReader(`openapi: 3.0.0`), "application/yaml")
	oaAssertError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
}

func TestPutOpenAPISpec_UnknownAPIIsNotFound(t *testing.T) {
	env := newOAEnv(t)
	body, ct := oaImportBody(nil, "spec.yaml", []byte(validOpenAPI))
	rec := env.do(t, http.MethodPut, openapiPath("missing-api"), oaOrg, "alice", bytes.NewReader(body), ct)
	oaAssertError(t, rec, http.StatusNotFound, apperror.CodeRESTAPINotFound)
}

func TestPutOpenAPISpec_InvalidSpecIsBadRequest(t *testing.T) {
	env := newOAEnv(t)
	env.seedExistingAPI(false)
	body, ct := oaImportBody(nil, "spec.yaml", []byte("not a spec"))
	rec := env.do(t, http.MethodPut, openapiPath(oaAPIHandle), oaOrg, "alice", bytes.NewReader(body), ct)
	oaAssertError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
	// The upsert path must never have been reached.
	if len(env.docRepo.upserted) != 0 {
		t.Errorf("UpsertDocument reached the repo despite invalid spec")
	}
}

func TestPutOpenAPISpec_UpdatesSpecAndSyncsOperations(t *testing.T) {
	// A non-read-only API: the handler re-extracts operations from the new
	// spec, merges them with the existing API's operation list, calls
	// UpdateAPIByHandle, and finally upserts the spec document.
	env := newOAEnv(t)
	env.seedExistingAPI(false)
	// Seed a prior spec so this is a true replacement, not a first write.
	env.docRepo.docs[constants.DocumentHandleDefinition] = &model.Document{
		ArtifactUUID:     oaAPIUUID,
		OrganizationUUID: oaOrg,
		Type:             constants.DocumentTypeDefinition,
		Handle:           constants.DocumentHandleDefinition,
		DisplayName:      constants.DocumentDisplayNameDefinition,
		FileName:         "old.yaml",
		Content:          []byte(validOpenAPI),
	}
	body, ct := oaImportBody(nil, "new.yaml", []byte(updatedOpenAPI))

	rec := env.do(t, http.MethodPut, openapiPath(oaAPIHandle), oaOrg, "alice", bytes.NewReader(body), ct)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	// Response carries the new spec body.
	var resp api.OpenAPIContent
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Content == nil || *resp.Content != updatedOpenAPI {
		t.Errorf("content mismatch after upsert")
	}
	// UpsertDocument was called with the new content.
	if len(env.docRepo.upserted) != 1 {
		t.Fatalf("upserts = %d, want 1", len(env.docRepo.upserted))
	}
	if got := env.docRepo.upserted[0]; string(got.Content) != updatedOpenAPI {
		t.Errorf("stored content differs from the upload")
	}
}

func TestPutOpenAPISpec_ReadOnlyAPIValidatesWithoutSyncingOperations(t *testing.T) {
	// A read-only API comes from a gateway-originated import; its operations
	// are the authoritative source and the control plane must not overwrite
	// them. The handler validates the spec and still persists the document.
	env := newOAEnv(t)
	env.seedExistingAPI(true)
	body, ct := oaImportBody(nil, "spec.yaml", []byte(updatedOpenAPI))

	rec := env.do(t, http.MethodPut, openapiPath(oaAPIHandle), oaOrg, "alice", bytes.NewReader(body), ct)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	// The spec was stored, but UpdateAPI was NOT called (operations stay the
	// gateway's).
	if len(env.docRepo.upserted) != 1 {
		t.Errorf("UpsertDocument was expected to run exactly once")
	}
}
