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
	"errors"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/vault"
)

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
// them.
func newTestAPIPortalService(t *testing.T,
	portalRepo repository.APIPortalRepository,
	orgRepo repository.OrganizationRepository,
	auditRepo repository.AuditRepository,
) *APIPortalService {
	return NewAPIPortalService(portalRepo, orgRepo, auditRepo, newTestVault(t), nil, nil, nil)
}

func apiPortalStrPtr(s string) *string { return &s }

// --- Create tests ---

func TestAPIPortalService_CreateAPIPortal_HappyPath(t *testing.T) {
	portalRepo := &mockAPIPortalRepository{}
	orgRepo := &mockAPIPortalOrgRepository{result: &model.Organization{}}
	auditRepo := &mockAPIPortalAuditRepository{}
	svc := newTestAPIPortalService(t, portalRepo, orgRepo, auditRepo)

	req := &CreateAPIPortalRequest{
		Handle:      "acme",
		Name:        "Acme Portal",
		Description: "test",
		URL:         "https://acme.example.com",
		AuthType:    constants.APIPortalAuthTypeLocal,
		Metadata:    map[string]interface{}{"stsIssuer": "https://sts.example.com"},
	}
	got, err := svc.CreateAPIPortal(req, "org-1", "user-1")
	if err != nil {
		t.Fatalf("CreateAPIPortal: %v", err)
	}
	if got == nil || got.Handle != "acme" || got.Name != "Acme Portal" {
		t.Errorf("returned portal wrong shape: %+v", got)
	}
	// OSS registers a portal that's already running; status is always
	// active from create, and is not exposed on the wire.
	if got.Status != constants.APIPortalStatusActive {
		t.Errorf("default status: want active, got %q", got.Status)
	}
	if got.ID == "" {
		t.Error("expected generated UUID, got empty")
	}
	if got.CreatedBy != "user-1" || got.UpdatedBy != "user-1" {
		t.Errorf("actor not populated: createdBy=%q updatedBy=%q", got.CreatedBy, got.UpdatedBy)
	}
	if portalRepo.createCapturedInput == nil {
		t.Error("repository Create not called")
	}
	if len(auditRepo.records) != 1 || auditRepo.records[0].action != "CREATE" {
		t.Errorf("expected 1 CREATE audit record, got %+v", auditRepo.records)
	}
}

func TestAPIPortalService_CreateAPIPortal_MissingName(t *testing.T) {
	svc := newTestAPIPortalService(t, &mockAPIPortalRepository{}, &mockAPIPortalOrgRepository{result: &model.Organization{}}, &mockAPIPortalAuditRepository{})
	_, err := svc.CreateAPIPortal(&CreateAPIPortalRequest{
		Handle:   "acme",
		AuthType: constants.APIPortalAuthTypeLocal,
	}, "org-1", "user-1")
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !apperror.ValidationFailed.Is(err) {
		t.Errorf("want ValidationFailed, got %v", err)
	}
}

func TestAPIPortalService_CreateAPIPortal_InvalidHandle(t *testing.T) {
	svc := newTestAPIPortalService(t, &mockAPIPortalRepository{}, &mockAPIPortalOrgRepository{result: &model.Organization{}}, &mockAPIPortalAuditRepository{})
	_, err := svc.CreateAPIPortal(&CreateAPIPortalRequest{
		Handle:   "AB", // too short + uppercase
		Name:     "x",
		AuthType: constants.APIPortalAuthTypeLocal,
	}, "org-1", "user-1")
	if err == nil {
		t.Fatal("expected error for invalid handle")
	}
}

func TestAPIPortalService_CreateAPIPortal_InvalidAuthType(t *testing.T) {
	svc := newTestAPIPortalService(t, &mockAPIPortalRepository{}, &mockAPIPortalOrgRepository{result: &model.Organization{}}, &mockAPIPortalAuditRepository{})
	_, err := svc.CreateAPIPortal(&CreateAPIPortalRequest{
		Handle:   "acme",
		Name:     "Acme",
		AuthType: "bogus",
	}, "org-1", "user-1")
	if err == nil {
		t.Fatal("expected error for invalid authType")
	}
	if !apperror.ValidationFailed.Is(err) {
		t.Errorf("want ValidationFailed, got %v", err)
	}
}

func TestAPIPortalService_CreateAPIPortal_OrgNotFound(t *testing.T) {
	svc := newTestAPIPortalService(t, &mockAPIPortalRepository{}, &mockAPIPortalOrgRepository{result: nil}, &mockAPIPortalAuditRepository{})
	_, err := svc.CreateAPIPortal(&CreateAPIPortalRequest{
		Handle: "acme", Name: "Acme", AuthType: constants.APIPortalAuthTypeLocal,
		URL: "https://acme.example.com",
	}, "org-missing", "user-1")
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
	_, err := svc.CreateAPIPortal(&CreateAPIPortalRequest{
		Handle: "acme", Name: "Acme", AuthType: constants.APIPortalAuthTypeLocal,
		URL: "https://acme.example.com",
	}, "org-1", "user-1")
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
	_, err := svc.CreateAPIPortal(&CreateAPIPortalRequest{
		Handle: "acme", Name: "Acme", AuthType: constants.APIPortalAuthTypeLocal,
		URL: "https://acme.example.com",
	}, "org-1", "user-1")
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
			_, err := svc.CreateAPIPortal(&CreateAPIPortalRequest{
				Handle:   "acme",
				Name:     "Acme",
				AuthType: constants.APIPortalAuthTypeLocal,
				URL:      tc.url,
			}, "org-1", "user-1")
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
	got, err := svc.CreateAPIPortal(&CreateAPIPortalRequest{
		Handle:   "acme",
		Name:     "Acme",
		AuthType: constants.APIPortalAuthTypeLocal,
		URL:      "https://portal.example.com:9443/base",
	}, "org-1", "user-1")
	if err != nil {
		t.Fatalf("valid https URL rejected: %v", err)
	}
	if got.URL != "https://portal.example.com:9443/base" {
		t.Errorf("URL not preserved: %q", got.URL)
	}
}

func TestAPIPortalService_CreateAPIPortal_EmptyURLRejected(t *testing.T) {
	// OSS requires the operator to supply a reachable URL. Empty is rejected.
	svc := newTestAPIPortalService(t,
		&mockAPIPortalRepository{},
		&mockAPIPortalOrgRepository{result: &model.Organization{}},
		&mockAPIPortalAuditRepository{},
	)
	_, err := svc.CreateAPIPortal(&CreateAPIPortalRequest{
		Handle:   "acme",
		Name:     "Acme",
		AuthType: constants.APIPortalAuthTypeLocal,
		URL:      "",
	}, "org-1", "user-1")
	if err == nil || !apperror.ValidationFailed.Is(err) {
		t.Fatalf("want ValidationFailed for empty URL, got %v", err)
	}
}

func TestAPIPortalService_CreateAPIPortal_STSTokenURL_Rejected(t *testing.T) {
	// stsTokenUrl is the outbound target of a client_credentials request
	// carrying clientSecret; input-time checks enforce the same shape rules
	// as the portal URL (absolute, host, https, non-empty). Host-based
	// egress controls (loopback / private / metadata literal blocks,
	// DNS-based resolve-and-recheck) belong in an operator-aware shared
	// outbound HTTP client; local / on-prem deployments legitimately need
	// https://localhost or private-range addresses here.
	cases := []struct {
		name string
		url  string
	}{
		{"empty", ""},
		{"http_scheme", "http://sts.example.com/oauth2/token"},
		{"missing_scheme", "sts.example.com/oauth2/token"},
		{"file_scheme", "file:///etc/passwd"},
		{"javascript_scheme", "javascript:alert(1)"},
		{"scheme_only", "https://"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestAPIPortalService(t,
				&mockAPIPortalRepository{},
				&mockAPIPortalOrgRepository{result: &model.Organization{}},
				&mockAPIPortalAuditRepository{},
			)
			_, err := svc.CreateAPIPortal(&CreateAPIPortalRequest{
				Handle:   "acme",
				Name:     "Acme",
				URL:      "https://acme.example.com",
				AuthType: constants.APIPortalAuthTypeOAuth2,
				AuthConfig: map[string]interface{}{
					"stsTokenUrl":  tc.url,
					"clientId":     "abc",
					"clientSecret": "s3cr3t",
				},
			}, "org-1", "user-1")
			if err == nil || !apperror.ValidationFailed.Is(err) {
				t.Errorf("want ValidationFailed for stsTokenUrl=%q, got %v", tc.url, err)
			}
		})
	}
}

func TestAPIPortalService_CreateAPIPortal_STSTokenURL_Accepted(t *testing.T) {
	// Positive control: a reachable-shaped https URL is accepted.
	svc := newTestAPIPortalService(t,
		&mockAPIPortalRepository{},
		&mockAPIPortalOrgRepository{result: &model.Organization{}},
		&mockAPIPortalAuditRepository{},
	)
	_, err := svc.CreateAPIPortal(&CreateAPIPortalRequest{
		Handle:   "acme",
		Name:     "Acme",
		URL:      "https://acme.example.com",
		AuthType: constants.APIPortalAuthTypeOAuth2,
		AuthConfig: map[string]interface{}{
			"stsTokenUrl":  "https://sts.example.com/oauth2/token",
			"clientId":     "abc",
			"clientSecret": "s3cr3t",
		},
	}, "org-1", "user-1")
	if err != nil {
		t.Fatalf("valid stsTokenUrl rejected: %v", err)
	}
}

func TestAPIPortalService_UpdateAPIPortal_SwitchOAuth2ToLocal(t *testing.T) {
	// Regression: switching authType from oauth2 to local must clear the stored
	// oauth2 authConfig — otherwise the post-mutation validator rejects the
	// carried-over keys and no wire body can satisfy the request.
	existing := &model.APIPortal{
		ID: "p1", Handle: "acme", OrganizationID: "org-1",
		Name: "Acme", URL: "https://acme.example.com",
		Status:     constants.APIPortalStatusActive,
		AuthType:   constants.APIPortalAuthTypeOAuth2,
		AuthConfig: map[string]interface{}{
			"stsTokenUrl":  "https://sts.example.com/token",
			"clientId":     "abc",
			"clientSecret": "already-ciphertext-base64",
		},
	}
	svc := newTestAPIPortalService(t,
		&mockAPIPortalRepository{getResult: existing},
		&mockAPIPortalOrgRepository{},
		&mockAPIPortalAuditRepository{},
	)
	got, err := svc.UpdateAPIPortal("acme", &UpdateAPIPortalRequest{
		AuthType: apiPortalStrPtr(constants.APIPortalAuthTypeLocal),
	}, "org-1", "editor")
	if err != nil {
		t.Fatalf("switch oauth2 → local: %v", err)
	}
	if got.AuthType != constants.APIPortalAuthTypeLocal {
		t.Errorf("authType not applied: %q", got.AuthType)
	}
	if len(got.AuthConfig) != 0 {
		t.Errorf("stored authConfig not cleared on transition to local: %+v", got.AuthConfig)
	}
}

func TestAPIPortalService_UpdateAPIPortal_InvalidURLRejected(t *testing.T) {
	existing := &model.APIPortal{
		ID: "p1", Handle: "acme", OrganizationID: "org-1",
		Name: "Acme", Status: constants.APIPortalStatusActive,
		AuthType: constants.APIPortalAuthTypeLocal,
	}
	svc := newTestAPIPortalService(t, 
		&mockAPIPortalRepository{getResult: existing},
		&mockAPIPortalOrgRepository{},
		&mockAPIPortalAuditRepository{},
	)
	_, err := svc.UpdateAPIPortal("acme", &UpdateAPIPortalRequest{
		URL: apiPortalStrPtr("http://insecure.example.com"),
	}, "org-1", "editor")
	if err == nil || !apperror.ValidationFailed.Is(err) {
		t.Fatalf("want ValidationFailed for http URL on Update, got %v", err)
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
	if got != portal {
		t.Errorf("want %p, got %p", portal, got)
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
	resp, err := svc.ListAPIPortals("org-1", APIPortalListOptions{})
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
	_, err := svc.ListAPIPortals("org-missing", APIPortalListOptions{})
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
	resp, err := svc.ListAPIPortals("org-1", APIPortalListOptions{ListOptions: repository.ListOptions{Limit: 500, Offset: -5}})
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
		Name: "old", URL: "https://acme.example.com",
		Status:   constants.APIPortalStatusPending,
		AuthType: constants.APIPortalAuthTypeLocal,
	}
	portalRepo := &mockAPIPortalRepository{getResult: existing}
	auditRepo := &mockAPIPortalAuditRepository{}
	svc := newTestAPIPortalService(t, portalRepo, &mockAPIPortalOrgRepository{}, auditRepo)

	req := &UpdateAPIPortalRequest{
		Name:     apiPortalStrPtr("Renamed"),
		AuthType: apiPortalStrPtr(constants.APIPortalAuthTypeOAuth2),
		AuthConfig: map[string]interface{}{
			"stsTokenUrl":  "https://sts.example.com/token",
			"clientId":     "abc",
			"clientSecret": "s3cr3t",
		},
	}
	got, err := svc.UpdateAPIPortal("acme", req, "org-1", "editor")
	if err != nil {
		t.Fatalf("UpdateAPIPortal: %v", err)
	}
	if got.Name != "Renamed" || got.AuthType != constants.APIPortalAuthTypeOAuth2 {
		t.Errorf("mutable fields not applied: %+v", got)
	}
	if got.Handle != "acme" || got.ID != "p1" {
		t.Errorf("immutable fields changed: %+v", got)
	}
	if got.UpdatedBy != "editor" {
		t.Errorf("updatedBy not populated: %q", got.UpdatedBy)
	}
	if portalRepo.updateCapturedInput == nil {
		t.Error("repository Update not called")
	}
	if len(auditRepo.records) != 1 || auditRepo.records[0].action != "UPDATE" {
		t.Errorf("expected 1 UPDATE audit record, got %+v", auditRepo.records)
	}
}

func TestAPIPortalService_UpdateAPIPortal_PartialUpdate(t *testing.T) {
	existing := &model.APIPortal{
		ID: "p1", Handle: "acme", OrganizationID: "org-1",
		Name: "keep", URL: "https://keep.example.com",
		Status:   constants.APIPortalStatusActive,
		AuthType: constants.APIPortalAuthTypeLocal,
	}
	svc := newTestAPIPortalService(t, &mockAPIPortalRepository{getResult: existing}, &mockAPIPortalOrgRepository{}, &mockAPIPortalAuditRepository{})
	// Only Description supplied; everything else must remain unchanged.
	got, err := svc.UpdateAPIPortal("acme", &UpdateAPIPortalRequest{Description: apiPortalStrPtr("new desc")}, "org-1", "editor")
	if err != nil {
		t.Fatalf("UpdateAPIPortal: %v", err)
	}
	if got.Description != "new desc" {
		t.Errorf("Description not updated: %q", got.Description)
	}
	if got.Name != "keep" || got.URL != "https://keep.example.com" ||
		got.AuthType != constants.APIPortalAuthTypeLocal {
		t.Errorf("unset fields were mutated: %+v", got)
	}
}

func TestAPIPortalService_UpdateAPIPortal_NotFound(t *testing.T) {
	svc := newTestAPIPortalService(t, &mockAPIPortalRepository{getResult: nil}, &mockAPIPortalOrgRepository{}, &mockAPIPortalAuditRepository{})
	_, err := svc.UpdateAPIPortal("ghost", &UpdateAPIPortalRequest{Name: apiPortalStrPtr("x")}, "org-1", "editor")
	if err == nil || !apperror.APIPortalNotFound.Is(err) {
		t.Fatalf("want APIPortalNotFound, got %v", err)
	}
}

func TestAPIPortalService_UpdateAPIPortal_EmptyName(t *testing.T) {
	existing := &model.APIPortal{ID: "p1", Handle: "acme", OrganizationID: "org-1", Name: "old"}
	svc := newTestAPIPortalService(t, &mockAPIPortalRepository{getResult: existing}, &mockAPIPortalOrgRepository{}, &mockAPIPortalAuditRepository{})
	_, err := svc.UpdateAPIPortal("acme", &UpdateAPIPortalRequest{Name: apiPortalStrPtr("   ")}, "org-1", "editor")
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
