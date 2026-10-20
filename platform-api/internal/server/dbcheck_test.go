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

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/health"
)

// pingFunc adapts a function to the pinger interface.
type pingFunc func(ctx context.Context) error

func (f pingFunc) PingContext(ctx context.Context) error { return f(ctx) }

// newTestChecker returns a platform-api checker with only the database check, a short
// timeout and no caching, so every evaluation pings.
func newTestChecker(db pinger) *health.Checker {
	c := health.NewChecker(health.Options{
		Component:     "platform-api",
		HealthyStatus: "ok",
		CheckTimeout:  200 * time.Millisecond,
		CacheTTL:      time.Nanosecond,
	})
	c.Register("database", newDatabaseCheck(db))
	return c
}

type renderedCheck struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Since  string `json:"since"`
}

type renderedBody struct {
	Status    string                   `json:"status"`
	Component string                   `json:"component"`
	Checks    map[string]renderedCheck `json:"checks"`
}

func evaluateDatabase(t *testing.T, c *health.Checker) (renderedBody, string) {
	t.Helper()
	raw, err := health.RenderBody(c.Evaluate())
	if err != nil {
		t.Fatalf("RenderBody: %v", err)
	}
	var b renderedBody
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("unmarshal %q: %v", raw, err)
	}
	return b, string(raw)
}

func TestDatabaseCheckUp(t *testing.T) {
	c := newTestChecker(pingFunc(func(context.Context) error { return nil }))

	b, _ := evaluateDatabase(t, c)

	if b.Status != "ok" || b.Checks["database"].Status != "up" {
		t.Fatalf("got %+v, want ok with database up", b)
	}
}

func TestDatabaseCheckUnreachableHidesDriverError(t *testing.T) {
	driverErr := errors.New(`failed to connect to host=db.internal user=admin password=s3cret: connection refused`)
	c := newTestChecker(pingFunc(func(context.Context) error { return driverErr }))

	b, raw := evaluateDatabase(t, c)

	if b.Status != "unhealthy" {
		t.Errorf("status = %q, want unhealthy", b.Status)
	}
	db := b.Checks["database"]
	if db.Status != "down" || db.Error != "database unreachable" || db.Since == "" {
		t.Errorf("database check = %+v, want down / database unreachable with since", db)
	}
	for _, leak := range []string{"db.internal", "admin", "s3cret", "connection refused"} {
		if strings.Contains(raw, leak) {
			t.Errorf("body leaks %q: %s", leak, raw)
		}
	}
}

func TestDatabaseCheckKeepsDriverErrorAsCause(t *testing.T) {
	driverErr := errors.New("connection refused")
	check := newDatabaseCheck(pingFunc(func(context.Context) error { return driverErr }))

	err := check(context.Background())

	if !errors.Is(err, errDatabaseUnreachable) || !errors.Is(err, driverErr) {
		t.Fatalf("err = %v, want both the reason and the driver error wrapped", err)
	}
}

func TestDatabaseCheckTimeout(t *testing.T) {
	c := newTestChecker(pingFunc(func(ctx context.Context) error {
		<-ctx.Done()
		return errors.New("driver: canceling query due to user request")
	}))

	b, _ := evaluateDatabase(t, c)

	if got := b.Checks["database"]; got.Status != "down" || got.Error != "timed out after 200ms" {
		t.Fatalf("database check = %+v, want down / timed out after 200ms", got)
	}
}

func TestHealthRouteReflectsDatabase(t *testing.T) {
	var dbErr error
	c := newTestChecker(pingFunc(func(context.Context) error { return dbErr }))
	mux := http.NewServeMux()
	mux.Handle("GET /health", c.Handler())

	get := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
		return rec
	}

	if rec := get(); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("healthy database: got %d %s", rec.Code, rec.Body.String())
	}
	dbErr = errors.New("connection refused")
	if rec := get(); rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"status":"unhealthy"`) {
		t.Fatalf("database down: got %d %s", rec.Code, rec.Body.String())
	}
	dbErr = nil
	if rec := get(); rec.Code != http.StatusOK {
		t.Fatalf("database back: got %d %s", rec.Code, rec.Body.String())
	}
}

func TestDatabaseCheckDoesNotStackPingsOnHungDatabase(t *testing.T) {
	release := make(chan struct{})
	var pings atomic.Int32
	// A driver that ignores its context: the ping blocks until released.
	check := newDatabaseCheck(pingFunc(func(context.Context) error {
		pings.Add(1)
		<-release
		return nil
	}))
	call := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		return check(ctx)
	}

	if err := call(); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("first call: err = %v, want a timeout", err)
	}
	if err := call(); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second call: err = %v, want a timeout", err)
	}
	if n := pings.Load(); n != 1 {
		t.Fatalf("pings = %d while the first is still running, want 1", n)
	}

	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for {
		err := call()
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("check never recovered after the hung ping returned: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if n := pings.Load(); n != 2 {
		t.Fatalf("pings = %d, want exactly one new ping after the hung one returned", n)
	}
}

func TestDatabaseCheckReportsHungPingAsTimeout(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	c := newTestChecker(pingFunc(func(context.Context) error {
		<-release
		return nil
	}))

	for i := 0; i < 3; i++ {
		b, _ := evaluateDatabase(t, c)
		if got := b.Checks["database"]; got.Status != "down" || got.Error != "timed out after 200ms" {
			t.Fatalf("evaluation %d: database check = %+v, want down / timed out after 200ms", i, got)
		}
	}
}
