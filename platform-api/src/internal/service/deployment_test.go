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
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"platform-api/src/config"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

func isValidUUIDString(id string) bool {
	_, err := uuid.Parse(id)
	return err == nil
}

// TestValidateEndpointURL tests the validateEndpointURL helper function
func TestValidateEndpointURL(t *testing.T) {
	tests := []struct {
		name        string
		endpointURL string
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid http URL",
			endpointURL: "http://api.example.com:8080/v1",
			wantErr:     false,
		},
		{
			name:        "valid https URL",
			endpointURL: "https://api.example.com/v1/resources",
			wantErr:     false,
		},
		{
			name:        "valid URL with port",
			endpointURL: "http://localhost:8080",
			wantErr:     false,
		},
		{
			name:        "valid URL with IP address",
			endpointURL: "http://192.168.1.100:8080/api",
			wantErr:     false,
		},
		{
			name:        "empty URL",
			endpointURL: "",
			wantErr:     true,
			errContains: "cannot be empty",
		},
		{
			name:        "invalid scheme - ftp",
			endpointURL: "ftp://api.example.com/v1",
			wantErr:     true,
			errContains: "scheme must be http or https",
		},
		{
			name:        "invalid scheme - ws",
			endpointURL: "ws://api.example.com/v1",
			wantErr:     true,
			errContains: "scheme must be http or https",
		},
		{
			name:        "missing scheme",
			endpointURL: "api.example.com/v1",
			wantErr:     true,
			errContains: "scheme must be http or https",
		},
		{
			name:        "missing host",
			endpointURL: "http:///path",
			wantErr:     true,
			errContains: "must have a valid host",
		},
		{
			name:        "invalid URL format",
			endpointURL: "://invalid",
			wantErr:     true,
			errContains: "invalid URL format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEndpointURL(tt.endpointURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEndpointURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("validateEndpointURL() error = %v, want error containing %q", err, tt.errContains)
			}
		})
	}
}

// TestOverrideEndpointURL tests the overrideEndpointURL helper function
func TestOverrideEndpointURL(t *testing.T) {
	tests := []struct {
		name           string
		inputYAML      string
		newURL         string
		wantErr        bool
		errContains    string
		validateResult func(t *testing.T, result []byte)
	}{
		{
			name: "override existing main URL",
			inputYAML: `apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: test-api
spec:
  displayName: Test API
  version: v1
  context: /test
  upstream:
    main:
      url: http://old-backend.com/api
  operations: []
`,
			newURL:  "https://new-backend.com/api/v2",
			wantErr: false,
			validateResult: func(t *testing.T, result []byte) {
				var apiDeployment dto.APIDeploymentYAML
				if err := yaml.Unmarshal(result, &apiDeployment); err != nil {
					t.Fatalf("Failed to unmarshal result YAML: %v", err)
				}
				if apiDeployment.Spec.Upstream == nil || apiDeployment.Spec.Upstream.Main == nil {
					t.Fatal("Upstream.Main should not be nil")
				}
				if apiDeployment.Spec.Upstream.Main.URL != "https://new-backend.com/api/v2" {
					t.Errorf("Expected URL to be 'https://new-backend.com/api/v2', got %q", apiDeployment.Spec.Upstream.Main.URL)
				}
				if apiDeployment.Spec.Upstream.Main.Ref != "" {
					t.Errorf("Expected Ref to be empty, got %q", apiDeployment.Spec.Upstream.Main.Ref)
				}
			},
		},
		{
			name: "override when upstream is nil",
			inputYAML: `apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: test-api
spec:
  displayName: Test API
  version: v1
  context: /test
  operations: []
`,
			newURL:  "http://backend.example.com:8080",
			wantErr: false,
			validateResult: func(t *testing.T, result []byte) {
				var apiDeployment dto.APIDeploymentYAML
				if err := yaml.Unmarshal(result, &apiDeployment); err != nil {
					t.Fatalf("Failed to unmarshal result YAML: %v", err)
				}
				if apiDeployment.Spec.Upstream == nil || apiDeployment.Spec.Upstream.Main == nil {
					t.Fatal("Upstream.Main should be created")
				}
				if apiDeployment.Spec.Upstream.Main.URL != "http://backend.example.com:8080" {
					t.Errorf("Expected URL to be 'http://backend.example.com:8080', got %q", apiDeployment.Spec.Upstream.Main.URL)
				}
			},
		},
		{
			name: "override URL that had ref instead",
			inputYAML: `apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: test-api
spec:
  displayName: Test API
  version: v1
  context: /test
  upstream:
    main:
      ref: backend-service-ref
  operations: []
`,
			newURL:  "https://direct-url.com/api",
			wantErr: false,
			validateResult: func(t *testing.T, result []byte) {
				var apiDeployment dto.APIDeploymentYAML
				if err := yaml.Unmarshal(result, &apiDeployment); err != nil {
					t.Fatalf("Failed to unmarshal result YAML: %v", err)
				}
				if apiDeployment.Spec.Upstream.Main.URL != "https://direct-url.com/api" {
					t.Errorf("Expected URL to be 'https://direct-url.com/api', got %q", apiDeployment.Spec.Upstream.Main.URL)
				}
				if apiDeployment.Spec.Upstream.Main.Ref != "" {
					t.Errorf("Expected Ref to be cleared, got %q", apiDeployment.Spec.Upstream.Main.Ref)
				}
			},
		},
		{
			name: "preserve sandbox endpoint",
			inputYAML: `apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: test-api
spec:
  displayName: Test API
  version: v1
  context: /test
  upstream:
    main:
      url: http://prod.example.com
    sandbox:
      url: http://sandbox.example.com
  operations: []
`,
			newURL:  "https://new-prod.example.com",
			wantErr: false,
			validateResult: func(t *testing.T, result []byte) {
				var apiDeployment dto.APIDeploymentYAML
				if err := yaml.Unmarshal(result, &apiDeployment); err != nil {
					t.Fatalf("Failed to unmarshal result YAML: %v", err)
				}
				if apiDeployment.Spec.Upstream.Main.URL != "https://new-prod.example.com" {
					t.Errorf("Expected main URL to be updated to 'https://new-prod.example.com', got %q", apiDeployment.Spec.Upstream.Main.URL)
				}
				if apiDeployment.Spec.Upstream.Sandbox == nil || apiDeployment.Spec.Upstream.Sandbox.URL != "http://sandbox.example.com" {
					t.Error("Expected sandbox URL to be preserved")
				}
			},
		},
		{
			name:        "invalid YAML",
			inputYAML:   `invalid: yaml: [unclosed`,
			newURL:      "http://example.com",
			wantErr:     true,
			errContains: "failed to parse deployment YAML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := overrideEndpointURL([]byte(tt.inputYAML), tt.newURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("overrideEndpointURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("overrideEndpointURL() error = %v, want error containing %q", err, tt.errContains)
				return
			}
			if !tt.wantErr && tt.validateResult != nil {
				tt.validateResult(t, result)
			}
		})
	}
}

// TestOverrideEndpointURLPreservesOtherFields tests that the override preserves other YAML fields
func TestOverrideEndpointURLPreservesOtherFields(t *testing.T) {
	inputYAML := `apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: my-api-123
spec:
  displayName: My Test API
  version: v2.0
  context: /myapi/v2
  upstream:
    main:
      url: http://old.example.com:8080/api
  operations:
    - method: GET
      path: /users
    - method: POST
      path: /users
`

	result, err := overrideEndpointURL([]byte(inputYAML), "https://new.example.com:9090/api/v2")
	if err != nil {
		t.Fatalf("overrideEndpointURL() failed: %v", err)
	}

	var apiDeployment dto.APIDeploymentYAML
	if err := yaml.Unmarshal(result, &apiDeployment); err != nil {
		t.Fatalf("Failed to unmarshal result YAML: %v", err)
	}

	// Verify all fields are preserved
	if apiDeployment.ApiVersion != "gateway.api-platform.wso2.com/v1alpha1" {
		t.Errorf("ApiVersion not preserved, got %q", apiDeployment.ApiVersion)
	}
	if apiDeployment.Kind != "RestApi" {
		t.Errorf("Kind not preserved, got %q", apiDeployment.Kind)
	}
	if apiDeployment.Metadata.Name != "my-api-123" {
		t.Errorf("Metadata.Name not preserved, got %q", apiDeployment.Metadata.Name)
	}
	if apiDeployment.Spec.DisplayName != "My Test API" {
		t.Errorf("Spec.DisplayName not preserved, got %q", apiDeployment.Spec.DisplayName)
	}
	if apiDeployment.Spec.Version != "v2.0" {
		t.Errorf("Spec.Version not preserved, got %q", apiDeployment.Spec.Version)
	}
	if apiDeployment.Spec.Context != "/myapi/v2" {
		t.Errorf("Spec.Context not preserved, got %q", apiDeployment.Spec.Context)
	}
	if len(apiDeployment.Spec.Operations) != 2 {
		t.Errorf("Expected 2 operations, got %d", len(apiDeployment.Spec.Operations))
	}

	// Verify URL was updated
	if apiDeployment.Spec.Upstream.Main.URL != "https://new.example.com:9090/api/v2" {
		t.Errorf("Expected URL to be updated to 'https://new.example.com:9090/api/v2', got %q", apiDeployment.Spec.Upstream.Main.URL)
	}
}

// ============================================================================
// Mock Repository Implementations for DeploymentService Tests
// ============================================================================

// mockDeploymentAPIRepository is a mock implementation of APIRepository and DeploymentRepository for deployment tests
type mockDeploymentAPIRepository struct {
	repository.APIRepository
	repository.DeploymentRepository

	// Mock data
	api                   *model.API
	deploymentWithContent *model.Deployment
	deploymentWithState   *model.Deployment
	deployments           []*model.Deployment
	associations          []*model.APIAssociation

	// Mock deployment status
	currentDeploymentID string
	currentStatus       model.DeploymentStatus
	statusUpdatedAt     *time.Time

	// Mock return values
	setCurrentUpdatedAt time.Time
	setCurrentError     error

	// Mock errors
	getAPIByUUIDError      error
	getWithContentError    error
	getWithStateError      error
	getDeploymentsError    error
	getStatusError         error
	deleteError            error
	createWithLimitError   error
	createAssociationError error

	// Call tracking
	setCurrentCalled        bool
	setCurrentArtifactID    string
	setCurrentGatewayID     string
	setCurrentStatus        model.DeploymentStatus
	deleteCalled            bool
	createAssociationCalled bool
}

func (m *mockDeploymentAPIRepository) GetAPIByUUID(uuid, orgUUID string) (*model.API, error) {
	if m.getAPIByUUIDError != nil {
		return nil, m.getAPIByUUIDError
	}
	return m.api, nil
}

func (m *mockDeploymentAPIRepository) GetAPIAssociations(apiUUID, associationType, orgUUID string) ([]*model.APIAssociation, error) {
	return m.associations, nil
}

func (m *mockDeploymentAPIRepository) CreateAPIAssociation(association *model.APIAssociation) error {
	m.createAssociationCalled = true
	return m.createAssociationError
}

// DeploymentRepository methods
func (m *mockDeploymentAPIRepository) GetWithContent(deploymentID, artifactUUID, orgUUID string) (*model.Deployment, error) {
	if m.getWithContentError != nil {
		return nil, m.getWithContentError
	}
	return m.deploymentWithContent, nil
}

func (m *mockDeploymentAPIRepository) GetWithState(deploymentID, artifactUUID, orgUUID string) (*model.Deployment, error) {
	if m.getWithStateError != nil {
		return nil, m.getWithStateError
	}
	return m.deploymentWithState, nil
}

func (m *mockDeploymentAPIRepository) GetDeploymentsWithState(artifactUUID, orgUUID string, gatewayID *string, status *string, maxPerAPIGW int) ([]*model.Deployment, error) {
	if m.getDeploymentsError != nil {
		return nil, m.getDeploymentsError
	}
	return m.deployments, nil
}

func (m *mockDeploymentAPIRepository) GetStatus(artifactUUID, orgUUID, gatewayID string) (string, model.DeploymentStatus, *time.Time, error) {
	if m.getStatusError != nil {
		return "", "", nil, m.getStatusError
	}
	return m.currentDeploymentID, m.currentStatus, m.statusUpdatedAt, nil
}

func (m *mockDeploymentAPIRepository) SetCurrent(artifactUUID, orgUUID, gatewayID, deploymentID string, status model.DeploymentStatus) (time.Time, error) {
	m.setCurrentCalled = true
	m.setCurrentArtifactID = artifactUUID
	m.setCurrentGatewayID = gatewayID
	m.setCurrentStatus = status
	if m.setCurrentError != nil {
		return time.Time{}, m.setCurrentError
	}
	return m.setCurrentUpdatedAt, nil
}

func (m *mockDeploymentAPIRepository) Delete(deploymentID, artifactUUID, orgUUID string) error {
	m.deleteCalled = true
	return m.deleteError
}

func (m *mockDeploymentAPIRepository) CreateWithLimitEnforcement(deployment *model.Deployment, hardLimit int) error {
	return m.createWithLimitError
}

func (m *mockDeploymentAPIRepository) GetCurrentByGateway(artifactUUID, gatewayID, orgUUID string) (*model.Deployment, error) {
	return m.deploymentWithContent, nil
}

// mockDeploymentRepo is a separate mock implementation of DeploymentRepository for deployment tests
type mockDeploymentRepo struct {
	repository.DeploymentRepository // Embed interface for unimplemented methods

	// Mock data
	deploymentWithContent *model.Deployment
	deploymentWithState   *model.Deployment
	deployments           []*model.Deployment

	// Mock deployment status
	currentDeploymentID string
	currentStatus       model.DeploymentStatus
	statusUpdatedAt     *time.Time

	// Mock return values
	setCurrentUpdatedAt time.Time
	setCurrentError     error

	// Mock errors
	getWithContentError  error
	getWithStateError    error
	getDeploymentsError  error
	getStatusError       error
	deleteError          error
	createWithLimitError error

	// Call tracking
	setCurrentCalled     bool
	setCurrentArtifactID string
	setCurrentGatewayID  string
	setCurrentStatus     model.DeploymentStatus
	deleteCalled         bool
}

func (m *mockDeploymentRepo) GetWithContent(deploymentID, artifactUUID, orgUUID string) (*model.Deployment, error) {
	if m.getWithContentError != nil {
		return nil, m.getWithContentError
	}
	return m.deploymentWithContent, nil
}

func (m *mockDeploymentRepo) GetWithState(deploymentID, artifactUUID, orgUUID string) (*model.Deployment, error) {
	if m.getWithStateError != nil {
		return nil, m.getWithStateError
	}
	return m.deploymentWithState, nil
}

func (m *mockDeploymentRepo) GetDeploymentsWithState(artifactUUID, orgUUID string, gatewayID *string, status *string, maxPerAPIGW int) ([]*model.Deployment, error) {
	if m.getDeploymentsError != nil {
		return nil, m.getDeploymentsError
	}
	return m.deployments, nil
}

func (m *mockDeploymentRepo) GetStatus(artifactUUID, orgUUID, gatewayID string) (string, model.DeploymentStatus, *time.Time, error) {
	if m.getStatusError != nil {
		return "", "", nil, m.getStatusError
	}
	return m.currentDeploymentID, m.currentStatus, m.statusUpdatedAt, nil
}

func (m *mockDeploymentRepo) SetCurrent(artifactUUID, orgUUID, gatewayID, deploymentID string, status model.DeploymentStatus) (time.Time, error) {
	m.setCurrentCalled = true
	m.setCurrentArtifactID = artifactUUID
	m.setCurrentGatewayID = gatewayID
	m.setCurrentStatus = status
	if m.setCurrentError != nil {
		return time.Time{}, m.setCurrentError
	}
	return m.setCurrentUpdatedAt, nil
}

func (m *mockDeploymentRepo) Delete(deploymentID, artifactUUID, orgUUID string) error {
	m.deleteCalled = true
	return m.deleteError
}

func (m *mockDeploymentRepo) CreateWithLimitEnforcement(deployment *model.Deployment, hardLimit int) error {
	return m.createWithLimitError
}

// mockDeploymentGatewayRepository is a mock implementation of GatewayRepository for deployment tests
type mockDeploymentGatewayRepository struct {
	repository.GatewayRepository // Embed interface for unimplemented methods

	// Mock data
	gateway *model.Gateway

	// Mock errors
	getByUUIDError error
}

func (m *mockDeploymentGatewayRepository) GetByUUID(gatewayID string) (*model.Gateway, error) {
	if m.getByUUIDError != nil {
		return nil, m.getByUUIDError
	}
	return m.gateway, nil
}

// ============================================================================
// DeploymentService.RestoreDeployment Tests
// ============================================================================

func TestRestoreDeployment(t *testing.T) {
	testOrgUUID := "00000000-0000-0000-0000-000000000123"
	testAPIUUID := "11111111-1111-1111-1111-111111111111"
	testGatewayID := "22222222-2222-2222-2222-222222222222"
	testDeploymentID := "33333333-3333-3333-3333-333333333333"
	testUpdatedAt := time.Now()

	tests := []struct {
		name                  string
		deploymentID          string
		gatewayID             string
		mockDeployment        *model.Deployment
		mockDeploymentError   error
		mockCurrentDeployment string
		mockCurrentStatus     model.DeploymentStatus
		mockStatusError       error
		mockGateway           *model.Gateway
		mockGatewayError      error
		mockSetCurrentError   error
		mockSetCurrentTime    time.Time
		wantErr               bool
		expectedErr           error
		errContains           string
		verifyUpdatedAt       bool
	}{
		{
			name:         "successful restore to UNDEPLOYED deployment",
			deploymentID: testDeploymentID,
			gatewayID:    testGatewayID,
			mockDeployment: &model.Deployment{
				DeploymentID: testDeploymentID,
				Name:         "test-deployment",
				ArtifactID:   testAPIUUID,
				GatewayID:    testGatewayID,
				Content:      []byte("test content"),
			},
			mockCurrentDeployment: "44444444-4444-4444-4444-444444444444",
			mockCurrentStatus:     model.DeploymentStatusDeployed,
			mockGateway: &model.Gateway{
				ID:             testGatewayID,
				OrganizationID: testOrgUUID,
				Vhost:          "api.example.com",
			},
			mockSetCurrentTime: testUpdatedAt,
			wantErr:            false,
			verifyUpdatedAt:    true,
		},
		{
			name:         "successful restore to ARCHIVED deployment",
			deploymentID: testDeploymentID,
			gatewayID:    testGatewayID,
			mockDeployment: &model.Deployment{
				DeploymentID: testDeploymentID,
				Name:         "archived-deployment",
				ArtifactID:   testAPIUUID,
				GatewayID:    testGatewayID,
				Content:      []byte("archived content"),
			},
			mockCurrentDeployment: "55555555-5555-5555-5555-555555555555",
			mockCurrentStatus:     model.DeploymentStatusDeployed,
			mockGateway: &model.Gateway{
				ID:             testGatewayID,
				OrganizationID: testOrgUUID,
				Vhost:          "api.example.com",
			},
			mockSetCurrentTime: testUpdatedAt,
			wantErr:            false,
			verifyUpdatedAt:    true,
		},
		{
			name:                "deployment not found",
			deploymentID:        testDeploymentID,
			gatewayID:           testGatewayID,
			mockDeployment:      nil,
			mockDeploymentError: nil,
			wantErr:             true,
			expectedErr:         constants.ErrDeploymentNotFound,
		},
		{
			name:                "deployment fetch error",
			deploymentID:        testDeploymentID,
			gatewayID:           testGatewayID,
			mockDeploymentError: errors.New("database error"),
			wantErr:             true,
			errContains:         "database error",
		},
		{
			name:         "cannot restore to already DEPLOYED deployment",
			deploymentID: testDeploymentID,
			gatewayID:    testGatewayID,
			mockDeployment: &model.Deployment{
				DeploymentID: testDeploymentID,
				Name:         "test-deployment",
				ArtifactID:   testAPIUUID,
				GatewayID:    testGatewayID,
				Content:      []byte("test content"),
			},
			mockCurrentDeployment: testDeploymentID, // Same as target
			mockCurrentStatus:     model.DeploymentStatusDeployed,
			wantErr:               true,
			expectedErr:           constants.ErrDeploymentAlreadyDeployed,
		},
		{
			name:         "deployment status fetch error",
			deploymentID: testDeploymentID,
			gatewayID:    testGatewayID,
			mockDeployment: &model.Deployment{
				DeploymentID: testDeploymentID,
				Name:         "test-deployment",
				ArtifactID:   testAPIUUID,
				GatewayID:    testGatewayID,
				Content:      []byte("test content"),
			},
			mockStatusError: errors.New("status lookup failed"),
			wantErr:         true,
			errContains:     "failed to get deployment status",
		},
		{
			name:         "gateway not found",
			deploymentID: testDeploymentID,
			gatewayID:    testGatewayID,
			mockDeployment: &model.Deployment{
				DeploymentID: testDeploymentID,
				Name:         "test-deployment",
				ArtifactID:   testAPIUUID,
				GatewayID:    testGatewayID,
				Content:      []byte("test content"),
			},
			mockCurrentDeployment: "44444444-4444-4444-4444-444444444444",
			mockCurrentStatus:     model.DeploymentStatusUndeployed,
			mockGateway:           nil,
			wantErr:               true,
			expectedErr:           constants.ErrGatewayNotFound,
		},
		{
			name:         "gateway organization mismatch",
			deploymentID: testDeploymentID,
			gatewayID:    testGatewayID,
			mockDeployment: &model.Deployment{
				DeploymentID: testDeploymentID,
				Name:         "test-deployment",
				ArtifactID:   testAPIUUID,
				GatewayID:    testGatewayID,
				Content:      []byte("test content"),
			},
			mockCurrentDeployment: "44444444-4444-4444-4444-444444444444",
			mockCurrentStatus:     model.DeploymentStatusUndeployed,
			mockGateway: &model.Gateway{
				ID:             testGatewayID,
				OrganizationID: "00000000-0000-0000-0000-000000000124", // Different organization
				Vhost:          "api.example.com",
			},
			wantErr:     true,
			expectedErr: constants.ErrGatewayNotFound,
		},
		{
			name:         "set current deployment fails",
			deploymentID: testDeploymentID,
			gatewayID:    testGatewayID,
			mockDeployment: &model.Deployment{
				DeploymentID: testDeploymentID,
				Name:         "test-deployment",
				ArtifactID:   testAPIUUID,
				GatewayID:    testGatewayID,
				Content:      []byte("test content"),
			},
			mockCurrentDeployment: "44444444-4444-4444-4444-444444444444",
			mockCurrentStatus:     model.DeploymentStatusUndeployed,
			mockGateway: &model.Gateway{
				ID:             testGatewayID,
				OrganizationID: testOrgUUID,
				Vhost:          "api.example.com",
			},
			mockSetCurrentError: errors.New("database write failed"),
			wantErr:             true,
			errContains:         "failed to set current deployment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPIRepo := &mockDeploymentAPIRepository{}

			mockDeploymentRepo := &mockDeploymentRepo{
				deploymentWithContent: tt.mockDeployment,
				getWithContentError:   tt.mockDeploymentError,
				currentDeploymentID:   tt.mockCurrentDeployment,
				currentStatus:         tt.mockCurrentStatus,
				getStatusError:        tt.mockStatusError,
				setCurrentUpdatedAt:   tt.mockSetCurrentTime,
				setCurrentError:       tt.mockSetCurrentError,
			}

			mockGatewayRepo := &mockDeploymentGatewayRepository{
				gateway:        tt.mockGateway,
				getByUUIDError: tt.mockGatewayError,
			}

			service := &DeploymentService{
				apiRepo:        mockAPIRepo,
				deploymentRepo: mockDeploymentRepo,
				gatewayRepo:    mockGatewayRepo,
			}

			result, err := service.RestoreDeployment(testAPIUUID, tt.deploymentID, tt.gatewayID, testOrgUUID)

			// Check error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("RestoreDeployment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) && err != tt.expectedErr {
					t.Errorf("RestoreDeployment() error = %v, expectedErr %v", err, tt.expectedErr)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("RestoreDeployment() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			// Verify successful result
			if result == nil {
				t.Fatal("RestoreDeployment() result is nil, expected non-nil")
			}

			if isValidUUIDString(tt.deploymentID) && result.DeploymentId.String() != tt.deploymentID {
				t.Errorf("RestoreDeployment() DeploymentID = %v, want %v", result.DeploymentId.String(), tt.deploymentID)
			}

			if string(result.Status) != string(model.DeploymentStatusDeployed) {
				t.Errorf("RestoreDeployment() Status = %v, want %v", result.Status, model.DeploymentStatusDeployed)
			}

			// Verify updatedAt is returned from SetCurrentDeployment
			if tt.verifyUpdatedAt {
				if result.UpdatedAt == nil {
					t.Error("RestoreDeployment() UpdatedAt is nil, expected non-nil")
				} else if !result.UpdatedAt.Equal(tt.mockSetCurrentTime) {
					t.Errorf("RestoreDeployment() UpdatedAt = %v, want %v", *result.UpdatedAt, tt.mockSetCurrentTime)
				}
			}

			// Verify SetCurrent was called with correct parameters
			if !mockDeploymentRepo.setCurrentCalled {
				t.Error("SetCurrent was not called")
			}
			if mockDeploymentRepo.setCurrentStatus != model.DeploymentStatusDeployed {
				t.Errorf("SetCurrent called with status %v, want %v", mockDeploymentRepo.setCurrentStatus, model.DeploymentStatusDeployed)
			}
		})
	}
}

// ============================================================================
// DeploymentService.UndeployDeployment Tests
// ============================================================================

func TestUndeployDeployment(t *testing.T) {
	testOrgUUID := "00000000-0000-0000-0000-000000000123"
	testAPIUUID := "11111111-1111-1111-1111-111111111111"
	testGatewayID := "22222222-2222-2222-2222-222222222222"
	testDeploymentID := "33333333-3333-3333-3333-333333333333"
	testUpdatedAt := time.Now()
	deployedStatus := model.DeploymentStatusDeployed
	undeployedStatus := model.DeploymentStatusUndeployed

	tests := []struct {
		name                string
		deploymentID        string
		gatewayID           string
		mockDeployment      *model.Deployment
		mockDeploymentError error
		mockGateway         *model.Gateway
		mockGatewayError    error
		mockSetCurrentError error
		mockSetCurrentTime  time.Time
		wantErr             bool
		expectedErr         error
		errContains         string
		verifyUpdatedAt     bool
	}{
		{
			name:         "successful undeploy",
			deploymentID: testDeploymentID,
			gatewayID:    testGatewayID,
			mockDeployment: &model.Deployment{
				DeploymentID: testDeploymentID,
				Name:         "test-deployment",
				ArtifactID:   testAPIUUID,
				GatewayID:    testGatewayID,
				Status:       &deployedStatus,
			},
			mockGateway: &model.Gateway{
				ID:             testGatewayID,
				OrganizationID: testOrgUUID,
				Vhost:          "api.example.com",
			},
			mockSetCurrentTime: testUpdatedAt,
			wantErr:            false,
			verifyUpdatedAt:    true,
		},
		{
			name:                "deployment not found",
			deploymentID:        testDeploymentID,
			gatewayID:           testGatewayID,
			mockDeployment:      nil,
			mockDeploymentError: nil,
			wantErr:             true,
			expectedErr:         constants.ErrDeploymentNotFound,
		},
		{
			name:         "deployment not active (UNDEPLOYED)",
			deploymentID: testDeploymentID,
			gatewayID:    testGatewayID,
			mockDeployment: &model.Deployment{
				DeploymentID: testDeploymentID,
				Name:         "test-deployment",
				ArtifactID:   testAPIUUID,
				GatewayID:    testGatewayID,
				Status:       &undeployedStatus,
			},
			wantErr:     true,
			expectedErr: constants.ErrDeploymentNotActive,
		},
		{
			name:         "deployment not active (nil status - ARCHIVED)",
			deploymentID: testDeploymentID,
			gatewayID:    testGatewayID,
			mockDeployment: &model.Deployment{
				DeploymentID: testDeploymentID,
				Name:         "test-deployment",
				ArtifactID:   testAPIUUID,
				GatewayID:    testGatewayID,
				Status:       nil, // ARCHIVED
			},
			wantErr:     true,
			expectedErr: constants.ErrDeploymentNotActive,
		},
		{
			name:         "gateway not found",
			deploymentID: testDeploymentID,
			gatewayID:    testGatewayID,
			mockDeployment: &model.Deployment{
				DeploymentID: testDeploymentID,
				Name:         "test-deployment",
				ArtifactID:   testAPIUUID,
				GatewayID:    testGatewayID,
				Status:       &deployedStatus,
			},
			mockGateway: nil,
			wantErr:     true,
			expectedErr: constants.ErrGatewayNotFound,
		},
		{
			name:         "set current deployment fails",
			deploymentID: testDeploymentID,
			gatewayID:    testGatewayID,
			mockDeployment: &model.Deployment{
				DeploymentID: testDeploymentID,
				Name:         "test-deployment",
				ArtifactID:   testAPIUUID,
				GatewayID:    testGatewayID,
				Status:       &deployedStatus,
			},
			mockGateway: &model.Gateway{
				ID:             testGatewayID,
				OrganizationID: testOrgUUID,
				Vhost:          "api.example.com",
			},
			mockSetCurrentError: errors.New("database write failed"),
			wantErr:             true,
			errContains:         "failed to update deployment status",
		},
		{
			name:         "gateway ID mismatch validation",
			deploymentID: testDeploymentID,
			gatewayID:    "wrong-gateway-id",
			mockDeployment: &model.Deployment{
				DeploymentID: testDeploymentID,
				Name:         "test-deployment",
				ArtifactID:   testAPIUUID,
				GatewayID:    testGatewayID, // Different from provided gatewayID
				Status:       &deployedStatus,
			},
			wantErr:     true,
			expectedErr: constants.ErrGatewayIDMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDeploymentRepo := &mockDeploymentRepo{
				deploymentWithState: tt.mockDeployment,
				getWithStateError:   tt.mockDeploymentError,
				setCurrentUpdatedAt: tt.mockSetCurrentTime,
				setCurrentError:     tt.mockSetCurrentError,
			}

			mockGatewayRepo := &mockDeploymentGatewayRepository{
				gateway:        tt.mockGateway,
				getByUUIDError: tt.mockGatewayError,
			}

			service := &DeploymentService{
				deploymentRepo: mockDeploymentRepo,
				gatewayRepo:    mockGatewayRepo,
			}

			result, err := service.UndeployDeployment(testAPIUUID, tt.deploymentID, tt.gatewayID, testOrgUUID)

			// Check error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("UndeployDeployment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) && err != tt.expectedErr {
					t.Errorf("UndeployDeployment() error = %v, expectedErr %v", err, tt.expectedErr)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("UndeployDeployment() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			// Verify successful result
			if result == nil {
				t.Fatal("UndeployDeployment() result is nil, expected non-nil")
			}

			if string(result.Status) != string(model.DeploymentStatusUndeployed) {
				t.Errorf("UndeployDeployment() Status = %v, want %v", result.Status, model.DeploymentStatusUndeployed)
			}

			// Verify updatedAt is returned from SetCurrentDeployment
			if tt.verifyUpdatedAt {
				if result.UpdatedAt == nil {
					t.Error("UndeployDeployment() UpdatedAt is nil, expected non-nil")
				} else if !result.UpdatedAt.Equal(tt.mockSetCurrentTime) {
					t.Errorf("UndeployDeployment() UpdatedAt = %v, want %v", *result.UpdatedAt, tt.mockSetCurrentTime)
				}
			}

			// Verify SetCurrentDeployment was called with UNDEPLOYED status
			if !mockDeploymentRepo.setCurrentCalled {
				t.Error("SetCurrentDeployment was not called")
			}
			if mockDeploymentRepo.setCurrentStatus != model.DeploymentStatusUndeployed {
				t.Errorf("SetCurrentDeployment called with status %v, want %v", mockDeploymentRepo.setCurrentStatus, model.DeploymentStatusUndeployed)
			}
		})
	}
}

// ============================================================================
// DeploymentService.DeleteDeployment Tests
// ============================================================================

func TestDeleteDeployment(t *testing.T) {
	testOrgUUID := "00000000-0000-0000-0000-000000000123"
	testAPIUUID := "11111111-1111-1111-1111-111111111111"
	testGatewayID := "22222222-2222-2222-2222-222222222222"
	testDeploymentID := "33333333-3333-3333-3333-333333333333"
	deployedStatus := model.DeploymentStatusDeployed
	undeployedStatus := model.DeploymentStatusUndeployed

	tests := []struct {
		name                string
		deploymentID        string
		mockDeployment      *model.Deployment
		mockDeploymentError error
		mockDeleteError     error
		wantErr             bool
		expectedErr         error
		errContains         string
		verifyDeleteCalled  bool
	}{
		{
			name:         "successful delete of UNDEPLOYED deployment",
			deploymentID: testDeploymentID,
			mockDeployment: &model.Deployment{
				DeploymentID: testDeploymentID,
				Name:         "test-deployment",
				ArtifactID:   testAPIUUID,
				GatewayID:    testGatewayID,
				Status:       &undeployedStatus,
			},
			wantErr:            false,
			verifyDeleteCalled: true,
		},
		{
			name:         "successful delete of ARCHIVED deployment (nil status)",
			deploymentID: testDeploymentID,
			mockDeployment: &model.Deployment{
				DeploymentID: testDeploymentID,
				Name:         "test-deployment",
				ArtifactID:   testAPIUUID,
				GatewayID:    testGatewayID,
				Status:       nil, // ARCHIVED
			},
			wantErr:            false,
			verifyDeleteCalled: true,
		},
		{
			name:                "deployment not found",
			deploymentID:        testDeploymentID,
			mockDeployment:      nil,
			mockDeploymentError: nil,
			wantErr:             true,
			expectedErr:         constants.ErrDeploymentNotFound,
		},
		{
			name:         "cannot delete DEPLOYED deployment",
			deploymentID: testDeploymentID,
			mockDeployment: &model.Deployment{
				DeploymentID: testDeploymentID,
				Name:         "test-deployment",
				ArtifactID:   testAPIUUID,
				GatewayID:    testGatewayID,
				Status:       &deployedStatus,
			},
			wantErr:     true,
			expectedErr: constants.ErrDeploymentIsDeployed,
		},
		{
			name:         "delete operation fails",
			deploymentID: testDeploymentID,
			mockDeployment: &model.Deployment{
				DeploymentID: testDeploymentID,
				Name:         "test-deployment",
				ArtifactID:   testAPIUUID,
				GatewayID:    testGatewayID,
				Status:       &undeployedStatus,
			},
			mockDeleteError:    errors.New("database delete failed"),
			wantErr:            true,
			errContains:        "failed to delete deployment",
			verifyDeleteCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDeploymentRepo := &mockDeploymentRepo{
				deploymentWithState: tt.mockDeployment,
				getWithStateError:   tt.mockDeploymentError,
				deleteError:         tt.mockDeleteError,
			}

			service := &DeploymentService{
				deploymentRepo: mockDeploymentRepo,
			}

			err := service.DeleteDeployment(testAPIUUID, tt.deploymentID, testOrgUUID)

			// Check error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteDeployment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) && err != tt.expectedErr {
					t.Errorf("DeleteDeployment() error = %v, expectedErr %v", err, tt.expectedErr)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("DeleteDeployment() error = %v, want error containing %q", err, tt.errContains)
				}
			}

			// Verify delete was called when expected
			if tt.verifyDeleteCalled && !mockDeploymentRepo.deleteCalled {
				t.Error("DeleteDeployment repository method was not called")
			}
		})
	}
}

// ============================================================================
// DeploymentService.GetDeployments Tests
// ============================================================================

func TestGetDeployments(t *testing.T) {
	testOrgUUID := "00000000-0000-0000-0000-000000000123"
	testAPIUUID := "11111111-1111-1111-1111-111111111111"
	testGatewayID := "22222222-2222-2222-2222-222222222222"
	deployedStatus := model.DeploymentStatusDeployed
	undeployedStatus := model.DeploymentStatusUndeployed
	archivedStatus := model.DeploymentStatusArchived

	tests := []struct {
		name            string
		gatewayID       *string
		status          *string
		mockAPI         *model.API
		mockAPIError    error
		mockDeployments []*model.Deployment
		wantErr         bool
		expectedErr     error
		expectedCount   int
	}{
		{
			name:      "successful get all deployments",
			gatewayID: nil,
			status:    nil,
			mockAPI: &model.API{
				ID:             testAPIUUID,
				OrganizationID: testOrgUUID,
			},
			mockDeployments: []*model.Deployment{
				{DeploymentID: "88888888-8888-8888-8888-888888888888", GatewayID: testGatewayID, Status: &deployedStatus},
				{DeploymentID: "99999999-9999-9999-9999-999999999999", GatewayID: testGatewayID, Status: &undeployedStatus},
				{DeploymentID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", GatewayID: testGatewayID, Status: &archivedStatus},
			},
			wantErr:       false,
			expectedCount: 3,
		},
		{
			name:      "filter by gateway",
			gatewayID: &testGatewayID,
			status:    nil,
			mockAPI: &model.API{
				ID:             testAPIUUID,
				OrganizationID: testOrgUUID,
			},
			mockDeployments: []*model.Deployment{
				{DeploymentID: "88888888-8888-8888-8888-888888888888", GatewayID: testGatewayID, Status: &deployedStatus},
			},
			wantErr:       false,
			expectedCount: 1,
		},
		{
			name:      "filter by status DEPLOYED",
			gatewayID: nil,
			status:    strPtr("DEPLOYED"),
			mockAPI: &model.API{
				ID:             testAPIUUID,
				OrganizationID: testOrgUUID,
			},
			mockDeployments: []*model.Deployment{
				{DeploymentID: "88888888-8888-8888-8888-888888888888", GatewayID: testGatewayID, Status: &deployedStatus},
			},
			wantErr:       false,
			expectedCount: 1,
		},
		{
			name:      "filter by status ARCHIVED",
			gatewayID: nil,
			status:    strPtr("ARCHIVED"),
			mockAPI: &model.API{
				ID:             testAPIUUID,
				OrganizationID: testOrgUUID,
			},
			mockDeployments: []*model.Deployment{
				{DeploymentID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", GatewayID: testGatewayID, Status: &archivedStatus},
			},
			wantErr:       false,
			expectedCount: 1,
		},
		{
			name:         "API not found",
			gatewayID:    nil,
			status:       nil,
			mockAPI:      nil,
			mockAPIError: nil,
			wantErr:      true,
			expectedErr:  constants.ErrAPINotFound,
		},
		{
			name:      "invalid status parameter",
			gatewayID: nil,
			status:    strPtr("INVALID_STATUS"),
			mockAPI: &model.API{
				ID:             testAPIUUID,
				OrganizationID: testOrgUUID,
			},
			wantErr:     true,
			expectedErr: constants.ErrInvalidDeploymentStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPIRepo := &mockDeploymentAPIRepository{
				api: tt.mockAPI,
			}
			mockAPIRepo.getAPIByUUIDError = tt.mockAPIError

			mockDeploymentRepo := &mockDeploymentRepo{
				deployments: tt.mockDeployments,
			}

			service := &DeploymentService{
				apiRepo:        mockAPIRepo,
				deploymentRepo: mockDeploymentRepo,
				cfg:            &testConfig,
			}

			result, err := service.GetDeployments(testAPIUUID, testOrgUUID, tt.gatewayID, tt.status)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetDeployments() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) && err != tt.expectedErr {
					t.Errorf("GetDeployments() error = %v, expectedErr %v", err, tt.expectedErr)
				}
				return
			}

			if result == nil {
				t.Fatal("GetDeployments() result is nil")
			}

			if result.Count != tt.expectedCount {
				t.Errorf("GetDeployments() count = %d, want %d", result.Count, tt.expectedCount)
			}
		})
	}
}

// ============================================================================
// DeploymentService.GetDeployment Tests
// ============================================================================

func TestGetDeployment(t *testing.T) {
	testOrgUUID := "00000000-0000-0000-0000-000000000123"
	testAPIUUID := "11111111-1111-1111-1111-111111111111"
	testGatewayID := "22222222-2222-2222-2222-222222222222"
	testDeploymentID := "33333333-3333-3333-3333-333333333333"
	deployedStatus := model.DeploymentStatusDeployed

	tests := []struct {
		name                string
		mockAPI             *model.API
		mockAPIError        error
		mockDeployment      *model.Deployment
		mockDeploymentError error
		wantErr             bool
		expectedErr         error
	}{
		{
			name: "successful get deployment",
			mockAPI: &model.API{
				ID:             testAPIUUID,
				OrganizationID: testOrgUUID,
			},
			mockDeployment: &model.Deployment{
				DeploymentID: testDeploymentID,
				Name:         "test-deployment",
				GatewayID:    testGatewayID,
				Status:       &deployedStatus,
			},
			wantErr: false,
		},
		{
			name:         "API not found",
			mockAPI:      nil,
			mockAPIError: nil,
			wantErr:      true,
			expectedErr:  constants.ErrAPINotFound,
		},
		{
			name: "deployment not found",
			mockAPI: &model.API{
				ID:             testAPIUUID,
				OrganizationID: testOrgUUID,
			},
			mockDeployment: nil,
			wantErr:        true,
			expectedErr:    constants.ErrDeploymentNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPIRepo := &mockDeploymentAPIRepository{
				api:               tt.mockAPI,
				getAPIByUUIDError: tt.mockAPIError,
			}

			mockDeploymentRepo := &mockDeploymentRepo{
				deploymentWithState: tt.mockDeployment,
				getWithStateError:   tt.mockDeploymentError,
			}

			service := &DeploymentService{
				apiRepo:        mockAPIRepo,
				deploymentRepo: mockDeploymentRepo,
			}

			result, err := service.GetDeployment(testAPIUUID, testDeploymentID, testOrgUUID)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetDeployment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) && err != tt.expectedErr {
					t.Errorf("GetDeployment() error = %v, expectedErr %v", err, tt.expectedErr)
				}
				return
			}

			if result == nil {
				t.Fatal("GetDeployment() result is nil")
			}

			if isValidUUIDString(testDeploymentID) && result.DeploymentId.String() != testDeploymentID {
				t.Errorf("GetDeployment() DeploymentID = %v, want %v", result.DeploymentId.String(), testDeploymentID)
			}
		})
	}
}

// ============================================================================
// Metadata Endpoint URL Type Assertion Tests
// ============================================================================

func TestDeployAPI_MetadataEndpointURLTypeAssertion(t *testing.T) {
	// This test verifies the safe type assertion for endpointUrl in metadata
	// The actual DeployAPI requires many dependencies, so we test the logic pattern

	tests := []struct {
		name        string
		metadata    map[string]interface{}
		expectError bool
		errContains string
	}{
		{
			name:        "valid string endpoint URL",
			metadata:    map[string]interface{}{"endpointUrl": "https://api.example.com"},
			expectError: false,
		},
		{
			name:        "empty string endpoint URL - should skip override",
			metadata:    map[string]interface{}{"endpointUrl": ""},
			expectError: false,
		},
		{
			name:        "nil metadata - should skip override",
			metadata:    nil,
			expectError: false,
		},
		{
			name:        "metadata without endpointUrl - should skip override",
			metadata:    map[string]interface{}{"otherKey": "value"},
			expectError: false,
		},
		{
			name:        "integer endpoint URL - should error",
			metadata:    map[string]interface{}{"endpointUrl": 12345},
			expectError: true,
			errContains: "expected string",
		},
		{
			name:        "boolean endpoint URL - should error",
			metadata:    map[string]interface{}{"endpointUrl": true},
			expectError: true,
			errContains: "expected string",
		},
		{
			name:        "array endpoint URL - should error",
			metadata:    map[string]interface{}{"endpointUrl": []string{"url1", "url2"}},
			expectError: true,
			errContains: "expected string",
		},
		{
			name:        "map endpoint URL - should error",
			metadata:    map[string]interface{}{"endpointUrl": map[string]string{"url": "value"}},
			expectError: true,
			errContains: "expected string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the type assertion logic from DeployAPI
			var err error
			if tt.metadata != nil {
				if v, exists := tt.metadata["endpointUrl"]; exists {
					_, ok := v.(string)
					if !ok {
						typeStr := fmt.Sprintf("%T", v)
						typeStr = strings.Replace(typeStr, "[]", "slice of ", 1)
						typeStr = strings.Replace(typeStr, "map[", "map of ", 1)
						typeStr = strings.TrimPrefix(typeStr, "*")
						if strings.HasPrefix(typeStr, "pointer to ") {
							// already processed
						} else if strings.HasPrefix(fmt.Sprintf("%T", v), "*") {
							typeStr = "pointer to " + typeStr
						}
						typeStr = strings.Replace(typeStr, "interface {}", "interface", 1)
						err = errors.New("invalid endpoint URL in metadata: expected string, got " + typeStr)
					}
				}
			}

			if (err != nil) != tt.expectError {
				t.Errorf("Type assertion error = %v, expectError %v", err, tt.expectError)
				return
			}

			if err != nil && tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Error = %v, want error containing %q", err, tt.errContains)
			}
		})
	}
}

// ============================================================================
// Helper functions for tests
// ============================================================================

func strPtr(s string) *string {
	return &s
}

// testConfig provides a test configuration for DeploymentService
var testConfig = config.Server{
	Deployments: config.Deployments{
		MaxPerAPIGateway: 20,
	},
}

// ============================================================================
// Edge Case Tests (from design doc Phase 7)
// ============================================================================

// TestRollbackDeployment_WhenAllDeploymentsArchived tests the edge case where
// the status table has no row because all previous deployments are ARCHIVED.
// Rolling back to an archived deployment should succeed and create a status row.
func TestRollbackDeployment_WhenAllDeploymentsArchived(t *testing.T) {
	testOrgUUID := "00000000-0000-0000-0000-000000000123"
	testAPIUUID := "11111111-1111-1111-1111-111111111111"
	testGatewayID := "22222222-2222-2222-2222-222222222222"
	testDeploymentID := "33333333-3333-3333-3333-333333333333"
	testUpdatedAt := time.Now()

	mockDeploymentRepo := &mockDeploymentRepo{
		deploymentWithContent: &model.Deployment{
			DeploymentID: testDeploymentID,
			Name:         "archived-deployment",
			ArtifactID:   testAPIUUID,
			GatewayID:    testGatewayID,
			Content:      []byte("archived content"),
		},
		// GetDeploymentStatus returns empty when no status row exists (all ARCHIVED)
		currentDeploymentID: "",
		currentStatus:       "",
		statusUpdatedAt:     nil,
		setCurrentUpdatedAt: testUpdatedAt,
	}

	mockGatewayRepo := &mockDeploymentGatewayRepository{
		gateway: &model.Gateway{
			ID:             testGatewayID,
			OrganizationID: testOrgUUID,
			Vhost:          "api.example.com",
		},
	}

	service := &DeploymentService{
		deploymentRepo: mockDeploymentRepo,
		gatewayRepo:    mockGatewayRepo,
	}

	result, err := service.RestoreDeployment(testAPIUUID, testDeploymentID, testGatewayID, testOrgUUID)

	if err != nil {
		t.Fatalf("RestoreDeployment() unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("RestoreDeployment() result is nil, expected non-nil")
	}

	if string(result.Status) != string(model.DeploymentStatusDeployed) {
		t.Errorf("Expected status DEPLOYED, got %s", result.Status)
	}

	if !mockDeploymentRepo.setCurrentCalled {
		t.Error("SetCurrentDeployment was not called")
	}
}

// TestRollbackDeployment_ToArchivedWhenCurrentUndeployed tests rollback to an
// ARCHIVED deployment when current deployment is UNDEPLOYED
func TestRollbackDeployment_ToArchivedWhenCurrentUndeployed(t *testing.T) {
	testOrgUUID := "00000000-0000-0000-0000-000000000123"
	testAPIUUID := "11111111-1111-1111-1111-111111111111"
	testGatewayID := "22222222-2222-2222-2222-222222222222"
	testDeploymentID := "66666666-6666-6666-6666-666666666666"
	currentDeploymentID := "77777777-7777-7777-7777-777777777777"
	testUpdatedAt := time.Now()

	mockDeploymentRepo := &mockDeploymentRepo{
		deploymentWithContent: &model.Deployment{
			DeploymentID: testDeploymentID,
			Name:         "old-archived-deployment",
			ArtifactID:   testAPIUUID,
			GatewayID:    testGatewayID,
			Content:      []byte("archived content"),
		},
		// Current deployment is UNDEPLOYED (different from target)
		currentDeploymentID: currentDeploymentID,
		currentStatus:       model.DeploymentStatusUndeployed,
		setCurrentUpdatedAt: testUpdatedAt,
	}

	mockGatewayRepo := &mockDeploymentGatewayRepository{
		gateway: &model.Gateway{
			ID:             testGatewayID,
			OrganizationID: testOrgUUID,
			Vhost:          "api.example.com",
		},
	}

	service := &DeploymentService{
		deploymentRepo: mockDeploymentRepo,
		gatewayRepo:    mockGatewayRepo,
	}

	result, err := service.RestoreDeployment(testAPIUUID, testDeploymentID, testGatewayID, testOrgUUID)

	if err != nil {
		t.Fatalf("RestoreDeployment() unexpected error: %v", err)
	}

	if isValidUUIDString(testDeploymentID) && result.DeploymentId.String() != testDeploymentID {
		t.Errorf("Expected deployment ID %s, got %s", testDeploymentID, result.DeploymentId.String())
	}

	if string(result.Status) != string(model.DeploymentStatusDeployed) {
		t.Errorf("Expected status DEPLOYED, got %s", result.Status)
	}
}

// TestDeleteDeployment_ArchivedWithNoStatusRow tests deleting an ARCHIVED deployment
// (deployment exists in deployments but not in deployment_status)
func TestDeleteDeployment_ArchivedWithNoStatusRow(t *testing.T) {
	testOrgUUID := "00000000-0000-0000-0000-000000000123"
	testAPIUUID := "11111111-1111-1111-1111-111111111111"
	testGatewayID := "22222222-2222-2222-2222-222222222222"
	testDeploymentID := "66666666-6666-6666-6666-666666666666"

	mockDeploymentRepo := &mockDeploymentRepo{
		deploymentWithState: &model.Deployment{
			DeploymentID: testDeploymentID,
			Name:         "archived-deployment",
			ArtifactID:   testAPIUUID,
			GatewayID:    testGatewayID,
			Status:       nil, // nil status means ARCHIVED
		},
	}

	service := &DeploymentService{
		deploymentRepo: mockDeploymentRepo,
	}

	err := service.DeleteDeployment(testAPIUUID, testDeploymentID, testOrgUUID)

	if err != nil {
		t.Fatalf("DeleteDeployment() unexpected error for ARCHIVED deployment: %v", err)
	}

	if !mockDeploymentRepo.deleteCalled {
		t.Error("DeleteDeployment repository method was not called")
	}
}

// TestGetDeployments_MixedStates tests retrieving deployments with mixed states
// (DEPLOYED, UNDEPLOYED, and ARCHIVED)
func TestGetDeployments_MixedStates(t *testing.T) {
	testOrgUUID := "00000000-0000-0000-0000-000000000123"
	testAPIUUID := "11111111-1111-1111-1111-111111111111"
	testGatewayID := "22222222-2222-2222-2222-222222222222"
	deployedStatus := model.DeploymentStatusDeployed
	undeployedStatus := model.DeploymentStatusUndeployed
	archivedStatus := model.DeploymentStatusArchived

	mockAPIRepo := &mockDeploymentAPIRepository{
		api: &model.API{ID: testAPIUUID, OrganizationID: testOrgUUID},
	}
	mockDeploymentRepo := &mockDeploymentRepo{
		deployments: []*model.Deployment{
			{
				DeploymentID: "88888888-8888-8888-8888-888888888888",
				Name:         "deployed-version",
				ArtifactID:   testAPIUUID,
				GatewayID:    testGatewayID,
				Status:       &deployedStatus,
				CreatedAt:    time.Now(),
			},
			{
				DeploymentID: "99999999-9999-9999-9999-999999999999",
				Name:         "undeployed-version",
				ArtifactID:   testAPIUUID,
				GatewayID:    testGatewayID,
				Status:       &undeployedStatus,
				CreatedAt:    time.Now().Add(-1 * time.Hour),
			},
			{
				DeploymentID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				Name:         "archived-version",
				ArtifactID:   testAPIUUID,
				GatewayID:    testGatewayID,
				Status:       &archivedStatus, // ARCHIVED - repository sets this, not nil
				CreatedAt:    time.Now().Add(-2 * time.Hour),
			},
		},
	}

	service := &DeploymentService{
		apiRepo:        mockAPIRepo,
		deploymentRepo: mockDeploymentRepo,
		cfg:            &testConfig,
	}

	result, err := service.GetDeployments(testAPIUUID, testOrgUUID, nil, nil)

	if err != nil {
		t.Fatalf("GetDeployments() unexpected error: %v", err)
	}

	if len(result.List) != 3 {
		t.Errorf("Expected 3 deployments, got %d", len(result.List))
	}

	// Verify states are correctly derived
	stateMap := make(map[string]string)
	for _, d := range result.List {
		stateMap[string(d.Status)] = stateMap[string(d.Status)] + "x"
	}

	if len(stateMap[string(model.DeploymentStatusDeployed)]) != 1 {
		t.Errorf("expected 1 DEPLOYED deployment, got %d", len(stateMap[string(model.DeploymentStatusDeployed)]))
	}
	if len(stateMap[string(model.DeploymentStatusUndeployed)]) != 1 {
		t.Errorf("expected 1 UNDEPLOYED deployment, got %d", len(stateMap[string(model.DeploymentStatusUndeployed)]))
	}
	if len(stateMap[string(model.DeploymentStatusArchived)]) != 1 {
		t.Errorf("expected 1 ARCHIVED deployment, got %d", len(stateMap[string(model.DeploymentStatusArchived)]))
	}
}

// TestGetDeployments_EmptyList tests retrieving deployments when none exist
func TestGetDeployments_EmptyList(t *testing.T) {
	testOrgUUID := "00000000-0000-0000-0000-000000000123"
	testAPIUUID := "11111111-1111-1111-1111-111111111111"

	mockAPIRepo := &mockDeploymentAPIRepository{
		api: &model.API{ID: testAPIUUID, OrganizationID: testOrgUUID},
	}
	mockDeploymentRepo := &mockDeploymentRepo{
		deployments: []*model.Deployment{}, // Empty list
	}

	service := &DeploymentService{
		apiRepo:        mockAPIRepo,
		deploymentRepo: mockDeploymentRepo,
		cfg:            &testConfig,
	}

	result, err := service.GetDeployments(testAPIUUID, testOrgUUID, nil, nil)

	if err != nil {
		t.Fatalf("GetDeployments() unexpected error: %v", err)
	}

	if len(result.List) != 0 {
		t.Errorf("Expected 0 deployments, got %d", len(result.List))
	}
}

// TestUndeployDeployment_WhenOnlyOneDeploymentExists tests undeploying the only
// existing deployment for an API+Gateway combination
func TestUndeployDeployment_WhenOnlyOneDeploymentExists(t *testing.T) {
	testOrgUUID := "00000000-0000-0000-0000-000000000123"
	testAPIUUID := "11111111-1111-1111-1111-111111111111"
	testGatewayID := "22222222-2222-2222-2222-222222222222"
	testDeploymentID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	testUpdatedAt := time.Now()
	deployedStatus := model.DeploymentStatusDeployed

	mockDeploymentRepo := &mockDeploymentRepo{
		deploymentWithState: &model.Deployment{
			DeploymentID: testDeploymentID,
			Name:         "only-deployment",
			ArtifactID:   testAPIUUID,
			GatewayID:    testGatewayID,
			Status:       &deployedStatus,
		},
		setCurrentUpdatedAt: testUpdatedAt,
	}

	mockGatewayRepo := &mockDeploymentGatewayRepository{
		gateway: &model.Gateway{
			ID:             testGatewayID,
			OrganizationID: testOrgUUID,
			Vhost:          "api.example.com",
		},
	}

	service := &DeploymentService{
		deploymentRepo: mockDeploymentRepo,
		gatewayRepo:    mockGatewayRepo,
	}

	result, err := service.UndeployDeployment(testAPIUUID, testDeploymentID, testGatewayID, testOrgUUID)

	if err != nil {
		t.Fatalf("UndeployDeployment() unexpected error: %v", err)
	}

	if string(result.Status) != string(model.DeploymentStatusUndeployed) {
		t.Errorf("Expected status UNDEPLOYED, got %s", result.Status)
	}

	// The deployment should transition to UNDEPLOYED, not be deleted
	if mockDeploymentRepo.deleteCalled {
		t.Error("DeleteDeployment should not be called - undeploy should only change status")
	}
}

// TestRollbackDeployment_SameDeploymentDifferentStatus tests that rollback fails
// when trying to rollback to the currently DEPLOYED deployment (even if status check passes first)
func TestRollbackDeployment_CurrentlyDeployedSameID(t *testing.T) {
	testOrgUUID := "00000000-0000-0000-0000-000000000123"
	testAPIUUID := "11111111-1111-1111-1111-111111111111"
	testGatewayID := "22222222-2222-2222-2222-222222222222"
	testDeploymentID := "77777777-7777-7777-7777-777777777777"

	mockDeploymentRepo := &mockDeploymentRepo{
		deploymentWithContent: &model.Deployment{
			DeploymentID: testDeploymentID,
			Name:         "current-deployment",
			ArtifactID:   testAPIUUID,
			GatewayID:    testGatewayID,
			Content:      []byte("current content"),
		},
		// Same deployment ID is currently DEPLOYED
		currentDeploymentID: testDeploymentID,
		currentStatus:       model.DeploymentStatusDeployed,
	}

	mockGatewayRepo := &mockDeploymentGatewayRepository{
		gateway: &model.Gateway{
			ID:             testGatewayID,
			OrganizationID: testOrgUUID,
			Vhost:          "api.example.com",
		},
	}

	service := &DeploymentService{
		deploymentRepo: mockDeploymentRepo,
		gatewayRepo:    mockGatewayRepo,
	}

	_, err := service.RestoreDeployment(testAPIUUID, testDeploymentID, testGatewayID, testOrgUUID)

	if err == nil {
		t.Fatal("Expected error when restoring to currently DEPLOYED deployment")
	}

	if !errors.Is(err, constants.ErrDeploymentAlreadyDeployed) {
		t.Errorf("Expected ErrDeploymentAlreadyDeployed, got %v", err)
	}

	// SetCurrentDeployment should NOT be called
	if mockDeploymentRepo.setCurrentCalled {
		t.Error("SetCurrentDeployment should not be called for already deployed deployment")
	}
}

// TestDeleteDeployment_CannotDeleteDeployed verifies that DEPLOYED deployments cannot be deleted
func TestDeleteDeployment_CannotDeleteDeployed(t *testing.T) {
	testOrgUUID := "00000000-0000-0000-0000-000000000123"
	testAPIUUID := "11111111-1111-1111-1111-111111111111"
	testGatewayID := "22222222-2222-2222-2222-222222222222"
	testDeploymentID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	deployedStatus := model.DeploymentStatusDeployed

	mockDeploymentRepo := &mockDeploymentRepo{
		deploymentWithState: &model.Deployment{
			DeploymentID: testDeploymentID,
			Name:         "deployed-deployment",
			ArtifactID:   testAPIUUID,
			GatewayID:    testGatewayID,
			Status:       &deployedStatus,
		},
	}

	service := &DeploymentService{
		deploymentRepo: mockDeploymentRepo,
	}

	err := service.DeleteDeployment(testAPIUUID, testDeploymentID, testOrgUUID)

	if err == nil {
		t.Fatal("Expected error when deleting DEPLOYED deployment")
	}

	if !errors.Is(err, constants.ErrDeploymentIsDeployed) {
		t.Errorf("Expected ErrDeploymentIsDeployed, got %v", err)
	}

	// Delete should NOT be called
	if mockDeploymentRepo.deleteCalled {
		t.Error("DeleteDeployment should not be called for DEPLOYED deployment")
	}
}

// TestGetDeployment_ArchivedDeployment tests retrieving an ARCHIVED deployment
func TestGetDeployment_ArchivedDeployment(t *testing.T) {
	testOrgUUID := "00000000-0000-0000-0000-000000000123"
	testAPIUUID := "11111111-1111-1111-1111-111111111111"
	testGatewayID := "22222222-2222-2222-2222-222222222222"
	testDeploymentID := "dddddddd-dddd-dddd-dddd-dddddddddddd"
	archivedStatus := model.DeploymentStatusArchived

	mockAPIRepo := &mockDeploymentAPIRepository{
		api: &model.API{ID: testAPIUUID, OrganizationID: testOrgUUID},
	}
	mockDeploymentRepo := &mockDeploymentRepo{
		deploymentWithState: &model.Deployment{
			DeploymentID: testDeploymentID,
			Name:         "archived-deployment",
			ArtifactID:   testAPIUUID,
			GatewayID:    testGatewayID,
			Status:       &archivedStatus, // ARCHIVED - repository sets this
			CreatedAt:    time.Now().Add(-24 * time.Hour),
		},
	}

	service := &DeploymentService{
		apiRepo:        mockAPIRepo,
		deploymentRepo: mockDeploymentRepo,
	}

	result, err := service.GetDeployment(testAPIUUID, testDeploymentID, testOrgUUID)

	if err != nil {
		t.Fatalf("GetDeployment() unexpected error: %v", err)
	}

	if string(result.Status) != string(model.DeploymentStatusArchived) {
		t.Errorf("Expected status ARCHIVED, got %s", result.Status)
	}
}

// TestRollbackDeployment_NonExistentDeployment tests rollback to a deployment that doesn't exist
func TestRollbackDeployment_NonExistentDeployment(t *testing.T) {
	testOrgUUID := "00000000-0000-0000-0000-000000000123"
	testAPIUUID := "11111111-1111-1111-1111-111111111111"
	testGatewayID := "22222222-2222-2222-2222-222222222222"
	testDeploymentID := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"

	mockDeploymentRepo := &mockDeploymentRepo{
		deploymentWithContent: nil, // Deployment not found
	}

	service := &DeploymentService{
		deploymentRepo: mockDeploymentRepo,
	}

	_, err := service.RestoreDeployment(testAPIUUID, testDeploymentID, testGatewayID, testOrgUUID)

	if err == nil {
		t.Fatal("Expected error when restoring to non-existent deployment")
	}

}

func TestIsValidVHostOrSentinel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"sentinel accepted", "_gateway_default_", true},
		{"simple hostname", "api.example.com", true},
		{"single label", "localhost", true},
		{"wildcard label", "*.example.com", false},
		{"empty string rejected", "", false},
		{"trailing dot rejected", "api.example.com.", false},
		{"label too long", strings.Repeat("a", 64) + ".com", false},
		{"underscore rejected", "api_v1.example.com", false},
		{"hyphen start rejected", "-api.example.com", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isValidVHostOrSentinel(tc.input)
			if got != tc.want {
				t.Errorf("isValidVHostOrSentinel(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestOverrideVhost(t *testing.T) {
	baseYAML := `apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: test-api
spec:
  displayName: Test API
  version: v1.0
  context: /test
  upstream:
    main:
      url: http://backend:8080
`

	t.Run("sets main only", func(t *testing.T) {
		result, err := overrideVhost([]byte(baseYAML), "api.example.com", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var parsed dto.APIDeploymentYAML
		if err := yaml.Unmarshal(result, &parsed); err != nil {
			t.Fatalf("failed to parse result: %v", err)
		}
		if parsed.Spec.Vhosts == nil {
			t.Fatal("expected vhosts to be set")
		}
		if parsed.Spec.Vhosts.Main != "api.example.com" {
			t.Errorf("main = %q, want %q", parsed.Spec.Vhosts.Main, "api.example.com")
		}
		if parsed.Spec.Vhosts.Sandbox != nil {
			t.Errorf("sandbox should be nil, got %q", *parsed.Spec.Vhosts.Sandbox)
		}
	})

	t.Run("sets main and sandbox", func(t *testing.T) {
		sandbox := "sandbox.example.com"
		result, err := overrideVhost([]byte(baseYAML), "api.example.com", &sandbox)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var parsed dto.APIDeploymentYAML
		if err := yaml.Unmarshal(result, &parsed); err != nil {
			t.Fatalf("failed to parse result: %v", err)
		}
		if parsed.Spec.Vhosts == nil {
			t.Fatal("expected vhosts to be set")
		}
		if parsed.Spec.Vhosts.Main != "api.example.com" {
			t.Errorf("main = %q, want %q", parsed.Spec.Vhosts.Main, "api.example.com")
		}
		if parsed.Spec.Vhosts.Sandbox == nil || *parsed.Spec.Vhosts.Sandbox != "sandbox.example.com" {
			t.Errorf("sandbox = %v, want %q", parsed.Spec.Vhosts.Sandbox, "sandbox.example.com")
		}
	})

	t.Run("overwrites existing vhosts", func(t *testing.T) {
		yamlWithVhosts := baseYAML + "  vhosts:\n    main: old.example.com\n    sandbox: old-sandbox.example.com\n"
		sandbox := "new-sandbox.example.com"
		result, err := overrideVhost([]byte(yamlWithVhosts), "new.example.com", &sandbox)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var parsed dto.APIDeploymentYAML
		if err := yaml.Unmarshal(result, &parsed); err != nil {
			t.Fatalf("failed to parse result: %v", err)
		}
		if parsed.Spec.Vhosts.Main != "new.example.com" {
			t.Errorf("main = %q, want %q", parsed.Spec.Vhosts.Main, "new.example.com")
		}
		if parsed.Spec.Vhosts.Sandbox == nil || *parsed.Spec.Vhosts.Sandbox != "new-sandbox.example.com" {
			t.Errorf("sandbox = %v, want %q", parsed.Spec.Vhosts.Sandbox, "new-sandbox.example.com")
		}
	})

	t.Run("accepts sentinel value", func(t *testing.T) {
		result, err := overrideVhost([]byte(baseYAML), "_gateway_default_", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var parsed dto.APIDeploymentYAML
		if err := yaml.Unmarshal(result, &parsed); err != nil {
			t.Fatalf("failed to parse result: %v", err)
		}
		if parsed.Spec.Vhosts == nil || parsed.Spec.Vhosts.Main != "_gateway_default_" {
			t.Errorf("expected sentinel, got %v", parsed.Spec.Vhosts)
		}
	})

	t.Run("invalid yaml returns error", func(t *testing.T) {
		_, err := overrideVhost([]byte("not: valid: yaml: :::"), "api.example.com", nil)
		if err == nil {
			t.Fatal("expected error for invalid YAML")
		}
	})
}
