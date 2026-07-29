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

package workerpool

import (
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestPool_RunsAllSubmittedTasks verifies every accepted task executes exactly once.
func TestPool_RunsAllSubmittedTasks(t *testing.T) {
	p := New("test", 4, 128, testLogger())
	defer p.Stop()

	var counter atomic.Int64
	var wg sync.WaitGroup
	const n = 100
	wg.Add(n)
	for range n {
		accepted := p.Submit(func() {
			defer wg.Done()
			counter.Add(1)
		})
		if !accepted {
			t.Fatal("task unexpectedly rejected")
		}
	}
	wg.Wait()
	if got := counter.Load(); got != n {
		t.Fatalf("expected %d tasks run, got %d", n, got)
	}
}

// TestPool_BoundsConcurrency verifies no more than the configured number of
// workers run simultaneously — the core property that fixes the concurrent-sync
// deadlock (#2903) when the pool is sized to 1.
func TestPool_BoundsConcurrency(t *testing.T) {
	const workers = 2
	p := New("test", workers, 128, testLogger())
	defer p.Stop()

	var inFlight atomic.Int64
	var maxInFlight atomic.Int64
	var wg sync.WaitGroup
	const n = 20
	wg.Add(n)
	for range n {
		p.Submit(func() {
			defer wg.Done()
			cur := inFlight.Add(1)
			for {
				old := maxInFlight.Load()
				if cur <= old || maxInFlight.CompareAndSwap(old, cur) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			inFlight.Add(-1)
		})
	}
	wg.Wait()
	if got := maxInFlight.Load(); got > workers {
		t.Fatalf("concurrency exceeded pool size: max in-flight %d > workers %d", got, workers)
	}
}

// TestPool_UnlimitedRunsEveryTask verifies the default (workers <= 0) unlimited
// mode runs every submitted task, never rejecting — preserving the historical
// per-operation goroutine behaviour.
func TestPool_UnlimitedRunsEveryTask(t *testing.T) {
	p := New("test", 0, 0, testLogger()) // 0 workers => unlimited
	defer p.Stop()

	var counter atomic.Int64
	var wg sync.WaitGroup
	const n = 200
	wg.Add(n)
	for range n {
		if !p.Submit(func() {
			defer wg.Done()
			counter.Add(1)
		}) {
			t.Fatal("unlimited pool must never reject a task")
		}
	}
	wg.Wait()
	if got := counter.Load(); got != n {
		t.Fatalf("expected %d tasks run, got %d", n, got)
	}
}

// TestPool_UnboundedQueueNeverRejects verifies that bounded workers with an
// unconfigured (<=0) queue size never reject work.
func TestPool_UnboundedQueueNeverRejects(t *testing.T) {
	// One worker, unbounded queue. Block the worker, then flood the queue.
	p := New("test", 1, 0, testLogger())
	release := make(chan struct{})
	started := make(chan struct{})
	if !p.Submit(func() {
		close(started)
		<-release
	}) {
		t.Fatal("first task should be accepted")
	}
	<-started

	const n = 1000
	for range n {
		if !p.Submit(func() {}) {
			t.Fatal("unbounded queue must never reject a task")
		}
	}
	close(release)
	p.Stop() // drains the queue
}

// TestPool_RejectsWhenQueueFull verifies the bounded queue rejects excess work
// instead of buffering it without limit.
func TestPool_RejectsWhenQueueFull(t *testing.T) {
	// One worker, queue of one. Block the worker so the queue fills.
	p := New("test", 1, 1, testLogger())
	defer p.Stop()

	release := make(chan struct{})
	started := make(chan struct{})
	// Occupy the single worker.
	if !p.Submit(func() {
		close(started)
		<-release
	}) {
		t.Fatal("first task should be accepted")
	}
	<-started

	// Fill the single queue slot.
	if !p.Submit(func() { <-release }) {
		t.Fatal("second task should fill the queue")
	}
	// Now both worker and queue are occupied → next submit must be rejected.
	if p.Submit(func() {}) {
		t.Fatal("third task should be rejected when the queue is full")
	}
	close(release)
}

// TestPool_UnlimitedSubmitStopRace exercises concurrent Submit and Stop on an
// unlimited pool: the stopped-check and WaitGroup increment must be synchronized
// so no task is admitted after Stop begins (no Add-after-Wait). Run under -race.
func TestPool_UnlimitedSubmitStopRace(t *testing.T) {
	p := New("test", 0, 0, testLogger()) // unlimited

	var ran atomic.Int64
	var submitters sync.WaitGroup
	for range 8 {
		submitters.Go(func() {
			for range 50 {
				p.Submit(func() { ran.Add(1) })
			}
		})
	}

	p.Stop() // races with the in-flight Submits; must not panic or leak a task past Stop
	submitters.Wait()
	// Any further submits after Stop must be rejected.
	if p.Submit(func() { ran.Add(1) }) {
		t.Fatal("submit after Stop must be rejected")
	}
}

// TestPool_SubmitAfterStopRejects verifies a stopped pool accepts no new work.
func TestPool_SubmitAfterStopRejects(t *testing.T) {
	p := New("test", 2, 8, testLogger())
	p.Stop()
	if p.Submit(func() {}) {
		t.Fatal("submit after Stop must be rejected")
	}
	// Stop is idempotent.
	p.Stop()
}

// TestPool_RecoversFromPanic verifies a panicking task doesn't kill the worker.
func TestPool_RecoversFromPanic(t *testing.T) {
	p := New("test", 1, 8, testLogger())
	defer p.Stop()

	if !p.Submit(func() { panic("boom") }) {
		t.Fatal("panicking task should be accepted")
	}
	done := make(chan struct{})
	if !p.Submit(func() { close(done) }) {
		t.Fatal("follow-up task should be accepted")
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not survive a panicking task")
	}
}
