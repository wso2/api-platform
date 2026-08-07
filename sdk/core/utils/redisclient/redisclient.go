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
type redisConnKey struct {
	addr         string
	username     string
	passwordHash string // sha256 hex; keeps the secret out of the in-process map key
	db           int
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
	key := redisConnKey{
		addr:         opts.Addr,
		username:     opts.Username,
		passwordHash: hashRedisPassword(opts.Password),
		db:           opts.DB,
		dialTimeout:  opts.DialTimeout,
		readTimeout:  opts.ReadTimeout,
		writeTimeout: opts.WriteTimeout,
		poolSize:     opts.PoolSize,
	}

	redisClients.mu.Lock()
	defer redisClients.mu.Unlock()

	if c, ok := redisClients.m[key]; ok {
		return c, false, nil
	}

	c := redis.NewClient(opts)
	redisClients.m[key] = c

	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	pingErr = c.Ping(ctx).Err()
	return c, true, pingErr
}
