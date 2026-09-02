/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package authenticators

import "sort"

// This file holds the OUTBOUND half of the auth.idp.role_mapping translation.
// The inbound half - resolving the claim values on a presented token into the
// local roles the route role map is written in - lives in JWTAuthenticator.resolvePermissions (jwt_authenticator.go)

// MapRolesToScopes is the outbound direction: the IdP claim values a caller
// must hold to be granted any of roles. The result is deduplicated and sorted,
// so an advertised scope set is byte-stable across restarts despite Go's
// randomized map iteration.
func MapRolesToScopes(mapping map[string][]string, roles []string) []string {
	seen := make(map[string]struct{}, len(roles))
	scopes := make([]string, 0, len(roles))
	add := func(s string) {
		if _, dup := seen[s]; dup {
			return
		}
		seen[s] = struct{}{}
		scopes = append(scopes, s)
	}

	for _, role := range roles {
		requestable := false
		for _, claimValue := range mapping[role] {
			if claimValue == "*" {
				continue // not requestable — a client cannot ask an IdP for "*"
			}
			add(claimValue)
			requestable = true
		}
		if !requestable {
			// No entry, an empty entry, or wildcard-only. resolvePermissions
			// resolves the bare role name back to this role in all three.
			add(role)
		}
	}

	sort.Strings(scopes)
	return scopes
}
