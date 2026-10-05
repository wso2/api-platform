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

// TEMP-READ-ONLY-MODE: this whole file is part of the temporary organization-scoped
// read-only mode used while Bijira migrates from Platform API v1 to v2. Delete it
// when the mode is removed.

package service

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

const (
	readOnlyTestOrgA = "11111111-1111-1111-1111-111111111111"
	readOnlyTestOrgB = "22222222-2222-2222-2222-222222222222"
)

// fakeTimeoutDeploymentRepo serves fixed stale rows and records which of them the
// job tried to resolve. (mockDeploymentRepo is taken by deployment_test.go.)
type fakeTimeoutDeploymentRepo struct {
	repository.DeploymentRepository // embedded for the methods the job never calls

	stale   []repository.StaleDeploymentStatus
	updated []string // "<orgUUID>/<artifactUUID>" per UpdateStatusWithPerformedAtGuard call
}

func (f *fakeTimeoutDeploymentRepo) GetStaleTransitionalStatuses(time.Duration) ([]repository.StaleDeploymentStatus, error) {
	return f.stale, nil
}

func (f *fakeTimeoutDeploymentRepo) UpdateStatusWithPerformedAtGuard(artifactUUID, orgUUID, gatewayID string,
	newStatus model.DeploymentStatus, statusReason string, performedAt time.Time,
	requireCurrentStatus []model.DeploymentStatus) (int64, error) {
	f.updated = append(f.updated, orgUUID+"/"+artifactUUID)
	return 1, nil
}

func TestDeploymentTimeoutService_ReadOnlySkipsFrozenOrganizations(t *testing.T) {
	newRepo := func() *fakeTimeoutDeploymentRepo {
		return &fakeTimeoutDeploymentRepo{stale: []repository.StaleDeploymentStatus{
			{ArtifactUUID: "art-a", OrganizationUUID: readOnlyTestOrgA, GatewayUUID: "gw-a", Status: model.DeploymentStatusDeploying, PerformedAt: time.Now().Add(-time.Hour)},
			{ArtifactUUID: "art-b", OrganizationUUID: readOnlyTestOrgB, GatewayUUID: "gw-b", Status: model.DeploymentStatusDeploying, PerformedAt: time.Now().Add(-time.Hour)},
		}}
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name        string
		readOnly    *config.ReadOnly
		wantUpdated []string
	}{
		{"unwired: both resolved", nil, []string{readOnlyTestOrgA + "/art-a", readOnlyTestOrgB + "/art-b"}},
		{"disabled: both resolved", &config.ReadOnly{Enabled: false}, []string{readOnlyTestOrgA + "/art-a", readOnlyTestOrgB + "/art-b"}},
		{"enabled, B writable: only B resolved", &config.ReadOnly{Enabled: true, WritableOrganizations: []string{readOnlyTestOrgB}}, []string{readOnlyTestOrgB + "/art-b"}},
		{"enabled, empty list: nothing resolved", &config.ReadOnly{Enabled: true}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo()
			svc := NewDeploymentTimeoutService(repo, DeploymentTimeoutConfig{Enabled: true}, logger)
			if tc.readOnly != nil {
				svc.SetReadOnly(tc.readOnly)
			}
			svc.processStaleStatuses(time.Minute)
			if len(repo.updated) != len(tc.wantUpdated) {
				t.Fatalf("updated = %v, want %v", repo.updated, tc.wantUpdated)
			}
			for i := range tc.wantUpdated {
				if repo.updated[i] != tc.wantUpdated[i] {
					t.Errorf("updated[%d] = %q, want %q", i, repo.updated[i], tc.wantUpdated[i])
				}
			}
		})
	}
}
