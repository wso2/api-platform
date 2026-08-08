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
	"net"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestGetOrCreateClient_SharesClientForIdenticalConfig(t *testing.T) {
	mr := miniredis.RunT(t)
	opts := &redis.Options{Addr: mr.Addr(), DB: 0}

	c1, created1, err1 := GetOrCreateRedisClient(opts, time.Second)
	if !created1 || err1 != nil {
		t.Fatalf("first call: created=%v err=%v (want true,nil)", created1, err1)
	}

	c2, created2, err2 := GetOrCreateRedisClient(opts, time.Second)
	if created2 || err2 != nil {
		t.Fatalf("second call: created=%v err=%v (want false,nil)", created2, err2)
	}
	if c1 != c2 {
		t.Error("expected identical connection settings to share one *redis.Client")
	}
}

func TestGetOrCreateClient_DistinctClientForDifferentConfig(t *testing.T) {
	mr := miniredis.RunT(t)

	c1, _, _ := GetOrCreateRedisClient(&redis.Options{Addr: mr.Addr(), DB: 0}, time.Second)
	c2, _, _ := GetOrCreateRedisClient(&redis.Options{Addr: mr.Addr(), DB: 1}, time.Second)

	if c1 == c2 {
		t.Error("expected different DB selection to produce a distinct *redis.Client")
	}
}

func TestGetOrCreateClient_DifferentPasswordProducesDistinctClient(t *testing.T) {
	mr := miniredis.RunT(t)

	c1, _, _ := GetOrCreateRedisClient(&redis.Options{Addr: mr.Addr(), Password: "one"}, time.Second)
	c2, _, _ := GetOrCreateRedisClient(&redis.Options{Addr: mr.Addr(), Password: "two"}, time.Second)
	c3, _, _ := GetOrCreateRedisClient(&redis.Options{Addr: mr.Addr()}, time.Second) // no password at all

	if c1 == c2 {
		t.Error("expected different passwords to produce distinct clients")
	}
	if c1 == c3 || c2 == c3 {
		t.Error("expected an absent password not to collide with a present one")
	}
}

func TestGetOrCreateClient_SharedAcrossSimulatedPolicies(t *testing.T) {
	mr := miniredis.RunT(t)
	opts := func() *redis.Options { return &redis.Options{Addr: mr.Addr(), DB: 0} }

	// Two distinct call sites with identical settings must share one
	// client - the whole point of centralizing the registry.
	fromPolicyA, _, _ := GetOrCreateRedisClient(opts(), time.Second)
	fromPolicyB, _, _ := GetOrCreateRedisClient(opts(), time.Second)

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
	mr := miniredis.RunT(t)
	addr := mr.Addr() // capture before mr.Close() below
	opts := &redis.Options{Addr: addr, DB: 0}

	c1, created1, err1 := GetOrCreateRedisClient(opts, time.Second)
	if !created1 || err1 != nil {
		t.Fatalf("first call: created=%v err=%v (want true,nil)", created1, err1)
	}

	mr.Close()
	c2, created2, err2 := GetOrCreateRedisClient(opts, time.Second)
	if created2 || err2 != nil || c2 != c1 {
		t.Fatalf("reuse after Redis went down should skip the ping: created=%v err=%v same=%v", created2, err2, c2 == c1)
	}
}

func TestGetOrCreateClient_DifferentProtocolProducesDistinctClient(t *testing.T) {
	mr := miniredis.RunT(t)

	c1, _, _ := GetOrCreateRedisClient(&redis.Options{Addr: mr.Addr(), Protocol: 2}, time.Second)
	c2, _, _ := GetOrCreateRedisClient(&redis.Options{Addr: mr.Addr(), Protocol: 3}, time.Second)
	c3, _, _ := GetOrCreateRedisClient(&redis.Options{Addr: mr.Addr(), Protocol: 2}, time.Second)

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
	mr := miniredis.RunT(t)

	optsA := &redis.Options{Addr: mr.Addr(), TLSConfig: &tls.Config{}} //nolint:gosec // test-only, no real handshake asserted
	optsB := &redis.Options{Addr: mr.Addr(), TLSConfig: &tls.Config{}} //nolint:gosec

	c1, created1, _ := GetOrCreateRedisClient(optsA, time.Second)
	c2, created2, _ := GetOrCreateRedisClient(optsB, time.Second)

	if !created1 || !created2 {
		t.Fatalf("expected every TLSConfig-bearing call to report created=true (never reused), got %v and %v", created1, created2)
	}
	if c1 == c2 {
		t.Error("expected two TLSConfig-bearing calls to never share a client, even with identical-looking options")
	}
}

func TestGetOrCreateClient_CredentialsProviderBypassesRegistry(t *testing.T) {
	mr := miniredis.RunT(t)
	provider := func() (string, string) { return "", "" }

	c1, created1, err1 := GetOrCreateRedisClient(&redis.Options{Addr: mr.Addr(), CredentialsProvider: provider}, time.Second)
	c2, created2, err2 := GetOrCreateRedisClient(&redis.Options{Addr: mr.Addr(), CredentialsProvider: provider}, time.Second)

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
	// Accepts but never responds, so Ping against it blocks until the
	// deadline - a reliable window to prove a concurrent, unrelated key
	// isn't blocked by it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
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
		_, _, _ = GetOrCreateRedisClient(&redis.Options{
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
	if _, _, err := GetOrCreateRedisClient(&redis.Options{Addr: mr.Addr(), DB: 1}, 500*time.Millisecond); err != nil {
		t.Fatalf("unexpected error on the fast, unrelated key: %v", err)
	}
	if elapsed := time.Since(fastStart); elapsed > 300*time.Millisecond {
		t.Errorf("expected the unrelated key's get-or-create to complete quickly (the registry lock must not be held during the other call's ping), took %s", elapsed)
	}

	<-done // let the slow goroutine finish before the test exits
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
