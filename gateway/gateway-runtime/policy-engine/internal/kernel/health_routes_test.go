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

package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/health"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
)

// requestHeadersFor builds the first message Envoy sends for a request on routeName.
func requestHeadersFor(routeName string) *extprocv3.ProcessingRequest {
	return &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{
				Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
					{Key: ":path", RawValue: []byte("/_gateway-health/ready")},
					{Key: ":method", RawValue: []byte("GET")},
				}},
				EndOfStream: true,
			},
		},
		Attributes: map[string]*structpb.Struct{
			constants.ExtProcFilter: {Fields: map[string]*structpb.Value{
				"xds.route_name": structpb.NewStringValue(routeName),
			}},
		},
	}
}

func newRuntimeChecker(healthyStatus string, checks map[string]health.CheckFunc) *health.Checker {
	c := health.NewChecker(health.Options{
		HealthyStatus: healthyStatus,
		CacheTTL:      time.Nanosecond,
	})
	for name, fn := range checks {
		c.Register(name, fn, health.WithTimeout(200*time.Millisecond))
	}
	return c
}

func up(context.Context) error { return nil }

type renderedHealth struct {
	Status string `json:"status"`
	Checks map[string]struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	} `json:"checks"`
}

func decodeHealth(t *testing.T, resp *extprocv3.ProcessingResponse) (int, renderedHealth, map[string]string) {
	t.Helper()
	imm := resp.GetImmediateResponse()
	require.NotNil(t, imm, "a health route is answered with an immediate response")
	headers := map[string]string{}
	for _, h := range imm.GetHeaders().GetSetHeaders() {
		headers[h.GetHeader().GetKey()] = string(h.GetHeader().GetRawValue()) + h.GetHeader().GetValue()
	}
	var body renderedHealth
	require.NoError(t, json.Unmarshal(imm.GetBody(), &body), "body %q", imm.GetBody())
	return int(imm.GetStatus().GetCode()), body, headers
}

func TestHealthRouteResponse(t *testing.T) {
	server := NewExternalProcessorServer(NewKernel(), nil, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes, nil,
		WithHealthRoutes(
			newRuntimeChecker("healthy", nil),
			newRuntimeChecker("ready", map[string]health.CheckFunc{
				"xds_connection": func(context.Context) error { return health.Reason("no configuration received yet") },
			})))

	resp, ok := server.healthRouteResponse(requestHeadersFor(gatewayHealthyRouteName))
	require.True(t, ok)
	code, body, headers := decodeHealth(t, resp)
	assert.Equal(t, 200, code)
	assert.Equal(t, "healthy", body.Status)
	assert.Empty(t, body.Checks, "liveness has no checks of its own")
	assert.Equal(t, "application/json", headers["content-type"])
	assert.Equal(t, "no-store", headers["cache-control"])

	resp, ok = server.healthRouteResponse(requestHeadersFor(gatewayReadyRouteName))
	require.True(t, ok)
	code, body, _ = decodeHealth(t, resp)
	assert.Equal(t, 503, code)
	assert.Equal(t, "unhealthy", body.Status)
	assert.Equal(t, "down", body.Checks["xds_connection"].Status)
	assert.Equal(t, "no configuration received yet", body.Checks["xds_connection"].Error)

	_, ok = server.healthRouteResponse(requestHeadersFor("GET|/pets|localhost"))
	assert.False(t, ok, "other routes take the normal path")
	_, ok = server.healthRouteResponse(&extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestBody{RequestBody: &extprocv3.HttpBody{}},
	})
	assert.False(t, ok, "only request headers are answered")

	plain := NewExternalProcessorServer(NewKernel(), nil, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes, nil)
	_, ok = plain.healthRouteResponse(requestHeadersFor(gatewayReadyRouteName))
	assert.False(t, ok, "without health checkers the route takes the normal path")
}

// countingCounterVec counts every increment, whatever the labels.
type countingCounterVec struct {
	mu sync.Mutex
	n  int
}

func (c *countingCounterVec) WithLabelValues(...string) metrics.Counter { return countingCounter{c} }
func (c *countingCounterVec) With(prometheus.Labels) metrics.Counter    { return countingCounter{c} }
func (c *countingCounterVec) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

type countingCounter struct{ vec *countingCounterVec }

func (c countingCounter) Inc()        { c.Add(1) }
func (c countingCounter) Add(float64) { c.vec.mu.Lock(); c.vec.n++; c.vec.mu.Unlock() }

type countingHistogramVec struct{ countingCounterVec }

func (h *countingHistogramVec) WithLabelValues(...string) metrics.Histogram {
	return countingHistogram{h}
}
func (h *countingHistogramVec) With(prometheus.Labels) metrics.Histogram { return countingHistogram{h} }

type countingHistogram struct{ vec *countingHistogramVec }

func (h countingHistogram) Observe(float64) { countingCounter{&h.vec.countingCounterVec}.Inc() }

// recordRequestMetrics swaps the per-request metrics for counting ones for the
// duration of the test.
func recordRequestMetrics(t *testing.T) (requests, durations, errs, lookups *countingCounterVec) {
	t.Helper()
	metrics.Init()
	prevReq, prevDur, prevErr, prevLookup := metrics.RequestsTotal, metrics.RequestDurationSeconds, metrics.RequestErrorsTotal, metrics.RouteLookupFailuresTotal
	requests, errs, lookups = &countingCounterVec{}, &countingCounterVec{}, &countingCounterVec{}
	dur := &countingHistogramVec{}
	metrics.RequestsTotal, metrics.RequestDurationSeconds, metrics.RequestErrorsTotal = requests, dur, errs
	metrics.RouteLookupFailuresTotal = countingCounter{lookups}
	t.Cleanup(func() {
		metrics.RequestsTotal, metrics.RequestDurationSeconds, metrics.RequestErrorsTotal = prevReq, prevDur, prevErr
		metrics.RouteLookupFailuresTotal = prevLookup
	})
	return requests, &dur.countingCounterVec, errs, lookups
}

func TestProcess_HealthRouteIsAnsweredBeforeAnyPolicyOrMetric(t *testing.T) {
	requests, durations, errs, lookups := recordRequestMetrics(t)
	server, k, spans := newSpanStatusServer(t)
	WithHealthRoutes(newRuntimeChecker("healthy", nil), newRuntimeChecker("ready", map[string]health.CheckFunc{"xds_connection": up}))(server)
	// A chain that would deny every request: if it ran, the probe would get a 403.
	registerTestRoute(k, gatewayReadyRouteName, buildChainWithPolicy(t, "deny-all", &denyHeaderPolicy{statusCode: 403}, map[string]interface{}{}))

	for _, route := range []string{gatewayReadyRouteName, gatewayHealthyRouteName} {
		stream := newMockStream([]*extprocv3.ProcessingRequest{requestHeadersFor(route)})

		require.NoError(t, server.Process(stream))

		require.Len(t, stream.responses, 1, route)
		code, _, _ := decodeHealth(t, stream.responses[0])
		assert.Equal(t, 200, code, "%s: answered by the health checker, not the route's policies", route)
	}
	assert.Zero(t, requests.count(), "no request metric")
	assert.Zero(t, durations.count(), "no request duration")
	assert.Zero(t, errs.count(), "no request error metric")
	assert.Zero(t, lookups.count(), "no route lookup")
	assert.Empty(t, spans.Ended(), "no span")
}

func TestProcess_OtherRoutesAreUnchanged(t *testing.T) {
	requests, _, _, lookups := recordRequestMetrics(t)
	server, _, _ := newSpanStatusServer(t)
	WithHealthRoutes(newRuntimeChecker("healthy", nil), newRuntimeChecker("ready", nil))(server)
	stream := newMockStream([]*extprocv3.ProcessingRequest{requestHeadersFor("GET|/pets|localhost")})

	require.NoError(t, server.Process(stream))

	require.Len(t, stream.responses, 1)
	assert.Equal(t, uint32(500), uint32(stream.responses[0].GetImmediateResponse().GetStatus().GetCode()),
		"a route with no policy chain still gets the existing 500")
	assert.Positive(t, requests.count())
	assert.Equal(t, 1, lookups.count())
}

func TestProcess_HealthRouteSendFailure(t *testing.T) {
	server, _, _ := newSpanStatusServer(t)
	WithHealthRoutes(newRuntimeChecker("healthy", nil), newRuntimeChecker("ready", nil))(server)
	stream := newMockStream([]*extprocv3.ProcessingRequest{requestHeadersFor(gatewayReadyRouteName)})
	stream.sendErr = errors.New("stream closed")

	assert.Error(t, server.Process(stream))
}

// A slow dependency cannot stretch the health answer: the readiness route answers 503
// with a timeout reason within the check timeout, well before Envoy would time out.
func TestProcess_HealthRouteAnswerIsBoundedByTheCheckTimeout(t *testing.T) {
	server, _, _ := newSpanStatusServer(t)
	WithHealthRoutes(newRuntimeChecker("healthy", nil), newRuntimeChecker("ready", map[string]health.CheckFunc{
		"python_executor": func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() },
	}))(server)
	stream := newMockStream([]*extprocv3.ProcessingRequest{requestHeadersFor(gatewayReadyRouteName)})

	start := time.Now()
	require.NoError(t, server.Process(stream))
	elapsed := time.Since(start)

	code, body, _ := decodeHealth(t, stream.responses[0])
	assert.Equal(t, 503, code)
	assert.Equal(t, "timed out after 200ms", body.Checks["python_executor"].Error)
	assert.Less(t, elapsed, 700*time.Millisecond, "answered within the check timeout")
}
