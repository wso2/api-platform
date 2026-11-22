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

package counters

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
)

// mockRedis is an in-memory mock for the Redis backend: it applies syncScript's rules
// (a report's adds once, under its marker; counters expire after the retention) without a
// Redis server. down makes every call fail, as an unreachable Redis does.
type mockRedis struct {
	mu      sync.Mutex
	values  map[string]string
	expires map[string]time.Time
	now     time.Time
	down    bool
}

func newMockRedis() *mockRedis {
	return &mockRedis{values: map[string]string{}, expires: map[string]time.Time{}, now: time.Unix(1_800_000_000, 0)}
}

var errMockRedisDown = errors.New("connection refused")

// setDown takes Redis away (true) or brings it back with its data (false).
func (f *mockRedis) setDown(down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down = down
}

// advance moves the mock's clock, expiring keys.
func (f *mockRedis) advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

// ttl is how long key has left, or 0 when it has no expiry or does not exist.
func (f *mockRedis) ttl(key string) time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	if at, ok := f.expires[key]; ok {
		return at.Sub(f.now)
	}
	return 0
}

// get returns key's value, dropping it when expired. Callers hold mu.
func (f *mockRedis) get(key string) (string, bool) {
	if at, ok := f.expires[key]; ok && !f.now.Before(at) {
		delete(f.values, key)
		delete(f.expires, key)
	}
	v, ok := f.values[key]
	return v, ok
}

func (f *mockRedis) sync(_ context.Context, keys []string, args []any) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		return nil, errMockRedisDown
	}
	retention := time.Duration(toInt(args[0])) * time.Second
	markerTTL := time.Duration(toInt(args[1])) * time.Second
	n := toInt(args[2])
	if _, seen := f.get(keys[0]); n > 0 && !seen {
		f.values[keys[0]] = "1"
		f.expires[keys[0]] = f.now.Add(markerTTL)
		for i := 1; i <= n; i++ {
			current, _ := f.get(keys[i])
			total, _ := strconv.ParseFloat(orZero(current), 64)
			delta, _ := strconv.ParseFloat(args[2+i].(string), 64)
			f.values[keys[i]] = strconv.FormatFloat(total+delta, 'f', -1, 64)
			f.expires[keys[i]] = f.now.Add(retention)
		}
	}
	out := make([]string, 0, len(keys)-n-1)
	for _, k := range keys[n+1:] {
		v, _ := f.get(k)
		out = append(out, orZero(v))
	}
	return out, nil
}

func (f *mockRedis) scan(_ context.Context, pattern string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		return nil, errMockRedisDown
	}
	// The store only scans for "<escaped literal prefix>*".
	prefix := globUnescaper.Replace(strings.TrimSuffix(pattern, "*"))
	var keys []string
	for k := range f.values {
		if _, ok := f.get(k); ok && strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

var globUnescaper = strings.NewReplacer(`\\`, `\`, `\*`, "*", `\?`, "?", `\[`, "[", `\]`, "]")

func (f *mockRedis) mget(_ context.Context, keys ...string) ([]any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		return nil, errMockRedisDown
	}
	out := make([]any, len(keys))
	for i, k := range keys {
		if v, ok := f.get(k); ok {
			out[i] = v
		}
	}
	return out, nil
}

func (f *mockRedis) unlink(_ context.Context, keys ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		return errMockRedisDown
	}
	for _, k := range keys {
		delete(f.values, k)
		delete(f.expires, k)
	}
	return nil
}

func (f *mockRedis) ping(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		return errMockRedisDown
	}
	return nil
}

func (f *mockRedis) close() error { return nil }

func toInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	}
	return 0
}

func orZero(v string) string {
	if v == "" {
		return "0"
	}
	return v
}
