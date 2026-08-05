/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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

package policyv1alpha2

import (
	"reflect"
	"testing"
)

// ─── Request phase: downstream snapshot writers ──────────────────────────────

func TestSetDownstreamHeader(t *testing.T) {
	t.Run("overwrites in the snapshot and is read back via DownstreamHeaders", func(t *testing.T) {
		c := &RequestHeaderContext{
			Headers:    NewHeaders(map[string][]string{"authorization": {"live"}}),
			Downstream: &DownstreamContext{Request: &DownstreamRequest{Headers: NewHeaders(map[string][]string{"authorization": {"original"}})}},
		}
		c.SetDownstreamHeader("Authorization", "injected") // mixed case normalizes
		if got := firstValue(c.DownstreamHeaders(), "authorization"); got != "injected" {
			t.Fatalf("DownstreamHeaders authorization = %q, want %q", got, "injected")
		}
	})

	t.Run("does NOT leak into the live working headers", func(t *testing.T) {
		live := NewHeaders(map[string][]string{"x-foo": {"live"}})
		c := &RequestContext{
			Headers:    live,
			Downstream: &DownstreamContext{Request: &DownstreamRequest{Headers: NewHeaders(nil)}},
		}
		c.SetDownstreamHeader("x-foo", "snap")
		// The working/upstream headers (what reaches the backend) must be untouched.
		if got := firstValue(live, "x-foo"); got != "live" {
			t.Fatalf("live working header x-foo = %q, want %q (snapshot write leaked to live)", got, "live")
		}
	})

	t.Run("no-op when the gateway did not provide a snapshot", func(t *testing.T) {
		live := NewHeaders(map[string][]string{"x-foo": {"live"}})
		c := &RequestContext{Headers: live, Downstream: nil}
		c.SetDownstreamHeader("x-foo", "snap")
		// Must not fall back to mutating live headers.
		if got := firstValue(live, "x-foo"); got != "live" {
			t.Fatalf("live header x-foo = %q, want %q (setter touched live on absent snapshot)", got, "live")
		}
	})

	t.Run("lazily creates Headers on a header-less snapshot", func(t *testing.T) {
		c := &RequestContext{Downstream: &DownstreamContext{Request: &DownstreamRequest{Headers: nil}}}
		c.SetDownstreamHeader("x-new", "v")
		if got := firstValue(c.DownstreamHeaders(), "x-new"); got != "v" {
			t.Fatalf("x-new = %q, want %q", got, "v")
		}
	})
}

func TestAddRemoveDownstreamHeader(t *testing.T) {
	c := &RequestHeaderContext{
		Downstream: &DownstreamContext{Request: &DownstreamRequest{Headers: NewHeaders(map[string][]string{"x-multi": {"a"}})}},
	}
	c.AddDownstreamHeader("X-Multi", "b")
	if got := c.DownstreamHeaders().Get("x-multi"); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("after Add, x-multi = %v, want [a b]", got)
	}
	c.RemoveDownstreamHeader("x-multi")
	if c.DownstreamHeaders().Has("x-multi") {
		t.Fatalf("x-multi should have been removed")
	}
}

func TestResponseSetDownstreamHeader(t *testing.T) {
	t.Run("mutates the carried-forward request snapshot, read via DownstreamHeaders", func(t *testing.T) {
		c := &ResponseHeaderContext{
			RequestHeaders: NewHeaders(map[string][]string{"x-trace": {"live-echo"}}),
			Downstream:     &DownstreamContext{Request: &DownstreamRequest{Headers: NewHeaders(map[string][]string{"x-trace": {"original"}})}},
		}
		c.SetDownstreamHeader("X-Trace", "annotated")
		if got := firstValue(c.DownstreamHeaders(), "x-trace"); got != "annotated" {
			t.Fatalf("DownstreamHeaders x-trace = %q, want %q", got, "annotated")
		}
	})

	t.Run("no-op when snapshot absent, does not touch live request echo", func(t *testing.T) {
		echo := NewHeaders(map[string][]string{"x-trace": {"live-echo"}})
		c := &ResponseContext{RequestHeaders: echo, Downstream: nil}
		c.SetDownstreamHeader("x-trace", "annotated")
		if got := firstValue(echo, "x-trace"); got != "live-echo" {
			t.Fatalf("live request echo x-trace = %q, want %q (setter touched live on absent snapshot)", got, "live-echo")
		}
	})
}

// ─── Response phase: upstream-response snapshot writers ───────────────────────

func TestSetUpstreamHeader(t *testing.T) {
	t.Run("overwrites in the snapshot and is read back via UpstreamHeaders", func(t *testing.T) {
		c := &ResponseHeaderContext{
			ResponseHeaders: NewHeaders(map[string][]string{"x-cache": {"live"}}),
			Upstream:        &UpstreamResponseContext{Response: &UpstreamResponse{Headers: NewHeaders(map[string][]string{"x-cache": {"HIT"}})}},
		}
		c.SetUpstreamHeader("X-Cache", "MISS")
		if got := firstValue(c.UpstreamHeaders(), "x-cache"); got != "MISS" {
			t.Fatalf("UpstreamHeaders x-cache = %q, want %q", got, "MISS")
		}
	})

	t.Run("does NOT leak into the live response headers", func(t *testing.T) {
		live := NewHeaders(map[string][]string{"x-cache": {"live"}})
		c := &ResponseContext{
			ResponseHeaders: live,
			Upstream:        &UpstreamResponseContext{Response: &UpstreamResponse{Headers: NewHeaders(nil)}},
		}
		c.SetUpstreamHeader("x-cache", "snap")
		// The working response headers (what reaches the client) must be untouched.
		if got := firstValue(live, "x-cache"); got != "live" {
			t.Fatalf("live response header x-cache = %q, want %q (snapshot write leaked to live)", got, "live")
		}
	})

	t.Run("no-op when the gateway did not provide a snapshot", func(t *testing.T) {
		live := NewHeaders(map[string][]string{"x-cache": {"live"}})
		c := &ResponseContext{ResponseHeaders: live, Upstream: nil}
		c.SetUpstreamHeader("x-cache", "snap")
		if got := firstValue(live, "x-cache"); got != "live" {
			t.Fatalf("live header x-cache = %q, want %q (setter touched live on absent snapshot)", got, "live")
		}
	})
}
