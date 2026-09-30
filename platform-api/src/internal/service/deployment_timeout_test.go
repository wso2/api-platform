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
	"io"
	"log/slog"
	"testing"
	"time"

	"platform-api/src/config"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

// fakeTimeoutDeploymentRepo implements only the two DeploymentRepository methods the
// timeout job uses and records which entries were updated.
type fakeTimeoutDeploymentRepo struct {
	repository.DeploymentRepository // embed for unimplemented methods
	stale                           []repository.StaleDeploymentStatus
	updated                         []string // "orgUUID/artifactUUID"
}

func (f *fakeTimeoutDeploymentRepo) GetStaleTransitionalStatuses(time.Duration) ([]repository.StaleDeploymentStatus, error) {
	return f.stale, nil
}

func (f *fakeTimeoutDeploymentRepo) UpdateStatusWithPerformedAtGuard(artifactUUID, orgUUID, _ string, _ model.DeploymentStatus, _ string, _ time.Time, _ []model.DeploymentStatus) (int64, error) {
	f.updated = append(f.updated, orgUUID+"/"+artifactUUID)
	return 1, nil
}

func TestDeploymentTimeoutService_processStaleStatuses_SkipsReadOnlyOrgs(t *testing.T) {
	const (
		orgA = "11111111-1111-1111-1111-111111111111"
		orgB = "22222222-2222-2222-2222-222222222222"
	)
	stale := []repository.StaleDeploymentStatus{
		{ArtifactUUID: "art-a", OrganizationUUID: orgA, GatewayUUID: "gw-a", PerformedAt: time.Now().Add(-time.Hour)},
		{ArtifactUUID: "art-b", OrganizationUUID: orgB, GatewayUUID: "gw-b", PerformedAt: time.Now().Add(-time.Hour)},
	}

	tests := []struct {
		name        string
		readOnly    *config.ReadOnly
		wantUpdated []string
	}{
		{"list mode skips the read-only org", &config.ReadOnly{Organizations: []string{orgA}}, []string{orgB + "/art-b"}},
		{"all-organizations mode skips everything", &config.ReadOnly{AllOrganizations: true}, nil},
		{"nil read-only config processes everything", nil, []string{orgA + "/art-a", orgB + "/art-b"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeTimeoutDeploymentRepo{stale: stale}
			svc := &DeploymentTimeoutService{
				deploymentRepo: repo,
				config:         DeploymentTimeoutConfig{Enabled: true},
				readOnly:       tc.readOnly,
				slogger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			svc.processStaleStatuses(time.Minute)

			if len(repo.updated) != len(tc.wantUpdated) {
				t.Fatalf("updated entries = %v, want %v", repo.updated, tc.wantUpdated)
			}
			for i := range tc.wantUpdated {
				if repo.updated[i] != tc.wantUpdated[i] {
					t.Errorf("updated[%d] = %q, want %q", i, repo.updated[i], tc.wantUpdated[i])
				}
			}
		})
	}
}
