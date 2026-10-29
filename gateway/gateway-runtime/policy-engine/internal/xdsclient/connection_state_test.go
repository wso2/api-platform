/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package xdsclient

import (
	"io"
	"sync"
	"testing"
	"time"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
)

// recordingGaugeVec is a GaugeVec that keeps each labelled value in memory, so tests can
// read what the client reported.
type recordingGaugeVec struct {
	mu     sync.Mutex
	values map[string]float64
}

type recordingGauge struct {
	vec   *recordingGaugeVec
	label string
}

func newRecordingGaugeVec() *recordingGaugeVec {
	return &recordingGaugeVec{values: map[string]float64{}}
}

func (v *recordingGaugeVec) WithLabelValues(labels ...string) metrics.Gauge {
	return recordingGauge{vec: v, label: labels[0]}
}

func (v *recordingGaugeVec) With(l prometheus.Labels) metrics.Gauge {
	for _, value := range l {
		return recordingGauge{vec: v, label: value}
	}
	return recordingGauge{vec: v}
}

func (v *recordingGaugeVec) value(label string) float64 {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.values[label]
}

func (g recordingGauge) Set(f float64) { g.add(f, true) }
func (g recordingGauge) Inc()          { g.add(1, false) }
func (g recordingGauge) Dec()          { g.add(-1, false) }
func (g recordingGauge) Add(f float64) { g.add(f, false) }
func (g recordingGauge) Sub(f float64) { g.add(-f, false) }

func (g recordingGauge) add(f float64, set bool) {
	g.vec.mu.Lock()
	defer g.vec.mu.Unlock()
	if set {
		g.vec.values[g.label] = f
		return
	}
	g.vec.values[g.label] += f
}

// recordConnectionMetrics swaps the connection metrics for recording ones for the
// duration of the test.
func recordConnectionMetrics(t *testing.T) (active, state *recordingGaugeVec) {
	t.Helper()
	prevActive, prevState := metrics.GRPCConnectionsActive, metrics.XDSConnectionState
	active, state = newRecordingGaugeVec(), newRecordingGaugeVec()
	metrics.GRPCConnectionsActive, metrics.XDSConnectionState = active, state
	t.Cleanup(func() { metrics.GRPCConnectionsActive, metrics.XDSConnectionState = prevActive, prevState })
	return active, state
}

func newTestClient(t *testing.T) *Client {
	t.Helper()
	k, reg := createTestKernelAndRegistry(t)
	client, err := NewClient(createValidTestConfig(), k, reg, resolver.DefaultRegistry())
	require.NoError(t, err)
	return client
}

func TestSetState_ActiveConnectionGaugeFollowsTheStream(t *testing.T) {
	active, connected := recordConnectionMetrics(t)
	client := newTestClient(t)

	for cycle := 0; cycle < 3; cycle++ {
		client.setState(StateConnecting)
		client.setState(StateConnected)
		assert.Equal(t, 1.0, active.value("xds"), "one established stream, cycle %d", cycle)
		assert.Equal(t, 1.0, connected.value("connected"), "cycle %d", cycle)

		client.setState(StateReconnecting)
		assert.Equal(t, 0.0, active.value("xds"), "no stream while reconnecting, cycle %d", cycle)
		assert.Equal(t, 0.0, connected.value("connected"), "cycle %d", cycle)
	}

	client.setState(StateConnecting)
	client.setState(StateConnected)
	client.setState(StateStopped)
	assert.Equal(t, 0.0, active.value("xds"))
	assert.Equal(t, 0.0, connected.value("connected"))
}

func TestSetState_ActiveConnectionGaugeNeverNegative(t *testing.T) {
	active, _ := recordConnectionMetrics(t)
	client := newTestClient(t)

	client.setState(StateConnecting)
	client.setState(StateReconnecting)
	client.setState(StateDisconnected)
	client.setState(StateStopped)

	assert.Equal(t, 0.0, active.value("xds"))
}

// fakeClock is a settable clock for connection state.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = t
}

func newTestClientWithClock(t *testing.T) (*Client, *fakeClock) {
	t.Helper()
	client := newTestClient(t)
	clock := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	client.mu.Lock()
	client.now = clock.Now
	client.disconnectedSince = clock.Now()
	client.mu.Unlock()
	return client, clock
}

func TestConnectionStatus_NewClientIsDisconnectedSinceCreation(t *testing.T) {
	before := time.Now()
	status := newTestClient(t).ConnectionStatus()

	assert.Equal(t, StateDisconnected, status.State)
	assert.False(t, status.Connected)
	assert.False(t, status.ConfigReceived)
	assert.False(t, status.DisconnectedSince.Before(before), "disconnected since the client was created")
}

func TestConnectionStatus_TracksWhenTheConnectionWasLost(t *testing.T) {
	recordConnectionMetrics(t)
	client, clock := newTestClientWithClock(t)
	start := clock.Now()

	client.setState(StateConnecting)
	client.setState(StateConnected)
	status := client.ConnectionStatus()
	assert.True(t, status.Connected)
	assert.True(t, status.DisconnectedSince.IsZero(), "cleared while connected")

	lost := start.Add(10 * time.Minute)
	clock.set(lost)
	client.setState(StateReconnecting)
	assert.Equal(t, lost, client.ConnectionStatus().DisconnectedSince)

	// Failed reconnect attempts keep the start of the outage.
	for i := 1; i <= 3; i++ {
		clock.set(lost.Add(time.Duration(i) * time.Minute))
		client.setState(StateConnecting)
		client.setState(StateReconnecting)
	}
	status = client.ConnectionStatus()
	assert.False(t, status.Connected)
	assert.Equal(t, StateReconnecting, status.State)
	assert.Equal(t, lost, status.DisconnectedSince, "the outage started at the first loss")

	client.setState(StateConnecting)
	client.setState(StateConnected)
	assert.True(t, client.ConnectionStatus().DisconnectedSince.IsZero(), "cleared on reconnect")
}

// runStream feeds responses to processStream and waits for it to finish.
func runStream(t *testing.T, client *Client, responses ...*discoveryv3.DiscoveryResponse) {
	t.Helper()
	stream := &mockADSStream{recvResponses: responses}
	client.mu.Lock()
	client.stream = stream
	client.mu.Unlock()
	assert.Equal(t, io.EOF, client.processStream(stream))
}

func TestConnectionStatus_ConfigReceivedOnAnyAcceptedResponse(t *testing.T) {
	client := newTestClient(t)

	// A response that is rejected does not count.
	runStream(t, client, &discoveryv3.DiscoveryResponse{
		TypeUrl: "type.unknown.com/UnknownType", VersionInfo: "v1", Nonce: "n1", Resources: []*anypb.Any{},
	})
	assert.False(t, client.ConnectionStatus().ConfigReceived)

	// Any accepted resource type counts, not only policy chains.
	runStream(t, client, &discoveryv3.DiscoveryResponse{
		TypeUrl: RouteConfigTypeURL, VersionInfo: "v1", Nonce: "n2", Resources: []*anypb.Any{},
	})
	assert.True(t, client.ConnectionStatus().ConfigReceived)

	// It stays set across a lost connection.
	recordConnectionMetrics(t)
	client.setState(StateConnected)
	client.setState(StateReconnecting)
	assert.True(t, client.ConnectionStatus().ConfigReceived)
}
