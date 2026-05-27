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

package rbac

import "context"

// PermissionResolver resolves whether a user holds a given permission.
// Implementations may query an identity service (Thunder) or derive
// permissions from claims already present in the request context.
type PermissionResolver interface {
	// HasPermission returns true when the user identified by userID holds perm
	// within the given org. token is the raw Bearer value from the request and
	// may be forwarded to an upstream identity service.
	HasPermission(ctx context.Context, userID, token, org string, perm Permission) (bool, error)
}

// ClaimsResolver resolves permissions from the platform roles that were
// already extracted from the JWT by the auth middleware (IDP mode).
// It performs no network calls — it purely maps role names to permissions
// using the predefined role table.
type ClaimsResolver struct{}

func NewClaimsResolver() *ClaimsResolver { return &ClaimsResolver{} }

func (r *ClaimsResolver) HasPermission(_ context.Context, _, _, _ string, perm Permission) (bool, error) {
	// Roles are not passed here directly; callers must use HasPermissionForRoles.
	// This method exists to satisfy the interface when called from middleware that
	// already attached the resolved permission set to the request context.
	return false, nil
}

// HasPermissionForRoles is the claims-mode helper: it checks whether any of
// the provided role names grant perm.
func HasPermissionForRoles(roles []string, perm Permission) bool {
	perms := PermissionsForRoles(roles)
	_, ok := perms[perm]
	return ok
}
