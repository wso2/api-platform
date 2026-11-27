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

package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// stubHmacSecretService records what the handler passed it and replays a
// canned answer. The real implementation lives in the event-gateway plugin,
// which is behind a build tag, so the handler contract is pinned here.
type stubHmacSecretService struct {
	calls   int
	gotAPI  string
	gotOrg  string
	secrets []*model.WebSubAPIHmacSecret
	err     error
}

func (s *stubHmacSecretService) ListByArtifactUUID(artifactUUID, orgUUID string) ([]*model.WebSubAPIHmacSecret, error) {
	s.calls++
	s.gotAPI = artifactUUID
	s.gotOrg = orgUUID
	return s.secrets, s.err
}

func (s *stubHmacSecretService) DecryptSecret(secret *model.WebSubAPIHmacSecret) (string, error) {
	return "plaintext-for-" + secret.Handle, nil
}

// The organization the lookup is scoped by must be the one the api-key
// authenticated to, not anything the caller supplied. The apiId path value is
// attacker-controlled; the organization is not.
func TestGetWebSubAPIHmacSecrets_ScopesLookupToAuthenticatedOrg(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()

	stub := &stubHmacSecretService{
		secrets: []*model.WebSubAPIHmacSecret{{Handle: "github-hook"}},
	}
	env.handler.SetHmacSecretService(stub)

	w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/websub-apis/art-websub-001/secrets", env.plainToken)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body.String())
	}
	if stub.calls != 1 {
		t.Fatalf("service called %d times, want 1", stub.calls)
	}
	if stub.gotOrg != env.orgID {
		t.Errorf("lookup org = %q, want the authenticated gateway's org %q", stub.gotOrg, env.orgID)
	}
	if stub.gotAPI != "art-websub-001" {
		t.Errorf("lookup apiID = %q, want %q", stub.gotAPI, "art-websub-001")
	}
	if !strings.Contains(w.Body.String(), "plaintext-for-github-hook") {
		t.Errorf("body should carry the decrypted secret, got %s", w.Body.String())
	}
}

// An apiId that does not resolve under the caller's organization answers 404 —
// the same answer an apiId that exists nowhere gets, so the response does not
// tell the caller which of the two it hit.
func TestGetWebSubAPIHmacSecrets_UnresolvableAPIIs404(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()

	stub := &stubHmacSecretService{err: apperror.WebSubAPINotFound.New()}
	env.handler.SetHmacSecretService(stub)

	w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/websub-apis/art-elsewhere/secrets", env.plainToken)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body %s", w.Code, w.Body.String())
	}
	if desc, _ := parseGWBody(w)["description"].(string); desc != "WebSub API not found" {
		t.Errorf("description = %q, want %q", desc, "WebSub API not found")
	}
	if strings.Contains(w.Body.String(), "secrets") {
		t.Errorf("404 body should carry no secret material, got %s", w.Body.String())
	}
}

// Without a valid api-key there is no organization to scope by, so the lookup
// must not run at all.
func TestGetWebSubAPIHmacSecrets_UnauthenticatedNeverReachesService(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()

	stub := &stubHmacSecretService{}
	env.handler.SetHmacSecretService(stub)

	w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/websub-apis/art-websub-001/secrets", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body %s", w.Code, w.Body.String())
	}
	if stub.calls != 0 {
		t.Errorf("service called %d times, want 0", stub.calls)
	}
}
