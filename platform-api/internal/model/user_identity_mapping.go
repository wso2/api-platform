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

package model

import "time"

// UserIdentityMapping maps our internal platform user UUID to the identity
// provider's resolved actor identifier (the OIDC "sub" claim, falling back to
// the configured claim / user_id when sub is absent — see
// middleware.GetActorIdentityFromRequest). Audit columns
// (created_by/updated_by/revoked_by/performed_by) store UserIdentityMapping.UUID;
// API responses and external/data-plane events resolve it back to IdpID at
// the boundary (see service.IdentityService).
type UserIdentityMapping struct {
	UUID      string    `json:"uuid" db:"uuid"`
	IdpID     string    `json:"idpId" db:"idp_id"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}
