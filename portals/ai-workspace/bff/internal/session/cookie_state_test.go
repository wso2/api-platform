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
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"ai-workspace-bff/internal/secure"
)

func testCodec(t *testing.T, material string, chunkSize, maxChunks int) *CookieCodec {
	t.Helper()
	sealer, err := secure.NewSealer(secure.DeriveKey(material, "test/state"))
	if err != nil {
		t.Fatalf("NewSealer: %v", err)
	}
	return NewCookieCodec(sealer, chunkSize, maxChunks, DefaultClaimMapping())
}

// testJWT builds a decodable (never verified) JWT, which the codec now needs for real:
// the display User and the access expiry are recomputed from the tokens rather than
// stored, so a placeholder string would decode to an empty user.
func testJWT(claims map[string]any) string {
	enc := func(v any) string {
		b, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	return enc(map[string]string{"alg": "RS256", "typ": "JWT"}) + "." + enc(claims) + ".c2lnbmF0dXJl"
}

var sampleAccessExp = time.Now().Add(time.Hour).Truncate(time.Second)

func sampleAccessToken() string {
	return testJWT(map[string]any{
		"sub": "u-1", "exp": sampleAccessExp.Unix(),
		"scope": "ap:project:read", "username": "alice",
	})
}

func sampleIDToken() string {
	return testJWT(map[string]any{"sub": "u-1", "email": "alice@example.com", "username": "alice"})
}

func sampleSession() *Session {
	access := sampleAccessToken()
	return &Session{
		ID:             access,
		Mode:           ModeOIDC,
		AccessToken:    access,
		RefreshToken:   "refresh-token",
		IDToken:        sampleIDToken(),
		AccessExpiry:   sampleAccessExp,
		AbsoluteExpiry: time.Now().Add(8 * time.Hour).Truncate(time.Second),
		User:           User{Name: "alice", Email: "alice@example.com", Scopes: []string{"ap:project:read"}},
		OrgHandle:      "acme",
		OrgDiscovered:  true,
		Exchanged: ExchangedToken{
			Token:             "exchanged-token",
			Expiry:            time.Now().Add(time.Hour).Truncate(time.Second),
			Scopes:            []string{"ap:project:read", "ap:gateway:read"},
			ConfigFingerprint: "fp",
			OrgHandle:         "acme",
			Org:               &Org{ID: "o1", Name: "Acme", Handle: "acme"},
		},
	}
}

// The whole point of the codec: everything a request needs survives a trip through
// the browser, and a codec built independently (another replica) reads it back.
func TestCookieStateRoundTripAcrossCodecs(t *testing.T) {
	writer := testCodec(t, "shared", 3500, 4)
	reader := testCodec(t, "shared", 3500, 4)
	want := sampleSession()

	parts, err := writer.Encode(want)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, ok := reader.Decode(parts, want.AccessToken)
	if !ok {
		t.Fatal("Decode reported no session")
	}

	if got.RefreshToken != want.RefreshToken || got.IDToken != want.IDToken {
		t.Errorf("tokens = %q/%q, want %q/%q", got.RefreshToken, got.IDToken, want.RefreshToken, want.IDToken)
	}
	if got.AccessToken != want.AccessToken || got.ID != want.AccessToken {
		t.Errorf("access token/id not rebuilt from the presented token: %q/%q", got.AccessToken, got.ID)
	}
	if got.OrgHandle != want.OrgHandle || !got.OrgDiscovered {
		t.Errorf("org = %q discovered=%v, want %q true", got.OrgHandle, got.OrgDiscovered, want.OrgHandle)
	}
	// User is no longer stored — it is recomputed from the access token and id_token.
	// The id_token is where the email lives, so finding it here proves both tokens
	// survived the trip and were fed back through the same mapping the login path uses.
	if got.User.Name != "alice" || got.User.Email != "alice@example.com" {
		t.Errorf("user = %+v, want it rebuilt from the access token and id_token", got.User)
	}
	if len(got.User.Scopes) != 1 || got.User.Scopes[0] != "ap:project:read" {
		t.Errorf("user scopes = %v, want the access token's own scope claim", got.User.Scopes)
	}
	if !got.AccessExpiry.Equal(sampleAccessExp) {
		t.Errorf("access expiry = %v, want it read off the access token's exp claim (%v)",
			got.AccessExpiry, sampleAccessExp)
	}
	if got.Exchanged.Token != want.Exchanged.Token || got.Exchanged.ConfigFingerprint != "fp" {
		t.Errorf("exchanged = %+v", got.Exchanged)
	}
	if got.Exchanged.Org == nil || got.Exchanged.Org.Handle != "acme" {
		t.Errorf("exchanged org = %+v, want acme", got.Exchanged.Org)
	}
	if !got.Exchanged.Expiry.Equal(want.Exchanged.Expiry) || !got.AbsoluteExpiry.Equal(want.AbsoluteExpiry) {
		t.Errorf("expiries not preserved: %v / %v", got.Exchanged.Expiry, got.AbsoluteExpiry)
	}

	// What the record must NOT be carrying: the envelope fields that are derivations.
	// Asserted on the envelope's own keys, not on values — the id_token's payload is
	// stored as plain text (that is what makes it compressible), so a claim like the
	// email does legitimately appear inside it.
	raw := decodeEnvelopeForTest(t, reader, parts)
	for key, derivation := range map[string]string{
		`"u":`:  "the display user (rebuilt from the access token + id_token)",
		`"ae":`: "the access expiry (read off the access token's exp claim)",
		`"m":`:  "the session mode (always OIDC for a stored session)",
	} {
		if strings.Contains(raw, key) {
			t.Errorf("the sealed record still carries %s — %s", key, derivation)
		}
	}
}

// A record is bound to one access token: salvaged from another session, or kept
// across a rotation, it decrypts fine and must still be refused.
func TestCookieStateRejectsRecordBoundToAnotherToken(t *testing.T) {
	c := testCodec(t, "shared", 3500, 4)
	parts, err := c.Encode(sampleSession())
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if _, ok := c.Decode(parts, testJWT(map[string]any{"sub": "u-2"})); ok {
		t.Fatal("a record bound to a different access token was accepted")
	}
}

func TestCookieStateRejectsUnusableRecords(t *testing.T) {
	c := testCodec(t, "shared", 3500, 4)
	parts, err := c.Encode(sampleSession())
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	t.Run("another deployment's key", func(t *testing.T) {
		if _, ok := testCodec(t, "different", 3500, 4).Decode(parts, sampleAccessToken()); ok {
			t.Fatal("a record sealed under another key was accepted")
		}
	})
	t.Run("tampered", func(t *testing.T) {
		bad := append([]string(nil), parts...)
		bad[0] = "A" + bad[0][1:]
		if _, ok := c.Decode(bad, sampleAccessToken()); ok {
			t.Fatal("a tampered record was accepted")
		}
	})
	t.Run("missing chunk", func(t *testing.T) {
		if _, ok := c.Decode([]string{parts[0], ""}, sampleAccessToken()); ok {
			t.Fatal("a record with a missing chunk was accepted")
		}
	})
	t.Run("no cookies", func(t *testing.T) {
		if _, ok := c.Decode(nil, sampleAccessToken()); ok {
			t.Fatal("an absent record was accepted")
		}
	})
	t.Run("past absolute expiry", func(t *testing.T) {
		expired := sampleSession()
		expired.AbsoluteExpiry = time.Now().Add(-time.Minute)
		p, err := c.Encode(expired)
		if err != nil {
			t.Fatalf("Encode: %v", err)
		}
		if _, ok := c.Decode(p, sampleAccessToken()); ok {
			t.Fatal("a session past its absolute expiry was accepted")
		}
	})
}

func TestCookieStateChunksToTheConfiguredSize(t *testing.T) {
	c := testCodec(t, "shared", 64, 64)
	parts, err := c.Encode(sampleSession())
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(parts) < 2 {
		t.Fatalf("expected the record to be split, got %d chunk(s)", len(parts))
	}
	for i, p := range parts {
		if len(p) > 64 {
			t.Fatalf("chunk %d is %d bytes, over the 64-byte budget", i, len(p))
		}
	}
	if _, ok := c.Decode(parts, sampleAccessToken()); !ok {
		t.Fatal("a chunked record did not reassemble")
	}
}

// Over budget, the session must still be storable — shedding the optional parts in
// order — because the alternative is a user who cannot stay logged in at all.
func TestCookieStateShedsOptionalFieldsToFitBudget(t *testing.T) {
	big := sampleSession()
	big.IDToken = strings.Repeat("i", 4000)
	big.Exchanged.Token = strings.Repeat("e", 4000)

	// Deliberately tight: enough for the refresh token and user, not for both JWTs.
	c := testCodec(t, "shared", 64, 4)
	parts, err := c.Encode(big)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, ok := c.Decode(parts, sampleAccessToken())
	if !ok {
		t.Fatal("Decode reported no session")
	}
	if got.RefreshToken != "refresh-token" {
		t.Errorf("the refresh token must never be shed, got %q", got.RefreshToken)
	}
	if got.IDToken != "" || got.Exchanged.Token != "" {
		t.Errorf("optional fields were not shed: id=%d exchanged=%d", len(got.IDToken), len(got.Exchanged.Token))
	}
}

func TestCookieStateFailsWhenNothingFits(t *testing.T) {
	s := sampleSession()
	s.RefreshToken = strings.Repeat("r", 100_000)
	if _, err := testCodec(t, "shared", 64, 1).Encode(s); !errors.Is(err, ErrStateTooLarge) {
		t.Fatalf("Encode error = %v, want ErrStateTooLarge", err)
	}
}

// decodeEnvelopeForTest unseals and inflates a record back to its raw JSON, so a test
// can assert on what the record does NOT contain.
func decodeEnvelopeForTest(t *testing.T, c *CookieCodec, parts []string) string {
	t.Helper()
	plain, err := c.sealer.Open(joinChunks(parts))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	raw, err := io.ReadAll(flate.NewReader(bytes.NewReader(plain)))
	if err != nil {
		t.Fatalf("inflate: %v", err)
	}
	return string(raw)
}
