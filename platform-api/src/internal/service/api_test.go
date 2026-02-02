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

	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
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
		req                       *UpdateAPIRequest
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
			req:     &UpdateAPIRequest{},
			wantErr: false,
		},
		{
			name: "name update - excludes current API handle from duplicate check",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:                       &UpdateAPIRequest{Name: ptr("Updated Name")},
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
			req:                   &UpdateAPIRequest{Name: ptr("Conflicting Name")},
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
			req:         &UpdateAPIRequest{LifeCycleStatus: ptr("INVALID_STATE")},
			wantErr:     true,
			expectedErr: constants.ErrInvalidLifecycleState,
		},
		{
			name: "invalid api type",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:         &UpdateAPIRequest{Type: ptr("INVALID_TYPE")},
			wantErr:     true,
			expectedErr: constants.ErrInvalidAPIType,
		},
		{
			name: "invalid transport",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:         &UpdateAPIRequest{Transport: slicePtr([]string{"invalid"})},
			wantErr:     true,
			expectedErr: constants.ErrInvalidTransport,
		},
		{
			name: "valid lifecycle state",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:     &UpdateAPIRequest{LifeCycleStatus: ptr("PUBLISHED")},
			wantErr: false,
		},
		{
			name: "valid api type",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:     &UpdateAPIRequest{Type: ptr("HTTP")},
			wantErr: false,
		},
		{
			name: "valid transport",
			existingAPI: &model.API{
				Handle:  "my-api",
				Version: "v1",
			},
			req:     &UpdateAPIRequest{Transport: slicePtr([]string{"https"})},
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
	tests := []struct {
		name                     string
		req                      *CreateAPIRequest
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
			req: &CreateAPIRequest{
				Name:      "Test API",
				Context:   "/test",
				Version:   "v1",
				ProjectID: "proj-123",
			},
			mockNameVersionExists:    false,
			wantErr:                  false,
			verifyExcludeHandleEmpty: true,
			expectedExcludeHandle:    "",
		},
		{
			name: "handle already exists",
			req: &CreateAPIRequest{
				ID:        "my-handle",
				Name:      "Test API",
				Context:   "/test",
				Version:   "v1",
				ProjectID: "proj-123",
			},
			mockHandleExists: true,
			wantErr:          true,
			expectedErr:      constants.ErrHandleExists,
		},
		{
			name: "name version already exists",
			req: &CreateAPIRequest{
				Name:      "Test API",
				Context:   "/test",
				Version:   "v1",
				ProjectID: "proj-123",
			},
			mockNameVersionExists:    true,
			wantErr:                  true,
			expectedErr:              constants.ErrAPINameVersionAlreadyExists,
			verifyExcludeHandleEmpty: true,
			expectedExcludeHandle:    "",
		},
		{
			name: "missing name",
			req: &CreateAPIRequest{
				Name:      "",
				Context:   "/test",
				Version:   "v1",
				ProjectID: "proj-123",
			},
			wantErr:     true,
			expectedErr: constants.ErrInvalidAPIName,
		},
		{
			name: "missing project id",
			req: &CreateAPIRequest{
				Name:      "Test API",
				Context:   "/test",
				Version:   "v1",
				ProjectID: "",
			},
			wantErr:     true,
			errContains: "project id is required",
		},
		{
			name: "invalid context",
			req: &CreateAPIRequest{
				Name:      "Test API",
				Context:   "invalid",
				Version:   "v1",
				ProjectID: "proj-123",
			},
			wantErr:     true,
			expectedErr: constants.ErrInvalidAPIContext,
		},
		{
			name: "invalid version",
			req: &CreateAPIRequest{
				Name:      "Test API",
				Context:   "/test",
				Version:   "",
				ProjectID: "proj-123",
			},
			wantErr:     true,
			expectedErr: constants.ErrInvalidAPIVersion,
		},
		{
			name: "invalid lifecycle state",
			req: &CreateAPIRequest{
				Name:            "Test API",
				Context:         "/test",
				Version:         "v1",
				ProjectID:       "proj-123",
				LifeCycleStatus: "INVALID_STATE",
			},
			wantErr:     true,
			expectedErr: constants.ErrInvalidLifecycleState,
		},
		{
			name: "invalid api type",
			req: &CreateAPIRequest{
				Name:      "Test API",
				Context:   "/test",
				Version:   "v1",
				ProjectID: "proj-123",
				Type:      "INVALID_TYPE",
			},
			wantErr:     true,
			expectedErr: constants.ErrInvalidAPIType,
		},
		{
			name: "invalid transport",
			req: &CreateAPIRequest{
				Name:      "Test API",
				Context:   "/test",
				Version:   "v1",
				ProjectID: "proj-123",
				Transport: []string{"invalid"},
			},
			wantErr:     true,
			expectedErr: constants.ErrInvalidTransport,
		},
		{
			name: "valid lifecycle state",
			req: &CreateAPIRequest{
				Name:            "Test API",
				Context:         "/test",
				Version:         "v1",
				ProjectID:       "proj-123",
				LifeCycleStatus: "PUBLISHED",
			},
			mockNameVersionExists:    false,
			wantErr:                  false,
			verifyExcludeHandleEmpty: true,
			expectedExcludeHandle:    "",
		},
		{
			name: "valid api type",
			req: &CreateAPIRequest{
				Name:      "Test API",
				Context:   "/test",
				Version:   "v1",
				ProjectID: "proj-123",
				Type:      "HTTP",
			},
			mockNameVersionExists:    false,
			wantErr:                  false,
			verifyExcludeHandleEmpty: true,
			expectedExcludeHandle:    "",
		},
		{
			name: "valid transport",
			req: &CreateAPIRequest{
				Name:      "Test API",
				Context:   "/test",
				Version:   "v1",
				ProjectID: "proj-123",
				Transport: []string{"https"},
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
	newPolicies := []dto.Policy{
		{
			ExecutionCondition: &condition,
			Name:               "rate-limit",
			Params:             &params,
			Version:            "v1",
		},
	}

	existing := &model.API{
		Handle:  "pets-api",
		Version: "v1",
		Policies: []model.Policy{
			{Name: "legacy-policy", Version: "v1"},
		},
	}

	updated, err := service.applyAPIUpdates(existing, &UpdateAPIRequest{Policies: &newPolicies}, "org-1")
	if err != nil {
		t.Fatalf("applyAPIUpdates() error = %v", err)
	}

	if !reflect.DeepEqual(updated.Policies, newPolicies) {
		t.Errorf("updated policies = %v, want %v", updated.Policies, newPolicies)
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

// Note: contains() and findSubstring() helper functions are defined in gateway_test.go
