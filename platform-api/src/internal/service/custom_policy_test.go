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
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"testing"
	"time"
)

// mockGatewayRepoForPolicy mocks only the GatewayRepository methods used by custom-policy operations.
type mockGatewayRepoForPolicy struct {
	repository.GatewayRepository

	gateway         *model.Gateway
	gatewayErr      error
	manifest        []byte
	manifestErr     error

	// call tracking
	getByUUIDCalled      bool
	getManifestCalled    bool
	lastGatewayID        string
}

func (m *mockGatewayRepoForPolicy) GetByUUID(gatewayID string) (*model.Gateway, error) {
	m.getByUUIDCalled = true
	m.lastGatewayID = gatewayID
	return m.gateway, m.gatewayErr
}

func (m *mockGatewayRepoForPolicy) GetGatewayManifest(gatewayID string) ([]byte, error) {
	m.getManifestCalled = true
	return m.manifest, m.manifestErr
}

// mockCustomPolicyRepo mocks all CustomPolicyRepository methods.
type mockCustomPolicyRepo struct {
	repository.CustomPolicyRepository
	insertErr                 error
	updateErr                 error
	getPolicyByNameVersion    *model.CustomPolicy
	getPolicyByNameVersionErr error
	getPolicyByUUID           *model.CustomPolicy
	getPolicyByUUIDErr        error
	getPoliciesByName         []*model.CustomPolicy
	getPoliciesByNameErr      error
	listPolicies              []*model.CustomPolicy
	listErr                   error
	deleteIfUnusedErr         error
	countUsages               int
	countUsagesErr            error
	insertCalled         bool
	updateCalled         bool
	updateOldVersion     string
	deleteIfUnusedCalled bool
}

func (m *mockCustomPolicyRepo) InsertCustomPolicy(policy *model.CustomPolicy) error {
	m.insertCalled = true
	return m.insertErr
}

func (m *mockCustomPolicyRepo) UpdateCustomPolicy(policy *model.CustomPolicy, oldVersion string) error {
	m.updateCalled = true
	m.updateOldVersion = oldVersion
	return m.updateErr
}

func (m *mockCustomPolicyRepo) GetCustomPolicyByNameAndVersion(orgUUID, name, version string) (*model.CustomPolicy, error) {
	return m.getPolicyByNameVersion, m.getPolicyByNameVersionErr
}

func (m *mockCustomPolicyRepo) GetCustomPolicyByUUID(orgUUID, policyUUID string) (*model.CustomPolicy, error) {
	return m.getPolicyByUUID, m.getPolicyByUUIDErr
}

func (m *mockCustomPolicyRepo) GetCustomPoliciesByName(orgUUID, name string) ([]*model.CustomPolicy, error) {
	return m.getPoliciesByName, m.getPoliciesByNameErr
}

func (m *mockCustomPolicyRepo) ListCustomPolicyByOrganization(orgUUID string) ([]*model.CustomPolicy, error) {
	return m.listPolicies, m.listErr
}

func (m *mockCustomPolicyRepo) DeleteCustomPolicyIfUnused(orgUUID, policyUUID string) error {
	m.deleteIfUnusedCalled = true
	return m.deleteIfUnusedErr
}

func (m *mockCustomPolicyRepo) CountCustomPolicyUsages(policyUUID string) (int, error) {
	return m.countUsages, m.countUsagesErr
}

// makeManifest serialises a slice of GatewayPolicyDefinition to JSON bytes.
func makeManifest(policies []GatewayPolicyDefinition) []byte {
	b, _ := json.Marshal(policies)
	return b
}

// makeCustomPolicy builds a minimal CustomPolicy for use in tests.
func makeCustomPolicy(uuid, orgID, name, version string) *model.CustomPolicy {
	return &model.CustomPolicy{
		UUID:             uuid,
		OrganizationUUID: orgID,
		Name:             name,
		Version:          version,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

// sampleManifest returns a manifest with one custom policy entry including both
// parameters and systemParameters sections.
func sampleManifest(name, version string) []byte {
	return makeManifest([]GatewayPolicyDefinition{
		{
			Name:      name,
			Version:   version,
			ManagedBy: constants.PolicyManagedByCustomer,
			PolicyDefinition: map[string]interface{}{
				"parameters": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
				"systemParameters": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
	})
}

// newTestGatewayService provides a GatewayService with given mock repos.
func newTestGatewayService(gwRepo repository.GatewayRepository, cpRepo repository.CustomPolicyRepository) *GatewayService {
	return &GatewayService{
		gatewayRepo:      gwRepo,
		customPolicyRepo: cpRepo,
		slogger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestSyncCustomPolicy(t *testing.T) {
	const (
		orgID     = "org-uuid-0001"
		gwID      = "gw-uuid-0001"
		otherOrg  = "org-uuid-OTHER"
		policyUUID = "pol-uuid-0001"
	)

	tests := []struct {
		name string
		gateway    *model.Gateway
		gatewayErr error
		manifest   []byte
		manifestErr error
		existingPolicies    []*model.CustomPolicy
		existingPoliciesErr error
		insertErr           error
		updateErr           error
		persistedPolicy *model.CustomPolicy
		policyName string
		version    string
		wantErr     bool
		errContains string
		wantInsert bool
		wantUpdate bool
	}{
		// gateway validation 
		{
			name:        "gateway not found - repo error",
			gatewayErr:  errors.New("db error"),
			policyName:  "rate-limit",
			version:     "1.0.0",
			wantErr:     true,
			errContains: "gateway not found",
		},
		{
			name:        "gateway not found - nil returned",
			gateway:     nil,
			policyName:  "rate-limit",
			version:     "1.0.0",
			wantErr:     true,
			errContains: "gateway not found",
		},
		{
			name: "gateway belongs to different org",
			gateway: &model.Gateway{ID: gwID, OrganizationID: otherOrg},
			policyName: "rate-limit",
			version:    "1.0.0",
			wantErr:    true,
			errContains: "gateway not found",
		},

		// manifest validation
		{
			name:        "manifest fetch error",
			gateway:     &model.Gateway{ID: gwID, OrganizationID: orgID},
			manifestErr: errors.New("storage error"),
			policyName:  "rate-limit",
			version:     "1.0.0",
			wantErr:     true,
			errContains: "failed to read gateway manifest",
		},
		{
			name:        "manifest is empty",
			gateway:     &model.Gateway{ID: gwID, OrganizationID: orgID},
			manifest:    []byte{},
			policyName:  "rate-limit",
			version:     "1.0.0",
			wantErr:     true,
			errContains: "gateway manifest is not available",
		},
		{
			name:    "policy not found in manifest",
			gateway: &model.Gateway{ID: gwID, OrganizationID: orgID},
			manifest: makeManifest([]GatewayPolicyDefinition{
				{Name: "other-policy", Version: "1.0.0", ManagedBy: constants.PolicyManagedByCustomer},
			}),
			policyName:  "rate-limit",
			version:     "1.0.0",
			wantErr:     true,
			errContains: "not found in gateway manifest",
		},
		{
			name:    "policy version mismatch in manifest",
			gateway: &model.Gateway{ID: gwID, OrganizationID: orgID},
			manifest: makeManifest([]GatewayPolicyDefinition{
				{Name: "rate-limit", Version: "1.1.0", ManagedBy: constants.PolicyManagedByCustomer},
			}),
			policyName:  "rate-limit",
			version:     "1.0.0",
			wantErr:     true,
			errContains: "not found in gateway manifest",
		},
		{
			name:    "policy is not a custom policy (wso2 managed)",
			gateway: &model.Gateway{ID: gwID, OrganizationID: orgID},
			manifest: makeManifest([]GatewayPolicyDefinition{
				{Name: "rate-limit", Version: "1.0.0", ManagedBy: "wso2"},
			}),
			policyName:  "rate-limit",
			version:     "1.0.0",
			wantErr:     true,
			errContains: "not a custom policy",
		},

		// version conflict rules
		{
			name:    "exact same version already exists",
			gateway: &model.Gateway{ID: gwID, OrganizationID: orgID},
			manifest: sampleManifest("rate-limit", "1.2.0"),
			existingPolicies: []*model.CustomPolicy{
				makeCustomPolicy(policyUUID, orgID, "rate-limit", "1.2.0"),
			},
			policyName:  "rate-limit",
			version:     "1.2.0",
			wantErr:     true,
			errContains: "already exists",
		},
		{
			name:    "patch version update is not allowed",
			gateway: &model.Gateway{ID: gwID, OrganizationID: orgID},
			manifest: sampleManifest("rate-limit", "1.2.1"),
			existingPolicies: []*model.CustomPolicy{
				makeCustomPolicy(policyUUID, orgID, "rate-limit", "1.2.0"),
			},
			policyName:  "rate-limit",
			version:     "1.2.1",
			wantErr:     true,
			errContains: "patch version updates are not allowed",
		},
		{
			name:    "downgrade is not allowed",
			gateway: &model.Gateway{ID: gwID, OrganizationID: orgID},
			manifest: sampleManifest("rate-limit", "1.1.0"),
			existingPolicies: []*model.CustomPolicy{
				makeCustomPolicy(policyUUID, orgID, "rate-limit", "1.3.0"),
			},
			policyName:  "rate-limit",
			version:     "1.1.0",
			wantErr:     true,
			errContains: "cannot downgrade",
		},

		//  custom policy successful paths
		{
			name:             "first sync - new policy inserted",
			gateway:          &model.Gateway{ID: gwID, OrganizationID: orgID},
			manifest:         sampleManifest("rate-limit", "1.0.0"),
			existingPolicies: []*model.CustomPolicy{},
			persistedPolicy:  makeCustomPolicy(policyUUID, orgID, "rate-limit", "1.0.0"),
			policyName:       "rate-limit",
			version:          "1.0.0",
			wantErr:          false,
			wantInsert:       true,
			wantUpdate:       false,
		},
		{
			name:    "minor version bump - existing record updated",
			gateway: &model.Gateway{ID: gwID, OrganizationID: orgID},
			manifest: sampleManifest("rate-limit", "1.3.0"),
			existingPolicies: []*model.CustomPolicy{
				makeCustomPolicy(policyUUID, orgID, "rate-limit", "1.2.0"),
			},
			persistedPolicy: makeCustomPolicy(policyUUID, orgID, "rate-limit", "1.3.0"),
			policyName:      "rate-limit",
			version:         "1.3.0",
			wantErr:         false,
			wantInsert:      false,
			wantUpdate:      true,
		},
		{
			name:    "new major version - separate record inserted",
			gateway: &model.Gateway{ID: gwID, OrganizationID: orgID},
			manifest: sampleManifest("rate-limit", "2.0.0"),
			existingPolicies: []*model.CustomPolicy{
				makeCustomPolicy(policyUUID, orgID, "rate-limit", "1.5.0"),
			},
			persistedPolicy: makeCustomPolicy("pol-uuid-0002", orgID, "rate-limit", "2.0.0"),
			policyName:      "rate-limit",
			version:         "2.0.0",
			wantErr:         false,
			wantInsert:      true,
			wantUpdate:      false,
		},
		{
			name:    "policy name normalised to lowercase before lookup",
			gateway: &model.Gateway{ID: gwID, OrganizationID: orgID},
			manifest: sampleManifest("rate-limit", "1.0.0"),
			existingPolicies: []*model.CustomPolicy{},
			persistedPolicy:  makeCustomPolicy(policyUUID, orgID, "rate-limit", "1.0.0"),
			policyName:       "Rate-Limit",
			version:          "1.0.0",
			wantErr:          false,
			wantInsert:       true,
		},
		{
			name:    "minor version update preserves the existing UUID",
			gateway: &model.Gateway{ID: gwID, OrganizationID: orgID},
			manifest: sampleManifest("auth-policy", "1.2.0"),
			existingPolicies: []*model.CustomPolicy{
				makeCustomPolicy("stable-uuid", orgID, "auth-policy", "1.1.0"),
			},
			persistedPolicy: makeCustomPolicy("stable-uuid", orgID, "auth-policy", "1.2.0"),
			policyName:      "auth-policy",
			version:         "1.2.0",
			wantErr:         false,
			wantUpdate:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gwRepo := &mockGatewayRepoForPolicy{
				gateway:     tt.gateway,
				gatewayErr:  tt.gatewayErr,
				manifest:    tt.manifest,
				manifestErr: tt.manifestErr,
			}
			cpRepo := &mockCustomPolicyRepo{
				getPoliciesByName:    tt.existingPolicies,
				getPoliciesByNameErr: tt.existingPoliciesErr,
				insertErr:            tt.insertErr,
				updateErr:            tt.updateErr,
				getPolicyByNameVersion: tt.persistedPolicy,
			}

			svc := newTestGatewayService(gwRepo, cpRepo)
			policy, err := svc.SyncCustomPolicy(gwID, orgID, tt.policyName, tt.version)

			if (err != nil) != tt.wantErr {
				t.Errorf("SyncCustomPolicy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" && !contains(err.Error(), tt.errContains) {
				t.Errorf("SyncCustomPolicy() error = %q, want it to contain %q", err.Error(), tt.errContains)
			}
			if !tt.wantErr && policy == nil {
				t.Error("SyncCustomPolicy() returned nil policy on success")
			}
			if tt.wantInsert && !cpRepo.insertCalled {
				t.Error("SyncCustomPolicy() expected InsertCustomPolicy to be called, but it was not")
			}
			if !tt.wantInsert && cpRepo.insertCalled {
				t.Error("SyncCustomPolicy() did not expect InsertCustomPolicy to be called")
			}
			if tt.wantUpdate && !cpRepo.updateCalled {
				t.Error("SyncCustomPolicy() expected UpdateCustomPolicy to be called, but it was not")
			}
			if !tt.wantUpdate && cpRepo.updateCalled {
				t.Error("SyncCustomPolicy() did not expect UpdateCustomPolicy to be called")
			}
		})
	}
}

// TestSyncCustomPolicy_MinorUpdatePreservesUUID verifies that a minor-version bump
// reuses the existing UUID instead of creating a new one.
func TestSyncCustomPolicy_MinorUpdatePreservesUUID(t *testing.T) {
	const (
		orgID      = "org-uuid-0001"
		gwID       = "gw-uuid-0001"
		existingID = "stable-policy-uuid"
	)

	existing := makeCustomPolicy(existingID, orgID, "my-policy", "1.1.0")
	persisted := makeCustomPolicy(existingID, orgID, "my-policy", "1.2.0")

	gwRepo := &mockGatewayRepoForPolicy{
		gateway:  &model.Gateway{ID: gwID, OrganizationID: orgID},
		manifest: sampleManifest("my-policy", "1.2.0"),
	}
	cpRepo := &mockCustomPolicyRepo{
		getPoliciesByName:      []*model.CustomPolicy{existing},
		getPolicyByNameVersion: persisted,
	}

	svc := newTestGatewayService(gwRepo, cpRepo)
	result, err := svc.SyncCustomPolicy(gwID, orgID, "my-policy", "1.2.0")
	if err != nil {
		t.Fatalf("SyncCustomPolicy() unexpected error: %v", err)
	}
	if result.UUID != existingID {
		t.Errorf("SyncCustomPolicy() UUID = %q, want %q (existing UUID should be preserved)", result.UUID, existingID)
	}
	if cpRepo.updateOldVersion != "1.1.0" {
		t.Errorf("UpdateCustomPolicy() called with oldVersion = %q, want %q", cpRepo.updateOldVersion, "1.1.0")
	}
}

// GetCustomPolicyByUUID tests

func TestGetCustomPolicyByUUID(t *testing.T) {
	const (
		orgID      = "org-uuid-0001"
		policyUUID = "pol-uuid-0001"
	)

	tests := []struct {
		name        string
		repoPolicy  *model.CustomPolicy
		repoErr     error
		version     string
		wantErr     bool
		expectedErr error
	}{
		{
			name:        "policy found with matching version",
			repoPolicy:  makeCustomPolicy(policyUUID, orgID, "rate-limit", "1.0.0"),
			version:     "1.0.0",
			wantErr:     false,
		},
		{
			name:        "policy not found - nil from repo",
			repoPolicy:  nil,
			version:     "1.0.0",
			wantErr:     true,
			expectedErr: constants.ErrCustomPolicyNotFound,
		},
		{
			name:        "repo returns error",
			repoErr:     errors.New("db failure"),
			version:     "1.0.0",
			wantErr:     true,
		},
		{
			name:        "version mismatch",
			repoPolicy:  makeCustomPolicy(policyUUID, orgID, "rate-limit", "1.0.0"),
			version:     "1.1.0",
			wantErr:     true,
			expectedErr: constants.ErrCustomPolicyVersionMismatch,
		},
		{
			name:        "major version mismatch",
			repoPolicy:  makeCustomPolicy(policyUUID, orgID, "rate-limit", "1.0.0"),
			version:     "2.0.0",
			wantErr:     true,
			expectedErr: constants.ErrCustomPolicyVersionMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpRepo := &mockCustomPolicyRepo{
				getPolicyByUUID:    tt.repoPolicy,
				getPolicyByUUIDErr: tt.repoErr,
			}
			svc := newTestGatewayService(&mockGatewayRepoForPolicy{}, cpRepo)

			policy, err := svc.GetCustomPolicyByUUID(orgID, policyUUID, tt.version)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetCustomPolicyByUUID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
				t.Errorf("GetCustomPolicyByUUID() error = %v, want %v", err, tt.expectedErr)
			}
			if !tt.wantErr && policy == nil {
				t.Error("GetCustomPolicyByUUID() returned nil policy on success")
			}
			if !tt.wantErr && policy != nil && policy.UUID != policyUUID {
				t.Errorf("GetCustomPolicyByUUID() UUID = %q, want %q", policy.UUID, policyUUID)
			}
		})
	}
}

// DeleteCustomPolicyByUUID tests

func TestDeleteCustomPolicyByUUID(t *testing.T) {
	const (
		orgID      = "org-uuid-0001"
		policyUUID = "pol-uuid-0001"
	)

	tests := []struct {
		name              string
		repoPolicy        *model.CustomPolicy
		repoErr           error
		deleteIfUnusedErr error
		version           string
		wantErr           bool
		expectedErr       error
		wantDeleteCalled  bool
	}{
		{
			name:             "successful delete",
			repoPolicy:       makeCustomPolicy(policyUUID, orgID, "rate-limit", "1.0.0"),
			version:          "1.0.0",
			wantErr:          false,
			wantDeleteCalled: true,
		},
		{
			name:        "policy not found",
			repoPolicy:  nil,
			version:     "1.0.0",
			wantErr:     true,
			expectedErr: constants.ErrCustomPolicyNotFound,
		},
		{
			name:        "repo returns error on lookup",
			repoErr:     errors.New("db failure"),
			version:     "1.0.0",
			wantErr:     true,
		},
		{
			name:        "version mismatch",
			repoPolicy:  makeCustomPolicy(policyUUID, orgID, "rate-limit", "1.0.0"),
			version:     "2.0.0",
			wantErr:     true,
			expectedErr: constants.ErrCustomPolicyVersionMismatch,
		},
		{
			name:              "policy in use by APIs",
			repoPolicy:        makeCustomPolicy(policyUUID, orgID, "rate-limit", "1.0.0"),
			version:           "1.0.0",
			deleteIfUnusedErr: constants.ErrCustomPolicyInUse,
			wantErr:           true,
			expectedErr:       constants.ErrCustomPolicyInUse,
			wantDeleteCalled:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpRepo := &mockCustomPolicyRepo{
				getPolicyByUUID:    tt.repoPolicy,
				getPolicyByUUIDErr: tt.repoErr,
				deleteIfUnusedErr:  tt.deleteIfUnusedErr,
			}
			svc := newTestGatewayService(&mockGatewayRepoForPolicy{}, cpRepo)

			err := svc.DeleteCustomPolicyByUUID(orgID, policyUUID, tt.version)

			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteCustomPolicyByUUID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
				t.Errorf("DeleteCustomPolicyByUUID() error = %v, want %v", err, tt.expectedErr)
			}
			if tt.wantDeleteCalled && !cpRepo.deleteIfUnusedCalled {
				t.Error("DeleteCustomPolicyByUUID() expected DeleteCustomPolicyIfUnused to be called, but it was not")
			}
			if !tt.wantDeleteCalled && cpRepo.deleteIfUnusedCalled {
				t.Error("DeleteCustomPolicyByUUID() did not expect DeleteCustomPolicyIfUnused to be called")
			}
		})
	}
}

// ListCustomPolicies tests

func TestListCustomPolicies(t *testing.T) {
	const orgID = "org-uuid-0001"

	tests := []struct {
		name        string
		repoPolicies []*model.CustomPolicy
		repoErr     error
		wantCount   int
		wantErr     bool
	}{
		{
			name: "returns all policies for org",
			repoPolicies: []*model.CustomPolicy{
				makeCustomPolicy("p1", orgID, "policy-a", "1.0.0"),
				makeCustomPolicy("p2", orgID, "policy-b", "2.0.0"),
			},
			wantCount: 2,
		},
		{
			name:         "returns empty list when no policies exist",
			repoPolicies: []*model.CustomPolicy{},
			wantCount:    0,
		},
		{
			name:    "repo error propagated",
			repoErr: errors.New("db failure"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpRepo := &mockCustomPolicyRepo{
				listPolicies: tt.repoPolicies,
				listErr:      tt.repoErr,
			}
			svc := newTestGatewayService(&mockGatewayRepoForPolicy{}, cpRepo)

			policies, err := svc.ListCustomPolicies(orgID)

			if (err != nil) != tt.wantErr {
				t.Errorf("ListCustomPolicies() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(policies) != tt.wantCount {
				t.Errorf("ListCustomPolicies() count = %d, want %d", len(policies), tt.wantCount)
			}
		})
	}
}
