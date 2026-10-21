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

package jwks

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	"github.com/wso2/api-platform/sdk/core/signing"
)

func request(method string, headers map[string][]string) *policy.RequestHeaderContext {
	return &policy.RequestHeaderContext{
		SharedContext: &policy.SharedContext{},
		Headers:       policy.NewHeaders(headers),
		Path:          "/.well-known/jwks.json",
		Method:        method,
	}
}

func respond(t *testing.T, method string, headers map[string][]string) policy.ImmediateResponse {
	t.Helper()
	p, err := GetPolicy(policy.PolicyMetadata{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	action := p.(*JWKSSystemPolicy).OnRequestHeaders(context.Background(), request(method, headers), nil)
	resp, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("action = %T, want ImmediateResponse: the JWKS route must never forward", action)
	}
	return resp
}

// stubService stands in for the policy engine's keyring: this policy only
// relays what the installed signing service publishes, and the JWKS document
// itself is generated (and tested) in the engine.
type stubService struct{ kid string }

func (s stubService) Sign([]byte) ([]byte, signing.KeyInfo, error) {
	return nil, signing.KeyInfo{}, signing.ErrNotConfigured
}

func (s stubService) SignJWS([]byte, signing.Options) (string, signing.KeyInfo, error) {
	return "", signing.KeyInfo{}, signing.ErrNotConfigured
}

func (s stubService) SignDetachedJWS([]byte, signing.Options) (signing.DetachedJWS, signing.KeyInfo, error) {
	return signing.DetachedJWS{}, signing.KeyInfo{}, signing.ErrNotConfigured
}

func (s stubService) ActiveKey() (signing.KeyInfo, bool) {
	return signing.KeyInfo{Kid: s.kid, Algorithm: signing.ES256}, true
}

func (s stubService) JWKS() ([]byte, string) {
	return []byte(`{"keys":[{"kty":"EC","kid":"` + s.kid + `","use":"sig","alg":"ES256","crv":"P-256","x":"x","y":"y"}]}`), `"0123456789abcdef"`
}

func (s stubService) JWKSCacheMaxAge() time.Duration { return 10 * time.Minute }

func installKey(t *testing.T) string {
	t.Helper()
	const kid = "gw-test"
	signing.Install(stubService{kid: kid})
	t.Cleanup(func() { signing.Install(nil) })
	return kid
}

func TestServesTheKeySet(t *testing.T) {
	kid := installKey(t)

	resp := respond(t, "GET", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := resp.Headers["content-type"]; got != "application/jwk-set+json" {
		t.Fatalf("content-type = %q", got)
	}
	if got := resp.Headers["cache-control"]; got != "public, max-age=600" {
		t.Fatalf("cache-control = %q", got)
	}
	if resp.Headers["etag"] == "" || resp.Headers["access-control-allow-origin"] != "*" {
		t.Fatalf("headers = %v", resp.Headers)
	}

	var set struct{ Keys []map[string]any }
	if err := json.Unmarshal(resp.Body, &set); err != nil {
		t.Fatal(err)
	}
	if len(set.Keys) != 1 || set.Keys[0]["kid"] != kid {
		t.Fatalf("body = %s", resp.Body)
	}
	if _, private := set.Keys[0]["d"]; private {
		t.Fatal("private key member published")
	}
}

func TestHeadMatchesGet(t *testing.T) {
	installKey(t)
	get, head := respond(t, "GET", nil), respond(t, "head", nil)
	if head.StatusCode != 200 || head.Headers["etag"] != get.Headers["etag"] {
		t.Fatalf("HEAD = %d %v", head.StatusCode, head.Headers)
	}
}

func TestConditionalGet(t *testing.T) {
	installKey(t)
	etag := respond(t, "GET", nil).Headers["etag"]

	for _, inm := range []string{etag, "W/" + etag, `"other", ` + etag, "*"} {
		resp := respond(t, "GET", map[string][]string{"if-none-match": {inm}})
		if resp.StatusCode != 304 || len(resp.Body) != 0 || resp.Headers["etag"] != etag {
			t.Fatalf("If-None-Match %s: status %d, body %d bytes", inm, resp.StatusCode, len(resp.Body))
		}
	}
	if resp := respond(t, "GET", map[string][]string{"if-none-match": {`"stale"`}}); resp.StatusCode != 200 {
		t.Fatalf("stale validator answered %d", resp.StatusCode)
	}
}

func TestOtherMethodsAreRejected(t *testing.T) {
	installKey(t)
	for _, m := range []string{"POST", "PUT", "DELETE", "PATCH", "OPTIONS"} {
		resp := respond(t, m, nil)
		if resp.StatusCode != 405 || resp.Headers["allow"] != "GET, HEAD" || len(resp.Body) != 0 {
			t.Fatalf("%s: %d %v", m, resp.StatusCode, resp.Headers)
		}
	}
}

func TestNoKeysServesAnEmptySet(t *testing.T) {
	signing.Install(nil)
	resp := respond(t, "GET", nil)
	if resp.StatusCode != 200 || string(resp.Body) != `{"keys":[]}` {
		t.Fatalf("status %d body %s", resp.StatusCode, resp.Body)
	}
	if resp.Headers["cache-control"] != "public, max-age=300" {
		t.Fatalf("cache-control = %q", resp.Headers["cache-control"])
	}
}

func TestModeIsRequestHeadersOnly(t *testing.T) {
	m := ins.Mode()
	if m.RequestHeaderMode != policy.HeaderModeProcess || m.RequestBodyMode != policy.BodyModeSkip ||
		m.ResponseHeaderMode != policy.HeaderModeSkip || m.ResponseBodyMode != policy.BodyModeSkip {
		t.Fatalf("mode = %+v", m)
	}
}

// The controller spells the policy name in its own module; this pins the
// literal on this side (see the controller's constants.JWKS_SYSTEM_POLICY_NAME).
func TestPolicyDefinitionName(t *testing.T) {
	def, err := os.ReadFile("policy-definition.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(def), "name: wso2_apip_sys_jwks\n") {
		t.Fatal("policy-definition.yaml must declare name wso2_apip_sys_jwks")
	}
}
