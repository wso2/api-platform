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

package session

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

// makeJWT builds an unsigned JWT (header.payload.signature) for decode tests.
// The BFF never verifies signatures, so a fake signature is fine.
func makeJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	pb, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(pb)
	return header + "." + payload + ".fakesignature"
}

func TestDecodeJWTClaims(t *testing.T) {
	tok := makeJWT(t, map[string]any{"sub": "alice", "scope": "ap:project:read ap:gateway:manage"})
	claims := DecodeJWTClaims(tok)
	if claims == nil {
		t.Fatal("expected claims, got nil")
	}
	if claims["sub"] != "alice" {
		t.Errorf("sub = %v, want alice", claims["sub"])
	}
}

func TestDecodeJWTClaims_Malformed(t *testing.T) {
	for _, bad := range []string{"", "notajwt", "only.two"} {
		if got := DecodeJWTClaims(bad); got != nil {
			t.Errorf("DecodeJWTClaims(%q) = %v, want nil", bad, got)
		}
	}
}

func TestUserFromClaims_FileBased(t *testing.T) {
	claims := map[string]any{
		"username":     "admin",
		"scope":        "ap:project:read ap:project:manage",
		"organization": "org-123",
		"org_name":     "Acme",
		"org_handle":   "acme",
	}
	u := UserFromClaims(claims, nil, DefaultClaimMapping())

	if u.Name != "admin" {
		t.Errorf("Name = %q, want admin", u.Name)
	}
	if len(u.Scopes) != 2 || u.Scopes[0] != "ap:project:read" {
		t.Errorf("Scopes = %v, want 2 scopes", u.Scopes)
	}
	if u.Org == nil || u.Org.ID != "org-123" || u.Org.Handle != "acme" {
		t.Errorf("Org = %+v, want org-123/acme", u.Org)
	}
}

func TestUserFromClaims_ScopesArray(t *testing.T) {
	// IDPs like Asgardeo may carry scopes as an array under "scp".
	claims := map[string]any{
		"sub": "u1",
		"scp": []any{"ap:rest_api:read", "ap:rest_api:create"},
	}
	u := UserFromClaims(claims, nil, DefaultClaimMapping())
	if len(u.Scopes) != 2 {
		t.Errorf("Scopes = %v, want 2", u.Scopes)
	}
	// Falls back to sub for the name when username claim is absent.
	if u.Name != "u1" {
		t.Errorf("Name = %q, want u1 (from sub)", u.Name)
	}
}

func TestUserFromClaims_IDClaimsPreferred(t *testing.T) {
	at := map[string]any{"given_name": "", "email": ""}
	id := map[string]any{"given_name": "Alice", "email": "alice@example.com"}
	m := DefaultClaimMapping()
	m.Username = "given_name"
	u := UserFromClaims(at, id, m)
	if u.Name != "Alice" || u.Email != "alice@example.com" {
		t.Errorf("got name=%q email=%q, want Alice/alice@example.com", u.Name, u.Email)
	}
}

func TestExpiryFromClaims(t *testing.T) {
	exp := time.Now().Add(time.Hour).Unix()
	claims := map[string]any{"exp": float64(exp)}
	got := ExpiryFromClaims(claims)
	if got.Unix() != exp {
		t.Errorf("ExpiryFromClaims = %d, want %d", got.Unix(), exp)
	}
	if !ExpiryFromClaims(map[string]any{}).IsZero() {
		t.Error("expected zero time when exp absent")
	}
}
