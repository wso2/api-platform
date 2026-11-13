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
	"crypto/tls"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/wso2/api-platform/gateway/common/counterproto"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

// syncScript applies a report once and reads totals, in one round trip.
//
// KEYS[1] is the report marker (empty when there are no adds), KEYS[2..n+1] the
// counters added to and KEYS[n+2..] the counters read. ARGV[1] is the retention in
// seconds, ARGV[2] the marker's lifetime in seconds, ARGV[3] the number of adds n and
// ARGV[4..] their deltas.
var syncScript = redis.NewScript(`
local n = tonumber(ARGV[3])
if n > 0 and redis.call('SET', KEYS[1], '1', 'NX', 'EX', tonumber(ARGV[2])) then
  for i = 1, n do
    redis.call('INCRBYFLOAT', KEYS[1 + i], ARGV[3 + i])
    redis.call('EXPIRE', KEYS[1 + i], tonumber(ARGV[1]))
  end
end
local out = {}
for j = n + 2, #KEYS do
  out[#out + 1] = redis.call('GET', KEYS[j]) or '0'
end
return out
`)

// Redis is a Store in Redis (or Valkey), shared by every controller using it.
//
// Every key carries the same hash tag, so a report's keys share one cluster slot and
// the script runs on one node.
type Redis struct {
	client    *redis.Client
	prefix    string
	retention time.Duration
}

// NewRedis connects to Redis.
func NewRedis(cfg config.CountersRedisConfig, retention time.Duration) (*Redis, error) {
	if retention <= 0 {
		retention = DefaultRetention
	}
	opts := &redis.Options{
		Addr:        cfg.Address,
		Username:    cfg.Username,
		Password:    cfg.Password,
		DB:          cfg.DB,
		DialTimeout: cfg.DialTimeout,
	}
	if cfg.TLS {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout+time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connecting to the counter store at %s: %w", cfg.Address, err)
	}
	return newRedisWithClient(client, cfg.KeyPrefix, retention), nil
}

func newRedisWithClient(client *redis.Client, prefix string, retention time.Duration) *Redis {
	return &Redis{client: client, prefix: prefix, retention: retention}
}

// key is the Redis key of a counter.
func (r *Redis) key(k counterproto.Key) string {
	return r.prefix + "{c}:" + escape(k.Namespace) + ":" + escape(k.ID) + ":" + strconv.FormatInt(k.Window, 10)
}

// escape keeps a key part from running into the next.
func escape(s string) string {
	return strings.NewReplacer("%", "%25", ":", "%3A").Replace(s)
}

// Sync applies adds once per (source, reportID) and returns the totals of keys.
func (r *Redis) Sync(ctx context.Context, source, reportID string, adds []counterproto.Add, keys []counterproto.Key) ([]counterproto.Total, error) {
	marker := r.prefix + "{c}:report:" + escape(source) + ":" + escape(reportID)
	scriptKeys := make([]string, 0, 1+len(adds)+len(keys))
	scriptKeys = append(scriptKeys, marker)
	args := make([]any, 0, 3+len(adds))
	args = append(args, int64(r.retention.Seconds()), int64(appliedReportTTL.Seconds()), len(adds))
	for _, add := range adds {
		scriptKeys = append(scriptKeys, r.key(add.Key))
		args = append(args, strconv.FormatFloat(add.Delta, 'f', -1, 64))
	}
	for _, k := range keys {
		scriptKeys = append(scriptKeys, r.key(k))
	}
	raw, err := syncScript.Run(ctx, r.client, scriptKeys, args...).StringSlice()
	if err != nil {
		return nil, fmt.Errorf("counter store: %w", err)
	}
	if len(raw) != len(keys) {
		return nil, fmt.Errorf("counter store returned %d totals for %d keys", len(raw), len(keys))
	}
	totals := make([]counterproto.Total, len(keys))
	for i, k := range keys {
		total, err := strconv.ParseFloat(raw[i], 64)
		if err != nil {
			return nil, fmt.Errorf("counter store: total of %v is %q: %w", k, raw[i], err)
		}
		totals[i] = counterproto.Total{Key: k, Total: total}
	}
	return totals, nil
}

// Close closes the connection.
func (r *Redis) Close() error { return r.client.Close() }
