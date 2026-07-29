package policyv1alpha2

import "testing"

// firstValue returns the first value of header name, or "" if absent.
func firstValue(h *Headers, name string) string {
	vals := h.Get(name)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

// snapshotRequest / liveRequest give the two sources distinct marker values so a
// test can tell which one an accessor returned.
func snapshotRequest() *DownstreamRequest {
	return &DownstreamRequest{
		Headers:   NewHeaders(map[string][]string{"x-src": {"snapshot"}}),
		Path:      "/snap/path",
		Method:    "POST",
		Authority: "snap.example.com",
		Scheme:    "https",
	}
}

func liveRequestHeaders() *Headers {
	return NewHeaders(map[string][]string{"x-src": {"live"}})
}

func snapshotResponse() *UpstreamResponse {
	return &UpstreamResponse{
		Headers:    NewHeaders(map[string][]string{"x-src": {"snapshot"}}),
		StatusCode: 201,
	}
}

// ─── resolveDownstreamRequest ────────────────────────────────────────────────

func TestResolveDownstreamRequest_PrefersSnapshot(t *testing.T) {
	ds := &DownstreamContext{Request: snapshotRequest()}
	got := resolveDownstreamRequest(ds, liveRequestHeaders(), "/live", "GET", "live.example.com", "http")

	if got != ds.Request {
		t.Fatalf("expected the snapshot struct to be returned as-is")
	}
	if v := firstValue(got.Headers, "x-src"); v != "snapshot" {
		t.Errorf("headers: got %q, want snapshot", v)
	}
	if got.Path != "/snap/path" || got.Method != "POST" || got.Authority != "snap.example.com" || got.Scheme != "https" {
		t.Errorf("unexpected snapshot fields: %+v", got)
	}
}

func TestResolveDownstreamRequest_FallbackWhenDownstreamNil(t *testing.T) {
	got := resolveDownstreamRequest(nil, liveRequestHeaders(), "/live", "GET", "live.example.com", "http")

	if v := firstValue(got.Headers, "x-src"); v != "live" {
		t.Errorf("headers: got %q, want live", v)
	}
	if got.Path != "/live" || got.Method != "GET" || got.Authority != "live.example.com" || got.Scheme != "http" {
		t.Errorf("unexpected live fields: %+v", got)
	}
}

func TestResolveDownstreamRequest_FallbackWhenRequestNil(t *testing.T) {
	ds := &DownstreamContext{Request: nil}
	got := resolveDownstreamRequest(ds, liveRequestHeaders(), "/live", "GET", "", "")

	if v := firstValue(got.Headers, "x-src"); v != "live" {
		t.Errorf("headers: got %q, want live", v)
	}
	if got.Path != "/live" || got.Method != "GET" {
		t.Errorf("unexpected live fields: %+v", got)
	}
}

func TestResolveDownstreamRequest_NilLiveHeadersStillNilSafe(t *testing.T) {
	got := resolveDownstreamRequest(nil, nil, "/live", "GET", "", "")
	// Headers reads must be nil-safe even when the fallback headers are nil.
	if got.Headers.Has("anything") {
		t.Errorf("expected no headers on nil fallback")
	}
}

// ─── resolveUpstreamResponse ─────────────────────────────────────────────────

func TestResolveUpstreamResponse_PrefersSnapshot(t *testing.T) {
	us := &UpstreamResponseContext{Response: snapshotResponse()}
	got := resolveUpstreamResponse(us, NewHeaders(map[string][]string{"x-src": {"live"}}), 500)

	if got != us.Response {
		t.Fatalf("expected the snapshot response to be returned as-is")
	}
	if v := firstValue(got.Headers, "x-src"); v != "snapshot" || got.StatusCode != 201 {
		t.Errorf("unexpected snapshot response: header=%q status=%d", v, got.StatusCode)
	}
}

func TestResolveUpstreamResponse_FallbackWhenUpstreamNil(t *testing.T) {
	got := resolveUpstreamResponse(nil, NewHeaders(map[string][]string{"x-src": {"live"}}), 500)

	if v := firstValue(got.Headers, "x-src"); v != "live" || got.StatusCode != 500 {
		t.Errorf("unexpected live response: header=%q status=%d", v, got.StatusCode)
	}
}

func TestResolveUpstreamResponse_FallbackWhenResponseNil(t *testing.T) {
	us := &UpstreamResponseContext{Response: nil}
	got := resolveUpstreamResponse(us, NewHeaders(map[string][]string{"x-src": {"live"}}), 502)

	if v := firstValue(got.Headers, "x-src"); v != "live" || got.StatusCode != 502 {
		t.Errorf("unexpected live response: header=%q status=%d", v, got.StatusCode)
	}
}

// ─── Request-phase context wrappers ──────────────────────────────────────────

func TestRequestContexts_DownstreamAccessors(t *testing.T) {
	// Each request-phase context exposes the same live request fields
	// (Headers/Path/Method/Authority/Scheme). Verify snapshot and fallback for
	// all three via a common shape check.
	liveHeaders := liveRequestHeaders()

	assertSnapshot := func(t *testing.T, dr *DownstreamRequest, hdrs *Headers) {
		t.Helper()
		if firstValue(dr.Headers, "x-src") != "snapshot" || dr.Path != "/snap/path" ||
			dr.Method != "POST" || dr.Authority != "snap.example.com" || dr.Scheme != "https" {
			t.Errorf("expected snapshot fields, got %+v", dr)
		}
		if firstValue(hdrs, "x-src") != "snapshot" {
			t.Errorf("DownstreamHeaders shortcut did not return snapshot headers")
		}
	}
	assertFallback := func(t *testing.T, dr *DownstreamRequest, hdrs *Headers) {
		t.Helper()
		if firstValue(dr.Headers, "x-src") != "live" || dr.Path != "/live" ||
			dr.Method != "GET" || dr.Authority != "live.example.com" || dr.Scheme != "http" {
			t.Errorf("expected live fields, got %+v", dr)
		}
		if firstValue(hdrs, "x-src") != "live" {
			t.Errorf("DownstreamHeaders shortcut did not return live headers")
		}
	}

	t.Run("RequestHeaderContext snapshot", func(t *testing.T) {
		c := &RequestHeaderContext{
			Headers: liveHeaders, Path: "/live", Method: "GET", Authority: "live.example.com", Scheme: "http",
			Downstream: &DownstreamContext{Request: snapshotRequest()},
		}
		assertSnapshot(t, c.DownstreamRequest(), c.DownstreamHeaders())
	})
	t.Run("RequestHeaderContext fallback", func(t *testing.T) {
		c := &RequestHeaderContext{Headers: liveHeaders, Path: "/live", Method: "GET", Authority: "live.example.com", Scheme: "http"}
		assertFallback(t, c.DownstreamRequest(), c.DownstreamHeaders())
	})

	t.Run("RequestContext snapshot", func(t *testing.T) {
		c := &RequestContext{
			Headers: liveHeaders, Path: "/live", Method: "GET", Authority: "live.example.com", Scheme: "http",
			Downstream: &DownstreamContext{Request: snapshotRequest()},
		}
		assertSnapshot(t, c.DownstreamRequest(), c.DownstreamHeaders())
	})
	t.Run("RequestContext fallback", func(t *testing.T) {
		c := &RequestContext{Headers: liveHeaders, Path: "/live", Method: "GET", Authority: "live.example.com", Scheme: "http"}
		assertFallback(t, c.DownstreamRequest(), c.DownstreamHeaders())
	})

	t.Run("RequestStreamContext snapshot", func(t *testing.T) {
		c := &RequestStreamContext{
			Headers: liveHeaders, Path: "/live", Method: "GET", Authority: "live.example.com", Scheme: "http",
			Downstream: &DownstreamContext{Request: snapshotRequest()},
		}
		assertSnapshot(t, c.DownstreamRequest(), c.DownstreamHeaders())
	})
	t.Run("RequestStreamContext fallback", func(t *testing.T) {
		c := &RequestStreamContext{Headers: liveHeaders, Path: "/live", Method: "GET", Authority: "live.example.com", Scheme: "http"}
		assertFallback(t, c.DownstreamRequest(), c.DownstreamHeaders())
	})
}

// ─── Response-phase context wrappers ─────────────────────────────────────────

func TestResponseContexts_DownstreamFallbackHasNoAuthorityOrScheme(t *testing.T) {
	// Response contexts carry no live Authority/Scheme, so the fallback leaves
	// them empty while still filling headers/path/method from the request echoes.
	live := liveRequestHeaders()

	check := func(t *testing.T, dr *DownstreamRequest, hdrs *Headers) {
		t.Helper()
		if firstValue(dr.Headers, "x-src") != "live" || dr.Path != "/live-req" || dr.Method != "GET" {
			t.Errorf("expected live request echoes, got %+v", dr)
		}
		if dr.Authority != "" || dr.Scheme != "" {
			t.Errorf("expected empty Authority/Scheme on response fallback, got authority=%q scheme=%q", dr.Authority, dr.Scheme)
		}
		if firstValue(hdrs, "x-src") != "live" {
			t.Errorf("DownstreamHeaders shortcut mismatch")
		}
	}

	t.Run("ResponseHeaderContext", func(t *testing.T) {
		c := &ResponseHeaderContext{RequestHeaders: live, RequestPath: "/live-req", RequestMethod: "GET"}
		check(t, c.DownstreamRequest(), c.DownstreamHeaders())
	})
	t.Run("ResponseContext", func(t *testing.T) {
		c := &ResponseContext{RequestHeaders: live, RequestPath: "/live-req", RequestMethod: "GET"}
		check(t, c.DownstreamRequest(), c.DownstreamHeaders())
	})
	t.Run("ResponseStreamContext", func(t *testing.T) {
		c := &ResponseStreamContext{RequestHeaders: live, RequestPath: "/live-req", RequestMethod: "GET"}
		check(t, c.DownstreamRequest(), c.DownstreamHeaders())
	})
}

func TestResponseContexts_DownstreamPrefersSnapshot(t *testing.T) {
	// With a snapshot present, response contexts do surface Authority/Scheme.
	check := func(t *testing.T, dr *DownstreamRequest) {
		t.Helper()
		if firstValue(dr.Headers, "x-src") != "snapshot" || dr.Authority != "snap.example.com" || dr.Scheme != "https" {
			t.Errorf("expected snapshot fields, got %+v", dr)
		}
	}

	ds := func() *DownstreamContext { return &DownstreamContext{Request: snapshotRequest()} }
	check(t, (&ResponseHeaderContext{RequestHeaders: liveRequestHeaders(), Downstream: ds()}).DownstreamRequest())
	check(t, (&ResponseContext{RequestHeaders: liveRequestHeaders(), Downstream: ds()}).DownstreamRequest())
	check(t, (&ResponseStreamContext{RequestHeaders: liveRequestHeaders(), Downstream: ds()}).DownstreamRequest())
}

func TestResponseContexts_UpstreamAccessors(t *testing.T) {
	liveHeaders := NewHeaders(map[string][]string{"x-src": {"live"}})

	assertSnapshot := func(t *testing.T, ur *UpstreamResponse, hdrs *Headers) {
		t.Helper()
		if firstValue(ur.Headers, "x-src") != "snapshot" || ur.StatusCode != 201 {
			t.Errorf("expected snapshot response, got %+v", ur)
		}
		if firstValue(hdrs, "x-src") != "snapshot" {
			t.Errorf("UpstreamHeaders shortcut did not return snapshot headers")
		}
	}
	assertFallback := func(t *testing.T, ur *UpstreamResponse, hdrs *Headers) {
		t.Helper()
		if firstValue(ur.Headers, "x-src") != "live" || ur.StatusCode != 503 {
			t.Errorf("expected live response, got %+v", ur)
		}
		if firstValue(hdrs, "x-src") != "live" {
			t.Errorf("UpstreamHeaders shortcut did not return live headers")
		}
	}

	t.Run("ResponseHeaderContext snapshot", func(t *testing.T) {
		c := &ResponseHeaderContext{ResponseHeaders: liveHeaders, ResponseStatus: 503, Upstream: &UpstreamResponseContext{Response: snapshotResponse()}}
		assertSnapshot(t, c.UpstreamResponse(), c.UpstreamHeaders())
	})
	t.Run("ResponseHeaderContext fallback", func(t *testing.T) {
		c := &ResponseHeaderContext{ResponseHeaders: liveHeaders, ResponseStatus: 503}
		assertFallback(t, c.UpstreamResponse(), c.UpstreamHeaders())
	})

	t.Run("ResponseContext snapshot", func(t *testing.T) {
		c := &ResponseContext{ResponseHeaders: liveHeaders, ResponseStatus: 503, Upstream: &UpstreamResponseContext{Response: snapshotResponse()}}
		assertSnapshot(t, c.UpstreamResponse(), c.UpstreamHeaders())
	})
	t.Run("ResponseContext fallback", func(t *testing.T) {
		c := &ResponseContext{ResponseHeaders: liveHeaders, ResponseStatus: 503}
		assertFallback(t, c.UpstreamResponse(), c.UpstreamHeaders())
	})

	t.Run("ResponseStreamContext snapshot", func(t *testing.T) {
		c := &ResponseStreamContext{ResponseHeaders: liveHeaders, ResponseStatus: 503, Upstream: &UpstreamResponseContext{Response: snapshotResponse()}}
		assertSnapshot(t, c.UpstreamResponse(), c.UpstreamHeaders())
	})
	t.Run("ResponseStreamContext fallback", func(t *testing.T) {
		c := &ResponseStreamContext{ResponseHeaders: liveHeaders, ResponseStatus: 503}
		assertFallback(t, c.UpstreamResponse(), c.UpstreamHeaders())
	})
}
