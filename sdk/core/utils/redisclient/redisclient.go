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

// Package redisclient shares one process-wide *redis.Client (one connection
// pool) per distinct connection configuration, across every Redis-using
// policy that imports it - see GetOrCreateRedisClient.
package redisclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisConnKey identifies a distinct Redis connection configuration. Two policy
// instances with identical connection settings share one *redis.Client (one pool).
//
// Excludes TLSConfig and any credentials-provider option - see
// GetOrCreateRedisClient's bypass for those.
type redisConnKey struct {
	addr         string
	username     string
	passwordHash string // sha256 hex; keeps the secret out of the in-process map key
	db           int
	protocol     int
	dialTimeout  time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration
	poolSize     int
}

// redisClients is the process-wide registry of shared Redis clients. Without it,
// GetPolicy creates a new *redis.Client (a whole connection pool) per policy instance
// and per config reload, leaking pools and exploding Redis connections at scale.
var redisClients = struct {
	mu sync.Mutex
	m  map[redisConnKey]*redis.Client
}{m: make(map[redisConnKey]*redis.Client)}

func hashRedisPassword(p string) string {
	if p == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(p))
	return hex.EncodeToString(sum[:])
}

// GetOrCreateRedisClient returns the process-wide shared client for these connection
// settings, creating (and pinging once) it on first use. created reports whether this
// call created the client; pingErr is non-nil only when created and the initial ping
// failed. The client is registered and returned even on ping failure (go-redis
// reconnects lazily). Clients are never closed — they live for the process lifetime.
func GetOrCreateRedisClient(opts *redis.Options, pingTimeout time.Duration) (client *redis.Client, created bool, pingErr error) {
	// TLSConfig and credentials-provider hooks can't be fingerprinted
	// safely: a *tls.Config's pointer says nothing about its content, and
	// Go func values aren't comparable at all. Bypass the registry rather
	// than risk silently reusing a client built for a different config.
	if opts.TLSConfig != nil || opts.CredentialsProvider != nil || opts.CredentialsProviderContext != nil || opts.StreamingCredentialsProvider != nil {
		c := redis.NewClient(opts)
		ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
		defer cancel()
		pingErr = c.Ping(ctx).Err()
		return c, true, pingErr
	}

	key := redisConnKey{
		addr:         opts.Addr,
		username:     opts.Username,
		passwordHash: hashRedisPassword(opts.Password),
		db:           opts.DB,
		protocol:     opts.Protocol,
		dialTimeout:  opts.DialTimeout,
		readTimeout:  opts.ReadTimeout,
		writeTimeout: opts.WriteTimeout,
		poolSize:     opts.PoolSize,
	}

	// Lock guards only the map lookup/insert, never the ping below - mu is
	// process-wide, so holding it during a slow/down connection's ping
	// would stall every other caller's get-or-create too. A concurrent
	// caller for the same key may see the just-inserted client before this
	// ping finishes - fine, since a reused client is already "assumed
	// healthy" regardless of timing, never gated on this call's pingErr.
	redisClients.mu.Lock()
	if c, ok := redisClients.m[key]; ok {
		redisClients.mu.Unlock()
		return c, false, nil
	}
	c := redis.NewClient(opts)
	redisClients.m[key] = c
	redisClients.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	pingErr = c.Ping(ctx).Err()
	return c, true, pingErr
}
