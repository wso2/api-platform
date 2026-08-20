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

package redisclient

import (
	"context"
	"crypto/tls"
	"math"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestGetOrCreateClient_SharesClientForIdenticalConfig(t *testing.T) {
	resetRegistryForTest(t)
	mr := miniredis.RunT(t)
	opts := &redis.Options{Addr: mr.Addr(), DB: 0}

	c1, created1, err1 := GetOrCreate(opts, time.Second)
	if !created1 || err1 != nil {
		t.Fatalf("first call: created=%v err=%v (want true,nil)", created1, err1)
	}

	c2, created2, err2 := GetOrCreate(opts, time.Second)
	if created2 || err2 != nil {
		t.Fatalf("second call: created=%v err=%v (want false,nil)", created2, err2)
	}
	if c1 != c2 {
		t.Error("expected identical connection settings to share one *redis.Client")
	}
}

func TestGetOrCreateClient_DistinctClientForDifferentConfig(t *testing.T) {
	resetRegistryForTest(t)
	mr := miniredis.RunT(t)

	c1, _, _ := GetOrCreate(&redis.Options{Addr: mr.Addr(), DB: 0}, time.Second)
	c2, _, _ := GetOrCreate(&redis.Options{Addr: mr.Addr(), DB: 1}, time.Second)

	if c1 == c2 {
		t.Error("expected different DB selection to produce a distinct *redis.Client")
	}
}

func TestGetOrCreateClient_DifferentPasswordProducesDistinctClient(t *testing.T) {
	resetRegistryForTest(t)
	mr := miniredis.RunT(t)

	c1, _, _ := GetOrCreate(&redis.Options{Addr: mr.Addr(), Password: "one"}, time.Second)
	c2, _, _ := GetOrCreate(&redis.Options{Addr: mr.Addr(), Password: "two"}, time.Second)
	c3, _, _ := GetOrCreate(&redis.Options{Addr: mr.Addr()}, time.Second) // no password at all

	if c1 == c2 {
		t.Error("expected different passwords to produce distinct clients")
	}
	if c1 == c3 || c2 == c3 {
		t.Error("expected an absent password not to collide with a present one")
	}
}

func TestGetOrCreateClient_SharedAcrossSimulatedPolicies(t *testing.T) {
	resetRegistryForTest(t)
	mr := miniredis.RunT(t)
	opts := func() *redis.Options { return &redis.Options{Addr: mr.Addr(), DB: 0} }

	// Two distinct call sites with identical settings must share one
	// client - the whole point of centralizing the registry.
	fromPolicyA, _, _ := GetOrCreate(opts(), time.Second)
	fromPolicyB, _, _ := GetOrCreate(opts(), time.Second)

	if fromPolicyA != fromPolicyB {
		t.Fatal("expected two distinct callers with identical config to share one client")
	}

	ctx := context.Background()
	if err := fromPolicyA.Set(ctx, "shared-key", "value", 0).Err(); err != nil {
		t.Fatalf("unexpected error writing via the shared client: %v", err)
	}
	got, err := fromPolicyB.Get(ctx, "shared-key").Result()
	if err != nil {
		t.Fatalf("unexpected error reading via the shared client: %v", err)
	}
	if got != "value" {
		t.Errorf("got %q, want %q", got, "value")
	}
}

// TestGetOrCreateClient_ReuseSkipsPing locks in that only creation pings -
// a reused client is assumed healthy and must never be re-pinged.
func TestGetOrCreateClient_ReuseSkipsPing(t *testing.T) {
	resetRegistryForTest(t)
	mr := miniredis.RunT(t)
	addr := mr.Addr() // capture before mr.Close() below
	opts := &redis.Options{Addr: addr, DB: 0}

	c1, created1, err1 := GetOrCreate(opts, time.Second)
	if !created1 || err1 != nil {
		t.Fatalf("first call: created=%v err=%v (want true,nil)", created1, err1)
	}

	mr.Close()
	c2, created2, err2 := GetOrCreate(opts, time.Second)
	if created2 || err2 != nil || c2 != c1 {
		t.Fatalf("reuse after Redis went down should skip the ping: created=%v err=%v same=%v", created2, err2, c2 == c1)
	}
}

func TestGetOrCreateClient_DifferentProtocolProducesDistinctClient(t *testing.T) {
	resetRegistryForTest(t)
	mr := miniredis.RunT(t)

	c1, _, _ := GetOrCreate(&redis.Options{Addr: mr.Addr(), Protocol: 2}, time.Second)
	c2, _, _ := GetOrCreate(&redis.Options{Addr: mr.Addr(), Protocol: 3}, time.Second)
	c3, _, _ := GetOrCreate(&redis.Options{Addr: mr.Addr(), Protocol: 2}, time.Second)

	if c1 == c2 {
		t.Error("expected different RESP protocol versions to produce distinct clients")
	}
	if c1 != c3 {
		t.Error("expected the same protocol version to reuse the existing client")
	}
}

// TestGetOrCreateClient_TLSConfigBypassesRegistry locks in that a TLSConfig
// always gets a fresh, unshared client, even with otherwise-identical
// options - neither it nor a credentials-provider func can be fingerprinted
// safely, so sharing would risk a silent cross-config mixup.
func TestGetOrCreateClient_TLSConfigBypassesRegistry(t *testing.T) {
	resetRegistryForTest(t)
	mr := miniredis.RunT(t)

	optsA := &redis.Options{Addr: mr.Addr(), TLSConfig: &tls.Config{}} //nolint:gosec // test-only, no real handshake asserted
	optsB := &redis.Options{Addr: mr.Addr(), TLSConfig: &tls.Config{}} //nolint:gosec

	c1, created1, _ := GetOrCreate(optsA, time.Second)
	c2, created2, _ := GetOrCreate(optsB, time.Second)

	if !created1 || !created2 {
		t.Fatalf("expected every TLSConfig-bearing call to report created=true (never reused), got %v and %v", created1, created2)
	}
	if c1 == c2 {
		t.Error("expected two TLSConfig-bearing calls to never share a client, even with identical-looking options")
	}
}

func TestGetOrCreateClient_CredentialsProviderBypassesRegistry(t *testing.T) {
	resetRegistryForTest(t)
	mr := miniredis.RunT(t)
	provider := func() (string, string) { return "", "" }

	c1, created1, err1 := GetOrCreate(&redis.Options{Addr: mr.Addr(), CredentialsProvider: provider}, time.Second)
	c2, created2, err2 := GetOrCreate(&redis.Options{Addr: mr.Addr(), CredentialsProvider: provider}, time.Second)

	if !created1 || err1 != nil {
		t.Fatalf("first call: created=%v err=%v (want true,nil)", created1, err1)
	}
	if !created2 || err2 != nil {
		t.Fatalf("second call: created=%v err=%v (want true,nil - bypassed, not reused)", created2, err2)
	}
	if c1 == c2 {
		t.Error("expected two CredentialsProvider-bearing calls to never share a client")
	}
}

// TestGetOrCreateClient_DoesNotHoldLockDuringPing proves the registry lock
// guards only the map lookup/insert, never c.Ping - mu is process-wide, so
// holding it during a slow/unreachable Redis's ping would stall every other
// caller too, even for an unrelated, healthy endpoint.
func TestGetOrCreateClient_DoesNotHoldLockDuringPing(t *testing.T) {
	resetRegistryForTest(t)
	// Accepts but never responds, so Ping against it blocks until the
	// deadline - a reliable window to prove a concurrent, unrelated key
	// isn't blocked by it.
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start hanging listener: %v", err)
	}
	defer func() { _ = ln.Close() }()

	// Signaled once the slow client's connection is actually accepted -
	// proof it has dialed and is now blocked reading the Ping reply, rather
	// than guessing via a fixed sleep how long that takes to happen.
	accepted := make(chan struct{})
	var acceptedOnce sync.Once
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			acceptedOnce.Do(func() { close(accepted) })
			_ = conn // held open, never responded to
		}
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		// ReadTimeout set explicitly - the dial succeeds, it's the
		// read-for-a-reply that hangs, and go-redis's default (5s) would
		// otherwise bound that wait regardless of pingTimeout.
		_, _, _ = GetOrCreate(&redis.Options{
			Addr:         ln.Addr().String(),
			DB:           0,
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
		}, time.Second)
	}()

	select {
	case <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the slow client's connection to be accepted")
	}

	mr := miniredis.RunT(t)
	fastStart := time.Now()
	if _, _, err := GetOrCreate(&redis.Options{Addr: mr.Addr(), DB: 1}, 500*time.Millisecond); err != nil {
		t.Fatalf("unexpected error on the fast, unrelated key: %v", err)
	}
	if elapsed := time.Since(fastStart); elapsed > 300*time.Millisecond {
		t.Errorf("expected the unrelated key's get-or-create to complete quickly (the registry lock must not be held during the other call's ping), took %s", elapsed)
	}

	<-done // let the slow goroutine finish before the test exits
}

// resetRegistryForTest clears the process-wide redisClients registry so a
// closed miniredis instance's address (freed and possibly reused by a later
// miniredis.RunT in the same test binary) can never resolve to another
// test's stale, already-pinged client - restoring the prior state once the
// test ends.
func resetRegistryForTest(t *testing.T) {
	t.Helper()
	redisClients.mu.Lock()
	prev := redisClients.m
	redisClients.m = make(map[redisConnKey]*redis.Client)
	redisClients.mu.Unlock()
	t.Cleanup(func() {
		redisClients.mu.Lock()
		redisClients.m = prev
		redisClients.mu.Unlock()
	})
}

// resetSharedForTest clears shared so InitFromConfig can run again despite
// its once-only guard, restoring the prior state once the test ends.
func resetSharedForTest(t *testing.T) {
	t.Helper()
	shared.mu.Lock()
	prevClient, prevInited := shared.client, shared.inited
	shared.client, shared.inited = nil, false
	shared.mu.Unlock()
	t.Cleanup(func() {
		shared.mu.Lock()
		shared.client, shared.inited = prevClient, prevInited
		shared.mu.Unlock()
	})
}

func TestResolveOptionsFromConfig_NoRedisSectionReturnsNil(t *testing.T) {
	opts, err := resolveOptionsFromConfig(map[string]interface{}{})
	if err != nil || opts != nil {
		t.Fatalf("got opts=%v err=%v, want nil,nil when \"redis\" is absent entirely", opts, err)
	}
}

// TestResolveOptionsFromConfig_IgnoresSiblingSections proves it looks at the
// top-level "redis" key only - other sections, including policy_configurations,
// have no bearing on it.
func TestResolveOptionsFromConfig_IgnoresSiblingSections(t *testing.T) {
	raw := map[string]interface{}{
		"router":                map[string]interface{}{"gateway_host": "*"},
		"policy_configurations": map[string]interface{}{"oauth2_generator_v1": map[string]interface{}{"redis": map[string]interface{}{"key_prefix": "x:"}}},
	}
	opts, err := resolveOptionsFromConfig(raw)
	if err != nil || opts != nil {
		t.Fatalf("got opts=%v err=%v, want nil,nil when top-level \"redis\" is absent (unrelated sibling sections present)", opts, err)
	}
}

func TestResolveOptionsFromConfig_AppliesDefaults(t *testing.T) {
	opts, err := resolveOptionsFromConfig(map[string]interface{}{"redis": map[string]interface{}{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := &redis.Options{
		Addr:         "localhost:6379",
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
	if opts.Addr != want.Addr || opts.DialTimeout != want.DialTimeout ||
		opts.ReadTimeout != want.ReadTimeout || opts.WriteTimeout != want.WriteTimeout ||
		opts.Username != "" || opts.Password != "" || opts.DB != 0 || opts.PoolSize != 0 {
		t.Errorf("got %+v, want defaults %+v (username/password/db/poolSize zero-valued)", opts, want)
	}
}

// TestResolveOptionsFromConfig_ParsesConfiguredValues covers both the string
// (typical koanf/TOML decode) and numeric (int64/float64 - decoder-dependent)
// shapes a value might arrive in.
func TestResolveOptionsFromConfig_ParsesConfiguredValues(t *testing.T) {
	raw := map[string]interface{}{
		"redis": map[string]interface{}{
			"host":               "redis.example.com",
			"port":               int64(6380),
			"username":           "app",
			"password":           "secret",
			"db":                 float64(2),
			"connection_timeout": "10s",
			"read_timeout":       "7s",
			"write_timeout":      "7s",
			"pool_size":          20,
		},
	}
	opts, err := resolveOptionsFromConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Addr != "redis.example.com:6380" || opts.Username != "app" || opts.Password != "secret" ||
		opts.DB != 2 || opts.DialTimeout != 10*time.Second || opts.ReadTimeout != 7*time.Second ||
		opts.WriteTimeout != 7*time.Second || opts.PoolSize != 20 {
		t.Errorf("got %+v, did not match configured values", opts)
	}
}

// TestResolveOptionsFromConfig_ParsesNumericStringPort locks in the shape
// config interpolation actually produces: a TOML {{ env "VAR" "6379" }}
// token resolves in place without changing the field's type, so a "numeric"
// value arrives as a numeric string, not an int.
func TestResolveOptionsFromConfig_ParsesNumericStringPort(t *testing.T) {
	raw := map[string]interface{}{
		"redis": map[string]interface{}{
			"host": "redis.example.com",
			"port": "6380",
			"db":   "2",
		},
	}
	opts, err := resolveOptionsFromConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Addr != "redis.example.com:6380" || opts.DB != 2 {
		t.Errorf("got %+v, want port 6380 and db 2 parsed from numeric strings", opts)
	}
}

// TestResolveOptionsFromConfig_BracketsIPv6Host proves Addr is built via
// net.JoinHostPort - a plain fmt.Sprintf("%s:%d", host, port) would produce
// "::1:6380", which is ambiguous/invalid, instead of the required
// "[::1]:6380".
func TestResolveOptionsFromConfig_BracketsIPv6Host(t *testing.T) {
	raw := map[string]interface{}{
		"redis": map[string]interface{}{"host": "::1", "port": 6380},
	}
	opts, err := resolveOptionsFromConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "[::1]:6380"; opts.Addr != want {
		t.Errorf("got Addr %q, want %q", opts.Addr, want)
	}
}

func TestResolveOptionsFromConfig_RejectsWrongShapedValue(t *testing.T) {
	_, err := resolveOptionsFromConfig(map[string]interface{}{
		"redis": map[string]interface{}{"port": "not-a-number"},
	})
	if err == nil {
		t.Error("expected an error for a non-numeric port, so a config typo surfaces at startup instead of silently defaulting")
	}
}

// TestResolveOptionsFromConfig_RejectsInvalidFloat64Port locks in that a
// float64 "port" is validated before conversion - NaN/Inf/fractional/
// out-of-range values must error, not silently truncate.
func TestResolveOptionsFromConfig_RejectsInvalidFloat64Port(t *testing.T) {
	cases := []struct {
		name string
		v    float64
	}{
		{"NaN", math.NaN()},
		{"+Inf", math.Inf(1)},
		{"-Inf", math.Inf(-1)},
		{"fractional", 1.5},
		{"aboveMaxInt", 1e19},
		{"belowMinInt", -1e19},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := resolveOptionsFromConfig(map[string]interface{}{"redis": map[string]interface{}{"port": c.v}})
			if err == nil {
				t.Errorf("expected an error for float64 port %v, got nil", c.v)
			}
		})
	}
}

func TestResolveOptionsFromConfig_AcceptsIntegralFloat64Port(t *testing.T) {
	opts, err := resolveOptionsFromConfig(map[string]interface{}{"redis": map[string]interface{}{"port": float64(6380)}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "localhost:6380"; opts.Addr != want {
		t.Errorf("got Addr %q, want %q", opts.Addr, want)
	}
}

func TestResolveOptionsFromConfig_RejectsNonTableSection(t *testing.T) {
	_, err := resolveOptionsFromConfig(map[string]interface{}{"redis": "not-a-table"})
	if err == nil {
		t.Error("expected an error when \"redis\" isn't a table")
	}
}

// TestResolveOptionsFromConfig_RejectsOutOfRangePort locks in that a port
// outside 1-65535 - a plain literal int, never routed through
// rejectNonIntegralFloatHookFunc since no type coercion is needed - errors
// instead of silently producing an unusable *redis.Options.
func TestResolveOptionsFromConfig_RejectsOutOfRangePort(t *testing.T) {
	for _, port := range []int{0, -1, 65536} {
		t.Run("", func(t *testing.T) {
			_, err := resolveOptionsFromConfig(map[string]interface{}{"redis": map[string]interface{}{"host": "x", "port": port}})
			if err == nil {
				t.Errorf("expected an error for out-of-range port %d", port)
			}
		})
	}
}

// TestResolveOptionsFromConfig_RejectsNegativeDBOrPoolSize locks in the same
// gap as the port check above, for db/pool_size.
func TestResolveOptionsFromConfig_RejectsNegativeDBOrPoolSize(t *testing.T) {
	if _, err := resolveOptionsFromConfig(map[string]interface{}{"redis": map[string]interface{}{"host": "x", "db": -1}}); err == nil {
		t.Error("expected an error for a negative db")
	}
	if _, err := resolveOptionsFromConfig(map[string]interface{}{"redis": map[string]interface{}{"host": "x", "pool_size": -1}}); err == nil {
		t.Error("expected an error for a negative pool_size")
	}
}

// TestResolveOptionsFromConfig_RejectsSubMillisecondTimeout locks in that a
// timeout below 1ms - including a negative value, which go-redis would
// otherwise silently treat as "disable timeout enforcement entirely" - errors
// rather than producing an unsafe client. Also catches a bare numeric
// duration value (e.g. connection_timeout = 5), which mapstructure decodes
// as nanoseconds since time.Duration's underlying kind is int64, not caught
// by rejectNonIntegralFloatHookFunc's int-only guard.
func TestResolveOptionsFromConfig_RejectsSubMillisecondTimeout(t *testing.T) {
	cases := []struct {
		name string
		key  string
		val  interface{}
	}{
		{"negative connection_timeout", "connection_timeout", "-1s"},
		{"negative read_timeout", "read_timeout", "-2s"},
		{"bare numeric write_timeout decodes as nanoseconds", "write_timeout", 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := resolveOptionsFromConfig(map[string]interface{}{"redis": map[string]interface{}{"host": "x", c.key: c.val}})
			if err == nil {
				t.Errorf("expected an error for %s=%v", c.key, c.val)
			}
		})
	}
}

func TestInitFromConfig_NoRedisSectionLeavesSharedUnconfigured(t *testing.T) {
	resetSharedForTest(t)

	if err := InitFromConfig(map[string]interface{}{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := Shared(); err == nil {
		t.Error("expected Shared() to report a config-gap error when no redis section was ever configured")
	}
}

func TestInitFromConfig_CalledTwiceErrors(t *testing.T) {
	resetSharedForTest(t)

	if err := InitFromConfig(map[string]interface{}{}); err != nil {
		t.Fatalf("first call: unexpected error: %v", err)
	}
	if err := InitFromConfig(map[string]interface{}{}); err == nil {
		t.Error("expected a second InitFromConfig call to error - it must run exactly once")
	}
}

func TestSharedBeforeInitFromConfigErrors(t *testing.T) {
	resetSharedForTest(t)

	if _, err := Shared(); err == nil {
		t.Error("expected Shared() to error when InitFromConfig was never called")
	}
}

// TestSharedReturnsIdenticalPointer is the actual "single instance" contract:
// not merely "these two configs happen to compare equal" (GetOrCreate's
// dedup guarantee) but "there is exactly one gateway-level client, full stop."
func TestSharedReturnsIdenticalPointer(t *testing.T) {
	resetSharedForTest(t)
	mr := miniredis.RunT(t)

	port, err := strconv.Atoi(mr.Port())
	if err != nil {
		t.Fatalf("failed to parse miniredis port: %v", err)
	}
	raw := map[string]interface{}{"redis": map[string]interface{}{"host": mr.Host(), "port": port}}
	if err := InitFromConfig(raw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	c1, err1 := Shared()
	c2, err2 := Shared()
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}
	if c1 != c2 {
		t.Error("expected every Shared() call to return the identical *redis.Client pointer")
	}
}

func TestResolve_NilOptsFallsBackToShared(t *testing.T) {
	resetSharedForTest(t)
	mr := miniredis.RunT(t)
	sharedClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	SetSharedForTesting(t, sharedClient)

	got, err := Resolve(nil, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != sharedClient {
		t.Error("expected Resolve(nil, ...) to return the gateway-level Shared client")
	}
}

func TestResolve_NonNilOptsBypassesShared(t *testing.T) {
	resetSharedForTest(t)
	resetRegistryForTest(t)
	sharedMR := miniredis.RunT(t)
	SetSharedForTesting(t, redis.NewClient(&redis.Options{Addr: sharedMR.Addr()}))

	overrideMR := miniredis.RunT(t)
	got, err := Resolve(&redis.Options{Addr: overrideMR.Addr()}, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Options().Addr != overrideMR.Addr() {
		t.Errorf("expected a policy-supplied override to take precedence over the shared client, got client for %q", got.Options().Addr)
	}
}

// TestResolve_NonNilOptsStillDedupes proves Resolve's override branch keeps
// GetOrCreate's existing sharing behavior - two policies that both
// explicitly override to the same config still get one pool between them,
// not one pool each.
func TestResolve_NonNilOptsStillDedupes(t *testing.T) {
	resetSharedForTest(t)
	resetRegistryForTest(t)
	mr := miniredis.RunT(t)

	c1, err1 := Resolve(&redis.Options{Addr: mr.Addr()}, time.Second)
	c2, err2 := Resolve(&redis.Options{Addr: mr.Addr()}, time.Second)
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}
	if c1 != c2 {
		t.Error("expected two identical explicit overrides to still share one client")
	}
}

// TestSetSharedForTesting_RestoresPreviousStateAfterTest proves the override
// is scoped to one (sub)test: a subtest's t.Cleanup runs when that subtest
// returns, before the parent continues, so the parent sees the pre-override
// "unconfigured" state again immediately afterward.
func TestSetSharedForTesting_RestoresPreviousStateAfterTest(t *testing.T) {
	resetSharedForTest(t)
	if err := InitFromConfig(map[string]interface{}{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := Shared(); err == nil {
		t.Fatal("expected Shared() to error before any override is set")
	}

	t.Run("override active inside subtest", func(t *testing.T) {
		mr := miniredis.RunT(t)
		SetSharedForTesting(t, redis.NewClient(&redis.Options{Addr: mr.Addr()}))
		if _, err := Shared(); err != nil {
			t.Fatalf("unexpected error while override was active: %v", err)
		}
	})

	if _, err := Shared(); err == nil {
		t.Error("expected the override to be reverted once the subtest returned")
	}
}

func TestHashPassword(t *testing.T) {
	if hashRedisPassword("") != "" {
		t.Error("expected an empty password to hash to empty, not sha256(\"\")")
	}
	if hashRedisPassword("secret") == "secret" {
		t.Error("expected the password to actually be hashed, not passed through")
	}
	const wantSecretSHA256 = "2bb80d537b1da3e38bd30361aa855686bde0eacd7162fef6a25fe97bf527a25b"
	if got := hashRedisPassword("secret"); got != wantSecretSHA256 {
		t.Errorf("hashRedisPassword(%q) = %q, want %q", "secret", got, wantSecretSHA256)
	}
	if hashRedisPassword("secret") == hashRedisPassword("different") {
		t.Error("expected different passwords to hash differently")
	}
}

func TestExtractOverrideFromParams_NoHostReturnsNil(t *testing.T) {
	if got := ExtractOverrideFromParams(map[string]interface{}{}); got != nil {
		t.Fatalf("got %+v, want nil when redis.host is absent", got)
	}
	if got := ExtractOverrideFromParams(map[string]interface{}{"redis": map[string]interface{}{"port": 6380}}); got != nil {
		t.Fatalf("got %+v, want nil when redis.host is absent even with sibling redis.* fields set", got)
	}
}

func TestExtractOverrideFromParams_NestedMapShape(t *testing.T) {
	params := map[string]interface{}{
		"redis": map[string]interface{}{
			"host":              "redis.example.com",
			"port":              6380,
			"username":          "app",
			"password":          "secret",
			"db":                2,
			"poolSize":          10,
			"connectionTimeout": "10s",
			"readTimeout":       "7s",
			"writeTimeout":      "7s",
		},
	}
	opts := ExtractOverrideFromParams(params)
	if opts == nil {
		t.Fatal("expected a non-nil override")
	}
	if opts.Addr != "redis.example.com:6380" || opts.Username != "app" || opts.Password != "secret" ||
		opts.DB != 2 || opts.PoolSize != 10 || opts.DialTimeout != 10*time.Second ||
		opts.ReadTimeout != 7*time.Second || opts.WriteTimeout != 7*time.Second {
		t.Errorf("got %+v, did not match configured values", opts)
	}
}

func TestExtractOverrideFromParams_FlattenedKeyShape(t *testing.T) {
	params := map[string]interface{}{
		"redis.host": "redis.example.com",
		"redis.port": "6380",
		"redis.db":   "2",
	}
	opts := ExtractOverrideFromParams(params)
	if opts == nil {
		t.Fatal("expected a non-nil override")
	}
	if opts.Addr != "redis.example.com:6380" || opts.DB != 2 {
		t.Errorf("got %+v, want port 6380 and db 2 parsed from a flattened-key, numeric-string shape", opts)
	}
}

func TestExtractOverrideFromParams_AppliesDefaults(t *testing.T) {
	opts := ExtractOverrideFromParams(map[string]interface{}{"redis": map[string]interface{}{"host": "redis.example.com"}})
	if opts == nil {
		t.Fatal("expected a non-nil override")
	}
	if opts.Addr != "redis.example.com:6379" || opts.DB != 0 || opts.PoolSize != 0 ||
		opts.DialTimeout != 5*time.Second || opts.ReadTimeout != 3*time.Second || opts.WriteTimeout != 3*time.Second {
		t.Errorf("got %+v, want defaults applied for every field but host", opts)
	}
}

func TestExtractOverrideFromParams_BracketsIPv6Host(t *testing.T) {
	opts := ExtractOverrideFromParams(map[string]interface{}{"redis": map[string]interface{}{"host": "::1", "port": 6380}})
	if opts == nil {
		t.Fatal("expected a non-nil override")
	}
	if want := "[::1]:6380"; opts.Addr != want {
		t.Errorf("got Addr %q, want %q", opts.Addr, want)
	}
}

func TestExtractOverrideFromParams_UnparseableValueFallsBackToDefaultSilently(t *testing.T) {
	params := map[string]interface{}{
		"redis": map[string]interface{}{
			"host":              "redis.example.com",
			"port":              "not-a-number",
			"connectionTimeout": "not-a-duration",
		},
	}
	opts := ExtractOverrideFromParams(params)
	if opts == nil {
		t.Fatal("expected a non-nil override")
	}
	if opts.Addr != "redis.example.com:6379" {
		t.Errorf("got Addr %q, want the default port 6379 silently applied for an unparseable port", opts.Addr)
	}
	if opts.DialTimeout != 5*time.Second {
		t.Errorf("got DialTimeout %v, want the default 5s silently applied for an unparseable duration", opts.DialTimeout)
	}
}

// TestExtractOverrideFromParams_RejectsMalformedFloat64Port locks in that
// paramInt validates a float64 before converting it - a NaN/Inf/fractional/
// out-of-range port (e.g. from a JSON-sourced params map, decoded as
// float64) falls back to the default rather than int(n)'s
// implementation-defined result.
func TestExtractOverrideFromParams_RejectsMalformedFloat64Port(t *testing.T) {
	cases := []struct {
		name string
		port float64
	}{
		{"NaN", math.NaN()},
		{"+Inf", math.Inf(1)},
		{"-Inf", math.Inf(-1)},
		{"fractional", 6380.5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opts := ExtractOverrideFromParams(map[string]interface{}{"redis": map[string]interface{}{"host": "x", "port": c.port}})
			if opts == nil {
				t.Fatal("expected a non-nil override")
			}
			if opts.Addr != "x:6379" {
				t.Errorf("got Addr %q, want the default port 6379 silently applied for malformed float64 port %v", opts.Addr, c.port)
			}
		})
	}
}
