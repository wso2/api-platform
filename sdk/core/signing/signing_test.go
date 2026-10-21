/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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

package signing

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// fakeService records what the package functions hand to the installed
// Service. The real implementation lives in the policy engine.
type fakeService struct {
	active   bool
	lastJWS  []byte
	lastOpts Options
}

func (f *fakeService) Sign(data []byte) ([]byte, KeyInfo, error) {
	return []byte("sig"), KeyInfo{Kid: "k1", Algorithm: ES256}, nil
}

func (f *fakeService) SignJWS(payload []byte, opts Options) (string, KeyInfo, error) {
	f.lastJWS, f.lastOpts = payload, opts
	return "h.p.s", KeyInfo{Kid: "k1", Algorithm: ES256}, nil
}

func (f *fakeService) SignDetachedJWS(payload []byte, opts Options) (DetachedJWS, KeyInfo, error) {
	f.lastJWS, f.lastOpts = payload, opts
	return DetachedJWS{Protected: "h", Signature: "s"}, KeyInfo{Kid: "k1", Algorithm: ES256}, nil
}

func (f *fakeService) ActiveKey() (KeyInfo, bool) {
	if !f.active {
		return KeyInfo{}, false
	}
	return KeyInfo{Kid: "k1", Algorithm: ES256}, true
}

func (f *fakeService) JWKS() ([]byte, string) { return []byte(`{"keys":[{"kid":"k1"}]}`), `"etag"` }

func (f *fakeService) JWKSCacheMaxAge() time.Duration { return time.Minute }

func install(t *testing.T, svc Service) {
	t.Helper()
	Install(svc)
	t.Cleanup(func() { Install(nil) })
}

func TestNothingInstalled(t *testing.T) {
	Install(nil)
	if Available() {
		t.Fatal("Available() with nothing installed")
	}
	if _, ok := ActiveKey(); ok {
		t.Fatal("ActiveKey() reported a key with nothing installed")
	}
	if _, _, err := SignJWS([]byte("x")); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("SignJWS err = %v, want ErrNotConfigured", err)
	}
	if _, _, err := SignJWT(map[string]any{"sub": "x"}); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("SignJWT err = %v, want ErrNotConfigured", err)
	}
	if _, _, err := SignDetachedJWS([]byte("x")); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("SignDetachedJWS err = %v, want ErrNotConfigured", err)
	}
	if _, _, err := Sign([]byte("x")); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("Sign err = %v, want ErrNotConfigured", err)
	}
	body, etag := JWKS()
	if string(body) != `{"keys":[]}` || etag == "" {
		t.Fatalf("JWKS() = %s %s", body, etag)
	}
	if JWKSCacheMaxAge() != DefaultJWKSCacheMaxAge {
		t.Fatalf("JWKSCacheMaxAge() = %s", JWKSCacheMaxAge())
	}
}

func TestInstalledServiceWithoutActiveKeyIsNotAvailable(t *testing.T) {
	install(t, &fakeService{})
	if Available() {
		t.Fatal("Available() with no active key")
	}
	body, _ := JWKS()
	if string(body) != `{"keys":[{"kid":"k1"}]}` {
		t.Fatalf("JWKS() not delegated: %s", body)
	}
}

func TestCallsDelegateToTheInstalledService(t *testing.T) {
	svc := &fakeService{active: true}
	install(t, svc)

	if !Available() {
		t.Fatal("Available() = false with an active key")
	}
	if JWKSCacheMaxAge() != time.Minute {
		t.Fatalf("JWKSCacheMaxAge() not delegated: %s", JWKSCacheMaxAge())
	}

	if _, _, err := SignJWT(map[string]any{"sub": "alice"}, WithHeader("cty", "x"), WithJKU()); err != nil {
		t.Fatal(err)
	}
	var claims map[string]any
	if err := json.Unmarshal(svc.lastJWS, &claims); err != nil || claims["sub"] != "alice" {
		t.Fatalf("claims = %s (%v)", svc.lastJWS, err)
	}
	if svc.lastOpts.Type != "JWT" || svc.lastOpts.Headers["cty"] != "x" || !svc.lastOpts.JKU {
		t.Fatalf("options = %+v", svc.lastOpts)
	}

	if _, _, err := SignJWT(map[string]any{}, WithType("at+jwt")); err != nil {
		t.Fatal(err)
	}
	if svc.lastOpts.Type != "at+jwt" {
		t.Fatalf("WithType did not override the JWT default: %q", svc.lastOpts.Type)
	}

	if _, _, err := SignDetachedJWS([]byte("card")); err != nil {
		t.Fatal(err)
	}
	if string(svc.lastJWS) != "card" {
		t.Fatalf("detached payload = %s", svc.lastJWS)
	}
}

func TestReservedHeadersCannotBeOverridden(t *testing.T) {
	svc := &fakeService{active: true}
	install(t, svc)
	for _, name := range []string{"alg", "kid", "jku", "crit", "b64", "typ"} {
		if _, _, err := SignJWS([]byte("x"), WithHeader(name, "none")); err == nil {
			t.Fatalf("WithHeader(%q) was accepted", name)
		}
		if _, _, err := SignDetachedJWS([]byte("x"), WithHeader(name, "none")); err == nil {
			t.Fatalf("WithHeader(%q) was accepted by SignDetachedJWS", name)
		}
	}
	if svc.lastJWS != nil {
		t.Fatal("a rejected option still reached the service")
	}
}
