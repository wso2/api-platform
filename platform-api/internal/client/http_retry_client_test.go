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

type redirectTransport struct {
	location    string
	plainHits   int
	plainAuthHd string
}

func (f *redirectTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme == "https" {
		return &http.Response{
			StatusCode: http.StatusTemporaryRedirect,
			Header:     http.Header{"Location": {f.location}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	}
	f.plainHits++
	f.plainAuthHd = r.Header.Get("Authorization")
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
}

// newClientWithRedirectingTransport builds the retryable client over a shared client that
// permits http redirects (the shipped allowed_schemes default), then swaps in a fake transport.
func newClientWithRedirectingTransport(t *testing.T, location string) (*RetryableHTTPClient, *redirectTransport) {
	t.Helper()
	policy := netguard.PermitPrivateBlockMetadata()
	policy.AllowedSchemes = []string{"http", "https"}
	utils.InitSharedHTTPClient(&http.Client{CheckRedirect: netguard.CheckRedirect(policy, 5)}, 0)

	c, err := NewRetryableHTTPClient(0, 5*time.Second)
	if err != nil {
		t.Fatalf("NewRetryableHTTPClient: %v", err)
	}
	ft := &redirectTransport{location: location}
	c.client.Transport = ft
	return c, ft
}

func doWithAuth(c *RetryableHTTPClient) error {
	req, _ := http.NewRequest(http.MethodPost, "https://portal.example/publish", strings.NewReader("x"))
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := c.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	return err
}

func TestRetryableClientRefusesRedirectToHTTP(t *testing.T) {
	c, ft := newClientWithRedirectingTransport(t, "http://portal.example/publish")

	err := doWithAuth(c)
	if err == nil || !errors.Is(err, errInsecureRedirect) {
		t.Fatalf("want errInsecureRedirect, got %v", err)
	}
	if ft.plainHits != 0 || ft.plainAuthHd != "" {
		t.Fatalf("request reached the http hop (hits=%d, Authorization=%q)", ft.plainHits, ft.plainAuthHd)
	}
}

func TestRetryableClientStillFollowsSameHostHTTPSRedirect(t *testing.T) {
	c, ft := newClientWithRedirectingTransport(t, "https://portal.example/publish/")
	ft.location = "https://portal.example/publish/"

	// The fake answers every https request with a redirect, so the shared client's hop cap
	// ends the chain; what matters is that it is not refused as insecure.
	err := doWithAuth(c)
	if err == nil || errors.Is(err, errInsecureRedirect) {
		t.Fatalf("want a hop-limit error from the shared check, got %v", err)
	}
	if ft.plainHits != 0 {
		t.Fatalf("unexpected http hop: %d", ft.plainHits)
	}
}
