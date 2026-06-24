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

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
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
	created      *model.LLMProviderTemplate

	managedByForHandleResult string
	managedByForHandleErr    error

	updateErr error
	updated   *model.LLMProviderTemplate

	getGroupVersionIDResult string
	getGroupVersionIDErr    error

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

	deleteErr    error
	deleteCalled bool

	deleteVersionErr    error
	deleteVersionCalled bool
}

func (m *mockLLMProviderTemplateCRUDRepo) Exists(templateID, orgUUID string) (bool, error) {
	return m.existsResult, m.existsErr
}

func (m *mockLLMProviderTemplateCRUDRepo) Create(t *model.LLMProviderTemplate) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = t
	return nil
}

func (m *mockLLMProviderTemplateCRUDRepo) ManagedByForHandle(handle, orgUUID string) (string, error) {
	return m.managedByForHandleResult, m.managedByForHandleErr
}

func (m *mockLLMProviderTemplateCRUDRepo) Update(t *model.LLMProviderTemplate) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.updated = t
	return nil
}

func (m *mockLLMProviderTemplateCRUDRepo) GetGroupVersionID(handle, orgUUID string) (string, error) {
	return m.getGroupVersionIDResult, m.getGroupVersionIDErr
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

func (m *mockLLMProviderTemplateCRUDRepo) Delete(templateID, orgUUID string) error {
	m.deleteCalled = true
	return m.deleteErr
}

func (m *mockLLMProviderTemplateCRUDRepo) DeleteVersion(templateID, orgUUID, version string) error {
	m.deleteVersionCalled = true
	return m.deleteVersionErr
}

func validTemplateRequest(name string) *api.LLMProviderTemplate {
	return &api.LLMProviderTemplate{
		Name:    name,
		Version: "v1.0",
	}
}

// ---- Create ----

func TestLLMProviderTemplateServiceCreate_Success(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{}
	svc := NewLLMProviderTemplateService(repo)

	resp, err := svc.Create("org-1", "alice", validTemplateRequest("My Custom Provider"))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp == nil || !strings.HasPrefix(resp.Id, "my-custom-provider") {
		t.Fatalf("expected handle to be derived from name, got: %#v", resp)
	}
	if repo.created == nil || repo.created.CreatedBy != "alice" {
		t.Fatalf("expected repo.Create to be called with createdBy, got: %#v", repo.created)
	}
	if repo.created.ManagedBy != "customer" {
		t.Fatalf("expected default managedBy 'customer', got: %q", repo.created.ManagedBy)
	}
}

func TestLLMProviderTemplateServiceCreate_RejectsEmptyName(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{}
	svc := NewLLMProviderTemplateService(repo)

	req := validTemplateRequest("")
	_, err := svc.Create("org-1", "alice", req)
	if err != constants.ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got: %v", err)
	}
	if repo.created != nil {
		t.Fatalf("did not expect repository create to be called")
	}
}

func TestLLMProviderTemplateServiceCreate_RejectsInvalidVersion(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{}
	svc := NewLLMProviderTemplateService(repo)

	req := validTemplateRequest("My Provider")
	req.Version = "not-a-version"
	_, err := svc.Create("org-1", "alice", req)
	if err != constants.ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got: %v", err)
	}
	if repo.created != nil {
		t.Fatalf("did not expect repository create to be called")
	}
}

func TestLLMProviderTemplateServiceCreate_ReturnsConflictForDuplicateHandle(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{existsResult: true}
	svc := NewLLMProviderTemplateService(repo)

	_, err := svc.Create("org-1", "alice", validTemplateRequest("My Provider"))
	if err != constants.ErrLLMProviderTemplateExists {
		t.Fatalf("expected ErrLLMProviderTemplateExists, got: %v", err)
	}
	if repo.created != nil {
		t.Fatalf("did not expect repository create to be called")
	}
}

// ---- Update ----

func TestLLMProviderTemplateServiceUpdate_RejectsReadOnlyBuiltin(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{}
	repo.getByIDFunc = func(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
		return &model.LLMProviderTemplate{ID: templateID, OrganizationUUID: orgUUID, ManagedBy: "wso2"}, nil
	}
	svc := NewLLMProviderTemplateService(repo)

	_, err := svc.Update("org-1", "openai", validTemplateRequest("OpenAI"))
	if err != constants.ErrLLMProviderTemplateReadOnly {
		t.Fatalf("expected ErrLLMProviderTemplateReadOnly, got: %v", err)
	}
	if repo.updated != nil {
		t.Fatalf("did not expect repository update to be called")
	}
}

func TestLLMProviderTemplateServiceUpdate_ReturnsNotFoundWhenTemplateMissing(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{}
	svc := NewLLMProviderTemplateService(repo)

	_, err := svc.Update("org-1", "does-not-exist", validTemplateRequest("Name"))
	if err != constants.ErrLLMProviderTemplateNotFound {
		t.Fatalf("expected ErrLLMProviderTemplateNotFound, got: %v", err)
	}
}

func TestLLMProviderTemplateServiceUpdate_PreservesOpenAPISpecWhenOmitted(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{}
	repo.getByIDFunc = func(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
		return &model.LLMProviderTemplate{
			ID:               templateID,
			OrganizationUUID: orgUUID,
			ManagedBy:        "customer",
			OpenAPISpec:      "openapi: 3.0.0",
			Version:          "v1.0",
		}, nil
	}
	svc := NewLLMProviderTemplateService(repo)

	req := validTemplateRequest("Renamed")
	req.Openapi = nil
	req.Provider = nil
	if _, err := svc.Update("org-1", "mistralai", req); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if repo.updated == nil || repo.updated.OpenAPISpec != "openapi: 3.0.0" {
		t.Fatalf("expected existing OpenAPI spec to be preserved, got: %#v", repo.updated)
	}
	if repo.updated.ManagedBy != "customer" {
		t.Fatalf("expected existing managedBy to be preserved, got: %q", repo.updated.ManagedBy)
	}
}

func TestLLMProviderTemplateServiceUpdate_PropagatesNameToFamily(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{
		managedByForHandleResult: "customer",
		getGroupVersionIDResult:     "mistralai",
	}
	repo.getByIDFunc = func(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
		return &model.LLMProviderTemplate{ID: templateID, OrganizationUUID: orgUUID, Name: "Mistral Updated", Version: "v1.0"}, nil
	}
	svc := NewLLMProviderTemplateService(repo)

	req := validTemplateRequest("Mistral Updated")
	resp, err := svc.Update("org-1", "mistralai", req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !repo.renameFamilyCalled || repo.renameFamilyBase != "mistralai" || repo.renameFamilyName != "Mistral Updated" {
		t.Fatalf("expected RenameFamily to be called with the family base handle and new name, got called=%v base=%q name=%q",
			repo.renameFamilyCalled, repo.renameFamilyBase, repo.renameFamilyName)
	}
	if resp == nil || resp.Name != "Mistral Updated" {
		t.Fatalf("expected updated template to be returned, got: %#v", resp)
	}
}

func TestLLMProviderTemplateServiceUpdate_RejectsMismatchedID(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{managedByForHandleResult: "customer"}
	svc := NewLLMProviderTemplateService(repo)

	req := validTemplateRequest("Name")
	req.Id = "some-other-handle"
	_, err := svc.Update("org-1", "mistralai", req)
	if err != constants.ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got: %v", err)
	}
}

// ---- CreateVersion ----

func TestLLMProviderTemplateServiceCreateVersion_Success(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{getGroupVersionIDResult: "mistralai"}
	svc := NewLLMProviderTemplateService(repo)

	req := &api.CreateLLMProviderTemplateVersionRequest{Name: "Mistral", Version: "v2.0"}
	resp, err := svc.CreateVersion("org-1", "mistralai", "test-user", req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp == nil || resp.Version != "v2.0" {
		t.Fatalf("expected new version v2.0, got: %#v", resp)
	}
	if repo.createdVersion == nil || repo.createdVersion.GroupVersionID != "mistralai" {
		t.Fatalf("expected created version to carry the family base handle, got: %#v", repo.createdVersion)
	}
	if repo.createdVersion.ManagedBy != "customer" {
		t.Fatalf("expected new custom versions to default to managedBy 'customer', got: %q", repo.createdVersion.ManagedBy)
	}
}

func TestLLMProviderTemplateServiceCreateVersion_NotFoundWhenFamilyMissing(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{getGroupVersionIDResult: ""}
	svc := NewLLMProviderTemplateService(repo)

	req := &api.CreateLLMProviderTemplateVersionRequest{Name: "Mistral", Version: "v2.0"}
	_, err := svc.CreateVersion("org-1", "does-not-exist", "test-user", req)
	if err != constants.ErrLLMProviderTemplateNotFound {
		t.Fatalf("expected ErrLLMProviderTemplateNotFound, got: %v", err)
	}
}

func TestLLMProviderTemplateServiceCreateVersion_ConflictWhenVersionExists(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{
		getGroupVersionIDResult: "mistralai",
		createNewVersionErr: constants.ErrLLMProviderTemplateVersionExists,
	}
	svc := NewLLMProviderTemplateService(repo)

	req := &api.CreateLLMProviderTemplateVersionRequest{Name: "Mistral", Version: "v1.0"}
	_, err := svc.CreateVersion("org-1", "mistralai", "test-user", req)
	if err != constants.ErrLLMProviderTemplateVersionExists {
		t.Fatalf("expected ErrLLMProviderTemplateVersionExists, got: %v", err)
	}
}

func TestLLMProviderTemplateServiceCreateVersion_RejectsInvalidVersionFormat(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{getGroupVersionIDResult: "mistralai"}
	svc := NewLLMProviderTemplateService(repo)

	req := &api.CreateLLMProviderTemplateVersionRequest{Name: "Mistral", Version: "2.0"}
	_, err := svc.CreateVersion("org-1", "mistralai", "test-user", req)
	if err != constants.ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got: %v", err)
	}
	if repo.createdVersion != nil {
		t.Fatalf("did not expect CreateNewVersion to be called")
	}
}

// ---- ListVersions / GetVersion ----

func TestLLMProviderTemplateServiceListVersions_NotFoundWhenNoVersions(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{countVersionsResult: 0}
	svc := NewLLMProviderTemplateService(repo)

	_, err := svc.ListVersions("org-1", "does-not-exist", 10, 0)
	if err != constants.ErrLLMProviderTemplateNotFound {
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
	svc := NewLLMProviderTemplateService(repo)

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
	svc := NewLLMProviderTemplateService(repo)

	_, err := svc.GetVersion("org-1", "mistralai", "v9.0")
	if err != constants.ErrLLMProviderTemplateNotFound {
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
	svc := NewLLMProviderTemplateService(repo)

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
			return &model.LLMProviderTemplate{ID: templateID, Version: version, Enabled: false}, nil
		},
	}
	svc := NewLLMProviderTemplateService(repo)

	resp, err := svc.SetVersionEnabled("org-1", "mistralai", "v1.0", false)
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
	svc := NewLLMProviderTemplateService(repo)

	_, err := svc.SetVersionEnabled("org-1", "does-not-exist", "v1.0", true)
	if err != constants.ErrLLMProviderTemplateNotFound {
		t.Fatalf("expected ErrLLMProviderTemplateNotFound, got: %v", err)
	}
}

// ---- Delete (whole template family) ----

func TestLLMProviderTemplateServiceDelete_BlocksReadOnlyBuiltin(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{managedByForHandleResult: "wso2"}
	svc := NewLLMProviderTemplateService(repo)

	err := svc.Delete("org-1", "openai")
	if err != constants.ErrLLMProviderTemplateReadOnly {
		t.Fatalf("expected ErrLLMProviderTemplateReadOnly, got: %v", err)
	}
	if repo.deleteCalled || repo.countProvidersUsingTemplateCalled {
		t.Fatalf("did not expect usage check or delete to be called for a read-only template")
	}
}

func TestLLMProviderTemplateServiceDelete_NotFound(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{managedByForHandleResult: ""}
	svc := NewLLMProviderTemplateService(repo)

	err := svc.Delete("org-1", "does-not-exist")
	if err != constants.ErrLLMProviderTemplateNotFound {
		t.Fatalf("expected ErrLLMProviderTemplateNotFound, got: %v", err)
	}
}

func TestLLMProviderTemplateServiceDelete_BlocksWhenInUse(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{
		managedByForHandleResult:           "customer",
		countProvidersUsingTemplateResult: 2,
	}
	svc := NewLLMProviderTemplateService(repo)

	err := svc.Delete("org-1", "mistralai")
	if err != constants.ErrLLMProviderTemplateInUse {
		t.Fatalf("expected ErrLLMProviderTemplateInUse, got: %v", err)
	}
	if repo.countProvidersUsingTemplateVersion != "" {
		t.Fatalf("expected usage check to span the whole family (empty version), got: %q", repo.countProvidersUsingTemplateVersion)
	}
	if repo.deleteCalled {
		t.Fatalf("did not expect repository delete to be called while template is in use")
	}
}

func TestLLMProviderTemplateServiceDelete_Success(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{
		managedByForHandleResult:           "customer",
		countProvidersUsingTemplateResult: 0,
	}
	svc := NewLLMProviderTemplateService(repo)

	if err := svc.Delete("org-1", "mistralai"); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !repo.deleteCalled {
		t.Fatalf("expected repository delete to be called")
	}
}

// ---- DeleteVersion ----

func TestLLMProviderTemplateServiceDeleteVersion_NotFoundWhenVersionMissing(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{
		getByVersionFunc: func(templateID, orgUUID, version string) (*model.LLMProviderTemplate, error) {
			return nil, nil
		},
	}
	svc := NewLLMProviderTemplateService(repo)

	err := svc.DeleteVersion("org-1", "mistralai", "v9.0")
	if err != constants.ErrLLMProviderTemplateNotFound {
		t.Fatalf("expected ErrLLMProviderTemplateNotFound, got: %v", err)
	}
}

func TestLLMProviderTemplateServiceDeleteVersion_BlocksReadOnlyBuiltin(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{
		getByVersionFunc: func(templateID, orgUUID, version string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{ID: templateID, Version: version, ManagedBy: "wso2"}, nil
		},
	}
	svc := NewLLMProviderTemplateService(repo)

	err := svc.DeleteVersion("org-1", "openai", "v1.0")
	if err != constants.ErrLLMProviderTemplateReadOnly {
		t.Fatalf("expected ErrLLMProviderTemplateReadOnly, got: %v", err)
	}
	if repo.deleteVersionCalled {
		t.Fatalf("did not expect repository delete to be called for a read-only template version")
	}
}

func TestLLMProviderTemplateServiceDeleteVersion_BlocksWhenInUse(t *testing.T) {
	repo := &mockLLMProviderTemplateCRUDRepo{
		getByVersionFunc: func(templateID, orgUUID, version string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{ID: templateID, Version: version, ManagedBy: "customer"}, nil
		},
		countProvidersUsingTemplateResult: 1,
	}
	svc := NewLLMProviderTemplateService(repo)

	err := svc.DeleteVersion("org-1", "mistralai-v2-0", "v2.0")
	if err != constants.ErrLLMProviderTemplateInUse {
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
			return &model.LLMProviderTemplate{ID: templateID, Version: version, ManagedBy: "customer"}, nil
		},
		countProvidersUsingTemplateResult: 0,
	}
	svc := NewLLMProviderTemplateService(repo)

	if err := svc.DeleteVersion("org-1", "mistralai-v2-0", "v2.0"); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !repo.deleteVersionCalled {
		t.Fatalf("expected repository DeleteVersion to be called")
	}
}
