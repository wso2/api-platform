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
	"reflect"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// mockAPIRepository is a mock implementation of the APIRepository interface
type mockAPIRepository struct {
	repository.APIRepository // Embed interface for unimplemented methods

	// Mock behavior configuration
	handleExistsResult      bool
	handleExistsError       error
	nameVersionExistsResult bool
	nameVersionExistsError  error

	// Call tracking for verification
	lastExcludeHandle string
	created           *model.API
	updated           *model.API
	getByUUIDFunc     func(apiUUID, orgUUID string) (*model.API, error)
}

func (m *mockAPIRepository) CheckAPIExistsByHandleInOrganization(handle, orgUUID string) (bool, error) {
	return m.handleExistsResult, m.handleExistsError
}

func (m *mockAPIRepository) CheckAPIExistsByNameAndVersionInOrganization(name, version, orgUUID, excludeHandle string) (bool, error) {
	m.lastExcludeHandle = excludeHandle // Track for verification
	return m.nameVersionExistsResult, m.nameVersionExistsError
}

func (m *mockAPIRepository) CreateAPI(api *model.API) error {
	m.created = api
	return nil
}

func (m *mockAPIRepository) GetAPIByUUID(apiUUID, orgUUID string) (*model.API, error) {
	if m.getByUUIDFunc != nil {
		return m.getByUUIDFunc(apiUUID, orgUUID)
	}
	return nil, nil
}

func (m *mockAPIRepository) UpdateAPI(api *model.API) error {
	m.updated = api
	return nil
}

// TestValidateUpdateAPIRequest tests the validateUpdateAPIRequest method
func TestValidateUpdateAPIRequest(t *testing.T) {
	tests := []struct {
		name                      string
		existingAPI               *model.API
		req                       *api.RESTAPI
		mockHandleExists          bool
		mockHandleError           error
		mockNameVersionExists     bool
		mockNameVersionError      error
		wantErr                   bool
		expectedErr               apperror.Def
		errContains               string
		expectedExcludeHandle     string
		verifyExcludeHandleCalled bool
	}{
		{
			name: "valid update - no changes",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:     &api.RESTAPI{},
			wantErr: false,
		},
		{
			name: "name update - excludes current API handle from duplicate check",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:                       &api.RESTAPI{DisplayName: "Updated Name"},
			mockNameVersionExists:     false,
			wantErr:                   false,
			expectedExcludeHandle:     "my-api",
			verifyExcludeHandleCalled: true,
		},
		{
			name: "name update - conflict with different API",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:                   &api.RESTAPI{DisplayName: "Conflicting Name"},
			mockNameVersionExists: true,
			wantErr:               true,
			expectedErr:           apperror.RESTAPIExists,
		},
		{
			name: "invalid lifecycle state",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:         &api.RESTAPI{LifeCycleStatus: statusPtr("INVALID_STATE")},
			wantErr:     true,
			expectedErr: apperror.ValidationFailed,
		},
		{
			name: "invalid api type",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:         &api.RESTAPI{Kind: ptr("INVALID_TYPE")},
			wantErr:     true,
			expectedErr: apperror.ValidationFailed,
		},
		{
			name: "invalid transport",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:         &api.RESTAPI{Transport: slicePtr([]string{"invalid"})},
			wantErr:     true,
			expectedErr: apperror.ValidationFailed,
		},
		{
			name: "valid lifecycle state",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:     &api.RESTAPI{LifeCycleStatus: statusPtr("PUBLISHED")},
			wantErr: false,
		},
		{
			name: "valid api type",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:     &api.RESTAPI{Kind: ptr("RestApi")},
			wantErr: false,
		},
		{
			name: "valid transport",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:     &api.RESTAPI{Transport: slicePtr([]string{"https"})},
			wantErr: false,
		},
		{
			name: "invalid operation policy version",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req: &api.RESTAPI{
				Operations: &[]api.Operation{
					{
						Request: api.OperationRequest{
							Policies: &[]api.Policy{{Name: "SET_HEADER", Version: "v1.0.0"}},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "must be major-only",
		},
		{
			name: "invalid channel policy version",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req: &api.RESTAPI{
				Channels: &[]api.Channel{
					{
						Request: api.ChannelRequest{
							Policies: &[]api.Policy{{Name: "SET_HEADER", Version: "1"}},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "must be major-only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockAPIRepository{
				handleExistsResult:      tt.mockHandleExists,
				handleExistsError:       tt.mockHandleError,
				nameVersionExistsResult: tt.mockNameVersionExists,
				nameVersionExistsError:  tt.mockNameVersionError,
			}

			service := &APIService{
				apiRepo:  mock,
				identity: newTestIdentityService(),
			}

			err := service.validateUpdateAPIRequest(tt.existingAPI, tt.req, "test-org-uuid")

			if (err != nil) != tt.wantErr {
				t.Errorf("validateUpdateAPIRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if tt.expectedErr.Code != "" && !tt.expectedErr.Is(err) {
					t.Errorf("validateUpdateAPIRequest() error = %v, expected code %s", err, tt.expectedErr.Code)
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("validateUpdateAPIRequest() error = %v, should contain %v", err, tt.errContains)
				}
			}

			if tt.verifyExcludeHandleCalled {
				if mock.lastExcludeHandle != tt.expectedExcludeHandle {
					t.Errorf("excludeHandle = %q, want %q", mock.lastExcludeHandle, tt.expectedExcludeHandle)
				}
			}
		})
	}
}

// TestValidateCreateAPIRequest tests the validateCreateAPIRequest method
func TestValidateCreateAPIRequest(t *testing.T) {
	projectID := "11111111-1111-1111-1111-111111111111"

	tests := []struct {
		name                     string
		req                      *api.CreateRESTAPIRequest
		mockHandleExists         bool
		mockHandleError          error
		mockNameVersionExists    bool
		mockNameVersionError     error
		wantErr                  bool
		expectedErr              apperror.Def
		errContains              string
		verifyExcludeHandleEmpty bool
		expectedExcludeHandle    string
	}{
		{
			name: "valid create request",
			req: &api.CreateRESTAPIRequest{
				DisplayName: "Test API",
				Context:     "/test",
				Version:     "v1",
				ProjectId:   projectID,
				Upstream:    validUpstream(),
			},
			mockNameVersionExists:    false,
			wantErr:                  false,
			verifyExcludeHandleEmpty: true,
			expectedExcludeHandle:    "",
		},
		{
			name: "handle already exists",
			req: &api.CreateRESTAPIRequest{
				Id:          ptr("my-handle"),
				DisplayName: "Test API",
				Context:     "/test",
				Version:     "v1",
				ProjectId:   projectID,
				Upstream:    validUpstream(),
			},
			mockHandleExists: true,
			wantErr:          true,
			expectedErr:      apperror.RESTAPIExists,
		},
		{
			name: "name version already exists",
			req: &api.CreateRESTAPIRequest{
				DisplayName: "Test API",
				Context:     "/test",
				Version:     "v1",
				ProjectId:   projectID,
				Upstream:    validUpstream(),
			},
			mockNameVersionExists:    true,
			wantErr:                  true,
			expectedErr:              apperror.RESTAPIExists,
			verifyExcludeHandleEmpty: true,
			expectedExcludeHandle:    "",
		},
		{
			name: "missing name",
			req: &api.CreateRESTAPIRequest{
				DisplayName: "",
				Context:     "/test",
				Version:     "v1",
				ProjectId:   projectID,
				Upstream:    validUpstream(),
			},
			wantErr:     true,
			expectedErr: apperror.ValidationFailed,
		},
		{
			name: "missing project id",
			req: &api.CreateRESTAPIRequest{
				DisplayName: "Test API",
				Context:     "/test",
				Version:     "v1",
				ProjectId:   "",
				Upstream:    validUpstream(),
			},
			wantErr:     true,
			errContains: "project id is required",
		},
		{
			name: "invalid context",
			req: &api.CreateRESTAPIRequest{
				DisplayName: "Test API",
				Context:     "invalid",
				Version:     "v1",
				ProjectId:   projectID,
				Upstream:    validUpstream(),
			},
			wantErr:     true,
			expectedErr: apperror.ValidationFailed,
		},
		{
			name: "invalid version",
			req: &api.CreateRESTAPIRequest{
				DisplayName: "Test API",
				Context:     "/test",
				Version:     "",
				ProjectId:   projectID,
				Upstream:    validUpstream(),
			},
			wantErr:     true,
			expectedErr: apperror.ValidationFailed,
		},
		{
			name: "invalid lifecycle state",
			req: &api.CreateRESTAPIRequest{
				DisplayName:     "Test API",
				Context:         "/test",
				Version:         "v1",
				ProjectId:       projectID,
				LifeCycleStatus: createStatusPtr("INVALID_STATE"),
				Upstream:        validUpstream(),
			},
			wantErr:     true,
			expectedErr: apperror.ValidationFailed,
		},
		{
			name: "invalid api type",
			req: &api.CreateRESTAPIRequest{
				DisplayName: "Test API",
				Context:     "/test",
				Version:     "v1",
				ProjectId:   projectID,
				Kind:        ptr("INVALID_TYPE"),
				Upstream:    validUpstream(),
			},
			wantErr:     true,
			expectedErr: apperror.ValidationFailed,
		},
		{
			name: "invalid transport",
			req: &api.CreateRESTAPIRequest{
				DisplayName: "Test API",
				Context:     "/test",
				Version:     "v1",
				ProjectId:   projectID,
				Transport:   slicePtr([]string{"invalid"}),
				Upstream:    validUpstream(),
			},
			wantErr:     true,
			expectedErr: apperror.ValidationFailed,
		},
		{
			name: "valid lifecycle state",
			req: &api.CreateRESTAPIRequest{
				DisplayName:     "Test API",
				Context:         "/test",
				Version:         "v1",
				ProjectId:       projectID,
				LifeCycleStatus: createStatusPtr("PUBLISHED"),
				Upstream:        validUpstream(),
			},
			mockNameVersionExists:    false,
			wantErr:                  false,
			verifyExcludeHandleEmpty: true,
			expectedExcludeHandle:    "",
		},
		{
			name: "valid api type",
			req: &api.CreateRESTAPIRequest{
				DisplayName: "Test API",
				Context:     "/test",
				Version:     "v1",
				ProjectId:   projectID,
				Kind:        ptr("RestApi"),
				Upstream:    validUpstream(),
			},
			mockNameVersionExists:    false,
			wantErr:                  false,
			verifyExcludeHandleEmpty: true,
			expectedExcludeHandle:    "",
		},
		{
			name: "valid transport",
			req: &api.CreateRESTAPIRequest{
				DisplayName: "Test API",
				Context:     "/test",
				Version:     "v1",
				ProjectId:   projectID,
				Transport:   slicePtr([]string{"https"}),
				Upstream:    validUpstream(),
			},
			mockNameVersionExists:    false,
			wantErr:                  false,
			verifyExcludeHandleEmpty: true,
			expectedExcludeHandle:    "",
		},
		{
			name: "invalid operation policy version",
			req: &api.CreateRESTAPIRequest{
				DisplayName: "Test API",
				Context:     "/test",
				Version:     "v1",
				ProjectId:   projectID,
				Upstream:    validUpstream(),
				Operations: &[]api.Operation{
					{
						Request: api.OperationRequest{
							Policies: &[]api.Policy{{Name: "SET_HEADER", Version: "v1.0.0"}},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "must be major-only",
		},
		{
			name: "invalid channel policy version",
			req: &api.CreateRESTAPIRequest{
				DisplayName: "Test API",
				Context:     "/test",
				Version:     "v1",
				ProjectId:   projectID,
				Upstream:    validUpstream(),
				Channels: &[]api.Channel{
					{
						Request: api.ChannelRequest{
							Policies: &[]api.Policy{{Name: "SET_HEADER", Version: "1"}},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "must be major-only",
		},
		{
			name: "unspecified operation policy version is allowed (gateway resolves to latest)",
			req: &api.CreateRESTAPIRequest{
				DisplayName: "Test API",
				Context:     "/test",
				Version:     "v1",
				ProjectId:   projectID,
				Upstream:    validUpstream(),
				Operations: &[]api.Operation{
					{
						Request: api.OperationRequest{
							Policies: &[]api.Policy{{Name: "SET_HEADER", Version: ""}},
						},
					},
				},
			},
			mockNameVersionExists:    false,
			wantErr:                  false,
			verifyExcludeHandleEmpty: true,
			expectedExcludeHandle:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockAPIRepository{
				handleExistsResult:      tt.mockHandleExists,
				handleExistsError:       tt.mockHandleError,
				nameVersionExistsResult: tt.mockNameVersionExists,
				nameVersionExistsError:  tt.mockNameVersionError,
			}

			service := &APIService{
				apiRepo:  mock,
				identity: newTestIdentityService(),
			}

			err := service.validateCreateAPIRequest(tt.req, "test-org-uuid")

			if (err != nil) != tt.wantErr {
				t.Errorf("validateCreateAPIRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if tt.expectedErr.Code != "" && !tt.expectedErr.Is(err) {
					t.Errorf("validateCreateAPIRequest() error = %v, expected code %s", err, tt.expectedErr.Code)
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("validateCreateAPIRequest() error = %v, should contain %v", err, tt.errContains)
				}
			}

			if tt.verifyExcludeHandleEmpty {
				if mock.lastExcludeHandle != tt.expectedExcludeHandle {
					t.Errorf("excludeHandle = %q, want %q", mock.lastExcludeHandle, tt.expectedExcludeHandle)
				}
			}
		})
	}
}

func TestApplyAPIUpdatesUpdatesPolicies(t *testing.T) {
	service := &APIService{
		apiRepo:     &mockAPIRepository{},
		projectRepo: &mockProjectRepository{projectByUUID: &model.Project{ID: "11111111-1111-1111-1111-111111111111", Handle: "test-project"}},
		apiUtil:     &utils.APIUtil{},
		identity:    newTestIdentityService(),
	}

	condition := "request.path == '/pets'"
	params := map[string]interface{}{"limit": 10}
	newPolicies := []api.Policy{
		{
			ExecutionCondition: &condition,
			Name:               "rate-limit",
			Params:             &params,
			Version:            "v1",
		},
	}
	updatedPolicies := []api.Policy{
		{
			ExecutionCondition: &condition,
			Name:               "rate-limit",
			Params:             &params,
			Version:            "v1",
		},
	}

	existing := &model.API{
		Handle:    "pets-api",
		ProjectID: "11111111-1111-1111-1111-111111111111",
		Version:   "v1",
		Configuration: model.RestAPIConfig{
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{URL: "http://backend"},
			},
			Policies: []model.Policy{
				{Name: "legacy-policy", Version: "v1"},
			},
		},
	}

	updated, err := service.applyAPIUpdates(existing, &api.RESTAPI{Policies: &newPolicies}, "org-1")
	if err != nil {
		t.Fatalf("applyAPIUpdates() error = %v", err)
	}

	if updated.Policies == nil {
		t.Fatalf("updated policies = nil, want %v", updatedPolicies)
	}
	if !reflect.DeepEqual(*updated.Policies, updatedPolicies) {
		t.Errorf("updated policies = %v, want %v", *updated.Policies, updatedPolicies)
	}
}

// TestApplyAPIUpdatesPreservesUpstreamAuthOnEmptyValue proves that updating an
// API's upstream URL without resending auth (routine — auth.value is redacted
// on GET, so a naive read-modify-write round-trip never has it to resend)
// does not wipe the stored credential.
func TestApplyAPIUpdatesPreservesUpstreamAuthOnEmptyValue(t *testing.T) {
	service := &APIService{
		apiRepo:     &mockAPIRepository{},
		projectRepo: &mockProjectRepository{projectByUUID: &model.Project{ID: "11111111-1111-1111-1111-111111111111", Handle: "test-project"}},
		apiUtil:     &utils.APIUtil{},
		identity:    newTestIdentityService(),
	}

	existing := &model.API{
		Handle:    "pets-api",
		ProjectID: "11111111-1111-1111-1111-111111111111",
		Version:   "v1",
		Configuration: model.RestAPIConfig{
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{
					URL:  "https://old-backend.internal/api",
					Auth: &model.UpstreamAuth{Type: "bearer", Header: "Authorization", Value: "stored-secret-token"},
				},
			},
		},
	}

	// Client updates only the URL — auth is omitted, as it would be after a
	// GET (which redacts auth.value) followed by a naive PUT of the same body.
	req := &api.RESTAPI{
		Upstream: api.Upstream{Main: api.UpstreamDefinition{Url: utils.StringPtrIfNotEmpty("https://new-backend.internal/api")}},
	}

	updated, err := service.applyAPIUpdates(existing, req, "org-1")
	if err != nil {
		t.Fatalf("applyAPIUpdates() error = %v", err)
	}

	if updated.Upstream.Main.Url == nil || *updated.Upstream.Main.Url != "https://new-backend.internal/api" {
		t.Errorf("expected URL to be updated, got %v", updated.Upstream.Main.Url)
	}
	if updated.Upstream.Main.Auth == nil || updated.Upstream.Main.Auth.Value == nil {
		t.Fatal("expected stored auth value to be preserved, got nil auth block/value")
	}
	if *updated.Upstream.Main.Auth.Value != "stored-secret-token" {
		t.Errorf("expected stored auth value to be preserved, got %q", *updated.Upstream.Main.Auth.Value)
	}
}

// TestApplyAPIUpdatesDoesNotReuseSecretAcrossAuthTypeChange proves that
// changing the auth Type without resending a Value does not reattach the
// old secret — the old secret was encrypted/formatted for the previous auth
// scheme (e.g. a bearer token) and must not be silently reused as, say, a
// basic-auth credential.
func TestApplyAPIUpdatesDoesNotReuseSecretAcrossAuthTypeChange(t *testing.T) {
	service := &APIService{
		apiRepo:     &mockAPIRepository{},
		projectRepo: &mockProjectRepository{projectByUUID: &model.Project{ID: "11111111-1111-1111-1111-111111111111", Handle: "test-project"}},
		apiUtil:     &utils.APIUtil{},
		identity:    newTestIdentityService(),
	}

	existing := &model.API{
		Handle:    "pets-api",
		ProjectID: "11111111-1111-1111-1111-111111111111",
		Version:   "v1",
		Configuration: model.RestAPIConfig{
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{
					URL:  "https://backend.internal/api",
					Auth: &model.UpstreamAuth{Type: "bearer", Header: "Authorization", Value: `{{ secret "bearer-handle" }}`},
				},
			},
		},
	}

	// Client switches auth Type from bearer to basic but does not resend a
	// Value (e.g. redacted-field round-trip, or simply an oversight).
	basicType := api.UpstreamAuthType("basic")
	req := &api.RESTAPI{
		Upstream: api.Upstream{
			Main: api.UpstreamDefinition{
				Url: utils.StringPtrIfNotEmpty("https://backend.internal/api"),
				Auth: &api.UpstreamAuth{
					Type:   &basicType,
					Header: utils.StringPtrIfNotEmpty("Authorization"),
				},
			},
		},
	}

	updated, err := service.applyAPIUpdates(existing, req, "org-1")
	if err != nil {
		t.Fatalf("applyAPIUpdates() error = %v", err)
	}

	if updated.Upstream.Main.Auth == nil {
		t.Fatal("expected auth block to be present")
	}
	if updated.Upstream.Main.Auth.Type == nil || *updated.Upstream.Main.Auth.Type != basicType {
		t.Errorf("expected auth type to be %q, got %v", basicType, updated.Upstream.Main.Auth.Type)
	}
	if updated.Upstream.Main.Auth.Value != nil && *updated.Upstream.Main.Auth.Value != "" {
		t.Errorf("expected old bearer secret to NOT be reused for the new basic auth type, got value %q", *updated.Upstream.Main.Auth.Value)
	}
}

// TestAPIServiceUpdate_CleansUpRotatedSecret proves that rotating an API's
// upstream auth to a new secret deprecates the secret it replaced.
func TestAPIServiceUpdate_CleansUpRotatedSecret(t *testing.T) {
	apiRepo := &mockAPIRepository{
		getByUUIDFunc: func(apiUUID, orgUUID string) (*model.API, error) {
			return &model.API{
				ID:             apiUUID,
				Handle:         "pets-api",
				OrganizationID: orgUUID,
				ProjectID:      "11111111-1111-1111-1111-111111111111",
				Version:        "v1",
				Configuration: model.RestAPIConfig{
					Upstream: model.UpstreamConfig{
						Main: &model.UpstreamEndpoint{
							URL:  "https://backend.internal/api",
							Auth: &model.UpstreamAuth{Type: "bearer", Header: "Authorization", Value: `{{ secret "old-handle" }}`},
						},
					},
				},
			}, nil
		},
	}
	secretRepo := newMockRepo()
	secretRepo.secrets["old-handle"] = &model.Secret{Handle: "old-handle", Status: model.SecretStatusActive}
	secretRepo.secrets["new-handle"] = &model.Secret{Handle: "new-handle", Status: model.SecretStatusActive}
	secretService := NewSecretService(secretRepo, &mockVault{}, newTestIdentityService())

	service := &APIService{
		apiRepo:       apiRepo,
		projectRepo:   &mockProjectRepository{projectByUUID: &model.Project{ID: "11111111-1111-1111-1111-111111111111", Handle: "test-project"}},
		apiUtil:       &utils.APIUtil{},
		secretService: secretService,
		identity:      newTestIdentityService(),
		auditRepo:     &noopAuditRepo{},
	}

	req := &api.RESTAPI{
		Upstream: api.Upstream{
			Main: api.UpstreamDefinition{
				Url: utils.StringPtrIfNotEmpty("https://backend.internal/api"),
				Auth: &api.UpstreamAuth{
					Type:   upstreamAuthTypePtr("bearer"),
					Header: ptr("Authorization"),
					Value:  ptr(`{{ secret "new-handle" }}`),
				},
			},
		},
	}

	_, err := service.UpdateAPI("api-uuid", req, "org-1", "alice")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if secretRepo.secrets["old-handle"].Status != model.SecretStatusDeprecated {
		t.Errorf("expected old secret to be deprecated, got status=%v", secretRepo.secrets["old-handle"].Status)
	}
}

// Helper functions

// ptr creates a string pointer
func ptr(s string) *string {
	return &s
}

// slicePtr creates a string slice pointer
func slicePtr(s []string) *[]string {
	return &s
}

func statusPtr(s string) *api.RESTAPILifeCycleStatus {
	status := api.RESTAPILifeCycleStatus(s)
	return &status
}

func createStatusPtr(s string) *api.CreateRESTAPIRequestLifeCycleStatus {
	status := api.CreateRESTAPIRequestLifeCycleStatus(s)
	return &status
}

// Note: contains() and findSubstring() helper functions are defined in gateway_test.go

func TestAPIServiceCreate_MissingSecretRef_Rejected(t *testing.T) {
	apiRepo := &mockAPIRepository{}
	secretService := NewSecretService(newMockRepo(), &mockVault{}, newTestIdentityService())
	service := &APIService{
		apiRepo:       apiRepo,
		secretService: secretService,
		identity:      newTestIdentityService(),
	}

	params := map[string]interface{}{"value": `{{ secret "nonexistent-api-secret" }}`}
	req := &api.CreateRESTAPIRequest{
		DisplayName: "Test API",
		Context:     "/test",
		Version:     "v1",
		ProjectId:   "11111111-1111-1111-1111-111111111111",
		Upstream:    validUpstream(),
		Policies:    &[]api.Policy{{Name: "set-headers", Version: "v1", Params: &params}},
	}

	_, err := service.CreateAPI(req, "org-1", "alice")
	if err == nil {
		t.Fatal("expected error for non-existent secret placeholder, got nil")
	}
	if !apperror.ValidationFailed.Is(err) {
		t.Errorf("expected a validation error for missing secret ref, got: %v", err)
	}
	if apiRepo.created != nil {
		t.Error("expected API creation to be aborted, but repo.CreateAPI was called")
	}
}

func TestAPIServiceUpdate_MissingSecretRef_Rejected(t *testing.T) {
	apiRepo := &mockAPIRepository{
		getByUUIDFunc: func(apiUUID, orgUUID string) (*model.API, error) {
			return &model.API{
				ID:             apiUUID,
				Handle:         "pets-api",
				OrganizationID: orgUUID,
				ProjectID:      "11111111-1111-1111-1111-111111111111",
				Version:        "v1",
				Configuration:  model.RestAPIConfig{},
			}, nil
		},
	}
	secretService := NewSecretService(newMockRepo(), &mockVault{}, newTestIdentityService())
	service := &APIService{
		apiRepo:       apiRepo,
		secretService: secretService,
		identity:      newTestIdentityService(),
	}

	params := map[string]interface{}{"value": `{{ secret "nonexistent-api-secret" }}`}
	req := &api.RESTAPI{
		DisplayName: "Test API",
		Context:     "/test",
		Version:     "v1",
		Policies:    &[]api.Policy{{Name: "set-headers", Version: "v1", Params: &params}},
	}

	_, err := service.UpdateAPI("api-uuid", req, "org-1", "alice")
	if err == nil {
		t.Fatal("expected error for non-existent secret placeholder, got nil")
	}
	if !apperror.ValidationFailed.Is(err) {
		t.Errorf("expected a validation error for missing secret ref, got: %v", err)
	}
	if apiRepo.updated != nil {
		t.Error("expected API update to be aborted, but repo.UpdateAPI was called")
	}
}
