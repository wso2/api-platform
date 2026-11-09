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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
)

func TestPlatformAPIDefinition(t *testing.T) {
	definition := PlatformAPI()
	require.Equal(t, "platform-api", definition.Name)
	require.True(t, definition.IsCompose())
	require.Equal(t, "platform-api", definition.Compose.PrimaryService)
	require.NotNil(t, definition.DB)
	require.NotEmpty(t, definition.Compose.GeneratedFiles)
	require.NotNil(t, definition.Health)
	_, ok := definition.Endpoint("https")
	require.True(t, ok)
	require.Equal(t, platformAPIBootAttempts, definition.Compose.BootAttempts,
		"platform-api self-heals from a transient boot crash, like the SQL Server engine")
}

func TestPlatformAPICoverageEnvironmentFollowsRunMode(t *testing.T) {
	t.Setenv(shared.EnvCoverageMode, "false")
	require.NotContains(t, PlatformAPI().Compose.Env, "GOCOVERDIR")

	t.Setenv(shared.EnvCoverageMode, "true")
	require.Equal(t, "/coverage", PlatformAPI().Compose.Env["GOCOVERDIR"])
}

func TestMintSecondOrganizationToken(t *testing.T) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM(shared.ControlPlaneCrypto()["keys/jwt_private.pem"])
	require.NoError(t, err)
	admin, err := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": "platform-api", "sub": "admin", "username": "admin", "organization": "default-org-uuid",
		"org_handle": "default", "org_name": "Default", "scope": "ap:rest_api:manage ap:gateway:manage",
		"roles": []string{"ap_admin"}, "exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString(key)
	require.NoError(t, err)

	now := time.Unix(1_800_000_000, 0)
	minted, err := mintSecondOrganizationToken(admin, now)
	require.NoError(t, err)

	public, err := jwt.ParseRSAPublicKeyFromPEM(shared.ControlPlaneCrypto()["keys/jwt_public.pem"])
	require.NoError(t, err)
	claims := jwt.MapClaims{}
	_, err = jwt.NewParser(jwt.WithValidMethods([]string{"RS256"}), jwt.WithTimeFunc(func() time.Time { return now })).
		ParseWithClaims(minted, claims, func(*jwt.Token) (any, error) { return public, nil })
	require.NoError(t, err, "the control plane verifies the token with its public key")

	require.Equal(t, secondOrganizationClaim, claims["organization"])
	require.Equal(t, SecondOrganizationHandle, claims["org_handle"])
	require.Equal(t, SecondOrganizationUser, claims["sub"])
	require.Equal(t, SecondOrganizationUser, claims["username"])
	require.Equal(t, "platform-api", claims["iss"], "the issuer the control plane requires is kept")
	require.Equal(t, "ap:rest_api:manage ap:gateway:manage", claims["scope"], "the administrator's scopes are kept")
	require.EqualValues(t, now.Add(time.Hour).Unix(), claims["exp"])

	_, err = mintSecondOrganizationToken("not-a-token", now)
	require.ErrorContains(t, err, "reading the administrator token")
}

func TestMintNoScopeToken(t *testing.T) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM(shared.ControlPlaneCrypto()["keys/jwt_private.pem"])
	require.NoError(t, err)
	admin, err := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": "platform-api", "sub": "admin", "username": "admin", "organization": "default-org-uuid",
		"org_handle": "default", "scope": "ap:rest_api:manage ap:gateway:manage",
		"roles": []string{"ap_admin"}, "exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString(key)
	require.NoError(t, err)

	now := time.Unix(1_800_000_000, 0)
	minted, err := mintNoScopeToken(admin, now)
	require.NoError(t, err)

	public, err := jwt.ParseRSAPublicKeyFromPEM(shared.ControlPlaneCrypto()["keys/jwt_public.pem"])
	require.NoError(t, err)
	claims := jwt.MapClaims{}
	_, err = jwt.NewParser(jwt.WithValidMethods([]string{"RS256"}), jwt.WithTimeFunc(func() time.Time { return now })).
		ParseWithClaims(minted, claims, func(*jwt.Token) (any, error) { return public, nil })
	require.NoError(t, err, "the control plane verifies the token with its public key")

	require.Equal(t, "", claims["scope"], "no scope is granted")
	require.Empty(t, claims["roles"], "no role is granted")
	require.Equal(t, "default-org-uuid", claims["organization"], "the caller stays in its organization")
	require.Equal(t, "admin", claims["sub"])
	require.Equal(t, "platform-api", claims["iss"])

	_, err = mintNoScopeToken("not-a-token", now)
	require.ErrorContains(t, err, "reading the administrator token")
}

func TestPlatformAPIMountsThePortalCertificateWithoutChangingTheSharedCryptoMaterial(t *testing.T) {
	definition := PlatformAPI()

	require.Equal(t, shared.APIPortalTLS().CertPEM, definition.Compose.GeneratedFiles["certs/api-portal-ca.pem"])
	require.Equal(t, shared.ControlPlaneCrypto()["certs/cert.pem"], definition.Compose.GeneratedFiles["certs/cert.pem"])
	require.NotContains(t, shared.ControlPlaneCrypto(), "certs/api-portal-ca.pem",
		"the control plane's shared material must not gain the portal certificate")
}

func TestIsLastProjectRetention(t *testing.T) {
	refusal := []byte(`{"status":"error","code":"VALIDATION_FAILED","message":"Organization must have at least one project"}`)
	require.True(t, IsLastProjectRetention(http.StatusBadRequest, refusal))
	require.False(t, IsLastProjectRetention(http.StatusConflict, refusal), "only the 400 refusal")
	require.False(t, IsLastProjectRetention(http.StatusBadRequest,
		[]byte(`{"code":"VALIDATION_FAILED","message":"Invalid project id"}`)), "another validation failure is a real error")
	require.False(t, IsLastProjectRetention(http.StatusBadRequest, []byte(`not json`)))
	require.False(t, IsLastProjectRetention(http.StatusBadRequest, nil))
}

func TestRegisterSecondOrganizationAcceptsAConflictOnlyForItsOwnOrganization(t *testing.T) {
	for name, tc := range map[string]struct {
		register, read int
		ok             bool
	}{
		"created":                 {register: http.StatusCreated, ok: true},
		"already registered":      {register: http.StatusConflict, read: http.StatusOK, ok: true},
		"handle taken by another": {register: http.StatusConflict, read: http.StatusNotFound},
		"refused":                 {register: http.StatusBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "Bearer token", r.Header.Get("Authorization"))
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/api/v0.9/organizations":
					w.WriteHeader(tc.register)
				case r.Method == http.MethodGet && r.URL.Path == "/api/v0.9/organizations/"+SecondOrganizationHandle:
					w.WriteHeader(tc.read)
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
			}))
			defer server.Close()
			err := registerSecondOrganization(context.Background(), server.Client(), server.URL, "token")
			if tc.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
