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
	"github.com/wso2/api-platform/platform-api/internal/apperror"
)

// SetReadOnly wires the read-only (maintenance) mode configuration into the webhook secret
// service. The REST routes are covered by the read-only middleware; the in-process pdk
// capability (IssuePortalSecret, DeletePortalSecret) does not pass through it, so the check is
// repeated here, where every entry point funnels through.
func (s *WebhookSecretService) SetReadOnly(ro *config.ReadOnly) {
	s.readOnly = ro
}

// rejectIfReadOnly refuses a change to a read-only organization's secrets before it is made.
func (s *WebhookSecretService) rejectIfReadOnly(orgID string) error {
	if s.readOnly != nil && s.readOnly.IsReadOnlyOrg(orgID) {
		return apperror.OrganizationReadOnly.New()
	}
	return nil
}
