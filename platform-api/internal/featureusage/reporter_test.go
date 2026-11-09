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

package featureusage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
)

const (
	testCompanyUUID = "b3e1c2d4-7f6a-4e5b-9c8d-0a1b2c3d4e5f"
	testUserUUID    = "0193e7a2-4c5d-7e6f-8a9b-0c1d2e3f4a5b"
)

// syncBuffer is a log sink safe to write from the delivery workers.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type fakeAttributor struct {
	companyID string
	userID    string
	err       error
}

func (a fakeAttributor) CompanyID(string) (string, error) { return a.companyID, a.err }
func (a fakeAttributor) UserID(string) (string, error)    { return a.userID, nil }

// fakeTokens hands out "token-1", "token-2", ... and counts fetches.
type fakeTokens struct {
	mu      sync.Mutex
	current string
	fetches int
}

func (f *fakeTokens) Token(context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.current == "" {
		f.fetches++
		f.current = "token-" + string(rune('0'+f.fetches))
	}
	return f.current, nil
}

func (f *fakeTokens) Invalidate(token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.current == token {
		f.current = ""
	}
}

type recordedRequest struct {
	method, path, auth, contentType string
	body                            map[string]any
}

// collector is a fake collector proxy answering each request with the next
// status from statuses (the last one repeats).
type collector struct {
	t        *testing.T
	mu       sync.Mutex
	requests []recordedRequest
	statuses []int
	respBody string
	hold     chan struct{} // when set, every request waits on it
}

func (c *collector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if c.hold != nil {
		<-c.hold
	}
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		c.t.Errorf("collector received a non-JSON body: %v", err)
	}
	c.mu.Lock()
	c.requests = append(c.requests, recordedRequest{
		method: r.Method, path: r.URL.Path, auth: r.Header.Get("Authorization"),
		contentType: r.Header.Get("Content-Type"), body: body,
	})
	status := c.statuses[min(len(c.requests), len(c.statuses))-1]
	c.mu.Unlock()
	w.WriteHeader(status)
	_, _ = io.WriteString(w, c.respBody)
}

func (c *collector) recorded() []recordedRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]recordedRequest(nil), c.requests...)
}

type testPublisher struct {
	*Publisher
	collector *collector
	tokens    *fakeTokens
	logs      *syncBuffer
}

func newTestPublisher(t *testing.T, attributor Attributor, statuses []int, tune func(*Options)) *testPublisher {
	t.Helper()
	c := &collector{t: t, statuses: statuses}
	server := httptest.NewServer(c)
	t.Cleanup(server.Close)

	logs := &syncBuffer{}
	tokens := &fakeTokens{}
	opts := Options{
		CollectorURL:   server.URL + "/moesif-collector/",
		HTTPClient:     server.Client(),
		Tokens:         tokens,
		Attributor:     attributor,
		Logger:         slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
		QueueSize:      8,
		Workers:        1,
		MaxAttempts:    3,
		RetryBackoff:   time.Millisecond,
		AttemptTimeout: 2 * time.Second,
		OverallTimeout: 5 * time.Second,
	}
	if tune != nil {
		tune(&opts)
	}
	p, err := NewPublisher(opts)
	if err != nil {
		t.Fatal(err)
	}
	return &testPublisher{Publisher: p, collector: c, tokens: tokens, logs: logs}
}

// flush closes the publisher, which waits for queued deliveries to finish.
func (p *testPublisher) flush(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestPublisherSendsAnAPIEvent(t *testing.T) {
	p := newTestPublisher(t, fakeAttributor{companyID: testCompanyUUID, userID: testUserUUID}, []int{http.StatusCreated}, nil)
	p.Record(validCreation())
	p.flush(t)

	requests := p.collector.recorded()
	if len(requests) != 1 {
		t.Fatalf("collector got %d requests, want 1", len(requests))
	}
	req := requests[0]
	if req.method != http.MethodPost || req.path != "/moesif-collector/v1/events" {
		t.Errorf("request = %s %s, want POST /moesif-collector/v1/events", req.method, req.path)
	}
	if req.auth != "Bearer token-1" || req.contentType != "application/json" {
		t.Errorf("auth = %q, content-type = %q", req.auth, req.contentType)
	}

	body := req.body
	if body["company_id"] != testCompanyUUID || body["user_id"] != testUserUUID {
		t.Errorf("unexpected identity fields: %v", body)
	}
	if tx, _ := body["transaction_id"].(string); !canonicalUUID(tx) {
		t.Errorf("transaction_id = %v, want a UUID", body["transaction_id"])
	}

	// The call is described by time, route URI and verb only: no caller IP
	// address or body, and the headers Moesif requires are sent empty.
	request, _ := body["request"].(map[string]any)
	want := map[string]any{"time": "2026-11-06T02:45:00Z", "uri": testURI, "verb": "POST", "headers": map[string]any{}}
	if !reflect.DeepEqual(request, want) {
		t.Errorf("request = %v, want exactly %v", request, want)
	}
	response, _ := body["response"].(map[string]any)
	wantResponse := map[string]any{"time": "2026-11-06T02:45:00.25Z", "status": float64(201), "headers": map[string]any{}}
	if !reflect.DeepEqual(response, wantResponse) {
		t.Errorf("response = %v, want exactly %v", response, wantResponse)
	}

	metadata, _ := body["metadata"].(map[string]any)
	if metadata["growth_action"] != FeatureCreateRESTAPI || metadata["platform"] != "WSO2 API Platform" ||
		metadata["source"] != "api" || metadata["creation_method"] != "contract" ||
		metadata["has_sandbox_upstream"] != true || metadata["action"] != "REST API Creation" ||
		metadata["category"] != "manage-apis" {
		t.Errorf("metadata = %v", metadata)
	}
	// Nothing beyond the Events API fields leaves the process.
	for key := range body {
		switch key {
		case "request", "response", "transaction_id", "user_id", "company_id", "metadata":
		default:
			t.Errorf("unexpected top-level field %q", key)
		}
	}
	if strings.Contains(p.logs.String(), testAPIUUID) {
		t.Error("the API UUID must not be logged or sent")
	}
}

// A failed call is reported too, with the status it answered.
func TestPublisherSendsAFailedCall(t *testing.T) {
	p := newTestPublisher(t, fakeAttributor{companyID: testCompanyUUID}, []int{http.StatusOK}, nil)
	o := validCreation()
	o.Invocation.Status = http.StatusConflict
	p.Record(o)
	p.flush(t)
	requests := p.collector.recorded()
	if len(requests) != 1 {
		t.Fatalf("collector got %d requests for a failed call, want 1", len(requests))
	}
	if response, _ := requests[0].body["response"].(map[string]any); response["status"] != float64(http.StatusConflict) {
		t.Errorf("response = %v, want status 409", response)
	}
}

func TestPublisherLeavesOutAUserThatIsNotAUUID(t *testing.T) {
	for _, subject := range []string{"alice@example.com", "sa:acme:ci-bot:" + testUserUUID, "113246589012", ""} {
		t.Run(subject, func(t *testing.T) {
			p := newTestPublisher(t, fakeAttributor{companyID: testCompanyUUID, userID: subject}, []int{http.StatusOK}, nil)
			p.Record(validCreation())
			p.flush(t)
			requests := p.collector.recorded()
			if len(requests) != 1 {
				t.Fatalf("collector got %d requests, want 1 (an unattributed action is still sent)", len(requests))
			}
			if _, present := requests[0].body["user_id"]; present {
				t.Errorf("user_id was sent for subject %q", subject)
			}
		})
	}
}

func TestPublisherDropsAnOrganizationThatIsNotAUUID(t *testing.T) {
	for name, attributor := range map[string]fakeAttributor{
		"a handle":       {companyID: "acme-corp"},
		"a lookup error": {err: errors.New("db down")},
	} {
		t.Run(name, func(t *testing.T) {
			p := newTestPublisher(t, attributor, []int{http.StatusOK}, nil)
			p.Record(validCreation())
			p.flush(t)
			if n := len(p.collector.recorded()); n != 0 {
				t.Fatalf("collector got %d requests, want none", n)
			}
			logs := p.logs.String()
			if !strings.Contains(logs, "REPORT_DROPPED") || strings.Contains(logs, "acme-corp") {
				t.Errorf("want a drop log without the organization value, got: %s", logs)
			}
		})
	}
}

func TestPublisherRetriesTransientFailures(t *testing.T) {
	p := newTestPublisher(t, fakeAttributor{companyID: testCompanyUUID}, []int{http.StatusServiceUnavailable, http.StatusTooManyRequests, http.StatusAccepted}, nil)
	p.Record(validCreation())
	p.flush(t)
	if n := len(p.collector.recorded()); n != 3 {
		t.Fatalf("collector got %d requests, want 3 (503, 429, then success)", n)
	}
	if strings.Contains(p.logs.String(), "REPORT_DROPPED") {
		t.Error("a delivery that eventually succeeded was logged as dropped")
	}
}

func TestPublisherGivesUpAfterMaxAttempts(t *testing.T) {
	p := newTestPublisher(t, fakeAttributor{companyID: testCompanyUUID}, []int{http.StatusBadGateway}, nil)
	p.Record(validCreation())
	p.flush(t)
	if n := len(p.collector.recorded()); n != 3 {
		t.Fatalf("collector got %d requests, want max_attempts = 3", n)
	}
	if !strings.Contains(p.logs.String(), "REPORT_DROPPED") {
		t.Error("an exhausted delivery must be logged as dropped")
	}
}

// A 4xx is Moesif's or the gateway's verdict: retrying repeats it. Its body may
// quote what was sent, so it is never logged.
func TestPublisherDoesNotRetryOrLogARejection(t *testing.T) {
	p := newTestPublisher(t, fakeAttributor{companyID: testCompanyUUID, userID: testUserUUID}, []int{http.StatusBadRequest}, nil)
	p.collector.respBody = `{"error":"bad user_id ` + testUserUUID + `"}`
	p.Record(validCreation())
	p.flush(t)

	if n := len(p.collector.recorded()); n != 1 {
		t.Fatalf("collector got %d requests, want 1", n)
	}
	logs := p.logs.String()
	if !strings.Contains(logs, "status 400") {
		t.Errorf("the drop log should name the status, got: %s", logs)
	}
	for _, leaked := range []string{testUserUUID, testCompanyUUID, "bad user_id"} {
		if strings.Contains(logs, leaked) {
			t.Errorf("logs contain %q from the rejected event or its response: %s", leaked, logs)
		}
	}
}

func TestPublisherRefreshesTheTokenOnceOn401(t *testing.T) {
	p := newTestPublisher(t, fakeAttributor{companyID: testCompanyUUID}, []int{http.StatusUnauthorized, http.StatusOK}, nil)
	p.Record(validCreation())
	p.flush(t)

	requests := p.collector.recorded()
	if len(requests) != 2 {
		t.Fatalf("collector got %d requests, want 2", len(requests))
	}
	if requests[0].auth != "Bearer token-1" || requests[1].auth != "Bearer token-2" {
		t.Errorf("auth headers = %q, %q; the retry must use a fresh token", requests[0].auth, requests[1].auth)
	}

	// A second 401 is final.
	q := newTestPublisher(t, fakeAttributor{companyID: testCompanyUUID}, []int{http.StatusUnauthorized}, nil)
	q.Record(validCreation())
	q.flush(t)
	if n := len(q.collector.recorded()); n != 2 {
		t.Fatalf("collector got %d requests, want exactly one retry after 401", n)
	}
}

func TestInvalidOccurrenceIsNeverSent(t *testing.T) {
	p := newTestPublisher(t, fakeAttributor{companyID: testCompanyUUID}, []int{http.StatusOK}, nil)
	o := validCreation()
	o.Dimensions["creation_method"] = "imported-by-alice@example.com"
	p.Record(o)
	p.flush(t)
	if n := len(p.collector.recorded()); n != 0 {
		t.Fatalf("collector got %d requests for an invalid occurrence", n)
	}
	logs := p.logs.String()
	if !strings.Contains(logs, "REPORT_DROPPED") || strings.Contains(logs, "alice") {
		t.Errorf("want a drop log without the rejected value, got: %s", logs)
	}
}

func TestRecordNeverBlocksWhenTheQueueIsFull(t *testing.T) {
	p := newTestPublisher(t, fakeAttributor{companyID: testCompanyUUID}, []int{http.StatusOK}, func(o *Options) {
		o.QueueSize = 1
		o.Workers = 1
	})
	p.collector.hold = make(chan struct{})

	started := time.Now()
	for i := 0; i < 20; i++ {
		p.Record(validCreation())
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("Record blocked for %s with a stalled collector", elapsed)
	}
	if !strings.Contains(p.logs.String(), "delivery queue is full") {
		t.Error("overflow must be logged as a drop")
	}
	close(p.collector.hold)
	p.flush(t)
	if n := len(p.collector.recorded()); n > 2 {
		t.Errorf("collector got %d requests; at most the in-flight one and the queued one may be delivered", n)
	}
}

func TestCloseIsBoundedByItsContext(t *testing.T) {
	p := newTestPublisher(t, fakeAttributor{companyID: testCompanyUUID}, []int{http.StatusOK}, nil)
	p.collector.hold = make(chan struct{})
	defer close(p.collector.hold)
	p.Record(validCreation())
	p.Record(validCreation())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := p.Close(ctx); err == nil {
		t.Error("Close must report that shutdown cut delivery short")
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Errorf("Close took %s past its deadline", elapsed)
	}
	p.Record(validCreation()) // after Close: dropped, must not panic
}

// Turned on but with no collector configured, there is nothing to send to.
func TestNewFromConfigEnabledWithoutACollectorIsNoop(t *testing.T) {
	logs := &syncBuffer{}
	r, err := NewFromConfig(config.FeatureUsage{Enabled: true}, fakeAttributor{}, slog.New(slog.NewTextHandler(logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.(noop); !ok {
		t.Fatalf("unconfigured reporting returned %T, want the no-op reporter", r)
	}
	if !strings.Contains(logs.String(), "no collector is configured") {
		t.Errorf("startup should say why nothing is reported, got: %s", logs.String())
	}
}

func TestNewFromConfigDisabledIsNoop(t *testing.T) {
	r, err := NewFromConfig(config.FeatureUsage{Enabled: false}, fakeAttributor{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.(noop); !ok {
		t.Fatalf("disabled reporting returned %T, want the no-op reporter", r)
	}
	r.Record(validCreation())
	if err := r.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestNewPublisherRejectsIncompleteOptions(t *testing.T) {
	if _, err := NewPublisher(Options{}); err == nil {
		t.Error("NewPublisher accepted empty options")
	}
}

// Guard against the fake token source drifting from the real contract.
var _ TokenSource = (*fakeTokens)(nil)
var _ TokenSource = (*clientCredentialsTokenSource)(nil)

// blockingAttributor stalls every lookup until release is closed, like a
// database query that never returns.
type blockingAttributor struct{ release chan struct{} }

func (a blockingAttributor) CompanyID(string) (string, error) {
	<-a.release
	return testCompanyUUID, nil
}
func (a blockingAttributor) UserID(string) (string, error) { return "", nil }

func TestCloseReturnsByItsDeadlineWhenALookupStalls(t *testing.T) {
	release := make(chan struct{})
	p := newTestPublisher(t, blockingAttributor{release: release}, []int{http.StatusOK}, nil)
	defer close(release) // let the stuck worker finish after the test
	p.Record(validCreation())
	p.Record(validCreation())
	time.Sleep(20 * time.Millisecond) // let a worker pick one up and block

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := p.Close(ctx); err == nil {
		t.Error("Close must report that shutdown cut delivery short")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("Close waited %s for a stalled lookup; it must return by its deadline", elapsed)
	}
	if n := len(p.collector.recorded()); n != 0 {
		t.Errorf("collector got %d requests from a delivery cut short", n)
	}
}
