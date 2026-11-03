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
// when the mode is removed — see README.md "Read-only (maintenance) mode — temporary"
// for the removal checklist.

package service

import (
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// readOnlyMode is the one thing the timeout job needs from config.ReadOnly. It
// is declared here, not in deployment_timeout.go, so that file needs no new
// import for its single marked field; config.ReadOnly satisfies it with a
// nil-safe method, so a never-wired (nil) field means "mode off".
type readOnlyMode interface {
	IsReadOnlyOrg(orgUUID string) bool
}

// SetReadOnly wires the read-only (maintenance) mode configuration into the
// timeout job: stale entries that belong to a read-only organization are left
// untouched (their deployment rows must not change while it is frozen).
func (s *DeploymentTimeoutService) SetReadOnly(ro *config.ReadOnly) {
	s.readOnly = ro
}

// partitionReadOnly splits the stale entries into the ones the job may act on
// and the count it skipped because their organization is read-only. It runs
// before the job's Info log so that log does not fire on every tick for
// permanently frozen rows; each skipped entry is logged at Debug instead.
func (s *DeploymentTimeoutService) partitionReadOnly(stale []repository.StaleDeploymentStatus) (actionable []repository.StaleDeploymentStatus, skipped int) {
	if s.readOnly == nil {
		return stale, 0
	}
	actionable = make([]repository.StaleDeploymentStatus, 0, len(stale))
	for _, entry := range stale {
		if s.readOnly.IsReadOnlyOrg(entry.OrganizationUUID) {
			skipped++
			s.slogger.Debug("Read-only mode: skipping stale deployment status",
				"orgUUID", entry.OrganizationUUID,
				"artifactUUID", entry.ArtifactUUID,
				"gatewayUUID", entry.GatewayUUID)
			continue
		}
		actionable = append(actionable, entry)
	}
	return actionable, skipped
}
