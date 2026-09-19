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

package client

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/httpkit/netguard"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// fakePortal answers every https request with a redirect to location, but only for the first
// hop when redirectOnce is set; any non-https request is recorded and answered 200.
type fakePortal struct {
	location          string
	redirectOnce      bool
	redirected        bool
	httpHits          int
	httpAuthorization string
}

func (f *fakePortal) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme == "https" && !(f.redirectOnce && f.redirected) {
		f.redirected = true
		return &http.Response{
			StatusCode: http.StatusTemporaryRedirect,
			Header:     http.Header{"Location": {f.location}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	}
	if r.URL.Scheme != "https" {
		f.httpHits++
		f.httpAuthorization = r.Header.Get("Authorization")
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
}

// newClientOverFakePortal builds the retryable client over a shared client that permits http
// redirects (the shipped allowed_schemes default), then swaps in the fake portal transport.
func newClientOverFakePortal(t *testing.T, portal *fakePortal) *RetryableHTTPClient {
	t.Helper()
	policy := netguard.PermitPrivateBlockMetadata()
	policy.AllowedSchemes = []string{"http", "https"}
	utils.InitSharedHTTPClient(&http.Client{CheckRedirect: netguard.CheckRedirect(policy, 5)}, 0)
	t.Cleanup(func() { utils.InitSharedHTTPClient(nil, 0) })

	c, err := NewRetryableHTTPClient(0, 5*time.Second)
	if err != nil {
		t.Fatalf("NewRetryableHTTPClient: %v", err)
	}
	c.client.Transport = portal
	return c
}

func publishWithAuth(c *RetryableHTTPClient) (*http.Response, error) {
	req, _ := http.NewRequest(http.MethodPost, "https://portal.example/publish", strings.NewReader("x"))
	req.Header.Set("Authorization", "Bearer secret")
	return c.Do(req)
}

// flakyPortal answers 503 to the first failures requests and 200 after that, recording each request body.
type flakyPortal struct {
	failures int
	bodies   []string
}

func (f *flakyPortal) RoundTrip(r *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(r.Body)
	f.bodies = append(f.bodies, string(body))
	status := http.StatusOK
	if len(f.bodies) <= f.failures {
		status = http.StatusServiceUnavailable
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
}

func TestRetryableClientResendsBodyOnRetry(t *testing.T) {
	c := newClientOverFakePortal(t, &fakePortal{})
	c.maxRetries = 1
	portal := &flakyPortal{failures: 1}
	c.client.Transport = portal

	resp, err := publishWithAuth(c)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 after the retry, got %d", resp.StatusCode)
	}
	if len(portal.bodies) != 2 || portal.bodies[0] != "x" || portal.bodies[1] != "x" {
		t.Fatalf("want the full body on both attempts, got %q", portal.bodies)
	}
}

func TestRetryableClientTotalTimeoutCoversAllAttempts(t *testing.T) {
	c := &RetryableHTTPClient{maxRetries: 3, timeout: 10 * time.Second}
	if got, want := c.TotalTimeout(), 43*time.Second; got != want {
		t.Fatalf("want %s, got %s", want, got)
	}
}

func TestRetryableClientRefusesRedirectToHTTP(t *testing.T) {
	portal := &fakePortal{location: "http://portal.example/publish"}
	c := newClientOverFakePortal(t, portal)

	_, err := publishWithAuth(c)
	if !errors.Is(err, errInsecureRedirect) {
		t.Fatalf("want errInsecureRedirect, got %v", err)
	}
	if portal.httpHits != 0 || portal.httpAuthorization != "" {
		t.Fatalf("request reached the http hop (hits=%d, Authorization=%q)", portal.httpHits, portal.httpAuthorization)
	}
}

func TestRetryableClientFollowsSameHostHTTPSRedirect(t *testing.T) {
	portal := &fakePortal{location: "https://portal.example/publish/", redirectOnce: true}
	c := newClientOverFakePortal(t, portal)

	resp, err := publishWithAuth(c)
	if err != nil {
		t.Fatalf("https redirect should be followed, got %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 after following the redirect, got %d", resp.StatusCode)
	}
}
