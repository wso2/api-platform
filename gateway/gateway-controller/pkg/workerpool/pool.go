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

// Package workerpool provides a worker pool used to optionally cap the
// concurrency of background work (e.g. DP->CP artifact sync).
//
// By default (workers <= 0) the pool runs in unlimited mode: every submitted
// task gets its own goroutine, exactly preserving the historical
// "one goroutine per operation" behaviour with no bound on concurrency or
// backlog. When an operator configures a positive worker count the pool runs a
// fixed set of workers instead; the pending queue is unbounded unless a positive
// queue size is configured, in which case excess work is rejected once the queue
// is full (see the go-network-service-hardening rule, directive 3).
package workerpool

import (
	"log/slog"
	"sync"
)

// Pool is a worker pool. In unlimited mode it spawns a goroutine per task; in
// bounded mode a fixed number of workers drain a FIFO queue (bounded or
// unbounded). It is safe for concurrent use.
type Pool struct {
	name      string
	logger    *slog.Logger
	unlimited bool
	maxQueue  int // 0 = unbounded; only meaningful in bounded mode

	mu      sync.Mutex
	cond    *sync.Cond
	queue   []func()
	head    int  // index of the next task to run in queue
	stopped bool // guarded by mu; set by Stop for both modes
	wg      sync.WaitGroup

	// unlimitedActive tracks the number of in-flight goroutines in unlimited mode so Stop can wait for them to drain.
	unlimitedActive sync.WaitGroup
}

// New creates and starts a worker pool identified by name.
//
//   - workers <= 0: unlimited mode — a goroutine per task, no bound on
//     concurrency or backlog (the default).
//   - workers > 0: exactly that many workers. queueSize <= 0 means an unbounded
//     pending queue (work is never rejected); queueSize > 0 caps the backlog and
//     Submit rejects once it is full.
//
// The pool must be shut down with Stop to release its resources.
func New(name string, workers, queueSize int, logger *slog.Logger) *Pool {
	if logger == nil {
		logger = slog.Default()
	}
	p := &Pool{name: name, logger: logger}
	p.cond = sync.NewCond(&p.mu)

	if workers <= 0 {
		p.unlimited = true
		logger.Info("Started worker pool (unlimited: goroutine per task)",
			slog.String("pool", name),
		)
		return p
	}

	if queueSize < 0 {
		queueSize = 0
	}
	p.maxQueue = queueSize
	for range workers {
		p.wg.Add(1)
		go p.worker()
	}
	queueLabel := "unbounded"
	if queueSize > 0 {
		queueLabel = "bounded"
	}
	logger.Info("Started worker pool",
		slog.String("pool", name),
		slog.Int("workers", workers),
		slog.Int("queue_size", queueSize),
		slog.String("queue_mode", queueLabel),
	)
	return p
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for {
		p.mu.Lock()
		for p.head >= len(p.queue) && !p.stopped {
			p.cond.Wait()
		}
		if p.head >= len(p.queue) && p.stopped {
			p.mu.Unlock()
			return
		}
		task := p.queue[p.head]
		p.queue[p.head] = nil
		p.head++
		// Compact the backing array once the consumed prefix dominates it so the
		// slice header doesn't pin an ever-growing, mostly-drained array.
		if p.head > 32 && p.head*2 >= len(p.queue) {
			p.queue = append(p.queue[:0:0], p.queue[p.head:]...)
			p.head = 0
		}
		p.mu.Unlock()
		p.runSafely(task)
	}
}

// runSafely executes a single task, recovering from any panic so one bad task
// cannot take down a worker goroutine (which would permanently shrink the pool).
func (p *Pool) runSafely(task func()) {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("Worker pool task panicked",
				slog.String("pool", p.name),
				slog.Any("panic", r),
			)
		}
	}()
	task()
}

// Submit enqueues a task for execution and returns true when it was accepted.
//
// In unlimited mode it always accepts (spawning a goroutine) unless the pool has
// been stopped. In bounded mode with a configured queue cap it returns false
// (dropping the task) if the queue is currently full. A false return is not an
// error: the caller must treat it as "not scheduled" — for DP->CP sync the
// affected artifact stays in its pending/failed cp_sync_status and is retried on
// the next (re)connect full resync.
func (p *Pool) Submit(task func()) bool {
	if task == nil {
		return false
	}

	if p.unlimited {
		p.mu.Lock()
		if p.stopped {
			p.mu.Unlock()
			return false
		}
		p.unlimitedActive.Add(1)
		p.mu.Unlock()
		go func() {
			defer p.unlimitedActive.Done()
			p.runSafely(task)
		}()
		return true
	}

	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return false
	}
	if p.maxQueue > 0 && (len(p.queue)-p.head) >= p.maxQueue {
		p.mu.Unlock()
		p.logger.Warn("Worker pool queue full; rejecting task",
			slog.String("pool", p.name),
			slog.Int("queue_size", p.maxQueue),
		)
		return false
	}
	p.queue = append(p.queue, task)
	p.cond.Signal()
	p.mu.Unlock()
	return true
}

// Stop stops accepting new tasks and blocks until all in-flight (and, in bounded
// mode, queued) tasks have finished and every worker goroutine has exited. It is
// idempotent.
func (p *Pool) Stop() {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}
	p.stopped = true
	if !p.unlimited {
		p.cond.Broadcast()
	}
	p.mu.Unlock()

	if p.unlimited {
		p.unlimitedActive.Wait()
	} else {
		p.wg.Wait()
	}
	p.logger.Info("Stopped worker pool", slog.String("pool", p.name))
}
