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

package migration

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testKey is a fixed 32-byte AES-256 key for deterministic crypto tests.
var testKey = []byte("0123456789abcdef0123456789abcdef")

func testKernels(t *testing.T) *Kernels {
	t.Helper()
	cp, err := LoadCheckpoint(filepath.Join(t.TempDir(), "ckpt.json"))
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	cfg := &Config{
		EncryptionKey:      testKey,
		V1Location:         time.UTC,
		MigrationActorUUID: DefaultMigrationActorUUID,
	}
	k, err := NewKernels(cfg, NewLogger("error", "text"), cp)
	if err != nil {
		t.Fatalf("kernels: %v", err)
	}
	return k
}

// ---- tsToTstz (§B.3) ----

func TestTsToTstz_AppZoneShiftsInstant(t *testing.T) {
	ist := time.FixedZone("IST", 5*3600+30*60) // +05:30
	// A naive v1 wall-clock read as UTC digits: 2026-01-02 10:00:00.
	naive := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)

	// Interpreted in +05:30, the true instant is 04:30:00 UTC.
	got := tsToTstz(naive, ist)
	want := time.Date(2026, 1, 2, 4, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("app-zone convert: got %s, want %s", got, want)
	}

	// Interpreted as UTC (the .UTC() tables), the digits ARE the instant.
	if utc := tsToTstz(naive, time.UTC); !utc.Equal(naive) {
		t.Fatalf("utc convert should be identity: got %s, want %s", utc, naive)
	}
}

// ---- secretHash reproduces v2 secret_service.hashSecret ----

func TestSecretHash_MatchesHMAC(t *testing.T) {
	pt := "Bearer super-secret"
	got := secretHash(testKey, pt)

	mac := hmac.New(sha256.New, testKey)
	mac.Write([]byte(pt))
	want := fmt.Sprintf("hmac-sha256:%x", mac.Sum(nil))

	if got != want {
		t.Fatalf("hash mismatch: got %q want %q", got, want)
	}
	if !strings.HasPrefix(got, "hmac-sha256:") {
		t.Fatalf("missing prefix: %q", got)
	}
	if secretHash(testKey, pt) != got {
		t.Fatalf("hash not deterministic")
	}
}

// ---- deterministic uuidv7 ----

func TestDetUUID_DeterministicAndV7(t *testing.T) {
	ts := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	a := detUUID("actor|alice", ts)
	b := detUUID("actor|alice", ts)
	c := detUUID("actor|bob", ts)
	if a != b {
		t.Fatalf("uuid not deterministic: %s != %s", a, b)
	}
	if a == c {
		t.Fatalf("distinct inputs collided: %s", a)
	}
	// version nibble (char 14, index of the 3rd group) must be '7'.
	if len(a) != 36 || a[14] != '7' {
		t.Fatalf("not a uuidv7: %s", a)
	}
	// actorUUID is stable across calls (order-independent seeding).
	if actorUUID("alice") != actorUUID("alice") {
		t.Fatalf("actorUUID not stable")
	}
}

func TestSecretHandle_FitsVarchar40(t *testing.T) {
	ts := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	h := secretHandle("019c-artifact", "upstream.main.auth.value", ts)
	if len(h) > maxHandleLen {
		t.Fatalf("secret handle %q is %d chars > %d", h, len(h), maxHandleLen)
	}
	if secretHandle("019c-artifact", "upstream.main.auth.value", ts) != h {
		t.Fatalf("secret handle not deterministic")
	}
}

// ---- handle carry / mint ----

func TestCarryHandle_BlocksOverLength(t *testing.T) {
	k := testKernels(t)
	if _, err := k.carryHandle("organizations", "u1", strings.Repeat("a", 41)); err == nil {
		t.Fatalf("expected over-length handle to be rejected")
	}
	if h, err := k.carryHandle("organizations", "u1", "valid-handle"); err != nil || h != "valid-handle" {
		t.Fatalf("valid handle carry failed: %v %q", err, h)
	}
}

func TestMintHandle_CheckpointStable(t *testing.T) {
	k := testKernels(t)
	h1, err := k.mintHandle("projects", "p1", "My Project", nil)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if h1 != "my-project" {
		t.Fatalf("unexpected slug: %q", h1)
	}
	// A second call reuses the checkpointed handle even if the source changes.
	h2, _ := k.mintHandle("projects", "p1", "Totally Different", nil)
	if h2 != h1 {
		t.Fatalf("checkpoint not reused: %q != %q", h2, h1)
	}
}

func TestAssertVarchar255(t *testing.T) {
	if err := assertVarchar255("issuer", "k1", strings.Repeat("x", 255)); err != nil {
		t.Fatalf("255 should pass: %v", err)
	}
	if err := assertVarchar255("issuer", "k1", strings.Repeat("x", 256)); err == nil {
		t.Fatalf("256 should fail")
	}
}

// ---- LLM policy split (§B.11) ----

func TestSplitLegacyPolicies(t *testing.T) {
	cfg := map[string]any{
		"policies": []any{
			map[string]any{
				"name": "guard", "version": "v1",
				"paths": []any{
					map[string]any{"path": "/*", "methods": []any{"*"}, "params": map[string]any{"a": 1}},
					map[string]any{"path": "/chat", "methods": []any{"POST"}},
				},
			},
		},
	}
	splitLegacyPoliciesInPlace(cfg)

	if _, ok := cfg["policies"]; ok {
		t.Fatalf("flat policies not cleared")
	}
	gp, ok := cfg["globalPolicies"].([]any)
	if !ok || len(gp) != 1 {
		t.Fatalf("expected 1 global policy, got %v", cfg["globalPolicies"])
	}
	if g := gp[0].(map[string]any); g["name"] != "guard" || g["params"] == nil {
		t.Fatalf("global policy wrong: %v", g)
	}
	op, ok := cfg["operationPolicies"].([]any)
	if !ok || len(op) != 1 {
		t.Fatalf("expected 1 operation policy, got %v", cfg["operationPolicies"])
	}
}

// ---- transport parse ----

func TestParseTransport(t *testing.T) {
	cases := map[string][]string{
		`["http","https"]`: {"http", "https"},
		`http,https`:       {"http", "https"},
		`http`:             {"http"},
		``:                 nil,
	}
	for in, want := range cases {
		got := parseTransport(in)
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("parseTransport(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIsCredentialLess(t *testing.T) {
	for _, in := range []string{"none", "None", "OTHER", "no-ne", "ot_her"} {
		if !isCredentialLessUpstreamAuthType(in) {
			t.Fatalf("%q should be credential-less", in)
		}
	}
	for _, in := range []string{"basic", "bearer", "api-key", ""} {
		if in != "" && isCredentialLessUpstreamAuthType(in) {
			t.Fatalf("%q should NOT be credential-less", in)
		}
	}
}

func TestMapThrottleUnit(t *testing.T) {
	cases := map[string]string{"min": "MINUTE", "Hour": "HOUR", "day": "DAY", "Month": "MONTH"}
	for in, want := range cases {
		if got := mapThrottleUnit(sql.NullString{String: in, Valid: true}); got != want {
			t.Fatalf("mapThrottleUnit(%q) = %q, want %q", in, got, want)
		}
	}
}

// ---- config externalization end-to-end (dry-run, no DB) ----

func TestTransformArtifactConfig_ExternalizesSecret(t *testing.T) {
	k := testKernels(t)
	in := map[string]any{
		"upstream": map[string]any{
			"main": map[string]any{
				"auth": map[string]any{"type": "basic", "value": "Bearer plaintext-token"},
			},
		},
	}
	blob, _ := json.Marshal(in)
	ts := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	out, err := k.transformArtifactConfig(context.Background(), nil /* dry-run */, "RestApi", "art-1", "org-1", "actor-1", ts, blob, "")
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	val := got["upstream"].(map[string]any)["main"].(map[string]any)["auth"].(map[string]any)["value"].(string)
	if !strings.HasPrefix(val, "{{ secret \"") {
		t.Fatalf("credential not externalized to a placeholder: %q", val)
	}
	if strings.Contains(string(out), "plaintext-token") {
		t.Fatalf("plaintext leaked into config blob")
	}
}

// TestVerifyRowCounts_DryRunDoesNotFail is a regression test: a --dry-run
// verification must NOT FAIL a resource against the unwritten (empty) target —
// the transform having run is the dry-run's success signal. With nil DBs the
// dry-run branch must return before any target query (no panic, status PASS).
func TestVerifyRowCounts_DryRunDoesNotFail(t *testing.T) {
	rc := &RunContext{DryRun: true} // Src and Tgt are nil
	rep := newReport("rest_apis")
	verifyRowCounts(context.Background(), rc, rep, "select 1", "select 1", false)
	if rep.Status != StatusPass {
		t.Fatalf("dry-run verify should stay PASS, got %s (%v)", rep.Status, rep.Messages)
	}
	if len(rep.Messages) == 0 {
		t.Fatalf("dry-run verify should note that parity is not applicable")
	}
}

func TestTransformArtifactConfig_SkipsCredentialLess(t *testing.T) {
	k := testKernels(t)
	in := map[string]any{
		"upstream": map[string]any{
			"main": map[string]any{"auth": map[string]any{"type": "none", "value": "ignored"}},
		},
	}
	blob, _ := json.Marshal(in)
	out, err := k.transformArtifactConfig(context.Background(), nil, "RestApi", "a", "o", "actor", time.Now(), blob, "")
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	if !strings.Contains(string(out), "ignored") {
		t.Fatalf("credential-less value should be left untouched")
	}
}

func TestMintScopedHandle_UniqueWithinScopeForThisRun(t *testing.T) {
	k := testKernels(t)
	// Dry-run: no target probe. Two plans with one name in one org must still
	// get distinct handles; the same name in another org keeps the base slug.
	h1, err := k.mintScopedHandle("subscription_plans", "org-A", "p1", "Gold", nil)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := k.mintScopedHandle("subscription_plans", "org-A", "p2", "Gold", nil)
	if err != nil {
		t.Fatal(err)
	}
	h3, err := k.mintScopedHandle("subscription_plans", "org-B", "p3", "Gold", nil)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != "gold" || h3 != "gold" || h2 == h1 || !strings.HasPrefix(h2, "gold-") {
		t.Fatalf("unexpected handles: h1=%q h2=%q h3=%q", h1, h2, h3)
	}
	// The same row asking again keeps its handle.
	if again, err := k.mintScopedHandle("subscription_plans", "org-A", "p1", "Gold", nil); err != nil || again != h1 {
		t.Fatalf("re-mint for same row = %q, %v; want %q", again, err, h1)
	}
}

func TestMintScopedHandle_StaleCheckpointCollisionFails(t *testing.T) {
	k := testKernels(t)
	if _, err := k.mintScopedHandle("gateways", "org-A", "g1", "edge", nil); err != nil {
		t.Fatal(err)
	}
	// A checkpoint from another run says g2's handle is "edge" too.
	if err := k.cp.PutHandle("gateways", "g2", "edge"); err != nil {
		t.Fatal(err)
	}
	if _, err := k.mintScopedHandle("gateways", "org-A", "g2", "edge", nil); err == nil || !strings.Contains(err.Error(), "checkpoint") {
		t.Fatalf("expected a stale-checkpoint collision error, got %v", err)
	}
}
