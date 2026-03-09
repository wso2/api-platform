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

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
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
}

func (m *mockAPIRepository) CheckAPIExistsByHandleInOrganization(handle, orgUUID string) (bool, error) {
	return m.handleExistsResult, m.handleExistsError
}

func (m *mockAPIRepository) CheckAPIExistsByNameAndVersionInOrganization(name, version, orgUUID, excludeHandle string) (bool, error) {
	m.lastExcludeHandle = excludeHandle // Track for verification
	return m.nameVersionExistsResult, m.nameVersionExistsError
}

// TestValidateUpdateAPIRequest tests the validateUpdateAPIRequest method
func TestValidateUpdateAPIRequest(t *testing.T) {
	tests := []struct {
		name                      string
		existingAPI               *model.API
		req                       *api.UpdateRESTAPIRequest
		mockHandleExists          bool
		mockHandleError           error
		mockNameVersionExists     bool
		mockNameVersionError      error
		wantErr                   bool
		expectedErr               error
		expectedExcludeHandle     string
		verifyExcludeHandleCalled bool
	}{
		{
			name: "valid update - no changes",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:     &api.UpdateRESTAPIRequest{},
			wantErr: false,
		},
		{
			name: "name update - excludes current API handle from duplicate check",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:                       &api.UpdateRESTAPIRequest{Name: "Updated Name"},
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
			req:                   &api.UpdateRESTAPIRequest{Name: "Conflicting Name"},
			mockNameVersionExists: true,
			wantErr:               true,
			expectedErr:           constants.ErrAPINameVersionAlreadyExists,
		},
		{
			name: "invalid lifecycle state",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:         &api.UpdateRESTAPIRequest{LifeCycleStatus: statusPtr("INVALID_STATE")},
			wantErr:     true,
			expectedErr: constants.ErrInvalidLifecycleState,
		},
		{
			name: "invalid api type",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:         &api.UpdateRESTAPIRequest{Kind: ptr("INVALID_TYPE")},
			wantErr:     true,
			expectedErr: constants.ErrInvalidAPIType,
		},
		{
			name: "invalid transport",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:         &api.UpdateRESTAPIRequest{Transport: slicePtr([]string{"invalid"})},
			wantErr:     true,
			expectedErr: constants.ErrInvalidTransport,
		},
		{
			name: "valid lifecycle state",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:     &api.UpdateRESTAPIRequest{LifeCycleStatus: statusPtr("PUBLISHED")},
			wantErr: false,
		},
		{
			name: "valid api type",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:     &api.UpdateRESTAPIRequest{Kind: ptr("RestApi")},
			wantErr: false,
		},
		{
			name: "valid transport",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:     &api.UpdateRESTAPIRequest{Transport: slicePtr([]string{"https"})},
			wantErr: false,
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
				apiRepo: mock,
			}

			err := service.validateUpdateAPIRequest(tt.existingAPI, tt.req, "test-org-uuid")

			if (err != nil) != tt.wantErr {
				t.Errorf("validateUpdateAPIRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil && tt.expectedErr != nil {
				if err != tt.expectedErr {
					t.Errorf("validateUpdateAPIRequest() error = %v, expectedErr %v", err, tt.expectedErr)
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
	projectID := openapi_types.UUID(uuid.MustParse("11111111-1111-1111-1111-111111111111"))

	tests := []struct {
		name                     string
		req                      *api.CreateRESTAPIRequest
		mockHandleExists         bool
		mockHandleError          error
		mockNameVersionExists    bool
		mockNameVersionError     error
		wantErr                  bool
		expectedErr              error
		errContains              string
		verifyExcludeHandleEmpty bool
		expectedExcludeHandle    string
	}{
		{
			name: "valid create request",
			req: &api.CreateRESTAPIRequest{
				Name:      "Test API",
				Context:   "/test",
				Version:   "v1",
				ProjectId: projectID,
				Upstream:  api.Upstream{},
			},
			mockNameVersionExists:    false,
			wantErr:                  false,
			verifyExcludeHandleEmpty: true,
			expectedExcludeHandle:    "",
		},
		{
			name: "handle already exists",
			req: &api.CreateRESTAPIRequest{
				Id:        ptr("my-handle"),
				Name:      "Test API",
				Context:   "/test",
				Version:   "v1",
				ProjectId: projectID,
				Upstream:  api.Upstream{},
			},
			mockHandleExists: true,
			wantErr:          true,
			expectedErr:      constants.ErrHandleExists,
		},
		{
			name: "name version already exists",
			req: &api.CreateRESTAPIRequest{
				Name:      "Test API",
				Context:   "/test",
				Version:   "v1",
				ProjectId: projectID,
				Upstream:  api.Upstream{},
			},
			mockNameVersionExists:    true,
			wantErr:                  true,
			expectedErr:              constants.ErrAPINameVersionAlreadyExists,
			verifyExcludeHandleEmpty: true,
			expectedExcludeHandle:    "",
		},
		{
			name: "missing name",
			req: &api.CreateRESTAPIRequest{
				Name:      "",
				Context:   "/test",
				Version:   "v1",
				ProjectId: projectID,
				Upstream:  api.Upstream{},
			},
			wantErr:     true,
			expectedErr: constants.ErrInvalidAPIName,
		},
		{
			name: "missing project id",
			req: &api.CreateRESTAPIRequest{
				Name:      "Test API",
				Context:   "/test",
				Version:   "v1",
				ProjectId: openapi_types.UUID{},
				Upstream:  api.Upstream{},
			},
			wantErr:     true,
			errContains: "project id is required",
		},
		{
			name: "invalid context",
			req: &api.CreateRESTAPIRequest{
				Name:      "Test API",
				Context:   "invalid",
				Version:   "v1",
				ProjectId: projectID,
				Upstream:  api.Upstream{},
			},
			wantErr:     true,
			expectedErr: constants.ErrInvalidAPIContext,
		},
		{
			name: "invalid version",
			req: &api.CreateRESTAPIRequest{
				Name:      "Test API",
				Context:   "/test",
				Version:   "",
				ProjectId: projectID,
				Upstream:  api.Upstream{},
			},
			wantErr:     true,
			expectedErr: constants.ErrInvalidAPIVersion,
		},
		{
			name: "invalid lifecycle state",
			req: &api.CreateRESTAPIRequest{
				Name:            "Test API",
				Context:         "/test",
				Version:         "v1",
				ProjectId:       projectID,
				LifeCycleStatus: createStatusPtr("INVALID_STATE"),
				Upstream:        api.Upstream{},
			},
			wantErr:     true,
			expectedErr: constants.ErrInvalidLifecycleState,
		},
		{
			name: "invalid api type",
			req: &api.CreateRESTAPIRequest{
				Name:      "Test API",
				Context:   "/test",
				Version:   "v1",
				ProjectId: projectID,
				Kind:      ptr("INVALID_TYPE"),
				Upstream:  api.Upstream{},
			},
			wantErr:     true,
			expectedErr: constants.ErrInvalidAPIType,
		},
		{
			name: "invalid transport",
			req: &api.CreateRESTAPIRequest{
				Name:      "Test API",
				Context:   "/test",
				Version:   "v1",
				ProjectId: projectID,
				Transport: slicePtr([]string{"invalid"}),
				Upstream:  api.Upstream{},
			},
			wantErr:     true,
			expectedErr: constants.ErrInvalidTransport,
		},
		{
			name: "valid lifecycle state",
			req: &api.CreateRESTAPIRequest{
				Name:            "Test API",
				Context:         "/test",
				Version:         "v1",
				ProjectId:       projectID,
				LifeCycleStatus: createStatusPtr("PUBLISHED"),
				Upstream:        api.Upstream{},
			},
			mockNameVersionExists:    false,
			wantErr:                  false,
			verifyExcludeHandleEmpty: true,
			expectedExcludeHandle:    "",
		},
		{
			name: "valid api type",
			req: &api.CreateRESTAPIRequest{
				Name:      "Test API",
				Context:   "/test",
				Version:   "v1",
				ProjectId: projectID,
				Kind:      ptr("RestApi"),
				Upstream:  api.Upstream{},
			},
			mockNameVersionExists:    false,
			wantErr:                  false,
			verifyExcludeHandleEmpty: true,
			expectedExcludeHandle:    "",
		},
		{
			name: "valid transport",
			req: &api.CreateRESTAPIRequest{
				Name:      "Test API",
				Context:   "/test",
				Version:   "v1",
				ProjectId: projectID,
				Transport: slicePtr([]string{"https"}),
				Upstream:  api.Upstream{},
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
				apiRepo: mock,
			}

			err := service.validateCreateAPIRequest(tt.req, "test-org-uuid")

			if (err != nil) != tt.wantErr {
				t.Errorf("validateCreateAPIRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if tt.expectedErr != nil && err != tt.expectedErr {
					t.Errorf("validateCreateAPIRequest() error = %v, expectedErr %v", err, tt.expectedErr)
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
		apiRepo: &mockAPIRepository{},
		apiUtil: &utils.APIUtil{},
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
			Policies: []model.Policy{
				{Name: "legacy-policy", Version: "v1"},
			},
		},
	}

	updated, err := service.applyAPIUpdates(existing, &api.UpdateRESTAPIRequest{Policies: &newPolicies}, "org-1")
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
