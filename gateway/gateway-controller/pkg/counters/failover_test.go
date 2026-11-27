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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/common/counterproto"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

var testRetry = config.CountersRedisRetryConfig{InitialDuration: time.Second, ProgressionFactor: 2, MaxDuration: 5 * time.Second}

var spendKey = counterproto.Key{Namespace: "budget", ID: "project-x-monthly", Window: november.Unix()}

// failoverHarness drives a Failover by hand: each reconnect attempt waits on next, and
// the waits it asked for are recorded.
type failoverHarness struct {
	f     *Failover
	redis *mockRedis
	next  chan bool
	mu    sync.Mutex
	waits []time.Duration
}

// newFailoverHarness starts a Failover on an in-memory Redis. reachable says whether connecting
// works; a test flips it to bring Redis back.
func newFailoverHarness(t *testing.T, reachable *atomic.Bool) *failoverHarness {
	t.Helper()
	h := &failoverHarness{redis: newMockRedis(), next: make(chan bool)}
	connect := func(context.Context) (*Redis, error) {
		if !reachable.Load() {
			return nil, errors.New("connection refused")
		}
		return newRedisWithBackend(h.redis, "test:", time.Hour), nil
	}
	h.f = newFailover(connect, testRetry, time.Hour, nil)
	h.f.sleep = func(d time.Duration, stop <-chan struct{}) bool {
		h.mu.Lock()
		h.waits = append(h.waits, d)
		h.mu.Unlock()
		select {
		case go_ := <-h.next:
			return go_
		case <-stop:
			return false
		}
	}
	t.Cleanup(func() { _ = h.f.Close() })
	h.f.start()
	return h
}

func (h *failoverHarness) sync(t *testing.T, reportID string, delta float64) float64 {
	t.Helper()
	var adds []counterproto.Add
	if delta != 0 {
		adds = []counterproto.Add{{Key: spendKey, Delta: delta}}
	}
	totals, err := h.f.Sync(context.Background(), "engine-1", reportID, adds, []counterproto.Key{spendKey})
	require.NoError(t, err)
	require.Len(t, totals, 1)
	return totals[0].Total
}

func (h *failoverHarness) isDown() bool {
	h.f.mu.Lock()
	defer h.f.mu.Unlock()
	return h.f.down
}

// redisTotal reads the total straight from Redis.
func (h *failoverHarness) redisTotal(t *testing.T) float64 {
	t.Helper()
	h.redis.setDown(false) // read even while the store is taken away
	r := newRedisWithBackend(h.redis, "test:", time.Hour)
	totals, err := r.Sync(context.Background(), "reader", "", nil, []counterproto.Key{spendKey})
	require.NoError(t, err)
	return totals[0].Total
}

// A controller whose Redis cannot be reached at start still starts: it counts in memory
// and, once Redis can be reached, adds what it counted.
func TestFailoverStartsInMemoryWhenRedisIsAway(t *testing.T) {
	var reachable atomic.Bool
	h := newFailoverHarness(t, &reachable)
	require.True(t, h.isDown())

	assert.Equal(t, 10.0, h.sync(t, "r1", 10))
	assert.Equal(t, 15.0, h.sync(t, "r2", 5))

	h.next <- true // Redis still away
	require.Eventually(t, func() bool { h.mu.Lock(); defer h.mu.Unlock(); return len(h.waits) >= 2 }, time.Second, time.Millisecond)
	assert.True(t, h.isDown())

	reachable.Store(true)
	h.next <- true
	require.Eventually(t, func() bool { return !h.isDown() }, time.Second, time.Millisecond)

	assert.Equal(t, 15.0, h.redisTotal(t), "what was counted in memory is added to Redis")
	assert.Equal(t, 18.0, h.sync(t, "r3", 3), "and Redis is used again")
	assert.Equal(t, 18.0, h.redisTotal(t))
}

// When Redis goes away, counting goes on in memory from the last totals Redis returned,
// so the limit is not reset; when it is back, what was counted meanwhile is added once.
func TestFailoverKeepsCountingFromTheLastTotalsWhenRedisGoesAway(t *testing.T) {
	var reachable atomic.Bool
	reachable.Store(true)
	h := newFailoverHarness(t, &reachable)
	require.False(t, h.isDown())
	assert.Equal(t, 100.0, h.sync(t, "r1", 100))

	h.redis.setDown(true)
	reachable.Store(false)
	assert.Equal(t, 120.0, h.sync(t, "r2", 20), "counted from the last Redis total")
	require.True(t, h.isDown())
	assert.Equal(t, 125.0, h.sync(t, "r3", 5))
	assert.Equal(t, 125.0, h.sync(t, "r3", 5), "a resent report is counted once")
	assert.Equal(t, 125.0, h.sync(t, "", 0), "a read only returns the total")

	h.redis.setDown(false)
	reachable.Store(true)
	h.next <- true
	require.Eventually(t, func() bool { return !h.isDown() }, time.Second, time.Millisecond)

	assert.Equal(t, 125.0, h.redisTotal(t), "Redis has every report once")
	assert.Equal(t, 126.0, h.sync(t, "r4", 1))
}

// A report Redis applied just before the connection failed is added again under its own
// report ID, so Redis does not count it twice.
func TestFailoverDoesNotAddAReportRedisAlreadyHas(t *testing.T) {
	var reachable atomic.Bool
	reachable.Store(true)
	h := newFailoverHarness(t, &reachable)
	assert.Equal(t, 40.0, h.sync(t, "r1", 40))

	// Redis applied r2, but the controller saw the call fail.
	h.f.mu.Lock()
	_, err := h.f.redis.Sync(context.Background(), "engine-1", "r2", []counterproto.Add{{Key: spendKey, Delta: 2}}, nil)
	require.NoError(t, err)
	h.f.goDownLocked()
	h.f.queueLocked("engine-1", "r2", []counterproto.Add{{Key: spendKey, Delta: 2}})
	h.f.mu.Unlock()

	h.next <- true
	require.Eventually(t, func() bool { return !h.isDown() }, time.Second, time.Millisecond)
	assert.Equal(t, 42.0, h.redisTotal(t))
}

// The wait before each attempt starts at initial_duration and is multiplied by
// progression_factor, up to max_duration.
func TestFailoverWaitsLongerAfterEachFailedAttempt(t *testing.T) {
	var reachable atomic.Bool
	h := newFailoverHarness(t, &reachable)
	for i := 0; i < 4; i++ {
		h.next <- true
	}
	require.Eventually(t, func() bool { h.mu.Lock(); defer h.mu.Unlock(); return len(h.waits) == 5 }, time.Second, time.Millisecond)
	h.mu.Lock()
	defer h.mu.Unlock()
	assert.Equal(t, []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second}, h.waits)
}

// Close stops reconnecting.
func TestFailoverCloseStopsReconnecting(t *testing.T) {
	var reachable atomic.Bool
	h := newFailoverHarness(t, &reachable)
	require.NoError(t, h.f.Close())
	require.NoError(t, h.f.Close())
	h.f.mu.Lock()
	defer h.f.mu.Unlock()
	assert.True(t, h.f.closed)
}

// A budget cleared while Redis is away is cleared in memory at once and in Redis when it
// is back, before what was counted afterwards is added.
func TestFailoverDeleteWhileRedisIsAway(t *testing.T) {
	var reachable atomic.Bool
	reachable.Store(true)
	h := newFailoverHarness(t, &reachable)
	assert.Equal(t, 100.0, h.sync(t, "r1", 100))

	h.redis.setDown(true)
	reachable.Store(false)
	assert.Equal(t, 105.0, h.sync(t, "r2", 5))
	require.True(t, h.isDown())

	require.NoError(t, h.f.Delete(context.Background(), spendKey.Namespace, spendKey.ID))
	assert.Equal(t, 0.0, h.sync(t, "", 0), "cleared in memory at once")
	assert.Equal(t, 2.0, h.sync(t, "r3", 2), "counting starts again from nothing")

	h.redis.setDown(false)
	reachable.Store(true)
	h.next <- true
	require.Eventually(t, func() bool { return !h.isDown() }, time.Second, time.Millisecond)
	assert.Equal(t, 2.0, h.redisTotal(t), "Redis was cleared, then given what came after")
}
