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
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/wso2/api-platform/gateway/common/counterproto"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

// flushTimeout bounds adding what was counted during an outage to Redis.
const flushTimeout = 10 * time.Second

// Failover is the Redis store that keeps counting in this controller's memory while Redis
// cannot be reached, at start or later. Memory starts from the last totals Redis returned,
// so a limit is not reset by an outage. Meanwhile it reconnects, waiting longer after each
// failed attempt (counters.redis.retry_config). Once Redis is back, what was counted meanwhile is
// added to it once and Redis is used again.
//
// Each controller adds only what its own policy engines reported, so totals are right
// again after every controller has caught up; until then a controller sees only its own
// engines' spend on top of the last totals.
type Failover struct {
	connect func(ctx context.Context) (*Redis, error)
	retry   config.CountersRedisRetryConfig
	logger  *slog.Logger
	now     func() time.Time
	// sleep waits d or until stop is closed; it reports whether to go on.
	sleep func(d time.Duration, stop <-chan struct{}) bool

	mu           sync.Mutex
	redis        *Redis // nil until the first connection
	down         bool
	reconnecting bool
	stop         chan struct{}
	closed       bool

	// last holds the latest total Redis returned for each key, and when it was read.
	last map[counterproto.Key]memoryCounter
	// queued are reports to add to Redis, in order, each under its own report ID so a
	// report Redis applied before failing is not applied twice.
	queued []queuedReport
	// held sums the adds reported while Redis was down that are not queued yet.
	held map[counterproto.Key]float64
	// applied remembers the reports counted in memory, so a resent one is counted once.
	applied   map[string]time.Time
	retention time.Duration
	lastSweep time.Time
}

type queuedReport struct {
	source, reportID string
	adds             []counterproto.Add
}

// failoverSource marks the report that adds what a controller counted while Redis was down.
const failoverSource = "failover"

// NewFailover connects to Redis and returns the store. When Redis cannot be reached it
// still returns the store, counting in memory and reconnecting in the background.
func NewFailover(cfg config.CountersRedisConfig, retention time.Duration, logger *slog.Logger) *Failover {
	f := newFailover(func(context.Context) (*Redis, error) { return NewRedis(cfg, retention) }, cfg.Retry, retention, logger)
	f.start()
	return f
}

func newFailover(connect func(context.Context) (*Redis, error), retry config.CountersRedisRetryConfig, retention time.Duration, logger *slog.Logger) *Failover {
	if retention <= 0 {
		retention = DefaultRetention
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Failover{
		connect:   connect,
		retry:     retry,
		logger:    logger,
		now:       time.Now,
		sleep:     sleepOrStop,
		stop:      make(chan struct{}),
		last:      map[counterproto.Key]memoryCounter{},
		held:      map[counterproto.Key]float64{},
		applied:   map[string]time.Time{},
		retention: retention,
	}
}

// start makes the first connection, or counts in memory and reconnects in the background.
func (f *Failover) start() {
	r, err := f.connect(context.Background())
	f.mu.Lock()
	defer f.mu.Unlock()
	if err != nil {
		f.logger.Warn("Redis cannot be reached: counting in memory until it can",
			slog.Any("error", err))
		f.goDownLocked()
		return
	}
	f.redis = r
}

// Sync applies adds once per (source, reportID) and returns the totals of keys: from Redis,
// or from memory while Redis cannot be reached. It fails only after Close.
func (f *Failover) Sync(ctx context.Context, source, reportID string, adds []counterproto.Add, keys []counterproto.Key) ([]counterproto.Total, error) {
	f.mu.Lock()
	r, down := f.redis, f.down
	f.mu.Unlock()
	if !down && r != nil {
		totals, err := r.Sync(ctx, source, reportID, adds, keys)
		if err == nil {
			f.remember(totals)
			return totals, nil
		}
		f.mu.Lock()
		if !f.down {
			f.logger.Warn("Redis cannot be reached: counting in memory until it can",
				slog.Any("error", err))
			f.goDownLocked()
		}
		// Redis may have applied it before failing: queue it under its own ID so it is
		// added once, and count it in memory.
		f.queueLocked(source, reportID, adds)
		f.mu.Unlock()
		return f.memoryTotals(keys), nil
	}

	f.mu.Lock()
	now := f.now()
	f.sweepLocked(now)
	if len(adds) > 0 {
		marker := source + "\x00" + reportID
		if _, done := f.applied[marker]; !done {
			f.applied[marker] = now
			for _, add := range adds {
				f.held[add.Key] += add.Delta
			}
		}
	}
	f.mu.Unlock()
	return f.memoryTotals(keys), nil
}

// Close stops reconnecting and closes the Redis connection.
func (f *Failover) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil
	}
	f.closed = true
	close(f.stop)
	if f.redis != nil {
		return f.redis.Close()
	}
	return nil
}

// remember keeps totals Redis returned, the base for memory if Redis goes away.
func (f *Failover) remember(totals []counterproto.Total) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.now()
	for _, t := range totals {
		f.last[t.Key] = memoryCounter{total: t.Total, touched: now}
	}
	f.sweepLocked(now)
}

// memoryTotals is the last Redis total of each key plus what is waiting to be added.
func (f *Failover) memoryTotals(keys []counterproto.Key) []counterproto.Total {
	f.mu.Lock()
	defer f.mu.Unlock()
	totals := make([]counterproto.Total, 0, len(keys))
	for _, key := range keys {
		total := f.last[key].total + f.held[key]
		for _, q := range f.queued {
			for _, add := range q.adds {
				if add.Key == key {
					total += add.Delta
				}
			}
		}
		totals = append(totals, counterproto.Total{Key: key, Total: total})
	}
	return totals
}

func (f *Failover) queueLocked(source, reportID string, adds []counterproto.Add) {
	if len(adds) == 0 {
		return
	}
	f.applied[source+"\x00"+reportID] = f.now()
	f.queued = append(f.queued, queuedReport{source: source, reportID: reportID, adds: adds})
}

// goDownLocked switches to memory and starts reconnecting, once.
func (f *Failover) goDownLocked() {
	f.down = true
	if f.reconnecting || f.closed {
		return
	}
	f.reconnecting = true
	go f.reconnect()
}

// reconnect tries Redis again until it is back, waiting longer after each failed attempt.
func (f *Failover) reconnect() {
	wait := f.retry.InitialDuration
	for attempt := 1; ; attempt++ {
		if !f.sleep(wait, f.stop) {
			return
		}
		err := f.recover()
		if err == nil {
			f.logger.Info("Redis is back: counters are kept in Redis again", slog.Int("attempts", attempt))
			return
		}
		next := time.Duration(float64(wait) * f.retry.ProgressionFactor)
		if next > f.retry.MaxDuration {
			next = f.retry.MaxDuration
		}
		f.logger.Warn("Redis still cannot be reached: counting in memory",
			slog.Int("attempt", attempt), slog.Duration("next_retry_in", next), slog.Any("error", err))
		wait = next
	}
}

// recover connects if needed and, once Redis answers, adds what was counted meanwhile;
// on success Redis is used again. It checks Redis before taking the lock, so reports keep
// being counted in memory, without waiting, while Redis is still away.
func (f *Failover) recover() error {
	ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()
	f.mu.Lock()
	r, closed := f.redis, f.closed
	f.mu.Unlock()
	if closed {
		return nil
	}
	if r == nil {
		var err error
		if r, err = f.connect(ctx); err != nil {
			return err
		}
	} else if err := r.Ping(ctx); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		if f.redis == nil {
			_ = r.Close()
		}
		return nil
	}
	f.redis = r
	// What was summed in memory becomes one report, kept under one ID until Redis takes it.
	if len(f.held) > 0 {
		adds := make([]counterproto.Add, 0, len(f.held))
		for key, delta := range f.held {
			adds = append(adds, counterproto.Add{Key: key, Delta: delta})
		}
		f.queued = append(f.queued, queuedReport{source: failoverSource, reportID: uuid.NewString(), adds: adds})
		f.held = map[counterproto.Key]float64{}
	}
	for len(f.queued) > 0 {
		q := f.queued[0]
		if _, err := f.redis.Sync(ctx, q.source, q.reportID, q.adds, nil); err != nil {
			return err
		}
		f.queued = f.queued[1:]
	}
	f.down = false
	f.reconnecting = false
	return nil
}

// sweepLocked forgets old report markers and idle totals, at most once a minute.
func (f *Failover) sweepLocked(now time.Time) {
	if now.Sub(f.lastSweep) < time.Minute {
		return
	}
	f.lastSweep = now
	for marker, at := range f.applied {
		if now.Sub(at) > appliedReportTTL {
			delete(f.applied, marker)
		}
	}
	for key, c := range f.last {
		if now.Sub(c.touched) > f.retention {
			delete(f.last, key)
		}
	}
}

func sleepOrStop(d time.Duration, stop <-chan struct{}) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-stop:
		return false
	}
}
