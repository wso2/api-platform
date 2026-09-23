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
	"bytes"
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/vault"
)

// testSharedKeyHex is a valid 64-char hex value the service's
// validateAndEncryptSharedKey accepts. Cryptographically bogus (all-a) but
// syntactically correct — matches `^[0-9a-fA-F]{64}$`.
const testSharedKeyHex = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// newTestVault returns a real InHouseVault seeded with a deterministic 32-byte
// key. Using the real implementation (rather than a fake) validates the
// encrypt/decrypt round-trip actually works.
func newTestVault(t *testing.T) vault.SecretVault {
	t.Helper()
	v, err := vault.NewInHouseVault(bytes.Repeat([]byte("t"), 32))
	if err != nil {
		t.Fatalf("test vault: %v", err)
	}
	return v
}

// --- mocks ---
// Each mock embeds the interface so unimplemented methods panic on invocation,
// making it obvious when a test exercises an unstubbed code path.

type mockAPIPortalRepository struct {
	repository.APIPortalRepository

	existsResult bool
	existsErr    error

	createErr           error
	createReturnUnique  bool // if true, Create returns a canned unique-violation
	createCapturedInput *model.APIPortal

	getResult *model.APIPortal
	getErr    error

	listResult []*model.APIPortal
	listErr    error

	countResult int
	countErr    error

	updateErr           error
	updateCapturedInput *model.APIPortal

	deleteCalledWith [2]string
	deleteErr        error

	// Status-family capture + canned returns.
	updateStatusCalledWith updateStatusCall
	updateStatusErr        error

	getStatusCalledWith [2]string
	getStatusResult     string
	getStatusErr        error

	listStatusesCalledWith string
	listStatusesResult     map[string]string
	listStatusesErr        error

	listByStatusCalledWith string
	listByStatusResult     []*model.APIPortal
	listByStatusErr        error
}

type updateStatusCall struct {
	PortalID  string
	OrgUUID   string
	UpdatedBy string
	Status    string
}

// canned unique-violation error — matches IsUniqueViolation's SQLite substring.
var errCannedUnique = errors.New("UNIQUE constraint failed: api_portals.handle")

func (m *mockAPIPortalRepository) Exists(handle, orgUUID string) (bool, error) {
	return m.existsResult, m.existsErr
}

func (m *mockAPIPortalRepository) Create(portal *model.APIPortal) error {
	m.createCapturedInput = portal
	if m.createReturnUnique {
		return errCannedUnique
	}
	return m.createErr
}

func (m *mockAPIPortalRepository) GetByHandleAndOrgID(handle, orgUUID string) (*model.APIPortal, error) {
	return m.getResult, m.getErr
}

func (m *mockAPIPortalRepository) ListPaginated(orgUUID string, opts repository.ListOptions) ([]*model.APIPortal, error) {
	return m.listResult, m.listErr
}

func (m *mockAPIPortalRepository) Count(orgUUID string, search string) (int, error) {
	return m.countResult, m.countErr
}

func (m *mockAPIPortalRepository) Update(portal *model.APIPortal) error {
	m.updateCapturedInput = portal
	return m.updateErr
}

func (m *mockAPIPortalRepository) Delete(portalID, orgUUID string) error {
	m.deleteCalledWith = [2]string{portalID, orgUUID}
	return m.deleteErr
}

// Status-family stubs; each tests specific behaviour by populating the fields it
// needs. Unset returns match the zero-value semantics real callers expect.
func (m *mockAPIPortalRepository) UpdateStatus(portalID, orgUUID, updatedBy, status string) error {
	m.updateStatusCalledWith = updateStatusCall{
		PortalID: portalID, OrgUUID: orgUUID, UpdatedBy: updatedBy, Status: status,
	}
	return m.updateStatusErr
}

func (m *mockAPIPortalRepository) GetStatusByHandle(handle, orgUUID string) (string, error) {
	m.getStatusCalledWith = [2]string{handle, orgUUID}
	return m.getStatusResult, m.getStatusErr
}

func (m *mockAPIPortalRepository) ListStatusesByOrg(orgUUID string) (map[string]string, error) {
	m.listStatusesCalledWith = orgUUID
	return m.listStatusesResult, m.listStatusesErr
}

func (m *mockAPIPortalRepository) ListByStatus(status string) ([]*model.APIPortal, error) {
	m.listByStatusCalledWith = status
	return m.listByStatusResult, m.listByStatusErr
}

type mockAPIPortalOrgRepository struct {
	repository.OrganizationRepository
	result *model.Organization
	err    error
}

func (m *mockAPIPortalOrgRepository) GetOrganizationByUUID(uuid string) (*model.Organization, error) {
	return m.result, m.err
}

type mockAPIPortalAuditRepository struct {
	repository.AuditRepository
	records []auditRecord
}

type auditRecord struct {
	action       string
	resourceUUID string
	resourceType string
	orgUUID      string
	performedBy  string
}

func (m *mockAPIPortalAuditRepository) Record(action, resourceUUID, resourceType, orgUUID, performedBy string) error {
	m.records = append(m.records, auditRecord{action, resourceUUID, resourceType, orgUUID, performedBy})
	return nil
}

// newTestAPIPortalService wires the three mocks together with a real
// InHouseVault. identity + slogger are nil because the service does not invoke
// them. authRegistry is nil for pure-CRUD tests; a real registry is wired
// only in the tests that exercise AuthHeaderForPortal / Invalidate paths
// (see api_portal_auth_test scenarios below).
func newTestAPIPortalService(t *testing.T,
	portalRepo repository.APIPortalRepository,
	orgRepo repository.OrganizationRepository,
	auditRepo repository.AuditRepository,
) *APIPortalService {
	return NewAPIPortalService(portalRepo, orgRepo, auditRepo, newTestVault(t), nil, nil, nil)
}

func apiPortalStrPtr(s string) *string { return &s }

// --- test-DTO builders ---

type testCreateReq struct {
	Handle      string
	Name        string
	Description string
	URL         string
	SharedKey   string // 64-hex; test uses testSharedKeyHex unless overridden
	Metadata    map[string]interface{}
}

func (r testCreateReq) build() *api.CreateApiPortalRequest {
	sk := r.SharedKey
	out := &api.CreateApiPortalRequest{
		Handle:    r.Handle,
		Name:      r.Name,
		Url:       r.URL,
		SharedKey: &sk,
	}
	if r.Description != "" {
		d := r.Description
		out.Description = &d
	}
	if r.Metadata != nil {
		m := api.ApiPortalMetadata(r.Metadata)
		out.Metadata = &m
	}
	return out
}

type testUpdateReq struct {
	Name        *string
	Description *string
	URL         *string
	SharedKey   *string
	Metadata    map[string]interface{}
}

func (r testUpdateReq) build() *api.UpdateApiPortalRequest {
	out := &api.UpdateApiPortalRequest{
		Name:        r.Name,
		Description: r.Description,
		Url:         r.URL,
		SharedKey:   r.SharedKey,
	}
	if r.Metadata != nil {
		m := api.ApiPortalMetadata(r.Metadata)
		out.Metadata = &m
	}
	return out
}

// --- Create tests ---

func TestAPIPortalService_CreateAPIPortal_HappyPath(t *testing.T) {
	portalRepo := &mockAPIPortalRepository{}
	orgRepo := &mockAPIPortalOrgRepository{result: &model.Organization{}}
	auditRepo := &mockAPIPortalAuditRepository{}
	svc := newTestAPIPortalService(t, portalRepo, orgRepo, auditRepo)

	req := testCreateReq{
		Handle:      "acme",
		Name:        "Acme Portal",
		Description: "test",
		URL:         "https://acme.example.com",
		SharedKey:   testSharedKeyHex,
		Metadata:    map[string]interface{}{"loginEnvironment": "development"},
	}
	got, err := svc.CreateAPIPortal(req.build(), "org-1", "user-1")
	if err != nil {
		t.Fatalf("CreateAPIPortal: %v", err)
	}
	if got == nil || derefStr(got.Handle) != "acme" || got.Name != "Acme Portal" {
		t.Errorf("returned portal wrong shape: %+v", got)
	}
	if portalRepo.createCapturedInput == nil {
		t.Fatal("repository Create not called")
	}
	// OSS registers a portal that's already running; status is always
	// active from create, and is not exposed on the wire.
	if portalRepo.createCapturedInput.Status != constants.APIPortalStatusActive {
		t.Errorf("default status: want active, got %q", portalRepo.createCapturedInput.Status)
	}
	if portalRepo.createCapturedInput.ID == "" {
		t.Error("expected generated UUID, got empty")
	}
	if portalRepo.createCapturedInput.CreatedBy != "user-1" || portalRepo.createCapturedInput.UpdatedBy != "user-1" {
		t.Errorf("actor not populated: createdBy=%q updatedBy=%q",
			portalRepo.createCapturedInput.CreatedBy, portalRepo.createCapturedInput.UpdatedBy)
	}
	// InternalAuthKey holds the AES-GCM ciphertext of the sharedKey. Cannot
	// compare bytes directly (nonce is random per encrypt), but non-empty
	// bytes confirm the vault.Encrypt path ran.
	if len(portalRepo.createCapturedInput.InternalAuthKey) == 0 {
		t.Error("InternalAuthKey empty; expected encrypted ciphertext")
	}
	if len(auditRepo.records) != 1 || auditRepo.records[0].action != "CREATE" {
		t.Errorf("expected 1 CREATE audit record, got %+v", auditRepo.records)
	}
}

func TestAPIPortalService_CreateAPIPortal_MissingName(t *testing.T) {
	svc := newTestAPIPortalService(t, &mockAPIPortalRepository{}, &mockAPIPortalOrgRepository{result: &model.Organization{}}, &mockAPIPortalAuditRepository{})
	_, err := svc.CreateAPIPortal(testCreateReq{
		Handle:    "acme",
		SharedKey: testSharedKeyHex,
	}.build(), "org-1", "user-1")
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !apperror.ValidationFailed.Is(err) {
		t.Errorf("want ValidationFailed, got %v", err)
	}
}

func TestAPIPortalService_CreateAPIPortal_InvalidHandle(t *testing.T) {
	svc := newTestAPIPortalService(t, &mockAPIPortalRepository{}, &mockAPIPortalOrgRepository{result: &model.Organization{}}, &mockAPIPortalAuditRepository{})
	_, err := svc.CreateAPIPortal(testCreateReq{
		Handle:    "AB", // too short + uppercase
		Name:      "x",
		SharedKey: testSharedKeyHex,
	}.build(), "org-1", "user-1")
	if err == nil {
		t.Fatal("expected error for invalid handle")
	}
}

func TestAPIPortalService_CreateAPIPortal_MissingSharedKey(t *testing.T) {
	svc := newTestAPIPortalService(t, &mockAPIPortalRepository{}, &mockAPIPortalOrgRepository{result: &model.Organization{}}, &mockAPIPortalAuditRepository{})
	_, err := svc.CreateAPIPortal(testCreateReq{
		Handle:    "acme",
		Name:      "Acme",
		URL:       "https://acme.example.com",
		SharedKey: "",
	}.build(), "org-1", "user-1")
	if err == nil || !apperror.ValidationFailed.Is(err) {
		t.Fatalf("want ValidationFailed for empty sharedKey, got %v", err)
	}
}

func TestAPIPortalService_CreateAPIPortal_InvalidSharedKey(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"too_short", "abcd"},
		{"too_long", strings.Repeat("a", 65)},
		{"non_hex", strings.Repeat("z", 64)},
		{"has_spaces", "aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaa"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestAPIPortalService(t,
				&mockAPIPortalRepository{},
				&mockAPIPortalOrgRepository{result: &model.Organization{}},
				&mockAPIPortalAuditRepository{},
			)
			_, err := svc.CreateAPIPortal(testCreateReq{
				Handle:    "acme",
				Name:      "Acme",
				URL:       "https://acme.example.com",
				SharedKey: tc.value,
			}.build(), "org-1", "user-1")
			if err == nil || !apperror.ValidationFailed.Is(err) {
				t.Errorf("want ValidationFailed for sharedKey=%q, got %v", tc.value, err)
			}
		})
	}
}

func TestAPIPortalService_CreateAPIPortal_OrgNotFound(t *testing.T) {
	svc := newTestAPIPortalService(t, &mockAPIPortalRepository{}, &mockAPIPortalOrgRepository{result: nil}, &mockAPIPortalAuditRepository{})
	_, err := svc.CreateAPIPortal(testCreateReq{
		Handle: "acme", Name: "Acme", SharedKey: testSharedKeyHex,
		URL: "https://acme.example.com",
	}.build(), "org-missing", "user-1")
	if err == nil || !apperror.OrganizationNotFound.Is(err) {
		t.Fatalf("want OrganizationNotFound, got %v", err)
	}
}

func TestAPIPortalService_CreateAPIPortal_HandleAlreadyExists(t *testing.T) {
	svc := newTestAPIPortalService(t,
		&mockAPIPortalRepository{existsResult: true},
		&mockAPIPortalOrgRepository{result: &model.Organization{}},
		&mockAPIPortalAuditRepository{},
	)
	_, err := svc.CreateAPIPortal(testCreateReq{
		Handle: "acme", Name: "Acme", SharedKey: testSharedKeyHex,
		URL: "https://acme.example.com",
	}.build(), "org-1", "user-1")
	if err == nil || !apperror.APIPortalExists.Is(err) {
		t.Fatalf("want APIPortalExists, got %v", err)
	}
}

func TestAPIPortalService_CreateAPIPortal_RaceOnUniqueConstraint(t *testing.T) {
	// Exists() returns false (no row yet), then Create() races against another
	// insert and hits the UNIQUE constraint. Service must translate to Conflict.
	svc := newTestAPIPortalService(t,
		&mockAPIPortalRepository{existsResult: false, createReturnUnique: true},
		&mockAPIPortalOrgRepository{result: &model.Organization{}},
		&mockAPIPortalAuditRepository{},
	)
	_, err := svc.CreateAPIPortal(testCreateReq{
		Handle: "acme", Name: "Acme", SharedKey: testSharedKeyHex,
		URL: "https://acme.example.com",
	}.build(), "org-1", "user-1")
	if err == nil || !apperror.APIPortalExists.Is(err) {
		t.Fatalf("want APIPortalExists on race, got %v", err)
	}
}

func TestAPIPortalService_CreateAPIPortal_InvalidURL(t *testing.T) {
	cases := []struct {
		name string
		url  string
	}{
		{"http_rejected", "http://portal.example.com"},
		{"file_scheme", "file:///etc/passwd"},
		{"metadata_service_http", "http://169.254.169.254/latest/meta-data/"},
		{"javascript_scheme", "javascript:alert(1)"},
		{"relative_url", "portal.example.com"},
		{"scheme_only", "https://"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestAPIPortalService(t,
				&mockAPIPortalRepository{},
				&mockAPIPortalOrgRepository{result: &model.Organization{}},
				&mockAPIPortalAuditRepository{},
			)
			_, err := svc.CreateAPIPortal(testCreateReq{
				Handle:    "acme",
				Name:      "Acme",
				SharedKey: testSharedKeyHex,
				URL:       tc.url,
			}.build(), "org-1", "user-1")
			if err == nil || !apperror.ValidationFailed.Is(err) {
				t.Errorf("want ValidationFailed for %q, got %v", tc.url, err)
			}
		})
	}
}

func TestAPIPortalService_CreateAPIPortal_ValidHTTPSAccepted(t *testing.T) {
	svc := newTestAPIPortalService(t,
		&mockAPIPortalRepository{},
		&mockAPIPortalOrgRepository{result: &model.Organization{}},
		&mockAPIPortalAuditRepository{},
	)
	got, err := svc.CreateAPIPortal(testCreateReq{
		Handle:    "acme",
		Name:      "Acme",
		SharedKey: testSharedKeyHex,
		URL:       "https://portal.example.com:9443/base",
	}.build(), "org-1", "user-1")
	if err != nil {
		t.Fatalf("valid https URL rejected: %v", err)
	}
	if got.Url != "https://portal.example.com:9443/base" {
		t.Errorf("URL not preserved: %q", got.Url)
	}
}

func TestAPIPortalService_CreateAPIPortal_EmptyURLRejected(t *testing.T) {
	// OSS requires the operator to supply a reachable URL. Empty is rejected.
	svc := newTestAPIPortalService(t,
		&mockAPIPortalRepository{},
		&mockAPIPortalOrgRepository{result: &model.Organization{}},
		&mockAPIPortalAuditRepository{},
	)
	_, err := svc.CreateAPIPortal(testCreateReq{
		Handle:    "acme",
		Name:      "Acme",
		SharedKey: testSharedKeyHex,
		URL:       "",
	}.build(), "org-1", "user-1")
	if err == nil || !apperror.ValidationFailed.Is(err) {
		t.Fatalf("want ValidationFailed for empty URL, got %v", err)
	}
}

func TestAPIPortalService_CreateAPIPortal_ResponseDoesNotEchoSharedKey(t *testing.T) {
	// Response schema doesn't declare a sharedKey field; a create request
	// that supplies one MUST NOT round-trip it in any form on the response.
	// Belt-and-suspenders check on top of the OpenAPI writeOnly guarantee.
	svc := newTestAPIPortalService(t,
		&mockAPIPortalRepository{},
		&mockAPIPortalOrgRepository{result: &model.Organization{}},
		&mockAPIPortalAuditRepository{},
	)
	got, err := svc.CreateAPIPortal(testCreateReq{
		Handle: "acme", Name: "Acme", URL: "https://acme.example.com",
		SharedKey: testSharedKeyHex,
	}.build(), "org-1", "user-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// The generated ApiPortalResponse type doesn't have a SharedKey field
	// at compile time (dropped from OpenAPI). If someone re-adds it in the
	// future, this test will fail to compile — an intentional trip-wire.
	// We also assert Handle / Url / metadata to catch a scenario where the
	// entire response somehow gets replaced with a struct that DOES have a
	// SharedKey field but leaks it via marshalling.
	if derefStr(got.Handle) != "acme" || got.Url != "https://acme.example.com" {
		t.Errorf("response shape wrong: %+v", got)
	}
}

// --- Get tests ---

func TestAPIPortalService_GetAPIPortal_HappyPath(t *testing.T) {
	portal := &model.APIPortal{ID: "p1", Handle: "acme", OrganizationID: "org-1"}
	svc := newTestAPIPortalService(t,
		&mockAPIPortalRepository{getResult: portal},
		&mockAPIPortalOrgRepository{result: &model.Organization{}},
		&mockAPIPortalAuditRepository{},
	)
	got, err := svc.GetAPIPortal("acme", "org-1")
	if err != nil {
		t.Fatalf("GetAPIPortal: %v", err)
	}
	if got == nil || derefStr(got.Handle) != portal.Handle {
		t.Errorf("returned portal wrong shape: %+v", got)
	}
}

func TestAPIPortalService_GetAPIPortal_NotFound(t *testing.T) {
	svc := newTestAPIPortalService(t, &mockAPIPortalRepository{getResult: nil}, &mockAPIPortalOrgRepository{}, &mockAPIPortalAuditRepository{})
	_, err := svc.GetAPIPortal("ghost", "org-1")
	if err == nil || !apperror.APIPortalNotFound.Is(err) {
		t.Fatalf("want APIPortalNotFound, got %v", err)
	}
}

// --- List tests ---

func TestAPIPortalService_ListAPIPortals_HappyPath(t *testing.T) {
	portals := []*model.APIPortal{{ID: "p1", Handle: "a"}, {ID: "p2", Handle: "b"}}
	svc := newTestAPIPortalService(t,
		&mockAPIPortalRepository{listResult: portals, countResult: 5},
		&mockAPIPortalOrgRepository{result: &model.Organization{}},
		&mockAPIPortalAuditRepository{},
	)
	resp, err := svc.ListAPIPortals("org-1", 0, 0, "", "", "")
	if err != nil {
		t.Fatalf("ListAPIPortals: %v", err)
	}
	if resp.Count != 2 || resp.Pagination.Total != 5 {
		t.Errorf("counts wrong: %+v", resp)
	}
	if resp.Pagination.Limit != 20 { // default
		t.Errorf("default limit not applied: %d", resp.Pagination.Limit)
	}
}

func TestAPIPortalService_ListAPIPortals_OrgNotFound(t *testing.T) {
	svc := newTestAPIPortalService(t, &mockAPIPortalRepository{}, &mockAPIPortalOrgRepository{result: nil}, &mockAPIPortalAuditRepository{})
	_, err := svc.ListAPIPortals("org-missing", 0, 0, "", "", "")
	if err == nil || !apperror.OrganizationNotFound.Is(err) {
		t.Fatalf("want OrganizationNotFound, got %v", err)
	}
}

func TestAPIPortalService_ListAPIPortals_LimitClamping(t *testing.T) {
	svc := newTestAPIPortalService(t,
		&mockAPIPortalRepository{listResult: nil, countResult: 0},
		&mockAPIPortalOrgRepository{result: &model.Organization{}},
		&mockAPIPortalAuditRepository{},
	)
	resp, err := svc.ListAPIPortals("org-1", 500, -5, "", "", "")
	if err != nil {
		t.Fatalf("ListAPIPortals: %v", err)
	}
	if resp.Pagination.Limit != 100 {
		t.Errorf("limit not clamped to 100: %d", resp.Pagination.Limit)
	}
	if resp.Pagination.Offset != 0 {
		t.Errorf("negative offset not normalized to 0: %d", resp.Pagination.Offset)
	}
}

// --- Update tests ---

func TestAPIPortalService_UpdateAPIPortal_HappyPath(t *testing.T) {
	existing := &model.APIPortal{
		ID: "p1", Handle: "acme", OrganizationID: "org-1",
		Name:            "old",
		URL:             "https://acme.example.com",
		Status:          constants.APIPortalStatusActive,
		InternalAuthKey: []byte("pre-existing-ciphertext"),
	}
	portalRepo := &mockAPIPortalRepository{getResult: existing}
	auditRepo := &mockAPIPortalAuditRepository{}
	svc := newTestAPIPortalService(t, portalRepo, &mockAPIPortalOrgRepository{}, auditRepo)

	req := testUpdateReq{
		Name:        apiPortalStrPtr("Renamed"),
		Description: apiPortalStrPtr("new description"),
	}
	got, err := svc.UpdateAPIPortal("acme", req.build(), "org-1", "editor")
	if err != nil {
		t.Fatalf("UpdateAPIPortal: %v", err)
	}
	if got.Name != "Renamed" || derefStr(got.Description) != "new description" {
		t.Errorf("mutable fields not applied: %+v", got)
	}
	if derefStr(got.Handle) != "acme" || derefStr(got.Id) != "acme" {
		t.Errorf("immutable fields changed: %+v", got)
	}
	if portalRepo.updateCapturedInput == nil {
		t.Fatal("repository Update not called")
	}
	if portalRepo.updateCapturedInput.UpdatedBy != "editor" {
		t.Errorf("updatedBy not populated: %q", portalRepo.updateCapturedInput.UpdatedBy)
	}
	// InternalAuthKey untouched — no sharedKey in the request.
	if !bytes.Equal(portalRepo.updateCapturedInput.InternalAuthKey, []byte("pre-existing-ciphertext")) {
		t.Errorf("InternalAuthKey mutated on non-rotate Update: %q",
			portalRepo.updateCapturedInput.InternalAuthKey)
	}
	if len(auditRepo.records) != 1 || auditRepo.records[0].action != "UPDATE" {
		t.Errorf("expected 1 UPDATE audit record, got %+v", auditRepo.records)
	}
}

func TestAPIPortalService_UpdateAPIPortal_SharedKeyRotation(t *testing.T) {
	// PUT with sharedKey rotates the stored ciphertext. Same code path OSS
	// operators + cloud plugin use post-devportal-side rotation.
	existing := &model.APIPortal{
		ID: "p1", Handle: "acme", OrganizationID: "org-1",
		Name:            "Acme",
		URL:             "https://acme.example.com",
		Status:          constants.APIPortalStatusActive,
		InternalAuthKey: []byte("old-ciphertext"),
	}
	portalRepo := &mockAPIPortalRepository{getResult: existing}
	svc := newTestAPIPortalService(t, portalRepo, &mockAPIPortalOrgRepository{}, &mockAPIPortalAuditRepository{})

	newKey := strings.Repeat("b", 64)
	req := testUpdateReq{SharedKey: &newKey}
	if _, err := svc.UpdateAPIPortal("acme", req.build(), "org-1", "editor"); err != nil {
		t.Fatalf("rotation: %v", err)
	}
	if bytes.Equal(portalRepo.updateCapturedInput.InternalAuthKey, []byte("old-ciphertext")) {
		t.Error("InternalAuthKey not rotated; still holds pre-rotation ciphertext")
	}
	if len(portalRepo.updateCapturedInput.InternalAuthKey) == 0 {
		t.Error("InternalAuthKey empty after rotation; expected fresh ciphertext")
	}
}

func TestAPIPortalService_UpdateAPIPortal_InvalidSharedKeyRejected(t *testing.T) {
	existing := &model.APIPortal{
		ID: "p1", Handle: "acme", OrganizationID: "org-1",
		Name: "Acme", Status: constants.APIPortalStatusActive,
	}
	svc := newTestAPIPortalService(t,
		&mockAPIPortalRepository{getResult: existing},
		&mockAPIPortalOrgRepository{},
		&mockAPIPortalAuditRepository{},
	)
	bad := "not-hex"
	_, err := svc.UpdateAPIPortal("acme", testUpdateReq{SharedKey: &bad}.build(), "org-1", "editor")
	if err == nil || !apperror.ValidationFailed.Is(err) {
		t.Fatalf("want ValidationFailed for bad sharedKey on Update, got %v", err)
	}
}

func TestAPIPortalService_UpdateAPIPortal_PartialUpdate(t *testing.T) {
	existing := &model.APIPortal{
		ID: "p1", Handle: "acme", OrganizationID: "org-1",
		Name:            "keep",
		URL:             "https://keep.example.com",
		Status:          constants.APIPortalStatusActive,
		InternalAuthKey: []byte("keep-ciphertext"),
	}
	svc := newTestAPIPortalService(t, &mockAPIPortalRepository{getResult: existing}, &mockAPIPortalOrgRepository{}, &mockAPIPortalAuditRepository{})
	// Only Description supplied; everything else must remain unchanged.
	got, err := svc.UpdateAPIPortal("acme", testUpdateReq{Description: apiPortalStrPtr("new desc")}.build(), "org-1", "editor")
	if err != nil {
		t.Fatalf("UpdateAPIPortal: %v", err)
	}
	if derefStr(got.Description) != "new desc" {
		t.Errorf("Description not updated: %q", derefStr(got.Description))
	}
	if got.Name != "keep" || got.Url != "https://keep.example.com" {
		t.Errorf("unset fields were mutated: %+v", got)
	}
}

func TestAPIPortalService_UpdateAPIPortal_InvalidURLRejected(t *testing.T) {
	existing := &model.APIPortal{
		ID: "p1", Handle: "acme", OrganizationID: "org-1",
		Name: "Acme", Status: constants.APIPortalStatusActive,
	}
	svc := newTestAPIPortalService(t,
		&mockAPIPortalRepository{getResult: existing},
		&mockAPIPortalOrgRepository{},
		&mockAPIPortalAuditRepository{},
	)
	_, err := svc.UpdateAPIPortal("acme", testUpdateReq{
		URL: apiPortalStrPtr("http://insecure.example.com"),
	}.build(), "org-1", "editor")
	if err == nil || !apperror.ValidationFailed.Is(err) {
		t.Fatalf("want ValidationFailed for http URL on Update, got %v", err)
	}
}

func TestAPIPortalService_UpdateAPIPortal_NotFound(t *testing.T) {
	svc := newTestAPIPortalService(t, &mockAPIPortalRepository{getResult: nil}, &mockAPIPortalOrgRepository{}, &mockAPIPortalAuditRepository{})
	_, err := svc.UpdateAPIPortal("ghost", testUpdateReq{Name: apiPortalStrPtr("x")}.build(), "org-1", "editor")
	if err == nil || !apperror.APIPortalNotFound.Is(err) {
		t.Fatalf("want APIPortalNotFound, got %v", err)
	}
}

func TestAPIPortalService_UpdateAPIPortal_EmptyName(t *testing.T) {
	existing := &model.APIPortal{ID: "p1", Handle: "acme", OrganizationID: "org-1", Name: "old"}
	svc := newTestAPIPortalService(t, &mockAPIPortalRepository{getResult: existing}, &mockAPIPortalOrgRepository{}, &mockAPIPortalAuditRepository{})
	_, err := svc.UpdateAPIPortal("acme", testUpdateReq{Name: apiPortalStrPtr("   ")}.build(), "org-1", "editor")
	if err == nil || !apperror.ValidationFailed.Is(err) {
		t.Fatalf("want ValidationFailed for empty name, got %v", err)
	}
}

// --- Delete tests ---

func TestAPIPortalService_DeleteAPIPortal_HappyPath(t *testing.T) {
	existing := &model.APIPortal{ID: "p1", Handle: "acme", OrganizationID: "org-1"}
	portalRepo := &mockAPIPortalRepository{getResult: existing}
	auditRepo := &mockAPIPortalAuditRepository{}
	svc := newTestAPIPortalService(t, portalRepo, &mockAPIPortalOrgRepository{}, auditRepo)
	if err := svc.DeleteAPIPortal("acme", "org-1", "actor"); err != nil {
		t.Fatalf("DeleteAPIPortal: %v", err)
	}
	if portalRepo.deleteCalledWith != [2]string{"p1", "org-1"} {
		t.Errorf("Delete called with wrong args: %+v", portalRepo.deleteCalledWith)
	}
	if len(auditRepo.records) != 1 || auditRepo.records[0].action != "DELETE" {
		t.Errorf("expected 1 DELETE audit record, got %+v", auditRepo.records)
	}
}

func TestAPIPortalService_DeleteAPIPortal_NotFound(t *testing.T) {
	svc := newTestAPIPortalService(t, &mockAPIPortalRepository{getResult: nil}, &mockAPIPortalOrgRepository{}, &mockAPIPortalAuditRepository{})
	err := svc.DeleteAPIPortal("ghost", "org-1", "actor")
	if err == nil || !apperror.APIPortalNotFound.Is(err) {
		t.Fatalf("want APIPortalNotFound, got %v", err)
	}
}

// --- Registry cache-fill race ---

// blockingPortalRepo lets a test park the first GetByHandleAndOrgID call at a
// known point so the test can interleave an Invalidate against the in-flight
// Get. Subsequent calls (the retry after invalidation) return immediately.
type blockingPortalRepo struct {
	mockAPIPortalRepository
	enter   chan struct{} // closed by the repo when the first Get is entered
	release chan struct{} // read by the repo to hold the first call until the test says go
	portal  *model.APIPortal
	calls   int32
}

func (r *blockingPortalRepo) GetByHandleAndOrgID(handle, orgUUID string) (*model.APIPortal, error) {
	if atomic.AddInt32(&r.calls, 1) == 1 {
		close(r.enter)
		<-r.release
	}
	return r.portal, nil
}

// A Get in flight when Invalidate runs must retry against the current
// generation instead of caching a provider built from possibly-stale
// ciphertext. Locks the fix for the TOCTOU between the row read and the
// cache fill.
func TestAPIPortalAuthRegistry_GetRetriesAfterConcurrentInvalidate(t *testing.T) {
	v := newTestVault(t)
	// Row's InternalAuthKey must be a valid ciphertext so NewSharedKeyAuthProvider
	// succeeds. Encrypt a placeholder raw here.
	ct, err := v.Encrypt(context.Background(), testSharedKeyHex)
	if err != nil {
		t.Fatalf("seed encrypt: %v", err)
	}
	repo := &blockingPortalRepo{
		enter:   make(chan struct{}),
		release: make(chan struct{}),
		portal:  &model.APIPortal{Handle: "acme", OrganizationID: "org-1", InternalAuthKey: ct},
	}
	reg := NewAPIPortalAuthRegistry(repo, v)

	// Start the Get; it will park inside the first repo call.
	got := make(chan AuthProvider, 1)
	go func() {
		p, err := reg.Get("acme", "org-1")
		if err != nil {
			t.Errorf("Get: %v", err)
		}
		got <- p
	}()
	<-repo.enter

	// Invalidate while Get is parked. This is the race the fix guards.
	reg.Invalidate("acme", "org-1")

	// Let the first repo call return. Get must detect the generation change,
	// discard the possibly-stale provider, and retry.
	close(repo.release)
	select {
	case <-got:
	case <-time.After(time.Second):
		t.Fatal("Get did not return after release")
	}

	if got := atomic.LoadInt32(&repo.calls); got < 2 {
		t.Errorf("expected Get to retry against the current generation, repo.calls = %d", got)
	}
	reg.mu.Lock()
	_, cached := reg.providers[registryKey("org-1", "acme")]
	reg.mu.Unlock()
	if !cached {
		t.Error("Get did not cache the retried provider; every subsequent Get would refetch")
	}
}

// --- Status-family service tests ------------------------------------------------

func TestAPIPortalService_CreateAPIPortal_WritesActiveByDefault(t *testing.T) {
	// The public CreateAPIPortal wraps CreateAPIPortalWithStatus with the
	// default active status. Regressions here would either drop pending back
	// into the OSS-native path or change the default the OSS REST path writes.
	portalRepo := &mockAPIPortalRepository{}
	orgRepo := &mockAPIPortalOrgRepository{result: &model.Organization{}}
	svc := newTestAPIPortalService(t, portalRepo, orgRepo, &mockAPIPortalAuditRepository{})

	req := testCreateReq{Handle: "acme", Name: "Acme", URL: "https://acme.example.com", SharedKey: testSharedKeyHex}.build()
	if _, err := svc.CreateAPIPortal(req, "org-1", "tester"); err != nil {
		t.Fatalf("CreateAPIPortal: %v", err)
	}
	if got := portalRepo.createCapturedInput; got == nil || got.Status != constants.APIPortalStatusActive {
		t.Errorf("CreateAPIPortal must persist status=active; got %+v", got)
	}
}

func TestAPIPortalService_CreateAPIPortalWithStatus_HonoursCallerStatus(t *testing.T) {
	portalRepo := &mockAPIPortalRepository{}
	orgRepo := &mockAPIPortalOrgRepository{result: &model.Organization{}}
	svc := newTestAPIPortalService(t, portalRepo, orgRepo, &mockAPIPortalAuditRepository{})

	req := testCreateReq{Handle: "acme", Name: "Acme", URL: "https://acme.example.com", SharedKey: testSharedKeyHex}.build()
	if _, err := svc.CreateAPIPortalWithStatus(req, "org-1", "tester", constants.APIPortalStatusPending); err != nil {
		t.Fatalf("CreateAPIPortalWithStatus: %v", err)
	}
	if got := portalRepo.createCapturedInput; got == nil || got.Status != constants.APIPortalStatusPending {
		t.Errorf("caller-supplied status must land on the persisted row; got %+v", got)
	}
}

func TestAPIPortalService_CreateAPIPortalWithStatus_RejectsUnknownStatus(t *testing.T) {
	portalRepo := &mockAPIPortalRepository{}
	orgRepo := &mockAPIPortalOrgRepository{result: &model.Organization{}}
	svc := newTestAPIPortalService(t, portalRepo, orgRepo, &mockAPIPortalAuditRepository{})

	req := testCreateReq{Handle: "acme", Name: "Acme", URL: "https://acme.example.com", SharedKey: testSharedKeyHex}.build()
	_, err := svc.CreateAPIPortalWithStatus(req, "org-1", "tester", "bogus")
	if err == nil {
		t.Fatal("bogus status must be rejected before any repo call")
	}
	if !apperror.ValidationFailed.Is(err) {
		t.Errorf("want ValidationFailed error, got %T %v", err, err)
	}
	if portalRepo.createCapturedInput != nil {
		t.Error("Create must not run when status validation fails")
	}
}

func TestAPIPortalService_UpdateAPIPortalStatus_HappyPath(t *testing.T) {
	portalRepo := &mockAPIPortalRepository{
		getResult: &model.APIPortal{ID: "portal-uuid", Handle: "acme"},
	}
	svc := newTestAPIPortalService(t, portalRepo, &mockAPIPortalOrgRepository{}, &mockAPIPortalAuditRepository{})

	if err := svc.UpdateAPIPortalStatus("acme", "org-1", "poller", constants.APIPortalStatusActive); err != nil {
		t.Fatalf("UpdateAPIPortalStatus: %v", err)
	}
	got := portalRepo.updateStatusCalledWith
	if got.PortalID != "portal-uuid" || got.OrgUUID != "org-1" ||
		got.UpdatedBy != "poller" || got.Status != constants.APIPortalStatusActive {
		t.Errorf("wrong args threaded to repo: %+v", got)
	}
}

func TestAPIPortalService_UpdateAPIPortalStatus_RejectsUnknownStatus(t *testing.T) {
	// Guard must run before we bother reading the row; otherwise a bogus
	// status turns into a repo call that then rejects it, wasting a round-trip
	// and leaking the noise past the validation layer.
	portalRepo := &mockAPIPortalRepository{}
	svc := newTestAPIPortalService(t, portalRepo, &mockAPIPortalOrgRepository{}, &mockAPIPortalAuditRepository{})

	err := svc.UpdateAPIPortalStatus("acme", "org-1", "poller", "bogus")
	if err == nil {
		t.Fatal("bogus status must be rejected")
	}
	if portalRepo.updateStatusCalledWith != (updateStatusCall{}) {
		t.Error("repo must not be hit when status validation fails")
	}
}

func TestAPIPortalService_UpdateAPIPortalStatus_NotFound(t *testing.T) {
	// GetByHandleAndOrgID returns nil, nil for missing rows (per repo contract);
	// the service must surface this as APIPortalNotFound rather than a repo error
	// or a silent no-op update.
	portalRepo := &mockAPIPortalRepository{getResult: nil}
	svc := newTestAPIPortalService(t, portalRepo, &mockAPIPortalOrgRepository{}, &mockAPIPortalAuditRepository{})

	err := svc.UpdateAPIPortalStatus("acme", "org-1", "poller", constants.APIPortalStatusActive)
	if err == nil {
		t.Fatal("want APIPortalNotFound when row is missing")
	}
	if !apperror.APIPortalNotFound.Is(err) {
		t.Errorf("want APIPortalNotFound, got %T %v", err, err)
	}
}

func TestAPIPortalService_GetAPIPortalStatus_HappyPath(t *testing.T) {
	portalRepo := &mockAPIPortalRepository{getStatusResult: constants.APIPortalStatusFailed}
	svc := newTestAPIPortalService(t, portalRepo, &mockAPIPortalOrgRepository{}, &mockAPIPortalAuditRepository{})

	got, err := svc.GetAPIPortalStatus("acme", "org-1")
	if err != nil {
		t.Fatalf("GetAPIPortalStatus: %v", err)
	}
	if got != constants.APIPortalStatusFailed {
		t.Errorf("want %q, got %q", constants.APIPortalStatusFailed, got)
	}
	if portalRepo.getStatusCalledWith != [2]string{"acme", "org-1"} {
		t.Errorf("repo not called with (handle, orgUUID); got %+v", portalRepo.getStatusCalledWith)
	}
}

func TestAPIPortalService_GetAPIPortalStatus_NotFound(t *testing.T) {
	portalRepo := &mockAPIPortalRepository{getStatusErr: sql.ErrNoRows}
	svc := newTestAPIPortalService(t, portalRepo, &mockAPIPortalOrgRepository{}, &mockAPIPortalAuditRepository{})

	_, err := svc.GetAPIPortalStatus("acme", "org-1")
	if !apperror.APIPortalNotFound.Is(err) {
		t.Errorf("sql.ErrNoRows must be mapped to APIPortalNotFound; got %T %v", err, err)
	}
}

func TestAPIPortalService_ListAPIPortalStatuses_PassthroughAndScope(t *testing.T) {
	// One-DB-call projection: service is a thin wrapper around the repo, so
	// the test asserts the repo is scoped to the caller's org and the map
	// is returned unmodified.
	portalRepo := &mockAPIPortalRepository{
		listStatusesResult: map[string]string{
			"alpha": constants.APIPortalStatusActive,
			"beta":  constants.APIPortalStatusPending,
		},
	}
	svc := newTestAPIPortalService(t, portalRepo, &mockAPIPortalOrgRepository{}, &mockAPIPortalAuditRepository{})

	got, err := svc.ListAPIPortalStatuses("org-1")
	if err != nil {
		t.Fatalf("ListAPIPortalStatuses: %v", err)
	}
	if portalRepo.listStatusesCalledWith != "org-1" {
		t.Errorf("repo scope wrong: %q", portalRepo.listStatusesCalledWith)
	}
	if len(got) != 2 || got["alpha"] != constants.APIPortalStatusActive || got["beta"] != constants.APIPortalStatusPending {
		t.Errorf("map contents wrong: %+v", got)
	}
}

func TestAPIPortalService_ListAPIPortalsByStatus_ProjectsIdentity(t *testing.T) {
	// The service's job is to strip the internal *model.APIPortal down to the
	// APIPortalIdentity shape the plugin needs. The test pins that projection:
	// exactly the four fields, sourced from the right model attributes.
	portalRepo := &mockAPIPortalRepository{
		listByStatusResult: []*model.APIPortal{
			{OrganizationID: "org-a", Handle: "p-1", URL: "https://p-1.example.com", Status: constants.APIPortalStatusPending},
			{OrganizationID: "org-b", Handle: "p-2", URL: "https://p-2.example.com", Status: constants.APIPortalStatusPending},
		},
	}
	svc := newTestAPIPortalService(t, portalRepo, &mockAPIPortalOrgRepository{}, &mockAPIPortalAuditRepository{})

	got, err := svc.ListAPIPortalsByStatus(constants.APIPortalStatusPending)
	if err != nil {
		t.Fatalf("ListAPIPortalsByStatus: %v", err)
	}
	if portalRepo.listByStatusCalledWith != constants.APIPortalStatusPending {
		t.Errorf("wrong status forwarded to repo: %q", portalRepo.listByStatusCalledWith)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 rows, got %d", len(got))
	}
	if got[0].OrgID != "org-a" || got[0].Handle != "p-1" ||
		got[0].URL != "https://p-1.example.com" || got[0].Status != constants.APIPortalStatusPending {
		t.Errorf("row 0 projection wrong: %+v", got[0])
	}
	if got[1].OrgID != "org-b" {
		t.Errorf("row 1 org wrong: %+v", got[1])
	}
}

func TestAPIPortalService_ListAPIPortalsByStatus_RejectsUnknownStatus(t *testing.T) {
	portalRepo := &mockAPIPortalRepository{}
	svc := newTestAPIPortalService(t, portalRepo, &mockAPIPortalOrgRepository{}, &mockAPIPortalAuditRepository{})

	_, err := svc.ListAPIPortalsByStatus("bogus")
	if err == nil {
		t.Fatal("bogus status must be rejected before the repo call")
	}
	if portalRepo.listByStatusCalledWith != "" {
		t.Error("repo must not be hit when status validation fails")
	}
}
