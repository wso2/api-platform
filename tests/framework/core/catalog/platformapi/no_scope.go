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

package platformapi

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/wso2/api-platform/tests/framework/core/actor"
)

// NoScopeLogin returns a bearer token for the default organization's administrator with every
// role and scope removed: a caller the control plane authenticates but authorizes for nothing.
// No file user has such a role, and every role in the shipped mapping grants REST API scopes.
func NoScopeLogin(ctx context.Context, base string) (string, error) {
	client, err := platformAPIClient()
	if err != nil {
		return "", err
	}
	admin, err := platformAPILogin(ctx, client, base, actor.Administrator())
	if err != nil {
		return "", err
	}
	return mintNoScopeToken(admin, time.Now())
}

// mintNoScopeToken re-signs the administrator's claims, with the scope and roles claims the
// login sets emptied, using the key the control plane verifies against.
func mintNoScopeToken(adminToken string, now time.Time) (string, error) {
	return reSignAdminToken(adminToken, now, jwt.MapClaims{"scope": "", "roles": []string{}})
}
