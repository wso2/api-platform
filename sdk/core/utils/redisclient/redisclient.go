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

// Package redisclient shares one process-wide *redis.Client per distinct
// connection config across every caller - see GetOrCreate. It also exposes a
// gateway-wide default client (Shared, backed by the top-level "redis"
// config section) that a policy without its own Redis config falls back to
// - see Resolve.
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
	"strings"
	"sync"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/redis/go-redis/v9"
)

// redisConnKey identifies a distinct Redis connection config; identical
// settings share one *redis.Client. Excludes TLSConfig/credentials-provider
// options - see GetOrCreate's bypass for those.
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

// redisClients is the process-wide registry of shared Redis clients - without
// it, every policy instance/reload would open its own connection pool.
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

// GetOrCreate returns the process-wide shared client for these connection
// settings, creating (and pinging once) it on first use. created reports
// whether this call created the client; pingErr is non-nil only then. The
// client is registered even on ping failure (go-redis reconnects lazily) and
// is never closed - it lives for the process lifetime.
func GetOrCreate(opts *redis.Options, pingTimeout time.Duration) (client *redis.Client, created bool, pingErr error) {
	// TLSConfig/credentials-provider can't be fingerprinted safely (a
	// *tls.Config pointer says nothing about content; func values aren't
	// comparable) - bypass the registry rather than risk reusing a client
	// built for a different config.
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

	// Lock guards only the map lookup/insert, never the ping below - holding
	// it during a slow ping would stall every other caller's get-or-create.
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

// pingTimeoutMargin is added on top of a client's own dial/read/write
// timeouts to derive the one-time creation ping's timeout - room for the
// ping's own round-trip on top of the connection attempt itself.
const pingTimeoutMargin = 2 * time.Second

// pingTimeoutFor derives the creation-ping timeout from opts's own timeouts,
// so a fixed constant can't cut the ping short before a legitimately slow
// connection attempt completes.
func pingTimeoutFor(opts *redis.Options) time.Duration {
	return opts.DialTimeout + opts.ReadTimeout + opts.WriteTimeout + pingTimeoutMargin
}

// shared holds the process-wide gateway-level default client. inited
// distinguishes "InitFromConfig ran, no redis section" (client nil, inited
// true) from "InitFromConfig never called" (a wiring bug) - Shared reports
// each distinctly.
var shared struct {
	mu     sync.Mutex
	client *redis.Client
	inited bool
}

// InitFromConfig resolves the top-level "redis" section from raw (e.g.
// cfg.PolicyEngine.RawConfig) and creates the process-wide shared client -
// gateway-wide infrastructure, deliberately not nested under
// "policy_configurations" (policy-engine's per-policy ${config...} namespace).
// Must be called exactly once, at startup, before any policy factory runs -
// see Shared/Resolve. A missing "redis" key or a failed ping is not fatal
// here; both only matter lazily, the first time a policy calls Shared.
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
// the top-level "redis" config section. Errors if InitFromConfig was never
// called, or if no "redis" section was configured - callers must treat the
// latter as a real config gap, not assume a shared Redis always exists.
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
// settings if the policy configured its own connection, otherwise the
// gateway-level Shared client. opts must be nil - never a schema-defaulted
// zero-value *redis.Options - when the policy's own section was absent, or
// this fallback never triggers; that presence check is the caller's own.
func Resolve(opts *redis.Options, pingTimeout time.Duration) (*redis.Client, error) {
	if opts == nil {
		return Shared()
	}
	client, _, _ := GetOrCreate(opts, pingTimeout)
	return client, nil
}

// newAndPingClient creates a client and pings it once. created is always
// true - only present so this matches GetOrCreate's own return
// shape at its call sites.
func newAndPingClient(opts *redis.Options, pingTimeout time.Duration) (client *redis.Client, created bool, pingErr error) {
	c := redis.NewClient(opts)
	return c, true, pingClient(c, pingTimeout)
}

// pingClient pings an already-constructed client once, bounded by
// pingTimeout. Split out since GetOrCreate's main path must insert the
// client into the registry BEFORE pinging, not create-then-ping atomically.
func pingClient(c *redis.Client, pingTimeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	return c.Ping(ctx).Err()
}

// redisSectionFields is the decode target for the "redis" table, pre-filled
// with defaults before mapstructure.Decode overwrites only the keys present
// in the config - the same weakly-typed decode every other config section
// goes through via koanf/mapstructure (see policy-engine's
// internal/config.Load), just without pulling koanf itself into sdk/core.
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
// top-level section, not nested under "policy_configurations" (this is
// gateway-wide infrastructure, not a per-policy setting). Returns (nil, nil)
// when raw has no "redis" key at all.
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
	if err := validateRedisSectionFields(fields); err != nil {
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

// minRedisTimeout is the smallest connection/read/write timeout accepted.
// go-redis treats a negative ReadTimeout/WriteTimeout (-1/-2) as "disable
// timeout enforcement entirely" rather than an error, and time.Duration's
// underlying kind is int64 (not caught by rejectNonIntegralFloatHookFunc's
// int-only guard), so a bare numeric config value like connection_timeout = 5
// would otherwise decode as 5 nanoseconds. Rejecting anything below 1ms
// catches both cases with one check.
const minRedisTimeout = time.Millisecond

// validateRedisSectionFields rejects decoded values that would otherwise
// silently produce an unsafe or unusable *redis.Options.
func validateRedisSectionFields(f redisSectionFields) error {
	if f.Port < 1 || f.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", f.Port)
	}
	if f.DB < 0 {
		return fmt.Errorf("db must not be negative, got %d", f.DB)
	}
	if f.PoolSize < 0 {
		return fmt.Errorf("pool_size must not be negative, got %d", f.PoolSize)
	}
	if f.ConnectionTimeout < minRedisTimeout {
		return fmt.Errorf("connection_timeout must be at least %s, got %s", minRedisTimeout, f.ConnectionTimeout)
	}
	if f.ReadTimeout < minRedisTimeout {
		return fmt.Errorf("read_timeout must be at least %s, got %s", minRedisTimeout, f.ReadTimeout)
	}
	if f.WriteTimeout < minRedisTimeout {
		return fmt.Errorf("write_timeout must be at least %s, got %s", minRedisTimeout, f.WriteTimeout)
	}
	return nil
}

// rejectNonIntegralFloatHookFunc runs before mapstructure's own weakly-typed
// float->int conversion, which is undefined/lossy for NaN, +-Inf, a
// fractional value, or an out-of-range magnitude - reject those instead of
// letting them silently truncate into a bogus port/db/pool size.
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

// ExtractOverrideFromParams reads systemParameters.redis.* from a policy's
// own params into a *redis.Options for that connection. Returns nil when
// redis.host is absent - unlike other fields, host has no default, or the
// gateway-wide fallback (Resolve/Shared) would never trigger. A malformed
// field silently falls back to its default rather than erroring, unlike
// resolveOptionsFromConfig's stricter decode of operator config. Dotted-key
// lookups tolerate both a flattened key (params["redis.host"]) and a nested
// map (params["redis"]["host"]).
func ExtractOverrideFromParams(params map[string]interface{}) *redis.Options {
	host := paramString(params, "redis.host", "")
	if host == "" {
		return nil
	}
	return &redis.Options{
		Addr:         net.JoinHostPort(host, strconv.Itoa(paramInt(params, "redis.port", 6379))),
		Username:     paramString(params, "redis.username", ""),
		Password:     paramString(params, "redis.password", ""),
		DB:           paramInt(params, "redis.db", 0),
		DialTimeout:  paramDuration(params, "redis.connectionTimeout", 5*time.Second),
		ReadTimeout:  paramDuration(params, "redis.readTimeout", 3*time.Second),
		WriteTimeout: paramDuration(params, "redis.writeTimeout", 3*time.Second),
		PoolSize:     paramInt(params, "redis.poolSize", 0),
	}
}

// paramLookup resolves a dotted key ("redis.host") against params, tolerating
// either a flattened key (params["redis.host"]) or nested maps
// (params["redis"]["host"]).
func paramLookup(params map[string]interface{}, dottedKey string) (interface{}, bool) {
	if v, ok := params[dottedKey]; ok {
		return v, true
	}
	var cur interface{} = params
	for _, part := range strings.Split(dottedKey, ".") {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil, false
		}
		v, ok := m[part]
		if !ok {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

func paramString(params map[string]interface{}, dottedKey, def string) string {
	if v, ok := paramLookup(params, dottedKey); ok {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return def
}

func paramInt(params map[string]interface{}, dottedKey string, def int) int {
	if v, ok := paramLookup(params, dottedKey); ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			if !math.IsNaN(n) && !math.IsInf(n, 0) && n == math.Trunc(n) &&
				n >= float64(math.MinInt) && n <= float64(math.MaxInt) {
				return int(n)
			}
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
				return parsed
			}
		}
	}
	return def
}

func paramDuration(params map[string]interface{}, dottedKey string, def time.Duration) time.Duration {
	if v, ok := paramLookup(params, dottedKey); ok {
		if s, ok := v.(string); ok {
			if d, err := time.ParseDuration(strings.TrimSpace(s)); err == nil {
				return d
			}
		}
	}
	return def
}
