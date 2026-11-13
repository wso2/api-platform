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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
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
	s := NewService("engine-1")
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
	s.apply(req, &counterproto.SyncResponse{ReportID: "1", Totals: []counterproto.Total{{Key: wire(key("a")), Total: 10}}})

	total, ok = s.Total(key("a"))
	assert.Equal(t, 12.0, total, "shared 10 plus the 2 not yet reported")
	assert.True(t, ok)
}

func TestReportIsResentUntilAcknowledged(t *testing.T) {
	s := NewService("engine-1")
	s.Add(key("a"), 1)
	first := s.nextRequest()

	s.Add(key("a"), 5)
	s.apply(first, &counterproto.SyncResponse{}) // the store failed: no acknowledgement
	again := s.nextRequest()

	assert.Equal(t, first.ReportID, again.ReportID)
	assert.Equal(t, first.Adds, again.Adds, "the same adds, so the store applies them once")

	s.apply(again, &counterproto.SyncResponse{ReportID: again.ReportID})
	next := s.nextRequest()
	assert.Equal(t, "2", next.ReportID)
	require.Len(t, next.Adds, 1)
	assert.Equal(t, 5.0, next.Adds[0].Delta)
}

func TestIdleCountersStopBeingSynced(t *testing.T) {
	s := NewService("engine-1")
	now := november
	s.now = func() time.Time { return now }
	s.Total(key("a"))
	assert.Len(t, s.nextRequest().Keys, 1)

	now = now.Add(watchFor + time.Second)

	assert.Empty(t, s.nextRequest().Keys)
}

func TestKeysForTheSameWindowAreEqual(t *testing.T) {
	s := NewService("engine-1")
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
		marker := req.Source + "/" + req.ReportID
		if len(req.Adds) > 0 && !c.applied[marker] {
			c.applied[marker] = true
			for _, a := range req.Adds {
				c.totals[a.Key] += a.Delta
			}
		}
		resp := &counterproto.SyncResponse{ReportID: req.ReportID}
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

	engines := []*Service{NewService("engine-1"), NewService("engine-2")}
	for _, s := range engines {
		syncer := &Syncer{
			Service: s, ServerAddress: "passthrough:///bufnet", Credentials: insecure.NewCredentials(),
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
