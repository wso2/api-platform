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

package server

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// burstGrace is how long a burst's callers get, once all of them have started, to
// reach the exchange single-flight while the first exchange is still in flight. The
// path from a request's start to that lock does no I/O the first exchange is not
// already blocking, so this only has to outlast scheduling, even under -race.
const burstGrace = 200 * time.Millisecond

// burstGate keeps a burst's first token exchange in flight until every caller of the
// burst has started. The single-flight only coalesces callers that overlap the
// exchange in flight, and a stub IDP answers in microseconds, so without the gate the
// first caller can finish before the others arrive. A caller that arrives later still
// carries the session it was sent with, which holds no exchanged token, so it rightly
// runs a second exchange, and a test asserting one exchange per burst fails at random.
type burstGate struct {
	started     atomic.Int32
	arrived     chan struct{}
	arriveOnce  sync.Once
	release     chan struct{}
	releaseOnce sync.Once
}

func newBurstGate(t *testing.T) *burstGate {
	t.Helper()
	g := &burstGate{arrived: make(chan struct{}), release: make(chan struct{})}
	// A failed test must not leave the stub IDP's handler blocked, or closing the
	// test server waits on it forever.
	t.Cleanup(g.open)
	return g
}

// start records that one caller of the burst has started its request.
func (g *burstGate) start() { g.started.Add(1) }

// hold is called by the stub IDP for every exchange. It blocks until the gate opens.
func (g *burstGate) hold() {
	g.arriveOnce.Do(func() { close(g.arrived) })
	<-g.release
}

func (g *burstGate) open() { g.releaseOnce.Do(func() { close(g.release) }) }

// openWhenOverlapping waits until the first exchange has reached the IDP and every
// one of callers has started, gives them burstGrace to queue behind that exchange,
// and then lets it complete.
func (g *burstGate) openWhenOverlapping(t *testing.T, callers int) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	select {
	case <-g.arrived:
	case <-deadline:
		t.Fatal("no token exchange reached the IDP")
	}
	for g.started.Load() < int32(callers) {
		select {
		case <-deadline:
			t.Fatalf("only %d of %d callers started", g.started.Load(), callers)
		case <-time.After(time.Millisecond):
		}
	}
	time.Sleep(burstGrace)
	g.open()
}
