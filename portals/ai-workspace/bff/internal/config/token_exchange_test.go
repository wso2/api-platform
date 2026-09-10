/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeTokenExchangeConfig writes a minimal valid config.toml with the given
// [ai_workspace.auth] body appended, and loads it.
func loadWithAuth(t *testing.T, authBody string) (*Config, error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[ai_workspace]
domain = "localhost:9643"

[ai_workspace.control_plane]
url = "https://platform-api:9243"

[ai_workspace.server.https]
enabled = true
port = 9643
cert_file = "/tmp/cert.pem"
key_file = "/tmp/key.pem"
` + authBody
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return Load(path)
}

// TestTokenExchangeDefaultsOff is the compatibility guarantee: a config that says
// nothing about token exchange must behave exactly as it did before the feature
// existed.
func TestTokenExchangeDefaultsOff(t *testing.T) {
	cfg, err := loadWithAuth(t, `
[ai_workspace.auth]
mode = "basic"
`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Auth.OIDC.TokenExchange.Enabled {
		t.Error("token exchange must default to disabled")
	}
	if cfg.Auth.TokenExchangeEnabled() {
		t.Error("TokenExchangeEnabled must be false by default")
	}
	if got := cfg.Auth.OIDC.TokenExchange.GrantType; got != GrantTokenExchange {
		t.Errorf("default grant_type = %q, want %q", got, GrantTokenExchange)
	}
	if got := cfg.Auth.OIDC.TokenExchange.MinValidity; got != 60*time.Second {
		t.Errorf("default min_validity = %s, want 60s", got)
	}
	if !cfg.Auth.OIDC.TokenExchange.CacheEnabled {
		t.Error("caching must default to on")
	}
}

// TestTokenExchangeInheritsOIDCCredentials keeps the common single-application
// deployment down to two keys (enabled + audience).
func TestTokenExchangeInheritsOIDCCredentials(t *testing.T) {
	cfg, err := loadWithAuth(t, `
[ai_workspace.auth]
mode = "oidc"

[ai_workspace.auth.oidc]
authority = "https://idp.example.com"
client_id = "login-client"
client_secret = "login-secret"
redirect_url = "https://localhost:9643/ai-workspace/api/auth/callback"
scope = "openid ap:project:read"

[ai_workspace.auth.oidc.token_exchange]
enabled = true
audience = "platform-api"
`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	te := cfg.Auth.OIDC.TokenExchange
	if te.ClientID != "login-client" || te.ClientSecret != "login-secret" {
		t.Errorf("credentials should inherit from [auth.oidc], got %q/%q", te.ClientID, te.ClientSecret)
	}
	if te.Scopes != "openid ap:project:read" {
		t.Errorf("scope should inherit from [auth.oidc], got %q", te.Scopes)
	}
	if !cfg.Auth.TokenExchangeEnabled() {
		t.Error("TokenExchangeEnabled must be true")
	}
}

// TestTokenExchangeSeparateCredentialsWin covers binding exchange rights to a
// narrower, exchange-only application than the login client.
func TestTokenExchangeSeparateCredentialsWin(t *testing.T) {
	cfg, err := loadWithAuth(t, `
[ai_workspace.auth]
mode = "oidc"

[ai_workspace.auth.oidc]
authority = "https://idp.example.com"
client_id = "login-client"
client_secret = "login-secret"
redirect_url = "https://localhost:9643/ai-workspace/api/auth/callback"

[ai_workspace.auth.oidc.token_exchange]
enabled = true
audience = "platform-api"
client_id = "exchange-client"
client_secret = "exchange-secret"
scope = "ap:project:read"
`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	te := cfg.Auth.OIDC.TokenExchange
	if te.ClientID != "exchange-client" || te.ClientSecret != "exchange-secret" {
		t.Errorf("explicit credentials must win, got %q/%q", te.ClientID, te.ClientSecret)
	}
	if te.Scopes != "ap:project:read" {
		t.Errorf("explicit scope must win, got %q", te.Scopes)
	}
}

func TestTokenExchangeValidationErrors(t *testing.T) {
	const oidcBase = `
[ai_workspace.auth]
mode = "oidc"

[ai_workspace.auth.oidc]
authority = "https://idp.example.com"
client_id = "login-client"
client_secret = "login-secret"
redirect_url = "https://localhost:9643/ai-workspace/api/auth/callback"
`
	for _, tc := range []struct {
		name    string
		body    string
		wantSub string
	}{
		{
			// A typo must not silently leave the feature off or pick a grant at random,
			// and the error must name the supported set.
			name: "unknown grant type",
			body: `
[ai_workspace.auth]
mode = "basic"

[ai_workspace.auth.oidc]
[ai_workspace.auth.oidc.token_exchange]
grant_type = "saml-swap"
`,
			wantSub: "supported values are token_exchange, urn:ietf:params:oauth:grant-type:token-exchange",
		},
		{
			name: "empty grant type",
			body: `
[ai_workspace.auth]
mode = "basic"

[ai_workspace.auth.oidc]
[ai_workspace.auth.oidc.token_exchange]
grant_type = ""
`,
			wantSub: "supported values are token_exchange, urn:ietf:params:oauth:grant-type:token-exchange",
		},
		{
			// Lowercasing normalizes case, not separators.
			name: "hyphenated grant type",
			body: `
[ai_workspace.auth]
mode = "basic"

[ai_workspace.auth.oidc]
[ai_workspace.auth.oidc.token_exchange]
grant_type = "token-exchange"
`,
			wantSub: "supported values are",
		},
		{
			// Basic mode has no subject token: the JWT the BFF holds is one the
			// Platform API signed for itself.
			name: "enabled in basic mode",
			body: `
[ai_workspace.auth]
mode = "basic"

[ai_workspace.auth.oidc]
[ai_workspace.auth.oidc.token_exchange]
enabled = true
audience = "platform-api"
`,
			wantSub: `requires [auth] mode = "oidc"`,
		},
		{
			name: "audience and resource together",
			body: oidcBase + `
[ai_workspace.auth.oidc.token_exchange]
enabled = true
audience = "platform-api"
resource = "https://platform-api.example.com"
`,
			wantSub: "at most one of audience / resource",
		},
		{
			name: "blank subject_token_type",
			body: oidcBase + `
[ai_workspace.auth.oidc.token_exchange]
enabled = true
audience = "platform-api"
subject_token_type = ""
`,
			wantSub: "subject_token_type is required",
		},
		{
			name: "blank requested_token_type",
			body: oidcBase + `
[ai_workspace.auth.oidc.token_exchange]
enabled = true
audience = "platform-api"
requested_token_type = ""
`,
			wantSub: "requested_token_type is required",
		},
		{
			// Entra names the target through scope; audience would be silently dropped.
			name: "jwt_bearer with audience",
			body: oidcBase + `
[ai_workspace.auth.oidc.token_exchange]
enabled = true
grant_type = "jwt_bearer"
audience = "platform-api"
scope = "api://platform/.default"
`,
			wantSub: "are not used by grant_type",
		},
		{
			name: "jwt_bearer without scope",
			body: oidcBase + `
[ai_workspace.auth.oidc.token_exchange]
enabled = true
grant_type = "jwt_bearer"
scope = ""
`,
			wantSub: "scope is required",
		},
		{
			name: "relative token_endpoint",
			body: oidcBase + `
[ai_workspace.auth.oidc.token_exchange]
enabled = true
audience = "platform-api"
token_endpoint = "/oauth2/token"
`,
			wantSub: "must be an absolute",
		},
		{
			// A zero window would re-exchange only after expiry, guaranteeing an
			// in-flight expiry on every renewal.
			name: "non-positive min_validity with cache on",
			body: oidcBase + `
[ai_workspace.auth.oidc.token_exchange]
enabled = true
audience = "platform-api"
cache_enabled = true
min_validity = "0s"
`,
			wantSub: "min_validity must be positive",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadWithAuth(t, tc.body)
			if err == nil {
				t.Fatalf("expected a validation error mentioning %q", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error = %q, want it to mention %q", err.Error(), tc.wantSub)
			}
		})
	}
}

// TestTokenExchangeValidConfigs are the two shapes the docs tell operators to use;
// they must load without error.
func TestTokenExchangeValidConfigs(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{
			name: "wso2 rfc8693",
			body: `
[ai_workspace.auth]
mode = "oidc"

[ai_workspace.auth.oidc]
authority = "https://iam.example.com/oauth2/token"
client_id = "login-client"
client_secret = "login-secret"
redirect_url = "https://localhost:9643/ai-workspace/api/auth/callback"

[ai_workspace.auth.oidc.token_exchange]
enabled = true
audience = "platform-api"
`,
		},
		{
			name: "entra jwt bearer",
			body: `
[ai_workspace.auth]
mode = "oidc"

[ai_workspace.auth.oidc]
authority = "https://login.microsoftonline.com/tenant/v2.0"
client_id = "login-client"
client_secret = "login-secret"
redirect_url = "https://localhost:9643/ai-workspace/api/auth/callback"

[ai_workspace.auth.oidc.token_exchange]
enabled = true
grant_type = "jwt_bearer"
scope = "api://platform-api/.default"
token_endpoint = "https://login.microsoftonline.com/tenant/oauth2/v2.0/token"
`,
		},
		{
			name: "grant type is case insensitive",
			body: `
[ai_workspace.auth]
mode = "oidc"

[ai_workspace.auth.oidc]
authority = "https://idp.example.com"
client_id = "c"
client_secret = "s"
redirect_url = "https://localhost:9643/ai-workspace/api/auth/callback"

[ai_workspace.auth.oidc.token_exchange]
enabled = true
grant_type = "Token_Exchange"
audience = "platform-api"
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := loadWithAuth(t, tc.body); err != nil {
				t.Errorf("Load: %v", err)
			}
		})
	}
}

// TestExchangeTokenEndpointOverride: an empty override tells the server to fall back
// to OIDC discovery rather than being an error.
func TestExchangeTokenEndpointOverride(t *testing.T) {
	cfg, err := loadWithAuth(t, `
[ai_workspace.auth]
mode = "oidc"

[ai_workspace.auth.oidc]
authority = "https://idp.example.com"
client_id = "c"
client_secret = "s"
redirect_url = "https://localhost:9643/ai-workspace/api/auth/callback"

[ai_workspace.auth.oidc.token_exchange]
enabled = true
audience = "platform-api"
`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Auth.ExchangeTokenEndpoint(); got != "" {
		t.Errorf("ExchangeTokenEndpoint = %q, want empty so the server uses discovery", got)
	}
}

// TestTokenExchangeKeysLiveInOIDCSubTable pins the keys to
// [ai_workspace.auth.oidc.token_exchange]. A key in the wrong table is silently
// ignored by koanf rather than rejected, so a misplaced key would leave the feature
// quietly off — hence a test, not a review note.
func TestTokenExchangeKeysLiveInOIDCSubTable(t *testing.T) {
	cfg, err := loadWithAuth(t, `
[ai_workspace.auth]
mode = "oidc"

[ai_workspace.auth.oidc]
authority = "https://idp.example.com"
client_id = "c"
client_secret = "s"
redirect_url = "https://localhost:9643/ai-workspace/api/auth/callback"
[ai_workspace.auth.oidc.token_exchange]
enabled = true
grant_type = "token_exchange"
token_endpoint = "https://sts.example.com/oauth2/token"
client_id = "exchange-client"
client_secret = "exchange-secret"
audience = "platform-api"
scope = "ap:project:read"
subject_token_type = "urn:ietf:params:oauth:token-type:access_token"
requested_token_type = "urn:ietf:params:oauth:token-type:jwt"
cache_enabled = false
min_validity = "90s"
`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	te := cfg.Auth.OIDC.TokenExchange
	for _, tc := range []struct {
		key       string
		got, want any
	}{
		{"enabled", te.Enabled, true},
		{"grant_type", te.GrantType, GrantTokenExchange},
		{"token_endpoint", te.TokenEndpoint, "https://sts.example.com/oauth2/token"},
		{"client_id", te.ClientID, "exchange-client"},
		{"client_secret", te.ClientSecret, "exchange-secret"},
		{"audience", te.Audience, "platform-api"},
		{"scope", te.Scopes, "ap:project:read"},
		{"subject_token_type", te.SubjectTokenType, "urn:ietf:params:oauth:token-type:access_token"},
		{"requested_token_type", te.RequestedTokenType, "urn:ietf:params:oauth:token-type:jwt"},
		{"cache_enabled", te.CacheEnabled, false},
		{"min_validity", te.MinValidity, 90 * time.Second},
	} {
		if tc.got != tc.want {
			t.Errorf("%s did not reach the config: got %v, want %v", tc.key, tc.got, tc.want)
		}
	}
}

// TestTokenExchangeKeysIgnoredInOldTable documents that the sibling table the keys
// never lived in is inert: keys placed there do NOT silently enable the feature.
func TestTokenExchangeKeysIgnoredInOldTable(t *testing.T) {
	cfg, err := loadWithAuth(t, `
[ai_workspace.auth]
mode = "oidc"

[ai_workspace.auth.oidc]
authority = "https://idp.example.com"
client_id = "c"
client_secret = "s"
redirect_url = "https://localhost:9643/ai-workspace/api/auth/callback"

[ai_workspace.auth.token_exchange]
enabled = true
audience = "platform-api"
`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Auth.TokenExchangeEnabled() {
		t.Error("the sibling [auth.token_exchange] table must not enable the feature")
	}
}

// TestTokenExchangeGrantTypeAliases accepts the IANA-registered grant-type URIs as
// spellings of the two short names. The URI is what an IDP's own documentation shows
// and what the request itself carries, so it is the more likely thing for an operator
// to write — rejecting it produced a startup failure that read as "this IDP is not
// supported" when the protocol was in fact the configured one.
//
// The set stays closed either way: grant_type picks which protocol Exchanger.buildForm
// speaks, so a value with no branch behind it has no implementation.
func TestTokenExchangeGrantTypeAliases(t *testing.T) {
	for _, tc := range []struct {
		spelling string
		want     string
	}{
		{"token_exchange", GrantTokenExchange},
		{"urn:ietf:params:oauth:grant-type:token-exchange", GrantTokenExchange},
		{"URN:IETF:params:oauth:grant-type:Token-Exchange", GrantTokenExchange},
		{"jwt_bearer", GrantJWTBearer},
		{"urn:ietf:params:oauth:grant-type:jwt-bearer", GrantJWTBearer},
	} {
		t.Run(tc.spelling, func(t *testing.T) {
			// scope is set unconditionally: jwt_bearer requires it, and for
			// token_exchange it is simply a narrower request than the inherited set.
			cfg, err := loadWithAuth(t, `
[ai_workspace.auth]
mode = "oidc"

[ai_workspace.auth.oidc]
authority = "https://idp.example.com"
client_id = "c"
client_secret = "s"
redirect_url = "https://localhost:9643/ai-workspace/api/auth/callback"

[ai_workspace.auth.oidc.token_exchange]
enabled = true
grant_type = "`+tc.spelling+`"
scope = "api://platform-api/.default"
`)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := cfg.Auth.OIDC.TokenExchange.GrantType; got != tc.want {
				t.Errorf("grant_type = %q, want it normalized to %q", got, tc.want)
			}
		})
	}
}

// TestTokenExchangeUnknownGrantErrorListsEverySpelling keeps the startup error
// actionable: an operator who wrote a near-miss must be able to see the registered
// URI in the list, not just the short names.
func TestTokenExchangeUnknownGrantErrorListsEverySpelling(t *testing.T) {
	_, err := loadWithAuth(t, `
[ai_workspace.auth]
mode = "basic"

[ai_workspace.auth.oidc.token_exchange]
grant_type = "urn:ietf:params:oauth:grant-type:saml2-bearer"
`)
	if err == nil {
		t.Fatal("expected a validation error for an unimplemented grant")
	}
	for _, want := range []string{
		GrantTokenExchange, GrantURITokenExchange,
		GrantJWTBearer, GrantURIJWTBearer,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error must list the %q spelling, got: %v", want, err)
		}
	}
}
