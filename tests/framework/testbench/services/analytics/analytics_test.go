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

package analytics

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func analyticsEventFor(uri string) Event {
	return Event{
		Request: RequestDetails{
			URI:     uri,
			Verb:    http.MethodGet,
			Headers: HeaderValues{"X-Test": {"request"}},
		},
		Response: ResponseDetails{
			Status:  http.StatusOK,
			Headers: HeaderValues{"X-Test": {"response"}},
		},
	}
}

func serveAnalytics(service *Service, method, path string, body io.Reader, headers http.Header) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	req.Header = headers
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, req)
	return recorder
}

func readAnalyticsEvents(t *testing.T, service *Service, block string) []Event {
	t.Helper()
	response := serveAnalytics(service, http.MethodGet, "/"+block+"/test/events", nil, nil)
	require.Equal(t, http.StatusOK, response.Code)
	var events []Event
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &events))
	return events
}

func TestServiceIsolatesPartitionsAndPreservesEventHeaders(t *testing.T) {
	service := New()
	event := analyticsEventFor("/api")
	body, err := json.Marshal(event)
	require.NoError(t, err)

	response := serveAnalytics(service, http.MethodPost, "/one/v1/events", bytes.NewReader(body), nil)
	require.Equal(t, http.StatusCreated, response.Code)
	require.Equal(t, "application/json", response.Header().Get("Content-Type"))

	response = serveAnalytics(service, http.MethodPost, "/two/v1/events", bytes.NewReader(body), nil)
	require.Equal(t, http.StatusCreated, response.Code)

	one := readAnalyticsEvents(t, service, "one")
	two := readAnalyticsEvents(t, service, "two")
	require.Len(t, one, 1)
	require.Empty(t, two[0].Request.Headers["X-Other"])
	require.Equal(t, []string{"request"}, one[0].Request.Headers["X-Test"])
	require.Equal(t, []string{"response"}, one[0].Response.Headers["X-Test"])
}

func TestServiceAcceptsMultiValuedAndScalarHeaders(t *testing.T) {
	service := New()
	body := bytes.NewBufferString(`{"request":{"uri":"/headers","headers":{"accept":["application/json","text/plain"]}},"response":{"headers":{"content-type":"application/json"}}}`)

	response := serveAnalytics(service, http.MethodPost, "/block/v1/events", body, nil)
	require.Equal(t, http.StatusCreated, response.Code)

	events := readAnalyticsEvents(t, service, "block")
	require.Len(t, events, 1)
	require.Equal(t, []string{"application/json", "text/plain"}, events[0].Request.Headers["accept"])
	require.Equal(t, []string{"application/json"}, events[0].Response.Headers["content-type"])
}

func TestServiceAcceptsBatchAndKeepsCountConsistent(t *testing.T) {
	service := New()
	events := []Event{analyticsEventFor("/one"), analyticsEventFor("/two")}
	body, err := json.Marshal(events)
	require.NoError(t, err)

	response := serveAnalytics(service, http.MethodPost, "/block/v1/events/batch", bytes.NewReader(body), nil)
	require.Equal(t, http.StatusCreated, response.Code)
	require.Equal(t, "application/json", response.Header().Get("Content-Type"))

	var result map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	require.Equal(t, float64(2), result["count"])
	require.Len(t, readAnalyticsEvents(t, service, "block"), 2)

	response = serveAnalytics(service, http.MethodGet, "/block/test/events/count", nil, nil)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "application/json", response.Header().Get("Content-Type"))
	require.JSONEq(t, `{"count":2}`, response.Body.String())
}

func TestServiceRejectsMalformedAndOversizedPayloads(t *testing.T) {
	service := New()
	response := serveAnalytics(service, http.MethodPost, "/block/v1/events", bytes.NewBufferString("{"), nil)
	require.Equal(t, http.StatusBadRequest, response.Code)

	// A syntactically valid (if truncated) JSON string, so the failure exercises the size
	// limit rather than a syntax error encountered before the limit is ever reached.
	oversized := append([]byte(`"`), bytes.Repeat([]byte("x"), maxBodyBytes+1)...)
	response = serveAnalytics(service, http.MethodPost, "/block/v1/events", bytes.NewReader(oversized), nil)
	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
	require.Empty(t, readAnalyticsEvents(t, service, "block"))
}

func TestServiceAcceptsGzipPayload(t *testing.T) {
	service := New()
	event, err := json.Marshal(analyticsEventFor("/gzip"))
	require.NoError(t, err)
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, err = writer.Write(event)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	response := serveAnalytics(service, http.MethodPost, "/block/v1/events", &compressed,
		http.Header{"Content-Encoding": []string{"gzip"}})
	require.Equal(t, http.StatusCreated, response.Code)
	require.Len(t, readAnalyticsEvents(t, service, "block"), 1)
}

func TestServiceRejectsInvalidAndReservedPartitions(t *testing.T) {
	service := New()
	for _, path := range []string{
		"/v1/events", "/test/v1/events", "/testbench/v1/events", "/Block/v1/events", "/block_name/v1/events",
	} {
		response := serveAnalytics(service, http.MethodPost, path, bytes.NewBufferString("{}"), nil)
		require.Equal(t, http.StatusBadRequest, response.Code, path)
	}
}

func TestServiceSupportsConcurrentIngestion(t *testing.T) {
	service := New()
	const total = 64
	var wg sync.WaitGroup
	for i := range total {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			event, err := json.Marshal(analyticsEventFor("/" + strconv.Itoa(i)))
			if err != nil {
				t.Errorf("marshal event: %v", err)
				return
			}
			response := serveAnalytics(service, http.MethodPost, "/block/v1/events", bytes.NewReader(event), nil)
			if response.Code != http.StatusCreated {
				t.Errorf("status = %d, want %d", response.Code, http.StatusCreated)
			}
		}(i)
	}
	wg.Wait()
	require.Len(t, readAnalyticsEvents(t, service, "block"), total)
}

func TestResetIsSafeWithConcurrentIngestion(t *testing.T) {
	service := New()
	var wg sync.WaitGroup
	for i := range 32 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			event, _ := json.Marshal(analyticsEventFor("/" + strconv.Itoa(i)))
			serveAnalytics(service, http.MethodPost, "/block/v1/events", bytes.NewReader(event), nil)
		}(i)
	}
	response := serveAnalytics(service, http.MethodPost, "/block/test/reset", nil, nil)
	require.Equal(t, http.StatusOK, response.Code)
	wg.Wait()

	response = serveAnalytics(service, http.MethodPost, "/block/test/reset", nil, nil)
	require.Equal(t, http.StatusOK, response.Code)
	require.Empty(t, readAnalyticsEvents(t, service, "block"))
}

func TestServiceReturnsHealthForAValidPartition(t *testing.T) {
	response := serveAnalytics(New(), http.MethodGet, "/block/test/health", nil, nil)
	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"status":"ok","service":"analytics"}`, response.Body.String())
}
