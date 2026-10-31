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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/wso2/api-platform/tests/framework/core/actor"
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
)

// File-based auth seeds a single organization. The second one is registered through the
// organizations API by a token whose organization claim the control plane has never seen;
// from then on, tokens carrying that claim resolve to the second organization. Every block
// starts with an empty database, so a fixed claim is always unseen until registration.
const (
	// SecondOrganizationHandle is the handle of the control plane's second organization.
	SecondOrganizationHandle = "it-second-org"
	// SecondOrganizationUser is the subject of the second organization's administrator.
	SecondOrganizationUser = "it-second-org-admin"

	secondOrganizationClaim = "5e0c4b7a-2d1f-4a8e-9c3b-7f6a1d2e8b40"
	secondOrganizationName  = "IT Second Organization"
)

// SecondOrganizationLogin returns a bearer token for the administrator of the control
// plane's second organization, registering the organization first when it is missing.
func SecondOrganizationLogin(ctx context.Context, base string) (string, error) {
	client, err := platformAPIClient()
	if err != nil {
		return "", err
	}
	return secondOrganizationBearer(ctx, client, base)
}

// secondOrganizationBearer mints the second organization's administrator token and makes
// sure the organization exists.
func secondOrganizationBearer(ctx context.Context, client *http.Client, base string) (string, error) {
	admin, err := platformAPILogin(ctx, client, base, actor.Administrator())
	if err != nil {
		return "", err
	}
	token, err := mintSecondOrganizationToken(admin, time.Now())
	if err != nil {
		return "", err
	}
	if err := registerSecondOrganization(ctx, client, base, token); err != nil {
		return "", err
	}
	return token, nil
}

// mintSecondOrganizationToken re-signs the administrator's claims, scopes and roles
// included, for the second organization, with the key the control plane verifies against.
func mintSecondOrganizationToken(adminToken string, now time.Time) (string, error) {
	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(adminToken, claims); err != nil {
		return "", fmt.Errorf("reading the administrator token: %w", err)
	}
	claims["organization"] = secondOrganizationClaim
	claims["org_handle"] = SecondOrganizationHandle
	claims["org_name"] = secondOrganizationName
	claims["sub"] = SecondOrganizationUser
	claims["username"] = SecondOrganizationUser
	claims["iat"] = now.Unix()
	claims["exp"] = now.Add(time.Hour).Unix()

	key, err := jwt.ParseRSAPrivateKeyFromPEM(shared.ControlPlaneCrypto()["keys/jwt_private.pem"])
	if err != nil {
		return "", fmt.Errorf("loading the control-plane signing key: %w", err)
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	if err != nil {
		return "", fmt.Errorf("signing the second organization token: %w", err)
	}
	return signed, nil
}

// registerSecondOrganization registers the second organization; a conflict means an earlier
// call already did.
func registerSecondOrganization(ctx context.Context, client *http.Client, base, token string) error {
	payload, err := json.Marshal(map[string]string{
		"id": SecondOrganizationHandle, "displayName": secondOrganizationName, "region": "us",
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		base+"/api/v0.9/organizations", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("registering the second organization: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("registering the second organization -> %d: %s", resp.StatusCode, bytes.TrimSpace(raw))
	}
	return nil
}
