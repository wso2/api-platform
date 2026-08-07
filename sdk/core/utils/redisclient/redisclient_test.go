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

	// Two "different policies" (distinct call sites, distinct *redis.Options
	// values) with identical connection settings must still land on the
	// same underlying client - this is the whole point of centralizing the
	// registry here instead of each policy keeping its own.
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

// TestGetOrCreateClient_ReuseSkipsPing locks in that only CREATION pings -
// a reused client is assumed healthy (go-redis reconnects lazily) and must
// never be re-pinged, or a client that legitimately reused a pool would
// spuriously start reporting errors the moment Redis blips after creation.
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

func TestHashPassword(t *testing.T) {
	if hashRedisPassword("") != "" {
		t.Error("expected an empty password to hash to empty, not sha256(\"\")")
	}
	if hashRedisPassword("secret") == "secret" {
		t.Error("expected the password to actually be hashed, not passed through")
	}
	if hashRedisPassword("secret") != hashRedisPassword("secret") {
		t.Error("expected hashing to be deterministic")
	}
	if hashRedisPassword("secret") == hashRedisPassword("different") {
		t.Error("expected different passwords to hash differently")
	}
}
