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

// Unit tests for the /apis/{apiType}/{apiId}/docs handlers. They run the real
// handler + service stack against in-memory repository fakes and drive it
// through a real *http.ServeMux, so route registration, path-parameter
// extraction, multipart parsing and error mapping are all exercised without a
// database. The fakes (docH*) are shared with api_thumbnail_test.go.

package handler

import (
	"bytes"
	"database/sql"
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
)

// ---------------------------------------------------------------------------
// Fakes and helpers (shared with api_thumbnail_test.go)
// ---------------------------------------------------------------------------

const (
	docHOrg      = "org-1"
	docHAPI      = "my-api"
	docHArtifact = "artifact-uuid-1"
)

var (
	docHKind      = constants.RestApi
	docHDocsBase  = constants.APIBasePath + "/apis/" + docHKind + "/" + docHAPI + "/docs"
	docHThumbPath = constants.APIBasePath + "/apis/" + docHKind + "/" + docHAPI + "/thumbnail"
)

// docHRepo is an in-memory repository.DocumentRepository. The embedded
// interface is nil, so a method the handlers should never reach panics instead
// of silently returning zero values.
type docHRepo struct {
	repository.DocumentRepository

	docs map[string]*model.Document // keyed by handle

	getErr, createErr, listErr, upsertErr, updateErr, deleteAPIErr, deleteErr, existsErr error
	handleExists                                                                        bool
	listResult                                                                          []*model.Document
	listTotal                                                                           int

	listCalls                     int
	lastListType                  string
	lastListLimit, lastListOffset int
	created, updated, upserted    []*model.Document
	updateContentFlags            []bool
	deletedAPIHandles             []string
	deletedHandles, deletedTypes  []string
}

func (r *docHRepo) GetDocument(artifactUUID, handle, orgUUID, docType string) (*model.Document, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	d, ok := r.docs[handle]
	if !ok {
		return nil, nil
	}
	if docType == "" {
		// User-facing lookups never see reserved rows (mirrors the real repo).
		for _, reserved := range constants.ReservedAPIDocumentTypes {
			if d.Type == reserved {
				return nil, nil
			}
		}
	} else if d.Type != docType {
		return nil, nil
	}
	return d, nil
}

func (r *docHRepo) ListDocumentsByArtifact(artifactUUID, orgUUID, docType string, limit, offset int) ([]*model.Document, int, error) {
	r.listCalls++
	r.lastListType, r.lastListLimit, r.lastListOffset = docType, limit, offset
	return r.listResult, r.listTotal, r.listErr
}

func (r *docHRepo) CreateDocument(doc *model.Document) error {
	if r.createErr != nil {
		return r.createErr
	}
	cp := *doc
	r.created = append(r.created, &cp)
	r.docs[cp.Handle] = &cp
	return nil
}

func (r *docHRepo) UpsertDocument(doc *model.Document) error {
	if r.upsertErr != nil {
		return r.upsertErr
	}
	cp := *doc
	r.upserted = append(r.upserted, &cp)
	r.docs[cp.Handle] = &cp
	return nil
}

func (r *docHRepo) UpdateApiDocument(doc *model.Document, updateContent bool) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	cp := *doc
	r.updated = append(r.updated, &cp)
	r.updateContentFlags = append(r.updateContentFlags, updateContent)
	r.docs[cp.Handle] = &cp
	return nil
}

func (r *docHRepo) DeleteApiDocument(artifactUUID, handle, orgUUID string) error {
	if r.deleteAPIErr != nil {
		return r.deleteAPIErr
	}
	r.deletedAPIHandles = append(r.deletedAPIHandles, handle)
	delete(r.docs, handle)
	return nil
}

func (r *docHRepo) DeleteDocument(artifactUUID, handle, orgUUID, docType string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletedHandles = append(r.deletedHandles, handle)
	r.deletedTypes = append(r.deletedTypes, docType)
	delete(r.docs, handle)
	return nil
}

func (r *docHRepo) DocumentHandleExistsForArtifact(artifactUUID, handle string) (bool, error) {
	_, ok := r.docs[handle]
	return ok || r.handleExists, r.existsErr
}

type docHAuditCall struct{ action, resourceUUID, resourceType, orgUUID, performedBy string }

type docHAudit struct{ calls []docHAuditCall }

func (a *docHAudit) Record(action, resourceUUID, resourceType, orgUUID, performedBy string) error {
	a.calls = append(a.calls, docHAuditCall{action, resourceUUID, resourceType, orgUUID, performedBy})
	return nil
}

// docHArtifactRepo resolves exactly one API (docHAPI in docHOrg, kind docHKind).
// Any other kind behaves like an unregistered artifact kind; any other handle or
// organization behaves like a missing row.
type docHArtifactRepo struct {
	repository.ArtifactRepository
	err error
}

func (a *docHArtifactRepo) GetAPIMetadataByHandleAndKind(handle, kind, orgUUID string) (*model.APIMetadata, error) {
	if a.err != nil {
		return nil, a.err
	}
	if kind != docHKind {
		return nil, repository.ErrUnknownArtifactKind
	}
	if handle != docHAPI || orgUUID != docHOrg {
		return nil, nil
	}
	return &model.APIMetadata{ID: docHArtifact, Handle: handle, Kind: kind, OrganizationID: orgUUID}, nil
}

// docHFailingIdentityRepo makes every actor lookup fail.
type docHFailingIdentityRepo struct{ repository.UserIdentityMappingRepository }

func (docHFailingIdentityRepo) GetOrCreateUUID(string) (string, error) {
	return "", errors.New("identity store unavailable")
}

type docHEnv struct {
	mux       *http.ServeMux
	docs      *docHRepo
	audit     *docHAudit
	artifacts *docHArtifactRepo
}

func newDocHEnv(t *testing.T, cfg *config.Server) *docHEnv {
	t.Helper()
	return newDocHEnvWithIdentity(t, cfg, nil)
}

// newDocHEnvWithIdentity wires both the docs and the thumbnail handler onto one
// mux. A nil identityRepo is fine for requests that carry no actor claim: the
// identity service then mints an anonymous UUID without touching the repo.
func newDocHEnvWithIdentity(t *testing.T, cfg *config.Server, identityRepo repository.UserIdentityMappingRepository) *docHEnv {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	env := &docHEnv{
		docs:      &docHRepo{docs: map[string]*model.Document{}},
		audit:     &docHAudit{},
		artifacts: &docHArtifactRepo{},
	}
	svc := service.NewAPIDocumentService(env.docs, env.artifacts, env.audit, logger)
	identity := service.NewIdentityService(identityRepo)
	env.mux = http.NewServeMux()
	NewAPIDocumentHandler(svc, identity, logger, cfg).RegisterRoutes(env.mux)
	NewAPIThumbnailHandler(svc, identity, logger, cfg).RegisterRoutes(env.mux)
	return env
}

// seed stores a document under the test artifact/org.
func (e *docHEnv) seed(d *model.Document) *model.Document {
	d.ArtifactUUID = docHArtifact
	d.OrganizationUUID = docHOrg
	e.docs.docs[d.Handle] = d
	return d
}

// do issues one request. An empty org omits the organization claim; an empty
// user omits the actor claim.
func (e *docHEnv) do(t *testing.T, method, target, org, user string, body io.Reader, contentType string) *httptest.ResponseRecorder {
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

// get is shorthand for an authenticated bodiless request in docHOrg.
func (e *docHEnv) get(t *testing.T, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	return e.do(t, method, target, docHOrg, "", nil, "")
}

type docHFile struct {
	field, name string
	content     []byte
}

func docHForm(t *testing.T, fields map[string]string, files ...docHFile) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("write field %q: %v", k, err)
		}
	}
	for _, f := range files {
		part, err := w.CreateFormFile(f.field, f.name)
		if err != nil {
			t.Fatalf("create form file %q: %v", f.field, err)
		}
		if _, err := part.Write(f.content); err != nil {
			t.Fatalf("write form file %q: %v", f.field, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return &buf, w.FormDataContentType()
}

// form sends a multipart request in docHOrg.
func (e *docHEnv) form(t *testing.T, method, target string, fields map[string]string, files ...docHFile) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := docHForm(t, fields, files...)
	return e.do(t, method, target, docHOrg, "", body, ct)
}

func docHAssertError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, wantStatus, rec.Body.String())
	}
	var body struct {
		Status string `json:"status"`
		Code   string `json:"code"`
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

func docHDecodeMetadata(t *testing.T, rec *httptest.ResponseRecorder) api.APIDocumentMetadata {
	t.Helper()
	var md api.APIDocumentMetadata
	if err := json.Unmarshal(rec.Body.Bytes(), &md); err != nil {
		t.Fatalf("decode metadata: %v; body: %s", err, rec.Body.String())
	}
	return md
}

func docHSampleDoc() *model.Document {
	return &model.Document{
		ID:          "doc-uuid-1",
		Type:        constants.DocumentTypePrefix + constants.DocumentTypeHowTo,
		Handle:      "guide",
		DisplayName: "Guide",
		FileName:    "guide.md",
		ContentType: "text/markdown; charset=utf-8",
		Content:     []byte("# Guide"),
	}
}

func docHAuditLast(t *testing.T, a *docHAudit) docHAuditCall {
	t.Helper()
	if len(a.calls) == 0 {
		t.Fatalf("no audit entry recorded")
	}
	return a.calls[len(a.calls)-1]
}

// ---------------------------------------------------------------------------
// Construction and routing
// ---------------------------------------------------------------------------

func TestNewAPIDocumentHandler_MaxBodyBytes(t *testing.T) {
	cases := []struct {
		name string
		cfg  *config.Server
		want int64
	}{
		{"nil config uses the default", nil, constants.DefaultOpenAPISpecMaxBytes},
		{"zero value uses the default", &config.Server{}, constants.DefaultOpenAPISpecMaxBytes},
		{"configured value wins", &config.Server{OpenAPISpecMaxFetchBytes: 4096}, 4096},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewAPIDocumentHandler(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), tc.cfg)
			if h.maxBodyBytes != tc.want {
				t.Errorf("maxBodyBytes = %d, want %d", h.maxBodyBytes, tc.want)
			}
		})
	}
}

func TestAPIDocumentRoutes_UnsupportedMethodsAreRejected(t *testing.T) {
	env := newDocHEnv(t, nil)
	cases := []struct{ method, target string }{
		{http.MethodPatch, docHDocsBase},
		{http.MethodDelete, docHDocsBase},
		{http.MethodPost, docHDocsBase + "/guide"},
		{http.MethodPut, docHDocsBase + "/guide/content"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.target, func(t *testing.T) {
			if rec := env.get(t, tc.method, tc.target); rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

// A request without an organization claim is rejected before any repository is
// consulted, on every operation.
func TestAPIDocumentHandlers_MissingOrganizationIsUnauthorized(t *testing.T) {
	env := newDocHEnv(t, nil)
	cases := []struct{ name, method, target string }{
		{"list", http.MethodGet, docHDocsBase},
		{"create", http.MethodPost, docHDocsBase},
		{"get", http.MethodGet, docHDocsBase + "/guide"},
		{"content", http.MethodGet, docHDocsBase + "/guide/content"},
		{"update", http.MethodPut, docHDocsBase + "/guide"},
		{"delete", http.MethodDelete, docHDocsBase + "/guide"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := env.do(t, tc.method, tc.target, "", "", nil, "")
			docHAssertError(t, rec, http.StatusUnauthorized, apperror.CodeCommonUnauthorized)
		})
	}
}

// Unknown kinds, unknown APIs and APIs that belong to another organization all
// collapse to the same 404, so a caller cannot probe for what exists.
func TestAPIDocumentHandlers_UnresolvableArtifactIsNotFound(t *testing.T) {
	env := newDocHEnv(t, nil)
	apiBase := constants.APIBasePath + "/apis/"
	cases := []struct{ name, target, org string }{
		{"unknown kind", apiBase + "bogus-kind/" + docHAPI + "/docs", docHOrg},
		{"unknown api", apiBase + docHKind + "/no-such-api/docs", docHOrg},
		{"api in another organization", docHDocsBase, "org-2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := env.do(t, http.MethodGet, tc.target, tc.org, "", nil, "")
			docHAssertError(t, rec, http.StatusNotFound, apperror.CodeCommonNotFound)
		})
	}
}

// An artifact lookup that fails for a reason other than "not found" must not
// leak its cause: it becomes a sterile 500.
func TestAPIDocumentHandlers_ArtifactLookupFailureIsInternalError(t *testing.T) {
	env := newDocHEnv(t, nil)
	env.artifacts.err = errors.New("connection refused")
	rec := env.get(t, http.MethodGet, docHDocsBase)
	docHAssertError(t, rec, http.StatusInternalServerError, apperror.CodeCommonInternalError)
	if strings.Contains(rec.Body.String(), "connection refused") {
		t.Errorf("internal error text leaked to the client: %s", rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// GET /docs
// ---------------------------------------------------------------------------

func TestListDocuments_ReturnsPageWithDecodedTypes(t *testing.T) {
	env := newDocHEnv(t, nil)
	env.docs.listResult = []*model.Document{
		{Handle: "guide", Type: "DOC_HowTo", DisplayName: "Guide", FileName: "guide.md"},
		{Handle: "faq", Type: "DOC_FAQ", DisplayName: "FAQ"},
	}
	env.docs.listTotal = 7

	rec := env.get(t, http.MethodGet, docHDocsBase+"?limit=2&offset=4")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var resp api.APIDocumentListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Count != 2 || len(resp.List) != 2 {
		t.Fatalf("count = %d, len(list) = %d, want 2/2", resp.Count, len(resp.List))
	}
	if resp.Pagination.Total != 7 || resp.Pagination.Offset != 4 || resp.Pagination.Limit != 2 {
		t.Errorf("pagination = %+v, want total=7 offset=4 limit=2", resp.Pagination)
	}
	// The DOC_ storage prefix must never reach the wire.
	if resp.List[0].Id != "guide" || resp.List[0].Type != "HowTo" {
		t.Errorf("first item = {%s %s}, want {guide HowTo}", resp.List[0].Id, resp.List[0].Type)
	}
	if resp.List[1].Id != "faq" || resp.List[1].Type != "FAQ" {
		t.Errorf("second item = {%s %s}, want {faq FAQ}", resp.List[1].Id, resp.List[1].Type)
	}
	if env.docs.lastListLimit != 2 || env.docs.lastListOffset != 4 {
		t.Errorf("repo got limit=%d offset=%d, want 2/4", env.docs.lastListLimit, env.docs.lastListOffset)
	}
}

func TestListDocuments_EmptyListIsAnArrayNotNull(t *testing.T) {
	env := newDocHEnv(t, nil)
	rec := env.get(t, http.MethodGet, docHDocsBase)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := strings.TrimSpace(string(raw["list"])); got != "[]" {
		t.Errorf("list = %s, want an empty JSON array rather than null", got)
	}
}

func TestListDocuments_TypeFilter(t *testing.T) {
	cases := []struct {
		name, query string
		wantType    string
		wantStatus  int
	}{
		{"no filter", "", "", http.StatusOK},
		{"fixed type is canonicalised", "?type=howto", "HowTo", http.StatusOK},
		{"surrounding whitespace is trimmed", "?type=%20SupportForum%20", "SupportForum", http.StatusOK},
		{"Other is canonicalised", "?type=OTHER", "Other", http.StatusOK},
		{"custom type passes through verbatim", "?type=FAQ", "FAQ", http.StatusOK},
		{"reserved DEFINITION is rejected", "?type=DEFINITION", "", http.StatusBadRequest},
		{"reserved THUMBNAIL is rejected in any case", "?type=thumbnail", "", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newDocHEnv(t, nil)
			rec := env.get(t, http.MethodGet, docHDocsBase+tc.query)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus != http.StatusOK {
				docHAssertError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
				if env.docs.listCalls != 0 {
					t.Errorf("repository was queried despite an invalid filter")
				}
				return
			}
			if env.docs.lastListType != tc.wantType {
				t.Errorf("repo filter = %q, want %q", env.docs.lastListType, tc.wantType)
			}
		})
	}
}

func TestListDocuments_PaginationIsClamped(t *testing.T) {
	cases := []struct {
		name, query           string
		wantLimit, wantOffset int
	}{
		{"defaults", "", 20, 0},
		{"limit above the maximum is clamped", "?limit=1000", 100, 0},
		{"limit below the minimum is clamped", "?limit=0", 1, 0},
		{"negative offset falls back to zero", "?offset=-5", 20, 0},
		{"malformed values fall back to defaults", "?limit=abc&offset=xyz", 20, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newDocHEnv(t, nil)
			rec := env.get(t, http.MethodGet, docHDocsBase+tc.query)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if env.docs.lastListLimit != tc.wantLimit || env.docs.lastListOffset != tc.wantOffset {
				t.Errorf("repo got limit=%d offset=%d, want %d/%d",
					env.docs.lastListLimit, env.docs.lastListOffset, tc.wantLimit, tc.wantOffset)
			}
		})
	}
}

func TestListDocuments_RepositoryFailureIsInternalError(t *testing.T) {
	env := newDocHEnv(t, nil)
	env.docs.listErr = errors.New("db down")
	rec := env.get(t, http.MethodGet, docHDocsBase)
	docHAssertError(t, rec, http.StatusInternalServerError, apperror.CodeCommonInternalError)
}

// ---------------------------------------------------------------------------
// GET /docs/{docId}
// ---------------------------------------------------------------------------

func TestGetDocument_ReturnsMetadataOnly(t *testing.T) {
	env := newDocHEnv(t, nil)
	env.seed(docHSampleDoc())

	rec := env.get(t, http.MethodGet, docHDocsBase+"/guide")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	md := docHDecodeMetadata(t, rec)
	if md.Id != "guide" || md.Type != "HowTo" || md.DisplayName != "Guide" {
		t.Errorf("metadata = %+v, want id=guide type=HowTo displayName=Guide", md)
	}
	if md.FileName == nil || *md.FileName != "guide.md" {
		t.Errorf("fileName = %v, want guide.md", md.FileName)
	}
	if strings.Contains(rec.Body.String(), "# Guide") {
		t.Errorf("metadata response must not carry document bytes: %s", rec.Body.String())
	}
}

func TestGetDocument_NotFound(t *testing.T) {
	env := newDocHEnv(t, nil)
	rec := env.get(t, http.MethodGet, docHDocsBase+"/missing")
	docHAssertError(t, rec, http.StatusNotFound, apperror.CodeCommonNotFound)
}

func TestGetDocument_ReservedHandlesAreRefused(t *testing.T) {
	for _, handle := range []string{constants.DocumentHandleDefinition, constants.DocumentHandleThumbnail} {
		t.Run(handle, func(t *testing.T) {
			env := newDocHEnv(t, nil)
			docHAssertError(t, env.get(t, http.MethodGet, docHDocsBase+"/"+handle),
				http.StatusBadRequest, apperror.CodeCommonValidationFailed)
			docHAssertError(t, env.get(t, http.MethodGet, docHDocsBase+"/"+handle+"/content"),
				http.StatusBadRequest, apperror.CodeCommonValidationFailed)
		})
	}
}

// A reserved-type row stored under an ordinary handle is still invisible here.
func TestGetDocument_ReservedTypeRowIsHidden(t *testing.T) {
	env := newDocHEnv(t, nil)
	env.seed(&model.Document{Handle: "sneaky", Type: constants.DocumentTypeDefinition, DisplayName: "Spec"})
	docHAssertError(t, env.get(t, http.MethodGet, docHDocsBase+"/sneaky"),
		http.StatusNotFound, apperror.CodeCommonNotFound)
}

func TestGetDocument_RepositoryFailureIsInternalError(t *testing.T) {
	env := newDocHEnv(t, nil)
	env.docs.getErr = errors.New("db down")
	docHAssertError(t, env.get(t, http.MethodGet, docHDocsBase+"/guide"),
		http.StatusInternalServerError, apperror.CodeCommonInternalError)
}

// ---------------------------------------------------------------------------
// GET /docs/{docId}/content
// ---------------------------------------------------------------------------

func TestGetDocumentContent_StreamsStoredBytes(t *testing.T) {
	env := newDocHEnv(t, nil)
	env.seed(docHSampleDoc())

	rec := env.get(t, http.MethodGet, docHDocsBase+"/guide/content")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "# Guide" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "# Guide")
	}
	if got := rec.Header().Get("Content-Type"); got != "text/markdown; charset=utf-8" {
		t.Errorf("Content-Type = %q, want the stored type", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := rec.Header().Get("Content-Disposition"); got != `inline; filename="guide.md"` {
		t.Errorf("Content-Disposition = %q", got)
	}
}

func TestGetDocumentContent_HeaderFallbacksAndEscaping(t *testing.T) {
	t.Run("missing content type falls back to octet-stream", func(t *testing.T) {
		env := newDocHEnv(t, nil)
		d := docHSampleDoc()
		d.ContentType = ""
		env.seed(d)
		rec := env.get(t, http.MethodGet, docHDocsBase+"/guide/content")
		if got := rec.Header().Get("Content-Type"); got != "application/octet-stream" {
			t.Errorf("Content-Type = %q, want application/octet-stream", got)
		}
	})
	t.Run("missing file name omits Content-Disposition", func(t *testing.T) {
		env := newDocHEnv(t, nil)
		d := docHSampleDoc()
		d.FileName = ""
		env.seed(d)
		rec := env.get(t, http.MethodGet, docHDocsBase+"/guide/content")
		if got := rec.Header().Get("Content-Disposition"); got != "" {
			t.Errorf("Content-Disposition = %q, want none", got)
		}
	})
	t.Run("quotes and backslashes in the file name are escaped", func(t *testing.T) {
		env := newDocHEnv(t, nil)
		d := docHSampleDoc()
		d.FileName = `we"ird\name.md`
		env.seed(d)
		rec := env.get(t, http.MethodGet, docHDocsBase+"/guide/content")
		want := `inline; filename="we\"ird\\name.md"`
		if got := rec.Header().Get("Content-Disposition"); got != want {
			t.Errorf("Content-Disposition = %q, want %q", got, want)
		}
	})
}

func TestGetDocumentContent_EmptyBodyIsNoContent(t *testing.T) {
	env := newDocHEnv(t, nil)
	d := docHSampleDoc()
	d.Content = nil
	env.seed(d)
	rec := env.get(t, http.MethodGet, docHDocsBase+"/guide/content")
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestGetDocumentContent_NotFoundAndFailures(t *testing.T) {
	env := newDocHEnv(t, nil)
	docHAssertError(t, env.get(t, http.MethodGet, docHDocsBase+"/missing/content"),
		http.StatusNotFound, apperror.CodeCommonNotFound)

	env.docs.getErr = errors.New("db down")
	docHAssertError(t, env.get(t, http.MethodGet, docHDocsBase+"/guide/content"),
		http.StatusInternalServerError, apperror.CodeCommonInternalError)
}

// ---------------------------------------------------------------------------
// POST /docs
// ---------------------------------------------------------------------------

func TestCreateDocument_InlineContent(t *testing.T) {
	env := newDocHEnv(t, nil)
	rec := env.form(t, http.MethodPost, docHDocsBase, map[string]string{
		"type":          "howto",
		"displayName":   "Getting Started",
		"id":            "getting-started",
		"inlineContent": "# Hello",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	if got, want := rec.Header().Get("Location"), docHDocsBase+"/getting-started"; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	md := docHDecodeMetadata(t, rec)
	if md.Id != "getting-started" || md.Type != "HowTo" || md.DisplayName != "Getting Started" {
		t.Errorf("metadata = %+v", md)
	}

	if len(env.docs.created) != 1 {
		t.Fatalf("created %d documents, want 1", len(env.docs.created))
	}
	got := env.docs.created[0]
	if got.Type != "DOC_HowTo" {
		t.Errorf("stored type = %q, want DOC_HowTo", got.Type)
	}
	if got.ContentType != "text/markdown; charset=utf-8" || string(got.Content) != "# Hello" {
		t.Errorf("stored content = %q (%s)", got.Content, got.ContentType)
	}
	if got.FileName != "getting-started.md" {
		t.Errorf("stored fileName = %q, want getting-started.md", got.FileName)
	}
	if got.ArtifactUUID != docHArtifact || got.OrganizationUUID != docHOrg {
		t.Errorf("stored under artifact=%q org=%q", got.ArtifactUUID, got.OrganizationUUID)
	}
	if call := docHAuditLast(t, env.audit); call.action != "CREATE" || call.resourceType != "api_document" {
		t.Errorf("audit = %+v, want CREATE api_document", call)
	}
}

func TestCreateDocument_GeneratesHandleFromDisplayName(t *testing.T) {
	env := newDocHEnv(t, nil)
	rec := env.form(t, http.MethodPost, docHDocsBase, map[string]string{
		"type": "Samples", "displayName": "Release Notes", "inlineContent": "x",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	md := docHDecodeMetadata(t, rec)
	if md.Id != "release-notes" {
		t.Errorf("generated id = %q, want release-notes", md.Id)
	}
	if got := rec.Header().Get("Location"); !strings.HasSuffix(got, "/"+md.Id) {
		t.Errorf("Location = %q, want a suffix of /%s", got, md.Id)
	}
}

func TestCreateDocument_CustomOtherType(t *testing.T) {
	env := newDocHEnv(t, nil)
	rec := env.form(t, http.MethodPost, docHDocsBase, map[string]string{
		"type": "Other", "otherTypeName": "FAQ", "displayName": "Common Questions", "inlineContent": "Q&A",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	if got := env.docs.created[0].Type; got != "DOC_FAQ" {
		t.Errorf("stored type = %q, want DOC_FAQ", got)
	}
	if md := docHDecodeMetadata(t, rec); md.Type != "FAQ" {
		t.Errorf("response type = %q, want the bare custom name FAQ", md.Type)
	}
}

func TestCreateDocument_RejectsInvalidRequests(t *testing.T) {
	cases := []struct {
		name   string
		fields map[string]string
	}{
		{"inlineContent is required", map[string]string{"type": "HowTo", "displayName": "x"}},
		{"type is required", map[string]string{"displayName": "x", "inlineContent": "x"}},
		{"unknown type", map[string]string{"type": "Bogus", "displayName": "x", "inlineContent": "x"}},
		{"reserved type", map[string]string{"type": "DEFINITION", "displayName": "x", "inlineContent": "x"}},
		{"displayName is required", map[string]string{"type": "HowTo", "inlineContent": "x"}},
		{"blank displayName", map[string]string{"type": "HowTo", "displayName": "   ", "inlineContent": "x"}},
		{"forbidden custom type name", map[string]string{"type": "Other", "otherTypeName": "HowTo", "displayName": "x", "inlineContent": "x"}},
		{"reserved handle", map[string]string{"type": "HowTo", "displayName": "x", "id": constants.DocumentHandleThumbnail, "inlineContent": "x"}},
		{"malformed handle", map[string]string{"type": "HowTo", "displayName": "x", "id": "Not A Handle", "inlineContent": "x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newDocHEnv(t, nil)
			rec := env.form(t, http.MethodPost, docHDocsBase, tc.fields)
			docHAssertError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
			if len(env.docs.created) != 0 {
				t.Errorf("a document was stored for an invalid request")
			}
		})
	}
}

func TestCreateDocument_DuplicateHandleIsConflict(t *testing.T) {
	env := newDocHEnv(t, nil)
	env.seed(docHSampleDoc())
	rec := env.form(t, http.MethodPost, docHDocsBase, map[string]string{
		"type": "HowTo", "displayName": "Another", "id": "guide", "inlineContent": "x",
	})
	docHAssertError(t, rec, http.StatusConflict, apperror.CodeCommonConflict)
}

// Two requests racing past the existence check are caught by the DB unique
// index and must still surface as a 409.
func TestCreateDocument_UniqueViolationIsConflict(t *testing.T) {
	env := newDocHEnv(t, nil)
	env.docs.createErr = errors.New("UNIQUE constraint failed: api_documents.handle")
	rec := env.form(t, http.MethodPost, docHDocsBase, map[string]string{
		"type": "HowTo", "displayName": "Racy", "inlineContent": "x",
	})
	docHAssertError(t, rec, http.StatusConflict, apperror.CodeCommonConflict)
}

func TestCreateDocument_RepositoryFailureIsInternalError(t *testing.T) {
	env := newDocHEnv(t, nil)
	env.docs.createErr = errors.New("disk full")
	rec := env.form(t, http.MethodPost, docHDocsBase, map[string]string{
		"type": "HowTo", "displayName": "Doc", "inlineContent": "x",
	})
	docHAssertError(t, rec, http.StatusInternalServerError, apperror.CodeCommonInternalError)
}

func TestCreateDocument_NotMultipartIsBadRequest(t *testing.T) {
	env := newDocHEnv(t, nil)
	rec := env.do(t, http.MethodPost, docHDocsBase, docHOrg, "", strings.NewReader(`{"type":"HowTo"}`), "application/json")
	docHAssertError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
}

func TestCreateDocument_OversizedBodyIsPayloadTooLarge(t *testing.T) {
	env := newDocHEnv(t, &config.Server{OpenAPISpecMaxFetchBytes: 16})
	rec := env.form(t, http.MethodPost, docHDocsBase, map[string]string{
		"type":          "HowTo",
		"displayName":   "Huge",
		"inlineContent": strings.Repeat("a", (1<<20)+1024), // beyond limit + the 1 MiB multipart allowance
	})
	docHAssertError(t, rec, http.StatusRequestEntityTooLarge, apperror.CodeCommonPayloadTooLarge)
}

// When the caller's identity cannot be mapped to an internal user id the write
// fails closed instead of storing an unattributed row.
func TestDocumentWrites_UnresolvableActorIsInternalError(t *testing.T) {
	env := newDocHEnvWithIdentity(t, nil, docHFailingIdentityRepo{})
	env.seed(docHSampleDoc())

	body, ct := docHForm(t, map[string]string{"type": "HowTo", "displayName": "Doc", "inlineContent": "x"})
	rec := env.do(t, http.MethodPost, docHDocsBase, docHOrg, "alice", body, ct)
	docHAssertError(t, rec, http.StatusInternalServerError, apperror.CodeCommonInternalError)

	body, ct = docHForm(t, map[string]string{"displayName": "Renamed"})
	rec = env.do(t, http.MethodPut, docHDocsBase+"/guide", docHOrg, "alice", body, ct)
	docHAssertError(t, rec, http.StatusInternalServerError, apperror.CodeCommonInternalError)

	rec = env.do(t, http.MethodDelete, docHDocsBase+"/guide", docHOrg, "alice", nil, "")
	docHAssertError(t, rec, http.StatusInternalServerError, apperror.CodeCommonInternalError)

	if len(env.docs.created)+len(env.docs.updated)+len(env.docs.deletedAPIHandles) != 0 {
		t.Errorf("a write reached the repository despite the identity failure")
	}
}

// ---------------------------------------------------------------------------
// PUT /docs/{docId}
// ---------------------------------------------------------------------------

func TestUpdateDocument_MetadataOnlyKeepsStoredBytes(t *testing.T) {
	env := newDocHEnv(t, nil)
	env.seed(docHSampleDoc())

	rec := env.form(t, http.MethodPut, docHDocsBase+"/guide", map[string]string{"displayName": "  Guide v2  "})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if len(env.docs.updated) != 1 {
		t.Fatalf("got %d updates, want 1", len(env.docs.updated))
	}
	if env.docs.updateContentFlags[0] {
		t.Errorf("updateContent = true for a metadata-only PUT; stored bytes would be overwritten")
	}
	if got := env.docs.updated[0].DisplayName; got != "Guide v2" {
		t.Errorf("stored displayName = %q, want the trimmed value", got)
	}
	if md := docHDecodeMetadata(t, rec); md.DisplayName != "Guide v2" {
		t.Errorf("response displayName = %q", md.DisplayName)
	}
	if call := docHAuditLast(t, env.audit); call.action != "UPDATE" {
		t.Errorf("audit action = %q, want UPDATE", call.action)
	}
}

func TestUpdateDocument_ReplacesContent(t *testing.T) {
	env := newDocHEnv(t, nil)
	env.seed(docHSampleDoc())

	rec := env.form(t, http.MethodPut, docHDocsBase+"/guide", map[string]string{
		"inlineContent": "# Rewritten", "fileName": "../notes/rewritten.md",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if !env.docs.updateContentFlags[0] {
		t.Errorf("updateContent = false although new bytes were supplied")
	}
	got := env.docs.updated[0]
	if string(got.Content) != "# Rewritten" || got.ContentType != "text/markdown; charset=utf-8" {
		t.Errorf("stored content = %q (%s)", got.Content, got.ContentType)
	}
	if got.FileName != "rewritten.md" {
		t.Errorf("stored fileName = %q, want the sanitised base name rewritten.md", got.FileName)
	}
}

func TestUpdateDocument_CanEmptyTheContent(t *testing.T) {
	env := newDocHEnv(t, nil)
	env.seed(docHSampleDoc())

	rec := env.form(t, http.MethodPut, docHDocsBase+"/guide", map[string]string{"inlineContent": ""})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if !env.docs.updateContentFlags[0] {
		t.Errorf("an explicitly empty inlineContent must replace the stored bytes")
	}
	if len(env.docs.updated[0].Content) != 0 {
		t.Errorf("stored content = %q, want empty", env.docs.updated[0].Content)
	}
}

func TestUpdateDocument_ChangesType(t *testing.T) {
	env := newDocHEnv(t, nil)
	env.seed(docHSampleDoc())

	rec := env.form(t, http.MethodPut, docHDocsBase+"/guide", map[string]string{
		"type": "Other", "otherTypeName": "FAQ",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if got := env.docs.updated[0].Type; got != "DOC_FAQ" {
		t.Errorf("stored type = %q, want DOC_FAQ", got)
	}
	if md := docHDecodeMetadata(t, rec); md.Type != "FAQ" {
		t.Errorf("response type = %q, want FAQ", md.Type)
	}
}

func TestUpdateDocument_RejectsInvalidRequests(t *testing.T) {
	cases := []struct {
		name, target string
		fields       map[string]string
	}{
		{"id differs from the path", "/guide", map[string]string{"id": "other-doc"}},
		{"blank id never matches the path", "/guide", map[string]string{"id": "  "}},
		{"unknown type", "/guide", map[string]string{"type": "Bogus"}},
		{"empty type", "/guide", map[string]string{"type": ""}},
		{"forbidden custom type name", "/guide", map[string]string{"type": "Other", "otherTypeName": "Samples"}},
		{"blank displayName", "/guide", map[string]string{"displayName": "   "}},
		{"reserved handle", "/" + constants.DocumentHandleDefinition, map[string]string{"displayName": "x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newDocHEnv(t, nil)
			env.seed(docHSampleDoc())
			rec := env.form(t, http.MethodPut, docHDocsBase+tc.target, tc.fields)
			docHAssertError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
			if len(env.docs.updated) != 0 {
				t.Errorf("an update reached the repository for an invalid request")
			}
		})
	}
}

func TestUpdateDocument_MatchingIdIsAccepted(t *testing.T) {
	env := newDocHEnv(t, nil)
	env.seed(docHSampleDoc())
	rec := env.form(t, http.MethodPut, docHDocsBase+"/guide", map[string]string{"id": "guide", "displayName": "Same id"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateDocument_NotFound(t *testing.T) {
	env := newDocHEnv(t, nil)
	rec := env.form(t, http.MethodPut, docHDocsBase+"/missing", map[string]string{"displayName": "x"})
	docHAssertError(t, rec, http.StatusNotFound, apperror.CodeCommonNotFound)
}

func TestUpdateDocument_NotMultipartIsBadRequest(t *testing.T) {
	env := newDocHEnv(t, nil)
	env.seed(docHSampleDoc())
	rec := env.do(t, http.MethodPut, docHDocsBase+"/guide", docHOrg, "", strings.NewReader("displayName=x"), "application/x-www-form-urlencoded")
	docHAssertError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
}

func TestUpdateDocument_RepositoryFailureIsInternalError(t *testing.T) {
	env := newDocHEnv(t, nil)
	env.seed(docHSampleDoc())
	env.docs.updateErr = errors.New("db down")
	rec := env.form(t, http.MethodPut, docHDocsBase+"/guide", map[string]string{"displayName": "x"})
	docHAssertError(t, rec, http.StatusInternalServerError, apperror.CodeCommonInternalError)
}

// ---------------------------------------------------------------------------
// DELETE /docs/{docId}
// ---------------------------------------------------------------------------

func TestDeleteDocument_Success(t *testing.T) {
	env := newDocHEnv(t, nil)
	env.seed(docHSampleDoc())

	rec := env.get(t, http.MethodDelete, docHDocsBase+"/guide")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	if len(env.docs.deletedAPIHandles) != 1 || env.docs.deletedAPIHandles[0] != "guide" {
		t.Errorf("deleted handles = %v, want [guide]", env.docs.deletedAPIHandles)
	}
	if call := docHAuditLast(t, env.audit); call.action != "DELETE" || call.resourceType != "api_document" || call.resourceUUID != docHArtifact {
		t.Errorf("audit = %+v, want DELETE api_document on the artifact", call)
	}
	// And the document is really gone.
	docHAssertError(t, env.get(t, http.MethodGet, docHDocsBase+"/guide"), http.StatusNotFound, apperror.CodeCommonNotFound)
}

func TestDeleteDocument_Failures(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		env := newDocHEnv(t, nil)
		docHAssertError(t, env.get(t, http.MethodDelete, docHDocsBase+"/missing"),
			http.StatusNotFound, apperror.CodeCommonNotFound)
	})
	t.Run("row vanishes between lookup and delete", func(t *testing.T) {
		env := newDocHEnv(t, nil)
		env.seed(docHSampleDoc())
		env.docs.deleteAPIErr = sql.ErrNoRows
		docHAssertError(t, env.get(t, http.MethodDelete, docHDocsBase+"/guide"),
			http.StatusNotFound, apperror.CodeCommonNotFound)
	})
	t.Run("reserved handle", func(t *testing.T) {
		env := newDocHEnv(t, nil)
		docHAssertError(t, env.get(t, http.MethodDelete, docHDocsBase+"/"+constants.DocumentHandleThumbnail),
			http.StatusBadRequest, apperror.CodeCommonValidationFailed)
		if len(env.docs.deletedAPIHandles) != 0 {
			t.Errorf("a reserved handle reached the repository")
		}
	})
	t.Run("repository failure", func(t *testing.T) {
		env := newDocHEnv(t, nil)
		env.seed(docHSampleDoc())
		env.docs.deleteAPIErr = errors.New("db down")
		docHAssertError(t, env.get(t, http.MethodDelete, docHDocsBase+"/guide"),
			http.StatusInternalServerError, apperror.CodeCommonInternalError)
	})
}
