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

import (
	"context"
	"errors"
)

type contextKey int

const rolesKey contextKey = iota

// WithRoles returns a copy of ctx with roles stored under the rbac roles key.
// The auth middleware should call this when attaching resolved platform roles
// to the request context so that ClaimsResolver.HasPermission can read them.
func WithRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, rolesKey, roles)
}

// RolesFromContext returns the platform roles stored by WithRoles, and whether
// the key was present.
func RolesFromContext(ctx context.Context) ([]string, bool) {
	roles, ok := ctx.Value(rolesKey).([]string)
	return roles, ok
}

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

// HasPermission extracts the resolved platform roles from ctx (stored by
// WithRoles) and delegates to HasPermissionForRoles. It returns an error when
// no roles are found in the context, which indicates the caller omitted the
// WithRoles step.
func (r *ClaimsResolver) HasPermission(ctx context.Context, _, _, _ string, perm Permission) (bool, error) {
	roles, ok := RolesFromContext(ctx)
	if !ok {
		return false, errors.New("rbac: HasPermission called without resolved roles in context; ensure WithRoles is called before invoking ClaimsResolver")
	}
	return HasPermissionForRoles(roles, perm), nil
}

// HasPermissionForRoles is the claims-mode helper: it checks whether any of
// the provided role names grant perm.
func HasPermissionForRoles(roles []string, perm Permission) bool {
	perms := PermissionsForRoles(roles)
	_, ok := perms[perm]
	return ok
}
