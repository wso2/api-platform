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

package counters

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/wso2/api-platform/gateway/common/counterproto"
	sdkcounters "github.com/wso2/api-platform/sdk/core/counters"
)

var november = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

func key(id string) sdkcounters.Key {
	return sdkcounters.Key{Namespace: "budget", ID: id, Window: november}
}

func TestTotalCombinesSharedInflightAndPending(t *testing.T) {
	s := NewService("engine-1", 0)
	s.Add(key("a"), 1)

	total, ok := s.Total(key("a"))
	assert.Equal(t, 1.0, total)
	assert.False(t, ok, "no shared total has come back yet")

	req := s.nextRequest()
	require.Equal(t, "1", req.ReportID)
	require.Len(t, req.Adds, 1)
	s.Add(key("a"), 2) // while the report is in flight

	total, _ = s.Total(key("a"))
	assert.Equal(t, 3.0, total, "in flight and pending both count")

	// The controller applied the report and other engines' spend: total 10.
	s.apply(req, &counterproto.SyncResponse{OK: true, ReportID: "1", Totals: []counterproto.Total{{Key: wire(key("a")), Total: 10}}})

	total, ok = s.Total(key("a"))
	assert.Equal(t, 12.0, total, "shared 10 plus the 2 not yet reported")
	assert.True(t, ok)
}

func TestForgetDropsTotalsPendingAndInflight(t *testing.T) {
	s := NewService("engine-1", 0)
	s.Add(key("gone/1"), 1)
	s.Add(key("kept/1"), 1)
	req := s.nextRequest() // both in flight
	s.apply(req, &counterproto.SyncResponse{OK: true, Totals: []counterproto.Total{
		{Key: wire(key("gone/1")), Total: 7}, {Key: wire(key("kept/1")), Total: 7},
	}})
	s.Add(key("gone/1"), 2) // pending

	s.Forget("budget", "gone/")

	total, ok := s.Total(key("gone/1"))
	assert.Equal(t, 0.0, total, "a forgotten counter starts again from nothing")
	assert.False(t, ok)
	total, _ = s.Total(key("kept/1"))
	assert.Equal(t, 8.0, total, "other counters keep their totals and in-flight adds")

	again := s.nextRequest()
	assert.Equal(t, req.ReportID, again.ReportID, "the report in flight keeps its ID")
	for _, add := range again.Adds {
		assert.NotEqual(t, "gone/1", add.Key.ID, "the forgotten counter's adds are not sent")
	}
}

func TestForgetEndsAReportLeftEmpty(t *testing.T) {
	s := NewService("engine-1", 0)
	s.Add(key("gone/1"), 1)
	s.nextRequest()
	s.Forget("budget", "gone/")
	s.Add(key("kept/1"), 1)
	next := s.nextRequest()
	assert.Equal(t, "2", next.ReportID, "a new report, not an empty one resent")
}

func TestReportIsResentUntilAcknowledged(t *testing.T) {
	s := NewService("engine-1", 0)
	s.Add(key("a"), 1)
	first := s.nextRequest()

	s.Add(key("a"), 5)
	s.apply(first, &counterproto.SyncResponse{}) // the store failed: no acknowledgement
	again := s.nextRequest()

	assert.Equal(t, first.ReportID, again.ReportID)
	assert.Equal(t, first.Adds, again.Adds, "the same adds, so the store applies them once")

	s.apply(again, &counterproto.SyncResponse{OK: true, ReportID: again.ReportID})
	next := s.nextRequest()
	assert.Equal(t, "2", next.ReportID)
	require.Len(t, next.Adds, 1)
	assert.Equal(t, 5.0, next.Adds[0].Delta)
}

func TestIdleCountersStopBeingSynced(t *testing.T) {
	s := NewService("engine-1", 0)
	now := november
	s.now = func() time.Time { return now }
	s.Total(key("a"))
	assert.Len(t, s.nextRequest().Keys, 1)

	now = now.Add(DefaultIdleTimeout + time.Second)

	assert.Empty(t, s.nextRequest().Keys)
}

func TestKeysForTheSameWindowAreEqual(t *testing.T) {
	s := NewService("engine-1", 0)
	s.Add(sdkcounters.Key{Namespace: "budget", ID: "a", Window: november.In(time.FixedZone("x", 3600))}, 1)
	total, _ := s.Total(key("a"))
	assert.Equal(t, 1.0, total)
}

// stubController is a counter store behind a real gRPC stream.
type stubController struct {
	mu      sync.Mutex
	totals  map[counterproto.Key]float64
	applied map[string]bool
	drop    int // acknowledgements to drop
	seen    int // sync requests received
}

func (c *stubController) requests() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.seen
}

func (c *stubController) sync(_ any, stream grpc.ServerStream) error {
	for {
		req := &counterproto.SyncRequest{}
		if err := stream.RecvMsg(req); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		c.mu.Lock()
		c.seen++
		marker := req.Source + "/" + req.ReportID
		if len(req.Adds) > 0 && !c.applied[marker] {
			c.applied[marker] = true
			for _, a := range req.Adds {
				c.totals[a.Key] += a.Delta
			}
		}
		resp := &counterproto.SyncResponse{OK: true, ReportID: req.ReportID}
		for _, k := range req.Keys {
			resp.Totals = append(resp.Totals, counterproto.Total{Key: k, Total: c.totals[k]})
		}
		if c.drop > 0 && req.ReportID != "" {
			c.drop--
			resp = &counterproto.SyncResponse{}
		}
		c.mu.Unlock()
		if err := stream.SendMsg(resp); err != nil {
			return err
		}
	}
}

func startStub(t *testing.T, stub *stubController) func(context.Context, string) (net.Conn, error) {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: counterproto.ServiceName,
		HandlerType: (*any)(nil),
		Streams:     []grpc.StreamDesc{{StreamName: counterproto.SyncMethod, Handler: stub.sync, ServerStreams: true, ClientStreams: true}},
	}, stub)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	return func(context.Context, string) (net.Conn, error) { return listener.Dial() }
}

func TestTwoEnginesShareTotalsThroughTheController(t *testing.T) {
	stub := &stubController{totals: map[counterproto.Key]float64{}, applied: map[string]bool{}, drop: 1}
	dialer := startStub(t, stub)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engines := []*Service{NewService("engine-1", 0), NewService("engine-2", 0)}
	for _, s := range engines {
		syncer := &Syncer{
			Service: s, ServerAddress: "passthrough:///bufnet", Credentials: insecureCredentials,
			Interval: 20 * time.Millisecond, Timeout: time.Second, MaxBackoff: 100 * time.Millisecond,
			Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), dialer: dialer,
		}
		go syncer.Run(ctx)
	}

	engines[0].Add(key("a"), 0.75)
	engines[1].Add(key("a"), 0.25)

	require.Eventually(t, func() bool {
		for _, s := range engines {
			if total, ok := s.Total(key("a")); !ok || total != 1.0 {
				return false
			}
		}
		return true
	}, 5*time.Second, 20*time.Millisecond, "both engines see 1.0, the first report counted once although its acknowledgement was lost")
}

func insecureCredentials() (credentials.TransportCredentials, error) {
	return insecure.NewCredentials(), nil
}

func TestAnEngineWithNothingToReportStillGetsTotals(t *testing.T) {
	s := NewService("engine-1", 0)
	_, known := s.Total(key("a")) // used, but never added to
	require.False(t, known)
	req := s.nextRequest()
	require.Empty(t, req.ReportID)
	require.Len(t, req.Keys, 1)

	s.apply(req, &counterproto.SyncResponse{OK: true, Totals: []counterproto.Total{{Key: wire(key("a")), Total: 9}}})
	total, known := s.Total(key("a"))
	assert.True(t, known)
	assert.Equal(t, 9.0, total, "another gateway's spend arrives without this engine adding")

	s.apply(s.nextRequest(), &counterproto.SyncResponse{})
	total, _ = s.Total(key("a"))
	assert.Equal(t, 9.0, total, "a failed sync keeps the last total")
}

func TestAReadOnlyEngineSeesAnotherEnginesSpend(t *testing.T) {
	stub := &stubController{totals: map[counterproto.Key]float64{}, applied: map[string]bool{}}
	dialer := startStub(t, stub)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	spender, reader := NewService("engine-1", 0), NewService("engine-2", 0)
	for _, s := range []*Service{spender, reader} {
		syncer := &Syncer{
			Service: s, ServerAddress: "passthrough:///bufnet", Credentials: insecureCredentials,
			Interval: 20 * time.Millisecond, Timeout: time.Second, MaxBackoff: 100 * time.Millisecond,
			Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), dialer: dialer,
		}
		go syncer.Run(ctx)
	}
	spender.Add(key("a"), 0.75)
	reader.Total(key("a"))

	require.Eventually(t, func() bool {
		total, ok := reader.Total(key("a"))
		return ok && total == 0.75
	}, 5*time.Second, 20*time.Millisecond, "the engine that only reads sees the other engine's spend")
}

func TestSyncerReloadsCredentialsAndResetsBackoff(t *testing.T) {
	stub := &stubController{totals: map[counterproto.Key]float64{}, applied: map[string]bool{}}
	dialer := startStub(t, stub)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var loads atomic.Int32
	syncer := &Syncer{
		Service: NewService("engine-1", 0), ServerAddress: "passthrough:///bufnet",
		Credentials: func() (credentials.TransportCredentials, error) {
			loads.Add(1)
			return insecure.NewCredentials(), nil
		},
		Interval: 10 * time.Millisecond, Timeout: 50 * time.Millisecond, MaxBackoff: time.Hour,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		dialer: func(ctx context.Context, addr string) (net.Conn, error) {
			conn, err := dialer(ctx, addr)
			if err != nil {
				return nil, err
			}
			// Break every connection after a short while, as a controller restart does.
			time.AfterFunc(40*time.Millisecond, func() { _ = conn.Close() })
			return conn, nil
		},
	}
	// A counter in use, so every interval has something to sync: an engine that counts
	// nothing sends nothing and so never notices a broken stream until it does.
	syncer.Service.Add(key("a"), 0.5)
	go syncer.Run(ctx)

	// With MaxBackoff an hour, a backoff that kept doubling across healthy streams would
	// stall reconnects; reset after each working stream, they keep happening.
	require.Eventually(t, func() bool { return loads.Load() >= 5 }, 5*time.Second, 10*time.Millisecond,
		"each reconnect loads the credentials again and comes back quickly")
}

func TestIdleTimeoutIsConfigurableAndUsePushesItForward(t *testing.T) {
	s := NewService("engine-1", time.Minute)
	now := november
	s.now = func() time.Time { return now }
	s.Total(key("a"))

	now = now.Add(50 * time.Second)
	s.Total(key("a")) // used again: expiry moves to 50s + 1m
	now = now.Add(50 * time.Second)
	assert.Len(t, s.nextRequest().Keys, 1, "still in use 50s after its last use")

	now = now.Add(11 * time.Second)
	assert.Empty(t, s.nextRequest().Keys, "forgotten a minute after its last use")
	_, synced := s.Total(key("a"))
	assert.False(t, synced, "a forgotten counter starts again without a shared total")
}

func TestAnExpiredCounterWithUnreportedSpendIsKept(t *testing.T) {
	s := NewService("engine-1", time.Minute)
	now := november
	s.now = func() time.Time { return now }
	s.Add(key("a"), 1)
	first := s.nextRequest()
	require.Len(t, first.Adds, 1)

	now = now.Add(2 * time.Minute)
	again := s.nextRequest() // not acknowledged yet
	assert.Equal(t, first.ReportID, again.ReportID)
	assert.Len(t, again.Keys, 1, "kept until its spend is acknowledged")

	s.apply(again, &counterproto.SyncResponse{OK: true, ReportID: again.ReportID})
	assert.Empty(t, s.nextRequest().Keys)
}

func TestDescribedCountersSendTheirLabelsWithTheirAdds(t *testing.T) {
	s := NewService("engine-1", time.Minute)
	window := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	described := sdkcounters.Key{Namespace: "budget", ID: "a-1", Window: window}
	plain := sdkcounters.Key{Namespace: "ratelimit", ID: "x"}
	labels := map[string]string{"budget": "per-agent", "agent": "api-platform/support-agent"}
	s.Describe(described, labels)
	labels["budget"] = "changed after Describe"
	s.Add(described, 0.5)
	s.Add(plain, 1)

	req := s.nextRequest()
	if len(req.Adds) != 2 {
		t.Fatalf("adds %+v", req.Adds)
	}
	for _, add := range req.Adds {
		switch add.Key.Namespace {
		case "budget":
			if add.Labels["budget"] != "per-agent" || add.Labels["agent"] != "api-platform/support-agent" {
				t.Fatalf("labels %v", add.Labels)
			}
		default:
			if add.Labels != nil {
				t.Fatalf("an undescribed counter carries labels %v", add.Labels)
			}
		}
	}
}

// An engine whose policies count nothing does not connect to its controller; the first
// counter a policy uses starts the syncs.
func TestSyncerConnectsOnlyOnceACounterIsUsed(t *testing.T) {
	stub := &stubController{totals: map[counterproto.Key]float64{}, applied: map[string]bool{}}
	dialer := startStub(t, stub)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var dials atomic.Int32
	syncer := &Syncer{
		Service: NewService("engine-1", 0), ServerAddress: "passthrough:///bufnet",
		Credentials: func() (credentials.TransportCredentials, error) { return insecure.NewCredentials(), nil },
		Interval:    5 * time.Millisecond, Timeout: time.Second, MaxBackoff: time.Second,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		dialer: func(ctx context.Context, addr string) (net.Conn, error) {
			dials.Add(1)
			return dialer(ctx, addr)
		},
	}
	go syncer.Run(ctx)
	time.Sleep(100 * time.Millisecond)
	assert.Zero(t, dials.Load(), "no counters in use: no connection")
	assert.Zero(t, stub.requests(), "and no syncs")

	syncer.Service.Add(key("a"), 1)
	require.Eventually(t, func() bool { return stub.requests() > 0 }, 5*time.Second, 5*time.Millisecond,
		"the first counter starts the syncs")
}

// An idle counter is no longer synced but keeps its last total for the rest of its
// window, so a used-up budget with rare traffic is still used up on its next request.
func TestAnIdleCounterKeepsItsTotalForItsWindow(t *testing.T) {
	s := NewService("engine-1", time.Minute)
	now := november
	s.now = func() time.Time { return now }
	s.Total(key("a"))
	req := s.nextRequest()
	s.apply(req, &counterproto.SyncResponse{OK: true, Totals: []counterproto.Total{{Key: wire(key("a")), Total: 12.5}}})

	now = now.Add(2 * time.Minute)
	assert.Empty(t, s.nextRequest().Keys, "idle: no longer synced")
	total, synced := s.Total(key("a"))
	assert.True(t, synced, "its last total is still known")
	assert.Equal(t, 12.5, total)
	assert.Len(t, s.nextRequest().Keys, 1, "used again: synced again")

	now = november.Add(keepIdleTotalsFor + time.Hour)
	assert.Empty(t, s.nextRequest().Keys)
	_, synced = s.Total(key("a"))
	assert.False(t, synced, "forgotten once its window can no longer be current")
}
