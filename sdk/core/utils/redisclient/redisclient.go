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
// pool) per distinct connection configuration, across every caller that
// imports it - see GetOrCreateRedisClient. It also exposes a single
// gateway-wide default client (Shared, backed by the operator's top-level
// "redis" config section - gateway infrastructure, not something scoped to
// policies) that a policy falls back to when it has no Redis config of its
// own - see Resolve.
package redisclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"net"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/go-viper/mapstructure/v2"
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
		return newAndPingClient(opts, pingTimeout)
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

	pingErr = pingClient(c, pingTimeout)
	return c, true, pingErr
}

// pingTimeoutMargin is added on top of a client's own configured dial/read/
// write timeouts to derive the one-time creation ping's timeout (see
// pingTimeoutFor) - enough for the ping's own command round-trip on top of
// whatever the connection attempt itself is allowed to take.
const pingTimeoutMargin = 2 * time.Second

// pingTimeoutFor derives the creation-ping timeout from opts's own configured
// timeouts, so the ping's context stays alive at least as long as the
// connection attempt permitted by DialTimeout (plus the read/write round-trip
// and a safety margin) - a fixed constant shorter than an operator's
// configured DialTimeout would cut the ping's context before a legitimately
// slow-but-successful connection attempt could complete.
func pingTimeoutFor(opts *redis.Options) time.Duration {
	return opts.DialTimeout + opts.ReadTimeout + opts.WriteTimeout + pingTimeoutMargin
}

// shared holds the process-wide gateway-level default client. inited
// distinguishes "InitFromConfig ran and found no redis section" (client nil,
// inited true - Shared reports a config-gap error) from "InitFromConfig was
// never called at all" (a gateway-runtime wiring bug - Shared reports that
// distinctly, since it means something programming-level is missing, not an
// operator config gap).
var shared struct {
	mu     sync.Mutex
	client *redis.Client
	inited bool
}

// InitFromConfig resolves the operator-level top-level "redis" section from
// raw (e.g. cfg.PolicyEngine.RawConfig - raw's other top-level sections like
// "analytics"/"router"/"policy_configurations" are ignored here) and creates
// the process-wide shared client. This is gateway-wide infrastructure, not
// something scoped to policies - deliberately NOT nested under
// "policy_configurations" (that namespace is policy-engine's own ${config...}
// CEL-resolution mechanism for per-policy system parameters; a shared
// resource other gateway components could reach doesn't belong inside it).
// Must be called exactly once, at gateway-runtime startup, before any policy
// factory runs - see Shared and Resolve. A missing "redis" key is not an
// error: most gateways may have zero Redis-consuming policies configured,
// and that absence only matters lazily, the first time some policy actually
// calls Shared. A connection/ping failure is likewise not fatal here - the
// client is still created and stored (go-redis reconnects lazily), matching
// GetOrCreateRedisClient's own create-time philosophy.
func InitFromConfig(raw map[string]interface{}) error {
	shared.mu.Lock()
	defer shared.mu.Unlock()
	if shared.inited {
		return fmt.Errorf("redisclient: InitFromConfig called more than once")
	}
	shared.inited = true

	opts, err := resolveOptionsFromConfig(raw)
	if err != nil {
		return fmt.Errorf("redisclient: invalid \"redis\" config: %w", err)
	}
	if opts == nil {
		return nil
	}

	c, _, _ := newAndPingClient(opts, pingTimeoutFor(opts))
	shared.client = c
	return nil
}

// Shared returns the process-wide gateway-level default client, backed by
// the top-level "redis" config section. It errors if InitFromConfig was
// never called (a gateway-runtime wiring bug, not a normal runtime
// condition) or if no "redis" section was configured at all - callers must
// treat the latter as a real configuration gap rather than assuming a
// shared Redis is always available.
func Shared() (*redis.Client, error) {
	shared.mu.Lock()
	defer shared.mu.Unlock()
	if !shared.inited {
		return nil, fmt.Errorf("redisclient: Shared() called before InitFromConfig")
	}
	if shared.client == nil {
		return nil, fmt.Errorf(`redisclient: no shared redis configured ("redis" section)`)
	}
	return shared.client, nil
}

// Resolve returns the client a policy instance should use: opts's own
// connection settings when the policy resolved one from its own config
// section, otherwise the gateway-level Shared client.
//
// opts must be nil - never a zero-value *redis.Options - when the policy's
// own section was absent. A struct pre-filled with the policy's schema
// defaults would always look "configured," and this fallback would never
// trigger; the presence check belongs to the caller's own config-extraction
// code, on whichever field has no default (e.g. host).
func Resolve(opts *redis.Options, pingTimeout time.Duration) (*redis.Client, error) {
	if opts == nil {
		return Shared()
	}
	client, _, _ := GetOrCreateRedisClient(opts, pingTimeout)
	return client, nil
}

// newAndPingClient creates a client and pings it once. created is always
// true - only present so this matches GetOrCreateRedisClient's own return
// shape at its call sites.
func newAndPingClient(opts *redis.Options, pingTimeout time.Duration) (client *redis.Client, created bool, pingErr error) {
	c := redis.NewClient(opts)
	return c, true, pingClient(c, pingTimeout)
}

// pingClient pings an already-constructed client once, bounded by
// pingTimeout. Split out from newAndPingClient because GetOrCreateRedisClient's
// main path must insert the client into the registry BEFORE pinging (so a
// concurrent caller for the same key sees it immediately), not create-then-ping
// as one atomic step.
func pingClient(c *redis.Client, pingTimeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	return c.Ping(ctx).Err()
}

// redisSectionFields is the decode target for the "redis" table, pre-filled
// with operator-facing defaults (host "localhost", port 6379, db 0, poolSize
// 0 (go-redis default), connectionTimeout 5s, readTimeout/writeTimeout 3s)
// before mapstructure.Decode overwrites only the keys actually present in
// the config - the same weakly-typed decode (string durations, a numeric
// string from a {{ env }} token, int/int64/float64 from a JSON-sourced
// config path) every other config section in the repo goes through via
// koanf/mapstructure; see gateway-runtime/policy-engine's
// internal/config.Load. sdk/core otherwise has no koanf dependency, so this
// package decodes the raw map directly with mapstructure rather than pulling
// in koanf's file/merge machinery just for this one section.
type redisSectionFields struct {
	Host              string        `mapstructure:"host"`
	Port              int           `mapstructure:"port"`
	Username          string        `mapstructure:"username"`
	Password          string        `mapstructure:"password"`
	DB                int           `mapstructure:"db"`
	PoolSize          int           `mapstructure:"pool_size"`
	ConnectionTimeout time.Duration `mapstructure:"connection_timeout"`
	ReadTimeout       time.Duration `mapstructure:"read_timeout"`
	WriteTimeout      time.Duration `mapstructure:"write_timeout"`
}

// resolveOptionsFromConfig extracts *redis.Options from raw["redis"] - a
// top-level section, sibling to "router"/"analytics"/etc in the gateway's
// complete config tree, not nested under "policy_configurations" (this is
// gateway-wide infrastructure, not a per-policy setting). Returns (nil, nil)
// - not an error - when raw has no "redis" key at all.
func resolveOptionsFromConfig(raw map[string]interface{}) (*redis.Options, error) {
	section, ok := raw["redis"]
	if !ok || section == nil {
		return nil, nil
	}
	m, ok := section.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf(`"redis" must be a table, got %T`, section)
	}

	fields := redisSectionFields{
		Host:              "localhost",
		Port:              6379,
		ConnectionTimeout: 5 * time.Second,
		ReadTimeout:       3 * time.Second,
		WriteTimeout:      3 * time.Second,
	}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			rejectNonIntegralFloatHookFunc,
			mapstructure.StringToTimeDurationHookFunc(),
		),
		Result: &fields,
	})
	if err != nil {
		return nil, fmt.Errorf("building redis config decoder: %w", err)
	}
	if err := decoder.Decode(m); err != nil {
		return nil, err
	}

	return &redis.Options{
		Addr:         net.JoinHostPort(fields.Host, strconv.Itoa(fields.Port)),
		Username:     fields.Username,
		Password:     fields.Password,
		DB:           fields.DB,
		DialTimeout:  fields.ConnectionTimeout,
		ReadTimeout:  fields.ReadTimeout,
		WriteTimeout: fields.WriteTimeout,
		PoolSize:     fields.PoolSize,
	}, nil
}

// rejectNonIntegralFloatHookFunc runs before mapstructure's own weakly-typed
// float->int conversion (int64(dataVal.Float()), reached for e.g. a
// JSON-sourced "port"/"db"/"pool_size" value decoded as float64) - that
// conversion is undefined/lossy for NaN, +-Inf, a fractional value, or a
// magnitude outside the int range, so reject those here instead of letting
// them silently truncate into a bogus port/db/pool size.
func rejectNonIntegralFloatHookFunc(_, to reflect.Kind, data interface{}) (interface{}, error) {
	if to != reflect.Int {
		return data, nil
	}
	f, ok := data.(float64)
	if !ok {
		return data, nil
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return nil, fmt.Errorf("expected an integer, got %v", f)
	}
	if f != math.Trunc(f) {
		return nil, fmt.Errorf("expected an integer, got non-integer value %v", f)
	}
	if f < float64(math.MinInt) || f > float64(math.MaxInt) {
		return nil, fmt.Errorf("value %v out of range for int", f)
	}
	return data, nil
}
