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
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testClientID     = "platform-api-m2m"
	testClientSecret = "s3cr3t/with+special=chars"
)

type idp struct {
	t       *testing.T
	fetches atomic.Int32
	status  int
	body    string
}

func (i *idp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	n := i.fetches.Add(1)
	if r.Method != http.MethodPost {
		i.t.Errorf("token request method = %s, want POST", r.Method)
	}
	if err := r.ParseForm(); err != nil || r.PostForm.Get("grant_type") != "client_credentials" {
		i.t.Errorf("token request form = %v, want grant_type=client_credentials", r.PostForm)
	}
	if r.PostForm.Has("client_secret") || r.URL.RawQuery != "" {
		i.t.Error("the client secret must travel only in the Authorization header")
	}
	id, secret, ok := r.BasicAuth()
	if !ok || id != testClientID || secret != "s3cr3t%2Fwith%2Bspecial%3Dchars" {
		i.t.Errorf("basic auth = %q/%q, want the form-encoded client credentials", id, secret)
	}
	status := i.status
	if status == 0 {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := i.body
	if body == "" {
		body = fmt.Sprintf(`{"access_token":"access-%d","token_type":"Bearer","expires_in":3600}`, n)
	}
	_, _ = io.WriteString(w, body)
}

func newTestTokenSource(t *testing.T, i *idp) *clientCredentialsTokenSource {
	t.Helper()
	i.t = t
	server := httptest.NewServer(i)
	t.Cleanup(server.Close)
	return newClientCredentialsTokenSource(server.Client(), server.URL+"/oauth2/token", testClientID, testClientSecret)
}

func TestTokenIsCachedUntilCloseToExpiry(t *testing.T) {
	i := &idp{}
	src := newTestTokenSource(t, i)
	clock := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	src.now = func() time.Time { return clock }

	first, err := src.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	again, _ := src.Token(context.Background())
	if first != "access-1" || again != first || i.fetches.Load() != 1 {
		t.Fatalf("tokens %q, %q after %d fetches; want one fetch, reused", first, again, i.fetches.Load())
	}

	// A one-hour token is reused until a minute before it expires.
	clock = clock.Add(58 * time.Minute)
	if tok, _ := src.Token(context.Background()); tok != "access-1" {
		t.Errorf("token refreshed too early: %q", tok)
	}
	clock = clock.Add(90 * time.Second)
	if tok, _ := src.Token(context.Background()); tok != "access-2" {
		t.Errorf("token not refreshed near expiry: %q", tok)
	}
}

func TestInvalidateForcesAFreshToken(t *testing.T) {
	i := &idp{}
	src := newTestTokenSource(t, i)
	stale, _ := src.Token(context.Background())

	src.Invalidate("some-other-token") // not the cached one: no effect
	if tok, _ := src.Token(context.Background()); tok != stale {
		t.Fatalf("invalidating a different token dropped the cached one")
	}
	src.Invalidate(stale)
	if tok, _ := src.Token(context.Background()); tok == stale {
		t.Fatal("an invalidated token was reused")
	}
}

func TestTokenFailuresDoNotLeakTheResponse(t *testing.T) {
	tests := map[string]struct {
		status    int
		body      string
		retryable bool
	}{
		"bad credentials":    {http.StatusUnauthorized, `{"error":"invalid_client","client_id":"` + testClientID + `"}`, false},
		"idp unavailable":    {http.StatusServiceUnavailable, `upstream ` + testClientID, true},
		"rate limited":       {http.StatusTooManyRequests, `slow down ` + testClientID, true},
		"no access token":    {http.StatusOK, `{"token_type":"Bearer"}`, false},
		"not a bearer token": {http.StatusOK, `{"access_token":"x","token_type":"mac"}`, false},
		"not json":           {http.StatusOK, `<html>` + testClientID, false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			src := newTestTokenSource(t, &idp{status: tc.status, body: tc.body})
			_, err := src.Token(context.Background())
			var te *tokenError
			if !errors.As(err, &te) {
				t.Fatalf("err = %v, want a tokenError", err)
			}
			if te.retryable != tc.retryable {
				t.Errorf("retryable = %v, want %v", te.retryable, tc.retryable)
			}
			for _, leaked := range []string{testClientID, testClientSecret, "s3cr3t"} {
				if strings.Contains(err.Error(), leaked) {
					t.Errorf("error %q leaks %q", err, leaked)
				}
			}
		})
	}
}

func TestRefreshMargin(t *testing.T) {
	for lifetime, want := range map[time.Duration]time.Duration{
		time.Hour:        time.Minute,
		5 * time.Minute:  30 * time.Second,
		20 * time.Second: 5 * time.Second,
		6 * time.Second:  5 * time.Second,
		4 * time.Second:  2 * time.Second,
	} {
		if got := refreshMargin(lifetime); got != want {
			t.Errorf("refreshMargin(%s) = %s, want %s", lifetime, got, want)
		}
	}
}
