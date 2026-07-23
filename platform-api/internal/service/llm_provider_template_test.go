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
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// mockLLMProviderTemplateCRUDRepo is a configurable fake covering the
// LLMProviderTemplateRepository methods exercised by LLMProviderTemplateService.
// Each field models the return value(s) of the corresponding repository call so
// tests can drive every branch in the service without standing up a database.
type mockLLMProviderTemplateCRUDRepo struct {
	repository.LLMProviderTemplateRepository

	existsResult bool
	existsErr    error
	createErr    error
	createCalled bool
	created      *model.LLMProviderTemplate

	updateErr error
	updated   *model.LLMProviderTemplate

	getGroupIDResult string
	getGroupIDErr    error

	managedByForGroupIDResult string
	managedByForGroupIDErr    error

	renameFamilyCalled bool
	renameFamilyBase   string
	renameFamilyOrg    string
	renameFamilyName   string
	renameFamilyErr    error

	getByIDFunc func(templateID, orgUUID string) (*model.LLMProviderTemplate, error)

	createNewVersionErr error
	createdVersion      *model.LLMProviderTemplate

	countVersionsResult int
	countVersionsErr    error
	listVersionsResult  []*model.LLMProviderTemplate
	listVersionsErr     error

	getByVersionFunc func(templateID, orgUUID, version string) (*model.LLMProviderTemplate, error)

	setEnabledErr     error
	setEnabledCalled  bool
	setEnabledVersion string
	setEnabledEnabled bool

	countProvidersUsingTemplateResult  int
	countProvidersUsingTemplateErr     error
	countProvidersUsingTemplateCalled  bool
	countProvidersUsingTemplateVersion string

	deleteVersionErr    error
	deleteVersionCalled bool
}

func (m *mockLLMProviderTemplateCRUDRepo) Exists(templateID, orgUUID string) (bool, error) {
	return m.existsResult, m.existsErr
}

func (m *mockLLMProviderTemplateCRUDRepo) Create(t *model.LLMProviderTemplate) error {
	m.createCalled = true
	if m.createErr != nil {
		return m.createErr
	}
	m.created = t
	return nil
}

func (m *mockLLMProviderTemplateCRUDRepo) Update(t *model.LLMProviderTemplate) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.updated = t
	return nil
}

func (m *mockLLMProviderTemplateCRUDRepo) GetGroupID(handle, orgUUID string) (string, error) {
	return m.getGroupIDResult, m.getGroupIDErr
}

func (m *mockLLMProviderTemplateCRUDRepo) ManagedByForGroupID(groupID, orgUUID string) (string, error) {
	return m.managedByForGroupIDResult, m.managedByForGroupIDErr
}

func (m *mockLLMProviderTemplateCRUDRepo) RenameFamily(baseHandle, orgUUID, name string) error {
	m.renameFamilyCalled = true
	m.renameFamilyBase = baseHandle
	m.renameFamilyOrg = orgUUID
	m.renameFamilyName = name
	return m.renameFamilyErr
}

func (m *mockLLMProviderTemplateCRUDRepo) GetByID(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(templateID, orgUUID)
	}
	return nil, nil
}

func (m *mockLLMProviderTemplateCRUDRepo) CreateNewVersion(t *model.LLMProviderTemplate) error {
	if m.createNewVersionErr != nil {
		return m.createNewVersionErr
	}
	m.createdVersion = t
	return nil
}

func (m *mockLLMProviderTemplateCRUDRepo) CreateImportedVersion(t *model.LLMProviderTemplate) (bool, error) {
	if m.createNewVersionErr != nil {
		return false, m.createNewVersionErr
	}
	makeLatest := true
	for _, ev := range m.listVersionsResult {
		if ev != nil && !utils.TemplateVersionNewer(t.Version, ev.Version) {
			makeLatest = false
		}
	}
	t.IsLatest = makeLatest
	m.createdVersion = t
	return makeLatest, nil
}

func (m *mockLLMProviderTemplateCRUDRepo) CountVersions(templateID, orgUUID string) (int, error) {
	return m.countVersionsResult, m.countVersionsErr
}

func (m *mockLLMProviderTemplateCRUDRepo) ListVersions(templateID, orgUUID string, limit, offset int) ([]*model.LLMProviderTemplate, error) {
	return m.listVersionsResult, m.listVersionsErr
}

func (m *mockLLMProviderTemplateCRUDRepo) GetByVersion(templateID, orgUUID, version string) (*model.LLMProviderTemplate, error) {
	if m.getByVersionFunc != nil {
		return m.getByVersionFunc(templateID, orgUUID, version)
	}
	return nil, nil
}

func (m *mockLLMProviderTemplateCRUDRepo) SetEnabled(templateID, orgUUID, version string, enabled bool) error {
	m.setEnabledCalled = true
	m.setEnabledVersion = version
	m.setEnabledEnabled = enabled
	return m.setEnabledErr
}

func (m *mockLLMProviderTemplateCRUDRepo) CountProvidersUsingTemplate(templateID, orgUUID, version string) (int, error) {
	m.countProvidersUsingTemplateCalled = true
	m.countProvidersUsingTemplateVersion = version
	return m.countProvidersUsingTemplateResult, m.countProvidersUsingTemplateErr
}

func (m *mockLLMProviderTemplateCRUDRepo) DeleteVersion(templateID, orgUUID, version string) error {
	m.deleteVersionCalled = true
	return m.deleteVersionErr
}

func validTemplateRequest(name string) *api.LLMProviderTemplate {
	endpoint := "https://api.example.com"
	return &api.LLMProviderTemplate{
		DisplayName: name,
		Version:     "v1.0",
		Metadata: &api.LLMProviderTemplateMetadata{
			EndpointUrl: &endpoint,
		},
	}
}

// ---- Create ----

func TestLLMProviderTemplateServiceCreate_Success(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	resp, err := svc.Create("org-1", "alice", validTemplateRequest("My Custom Provider"))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp == nil || resp.Id == nil || !strings.HasPrefix(*resp.Id, "my-custom-provider") {
		t.Fatalf("expected handle to be derived from name, got: %#v", resp)
	}
	if repo.created == nil || repo.created.CreatedBy != "alice" {
		t.Fatalf("expected repo.Create to be called with createdBy, got: %#v", repo.created)
	}
	if repo.created.ManagedBy != "organization" {
		t.Fatalf("expected default managedBy 'organization', got: %q", repo.created.ManagedBy)
	}
}

func TestLLMProviderTemplateServiceCreate_RewritesReservedWSO2Prefix(t *testing.T) {
	cases := map[string]string{
		"wso2 openai":   "xwso2-openai", // slugifies to "wso2-openai"
		"WSO2 Template": "xwso2-template",
		"wso2":          "xwso2",
	}
	for displayName, wantGroupID := range cases {
		repo := &mockLLMProviderTemplateCRUDRepo{}
		svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

		resp, err := svc.Create("org-1", "alice", validTemplateRequest(displayName))
		if err != nil {
			t.Fatalf("[%s] expected no error, got: %v", displayName, err)
		}
		if repo.created == nil || repo.created.GroupID != wantGroupID {
			t.Fatalf("[%s] expected reserved prefix rewrite to group_id %q, got: %#v", displayName, wantGroupID, repo.created)
		}
		if resp == nil || resp.Id == nil || !strings.HasPrefix(*resp.Id, wantGroupID) {
			t.Fatalf("[%s] expected handle derived from %q, got: %#v", displayName, wantGroupID, resp)
		}
	}
}

func TestLLMProviderTemplateServiceCreate_RejectsMissingEndpoint(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	// No metadata at all.
	req := &api.LLMProviderTemplate{DisplayName: "No Endpoint", Version: "v1.0"}
	if _, err := svc.Create("org-1", "alice", req); !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ErrInvalidInput when endpoint is missing, got: %v", err)
	}

	// Metadata present but endpointUrl blank.
	blank := "   "
	req2 := &api.LLMProviderTemplate{
		DisplayName: "Blank Endpoint",
		Version:     "v1.0",
		Metadata:    &api.LLMProviderTemplateMetadata{EndpointUrl: &blank},
	}
	if _, err := svc.Create("org-1", "alice", req2); !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ErrInvalidInput when endpoint is blank, got: %v", err)
	}
	if repo.createCalled {
		t.Fatalf("did not expect repository create to be called")
	}
}

func TestLLMProviderTemplateServiceCreate_RejectsEmptyName(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	req := validTemplateRequest("")
	_, err := svc.Create("org-1", "alice", req)
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ErrInvalidInput, got: %v", err)
	}
	if repo.createCalled {
		t.Fatalf("did not expect repository create to be called")
	}
}

func TestLLMProviderTemplateServiceCreate_RejectsInvalidVersion(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	req := validTemplateRequest("My Provider")
	req.Version = "not-a-version"
	_, err := svc.Create("org-1", "alice", req)
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ErrInvalidInput, got: %v", err)
	}
	if repo.createCalled {
		t.Fatalf("did not expect repository create to be called")
	}
}

func TestLLMProviderTemplateServiceCreate_RejectsReservedManagedBy(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	wso2 := "wso2"
	req := validTemplateRequest("My Provider")
	req.ManagedBy = &wso2
	_, err := svc.Create("org-1", "alice", req)
	if !apperror.LLMProviderTemplateManagedByReserved.Is(err) {
		t.Fatalf("expected ErrLLMProviderTemplateManagedByReserved, got: %v", err)
	}
	if repo.createCalled {
		t.Fatalf("did not expect repository create to be called")
	}
}

func TestLLMProviderTemplateServiceCreate_ReturnsConflictForDuplicateHandle(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{existsResult: true}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	_, err := svc.Create("org-1", "alice", validTemplateRequest("My Provider"))
	if !apperror.LLMProviderTemplateExists.Is(err) {
		t.Fatalf("expected ErrLLMProviderTemplateExists, got: %v", err)
	}
	if repo.createCalled {
		t.Fatalf("did not expect repository create to be called")
	}
}

// ---- Update ----

func TestLLMProviderTemplateServiceUpdate_RejectsReservedManagedBy(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	wso2 := "wso2"
	req := validTemplateRequest("My Provider")
	req.ManagedBy = &wso2
	_, err := svc.Update("org-1", "my-provider", "alice", req)
	if !apperror.LLMProviderTemplateManagedByReserved.Is(err) {
		t.Fatalf("expected ErrLLMProviderTemplateManagedByReserved, got: %v", err)
	}
	if repo.updated != nil {
		t.Fatalf("did not expect repository update to be called")
	}
}

func TestLLMProviderTemplateServiceUpdate_RejectsReadOnlyBuiltin(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{}
	repo.getByIDFunc = func(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
		return &model.LLMProviderTemplate{ID: templateID, OrganizationUUID: orgUUID, ManagedBy: "wso2"}, nil
	}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	_, err := svc.Update("org-1", "openai", "alice", validTemplateRequest("OpenAI"))
	if !apperror.LLMProviderTemplateReadOnly.Is(err) {
		t.Fatalf("expected ErrLLMProviderTemplateReadOnly, got: %v", err)
	}
	if repo.updated != nil {
		t.Fatalf("did not expect repository update to be called")
	}
}

func TestLLMProviderTemplateServiceUpdate_ReturnsNotFoundWhenTemplateMissing(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	_, err := svc.Update("org-1", "does-not-exist", "alice", validTemplateRequest("Name"))
	if !apperror.LLMProviderTemplateNotFound.Is(err) {
		t.Fatalf("expected ErrLLMProviderTemplateNotFound, got: %v", err)
	}
}

func TestLLMProviderTemplateServiceUpdate_PreservesOpenAPISpecWhenOmitted(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{}
	repo.getByIDFunc = func(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
		return &model.LLMProviderTemplate{
			ID:               templateID,
			OrganizationUUID: orgUUID,
			ManagedBy:        "organization",
			OpenAPISpec:      "openapi: 3.0.0",
			Version:          "v1.0",
		}, nil
	}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	req := validTemplateRequest("Renamed")
	req.Openapi = nil
	req.ManagedBy = nil
	if _, err := svc.Update("org-1", "mistralai", "alice", req); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if repo.updated == nil || repo.updated.OpenAPISpec != "openapi: 3.0.0" {
		t.Fatalf("expected existing OpenAPI spec to be preserved, got: %#v", repo.updated)
	}
	if repo.updated.ManagedBy != "organization" {
		t.Fatalf("expected existing managedBy to be preserved, got: %q", repo.updated.ManagedBy)
	}
}

func TestLLMProviderTemplateServiceUpdate_PropagatesNameToFamily(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{
		getGroupIDResult: "mistralai",
	}
	repo.getByIDFunc = func(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
		return &model.LLMProviderTemplate{ID: templateID, OrganizationUUID: orgUUID, Name: "Mistral Updated", Version: "v1.0"}, nil
	}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	req := validTemplateRequest("Mistral Updated")
	resp, err := svc.Update("org-1", "mistralai", "alice", req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !repo.renameFamilyCalled || repo.renameFamilyBase != "mistralai" || repo.renameFamilyName != "Mistral Updated" {
		t.Fatalf("expected RenameFamily to be called with the family base handle and new name, got called=%v base=%q name=%q",
			repo.renameFamilyCalled, repo.renameFamilyBase, repo.renameFamilyName)
	}
	if resp == nil || resp.DisplayName != "Mistral Updated" {
		t.Fatalf("expected updated template to be returned, got: %#v", resp)
	}
}

// ---- CreateVersion ----

func TestLLMProviderTemplateServiceCreateVersion_Success(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{countVersionsResult: 1, managedByForGroupIDResult: constants.TemplateManagedByOrganization}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	req := &api.CreateLLMProviderTemplateVersionRequest{DisplayName: stringPtr("Mistral"), Version: "v2.0"}
	resp, err := svc.CreateVersion("org-1", "mistralai", "test-user", req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp == nil || resp.Version != "v2.0" {
		t.Fatalf("expected new version v2.0, got: %#v", resp)
	}
	if repo.createdVersion == nil || repo.createdVersion.GroupID != "mistralai" {
		t.Fatalf("expected created version to carry the family base handle, got: %#v", repo.createdVersion)
	}
	if repo.createdVersion.ManagedBy != "organization" {
		t.Fatalf("expected new custom versions to default to managedBy 'organization', got: %q", repo.createdVersion.ManagedBy)
	}
}

func TestLLMProviderTemplateServiceCreateVersion_RejectsNewVersionOnBuiltinFamily(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{countVersionsResult: 1, managedByForGroupIDResult: constants.PolicyManagedByWSO2}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	req := &api.CreateLLMProviderTemplateVersionRequest{DisplayName: stringPtr("Mistral"), Version: "v2.0"}
	_, err := svc.CreateVersion("org-1", "wso2-mistralai", "test-user", req)
	if !apperror.LLMProviderTemplateBuiltInImmutable.Is(err) {
		t.Fatalf("expected LLMProviderTemplateBuiltInImmutable when adding a version to a built-in family, got: %v", err)
	}
	if repo.createdVersion != nil {
		t.Fatalf("expected no version to be created for a built-in family, got: %#v", repo.createdVersion)
	}
}

func TestLLMProviderTemplateServiceCreateVersion_NotFoundWhenFamilyMissing(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{countVersionsResult: 0}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	req := &api.CreateLLMProviderTemplateVersionRequest{DisplayName: stringPtr("Mistral"), Version: "v2.0"}
	_, err := svc.CreateVersion("org-1", "does-not-exist", "test-user", req)
	if !apperror.LLMProviderTemplateNotFound.Is(err) {
		t.Fatalf("expected ErrLLMProviderTemplateNotFound, got: %v", err)
	}
}

func TestLLMProviderTemplateServiceCreateVersion_ConflictWhenVersionExists(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{
		countVersionsResult: 1,
		createNewVersionErr: apperror.LLMProviderTemplateVersionExists.New(),
	}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	req := &api.CreateLLMProviderTemplateVersionRequest{DisplayName: stringPtr("Mistral"), Version: "v1.0"}
	_, err := svc.CreateVersion("org-1", "mistralai", "test-user", req)
	if !apperror.LLMProviderTemplateVersionExists.Is(err) {
		t.Fatalf("expected ErrLLMProviderTemplateVersionExists, got: %v", err)
	}
}

func TestLLMProviderTemplateServiceCreateVersion_RejectsInvalidVersionFormat(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{countVersionsResult: 1}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	req := &api.CreateLLMProviderTemplateVersionRequest{DisplayName: stringPtr("Mistral"), Version: "2.0"}
	_, err := svc.CreateVersion("org-1", "mistralai", "test-user", req)
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ErrInvalidInput, got: %v", err)
	}
	if repo.createdVersion != nil {
		t.Fatalf("did not expect CreateNewVersion to be called")
	}
}

func TestLLMProviderTemplateServiceCreateVersion_RejectsV0(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{countVersionsResult: 1}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	// Versions start at v1.0; v0.x is not creatable.
	req := &api.CreateLLMProviderTemplateVersionRequest{DisplayName: stringPtr("Mistral"), Version: "v0.0"}
	_, err := svc.CreateVersion("org-1", "mistralai", "test-user", req)
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ErrInvalidInput for v0.0, got: %v", err)
	}
	if repo.createdVersion != nil {
		t.Fatalf("did not expect CreateNewVersion to be called for v0.0")
	}
}

// ---- CopyVersion ----

func TestLLMProviderTemplateServiceCopyVersion_ClonesSourceAndOverrides(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{countVersionsResult: 1}
	desc := "original description"
	repo.getByIDFunc = func(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
		return &model.LLMProviderTemplate{
			ID:          "mistralai-v1-0",
			GroupID:     "mistralai",
			Name:        "Mistral",
			Description: desc,
			ManagedBy:   "organization",
			Version:     "v1.0",
			OpenAPISpec: "openapi: 3.0.0",
		}, nil
	}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	newDesc := "copied for v2"
	overrides := &api.CreateLLMProviderTemplateVersionRequest{Description: &newDesc}
	resp, err := svc.CopyVersion("org-1", "mistralai-v1-0", "mistralai-v2-0", "v2.0", "test-user", overrides)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp == nil || resp.Version != "v2.0" {
		t.Fatalf("expected copied version v2.0, got: %#v", resp)
	}
	if repo.createdVersion == nil {
		t.Fatal("expected CreateNewVersion to be called")
	}
	// Config copied from the source, description overridden by the body.
	if repo.createdVersion.OpenAPISpec != "openapi: 3.0.0" {
		t.Errorf("expected openapi copied from source, got: %q", repo.createdVersion.OpenAPISpec)
	}
	if repo.createdVersion.Description != newDesc {
		t.Errorf("expected description override %q, got: %q", newDesc, repo.createdVersion.Description)
	}
	if repo.createdVersion.ID != "mistralai-v2-0" {
		t.Errorf("expected derived handle mistralai-v2-0, got: %q", repo.createdVersion.ID)
	}
}

func TestLLMProviderTemplateServiceCopyVersion_NotFoundWhenSourceMissing(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{}
	repo.getByIDFunc = func(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
		return nil, nil
	}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	_, err := svc.CopyVersion("org-1", "nope-v1-0", "nope-v2-0", "v2.0", "test-user", nil)
	if !apperror.LLMProviderTemplateNotFound.Is(err) {
		t.Fatalf("expected ErrLLMProviderTemplateNotFound, got: %v", err)
	}
}

func TestLLMProviderTemplateServiceCopyVersion_RejectsMismatchedToTemplateID(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{countVersionsResult: 1}
	repo.getByIDFunc = func(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
		return &model.LLMProviderTemplate{ID: "mistralai-v1-0", GroupID: "mistralai", Name: "Mistral", Version: "v1.0"}, nil
	}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	_, err := svc.CopyVersion("org-1", "mistralai-v1-0", "other-family-v2-0", "v2.0", "test-user", nil)
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ErrInvalidInput for mismatched toTemplateId, got: %v", err)
	}
	if repo.createdVersion != nil {
		t.Fatalf("did not expect CreateNewVersion to be called")
	}
}

// ---- ListVersions / GetVersion ----

func TestLLMProviderTemplateServiceListVersions_NotFoundWhenNoVersions(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{countVersionsResult: 0}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	_, err := svc.ListVersions("org-1", "does-not-exist", 10, 0)
	if !apperror.LLMProviderTemplateNotFound.Is(err) {
		t.Fatalf("expected ErrLLMProviderTemplateNotFound, got: %v", err)
	}
}

func TestLLMProviderTemplateServiceListVersions_Success(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{
		countVersionsResult: 2,
		listVersionsResult: []*model.LLMProviderTemplate{
			{ID: "mistralai", Version: "v1.0"},
			{ID: "mistralai-v2-0", Version: "v2.0"},
		},
	}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	resp, err := svc.ListVersions("org-1", "mistralai", 10, 0)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp == nil || resp.Count != 2 || resp.Pagination.Total != 2 {
		t.Fatalf("expected two versions in response, got: %#v", resp)
	}
}

func TestLLMProviderTemplateServiceGetVersion_NotFound(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{
		getByVersionFunc: func(templateID, orgUUID, version string) (*model.LLMProviderTemplate, error) {
			return nil, nil
		},
	}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	_, err := svc.GetVersion("org-1", "mistralai", "v9.0")
	if !apperror.LLMProviderTemplateNotFound.Is(err) {
		t.Fatalf("expected ErrLLMProviderTemplateNotFound, got: %v", err)
	}
}

func TestLLMProviderTemplateServiceGetVersion_NormalizesVersionCasing(t *testing.T) {
	var receivedVersion string
	repo := &mockLLMProviderTemplateCRUDRepo{
		getByVersionFunc: func(templateID, orgUUID, version string) (*model.LLMProviderTemplate, error) {
			receivedVersion = version
			return &model.LLMProviderTemplate{ID: templateID, Version: version}, nil
		},
	}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	resp, err := svc.GetVersion("org-1", "mistralai", "V2.0")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if receivedVersion != "v2.0" {
		t.Fatalf("expected version to be normalized to v2.0, got: %q", receivedVersion)
	}
	if resp == nil || resp.Version != "v2.0" {
		t.Fatalf("expected response to carry normalized version, got: %#v", resp)
	}
}

// ---- SetVersionEnabled ----

func TestLLMProviderTemplateServiceSetVersionEnabled_Success(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{
		getByVersionFunc: func(templateID, orgUUID, version string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{ID: templateID, Version: version, ManagedBy: "wso2", Enabled: false}, nil
		},
	}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	resp, err := svc.SetVersionEnabled("org-1", "openai", "v1.0", false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !repo.setEnabledCalled || repo.setEnabledEnabled {
		t.Fatalf("expected SetEnabled to be called with enabled=false, got called=%v enabled=%v", repo.setEnabledCalled, repo.setEnabledEnabled)
	}
	if resp == nil || resp.Enabled == nil || *resp.Enabled {
		t.Fatalf("expected response to reflect disabled state, got: %#v", resp)
	}
}

func TestLLMProviderTemplateServiceSetVersionEnabled_NotFound(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{setEnabledErr: sql.ErrNoRows}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	_, err := svc.SetVersionEnabled("org-1", "does-not-exist", "v1.0", true)
	if !apperror.LLMProviderTemplateNotFound.Is(err) {
		t.Fatalf("expected ErrLLMProviderTemplateNotFound, got: %v", err)
	}
}

func TestLLMProviderTemplateServiceSetVersionEnabled_DisableBlocksWhenInUse(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{
		countProvidersUsingTemplateResult: 1,
		getByVersionFunc: func(templateID, orgUUID, version string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{ID: templateID, Version: version, ManagedBy: "wso2"}, nil
		},
	}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	_, err := svc.SetVersionEnabled("org-1", "openai", "v1.0", false)
	if !apperror.LLMProviderTemplateInUse.Is(err) {
		t.Fatalf("expected ErrLLMProviderTemplateInUse, got: %v", err)
	}
	if repo.countProvidersUsingTemplateVersion != "v1.0" {
		t.Fatalf("expected usage check scoped to the specific version, got: %q", repo.countProvidersUsingTemplateVersion)
	}
	if repo.setEnabledCalled {
		t.Fatalf("did not expect SetEnabled to be called while version is in use")
	}
}

func TestLLMProviderTemplateServiceSetVersionEnabled_EnableIgnoresUsage(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{
		countProvidersUsingTemplateResult: 5,
		getByVersionFunc: func(templateID, orgUUID, version string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{ID: templateID, Version: version, ManagedBy: "wso2", Enabled: true}, nil
		},
	}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	resp, err := svc.SetVersionEnabled("org-1", "openai", "v1.0", true)
	if err != nil {
		t.Fatalf("expected no error on enable regardless of usage, got: %v", err)
	}
	if repo.countProvidersUsingTemplateCalled {
		t.Fatalf("did not expect usage check to be called when enabling")
	}
	if resp == nil || resp.Enabled == nil || !*resp.Enabled {
		t.Fatalf("expected response to reflect enabled state, got: %#v", resp)
	}
}

func TestLLMProviderTemplateServiceSetVersionEnabled_AllowsCustomTemplate(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{
		getByVersionFunc: func(templateID, orgUUID, version string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{ID: templateID, Version: version, ManagedBy: "organization", Enabled: false}, nil
		},
	}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	resp, err := svc.SetVersionEnabled("org-1", "openai", "v2.0", false)
	if err != nil {
		t.Fatalf("expected custom template to be toggleable, got: %v", err)
	}
	if !repo.setEnabledCalled || repo.setEnabledEnabled {
		t.Fatalf("expected SetEnabled to be called with enabled=false, got called=%v enabled=%v", repo.setEnabledCalled, repo.setEnabledEnabled)
	}
	if resp == nil || resp.Enabled == nil || *resp.Enabled {
		t.Fatalf("expected response to reflect disabled state, got: %#v", resp)
	}
}

func TestLLMProviderTemplateServiceSetVersionEnabled_CustomTemplateDisableBlocksWhenInUse(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{
		countProvidersUsingTemplateResult: 1,
		getByVersionFunc: func(templateID, orgUUID, version string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{ID: templateID, Version: version, ManagedBy: "organization"}, nil
		},
	}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	_, err := svc.SetVersionEnabled("org-1", "openai", "v2.0", false)
	if !apperror.LLMProviderTemplateInUse.Is(err) {
		t.Fatalf("expected ErrLLMProviderTemplateInUse for in-use custom template, got: %v", err)
	}
	if repo.setEnabledCalled {
		t.Fatalf("did not expect SetEnabled to be called while version is in use")
	}
}

// ---- DeleteVersion ----

func TestLLMProviderTemplateServiceDeleteVersion_NotFoundWhenVersionMissing(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{
		getByVersionFunc: func(templateID, orgUUID, version string) (*model.LLMProviderTemplate, error) {
			return nil, nil
		},
	}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	err := svc.DeleteVersion("org-1", "mistralai", "v9.0")
	if !apperror.LLMProviderTemplateNotFound.Is(err) {
		t.Fatalf("expected ErrLLMProviderTemplateNotFound, got: %v", err)
	}
}

func TestLLMProviderTemplateServiceDeleteVersion_BlocksReadOnlyBuiltin(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{
		getByVersionFunc: func(templateID, orgUUID, version string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{ID: templateID, Version: version, ManagedBy: "wso2"}, nil
		},
	}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	err := svc.DeleteVersion("org-1", "openai", "v1.0")
	if !apperror.LLMProviderTemplateReadOnly.Is(err) {
		t.Fatalf("expected ErrLLMProviderTemplateReadOnly, got: %v", err)
	}
	if repo.deleteVersionCalled {
		t.Fatalf("did not expect repository delete to be called for a read-only template version")
	}
}

func TestLLMProviderTemplateServiceDeleteVersion_BlocksWhenInUse(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{
		getByVersionFunc: func(templateID, orgUUID, version string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{ID: templateID, Version: version, ManagedBy: "organization"}, nil
		},
		countProvidersUsingTemplateResult: 1,
	}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	err := svc.DeleteVersion("org-1", "mistralai-v2-0", "v2.0")
	if !apperror.LLMProviderTemplateInUse.Is(err) {
		t.Fatalf("expected ErrLLMProviderTemplateInUse, got: %v", err)
	}
	if repo.countProvidersUsingTemplateVersion != "v2.0" {
		t.Fatalf("expected usage check to be scoped to the specific version, got: %q", repo.countProvidersUsingTemplateVersion)
	}
	if repo.deleteVersionCalled {
		t.Fatalf("did not expect repository delete to be called while version is in use")
	}
}

func TestLLMProviderTemplateServiceDeleteVersion_Success(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{
		getByVersionFunc: func(templateID, orgUUID, version string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{ID: templateID, Version: version, ManagedBy: "organization"}, nil
		},
		countProvidersUsingTemplateResult: 0,
	}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	if err := svc.DeleteVersion("org-1", "mistralai-v2-0", "v2.0"); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !repo.deleteVersionCalled {
		t.Fatalf("expected repository DeleteVersion to be called")
	}
}
