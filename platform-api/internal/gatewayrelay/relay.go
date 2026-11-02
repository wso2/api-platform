/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package gatewayrelay

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/wso2/api-platform/httpkit/httpclient"

	"github.com/wso2/api-platform/platform-api/internal/gatewayrelay/egress"
)

// ErrBusy means every slot is taken and the pending queue is full. The queue is
// bounded: an unbounded one in front of a bounded pool only delays memory growth.
var ErrBusy = errors.New("gateway relay is at capacity")

// RelayOptions configures the relay. The config package supplies validated
// defaults, so a zero bound here is a wiring mistake, not "unbounded".
type RelayOptions struct {
	RequestTimeout   time.Duration
	MaxRequestBytes  int64
	MaxResponseBytes int64
	MaxConcurrent    int
	MaxPending       int

	CAFile        string
	TLSSkipVerify bool

	// Egress bounds where the relay may connect. Required: nil is refused, never
	// read as "no restrictions".
	Egress *egress.Policy
}

// Relay sends one sanitized request to a gateway and reads a bounded response.
// It never sees the caller's request or token, so it cannot leak the token to a
// tenant-controlled gateway.
type Relay struct {
	client *http.Client
	opts   RelayOptions

	// inFlight counts running and waiting calls, so the ceiling bounds the queue too.
	inFlight atomic.Int64
	slots    chan struct{}
}

// NewRelay builds the relay and its guarded outbound client.
func NewRelay(opts RelayOptions) (*Relay, error) {
	if opts.RequestTimeout <= 0 || opts.MaxRequestBytes <= 0 || opts.MaxResponseBytes <= 0 || opts.MaxConcurrent <= 0 {
		return nil, fmt.Errorf("gatewayrelay: RequestTimeout, MaxRequestBytes, MaxResponseBytes and MaxConcurrent must all be positive")
	}

	if opts.Egress == nil {
		return nil, fmt.Errorf("gatewayrelay: Egress policy is required")
	}

	cfg := httpclient.DefaultConfig()
	cfg.Timeouts.Overall = opts.RequestTimeout
	// Do truncates a long response itself; httpkit's limit is a backstop above it.
	cfg.Timeouts.MaxResponseBytes = opts.MaxResponseBytes * 2
	// The egress policy's dialer runs netguard's checks itself and also logs the
	// refused address, which httpclient's own SSRF mode does not.
	dial := opts.Egress.DialContext(cfg.Timeouts.Dial)
	cfg.TLS = httpclient.TLSConfig{
		MinVersion: "TLS1_2",
		MaxVersion: "TLS1_3",
		// The ML-KEM-768 hybrid first, classical curves after it for gateways
		// without it.
		CurvePreferences:               "X25519MLKEM768,X25519,P-256,P-384",
		RootCAFile:                     opts.CAFile,
		InsecureSkipVerify:             opts.TLSSkipVerify,
		InsecureSkipVerifyAcknowledged: opts.TLSSkipVerify,
	}

	client, err := httpclient.New(cfg, httpclient.WithDialContext(dial))
	if err != nil {
		return nil, fmt.Errorf("gatewayrelay: build relay client: %w", err)
	}
	// A 3xx is a result to show, and following it would send the test request's
	// headers to a location the gateway chose.
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return &Relay{
		client: client,
		opts:   opts,
		slots:  make(chan struct{}, opts.MaxConcurrent),
	}, nil
}

// Do relays one request. An error means the relay failed; a gateway answer with
// any status is a successful relay.
func (r *Relay) Do(ctx context.Context, target Target, req Request) (Relayed, error) {
	// Scheme, host and port need no DNS, so they are checked here, where a refusal
	// can still say "not allowed". The address checks stay at dial time (rebinding).
	if err := r.opts.Egress.CheckTarget(target.URL); err != nil {
		return Relayed{}, err
	}
	method, err := NormalizeMethod(req.Method)
	if err != nil {
		return Relayed{}, err
	}
	headers, err := SanitizeHeaders(req.Headers)
	if err != nil {
		return Relayed{}, err
	}
	rawQuery, err := SanitizeQuery(req.Query)
	if err != nil {
		return Relayed{}, err
	}
	decodedPath, escapedPath, err := ResolvePath(target.URL.Path, req.Path)
	if err != nil {
		return Relayed{}, err
	}
	body, err := req.DecodeBody()
	if err != nil {
		return Relayed{}, err
	}
	if int64(len(body)) > r.opts.MaxRequestBytes {
		return Relayed{}, ErrBodyTooLarge
	}

	u := *target.URL
	u.Path = decodedPath
	// RawPath matters only when it differs from Path's own encoding.
	if escapedPath != decodedPath {
		u.RawPath = escapedPath
	} else {
		u.RawPath = ""
	}
	u.RawQuery = rawQuery

	release, err := r.acquire(ctx)
	if err != nil {
		return Relayed{}, err
	}
	defer release()

	ctx, cancel := context.WithTimeout(ctx, r.opts.RequestTimeout)
	defer cancel()

	outbound, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return Relayed{}, err
	}
	// Assigned, not merged: the sanitized headers are all that go on the wire.
	outbound.Header = headers
	outbound.ContentLength = int64(len(body))

	started := time.Now()
	res, err := r.client.Do(outbound)
	if err != nil {
		return Relayed{}, err
	}
	defer res.Body.Close()

	// One byte past the limit shows the body was longer. Go inflates a gzip body it
	// negotiated, so this bounds what is held in memory, not the compressed size.
	limited := io.LimitReader(res.Body, r.opts.MaxResponseBytes+1)
	raw, readErr := io.ReadAll(limited)
	if readErr != nil {
		return Relayed{}, readErr
	}
	truncated := int64(len(raw)) > r.opts.MaxResponseBytes
	if truncated {
		raw = trimToRuneBoundary(raw[:r.opts.MaxResponseBytes])
	}

	encodedBody, encoding := EncodeBody(raw)
	return Relayed{
		Status:       res.StatusCode,
		StatusText:   statusText(res),
		Headers:      flattenHeaders(res.Header),
		Body:         encodedBody,
		BodyEncoding: encoding,
		Truncated:    truncated,
		DurationMs:   time.Since(started).Milliseconds(),
	}, nil
}

// acquire takes a slot, refusing at once when the pool and its queue are both full.
func (r *Relay) acquire(ctx context.Context) (func(), error) {
	ceiling := int64(r.opts.MaxConcurrent + r.opts.MaxPending)
	if r.inFlight.Add(1) > ceiling {
		r.inFlight.Add(-1)
		return nil, ErrBusy
	}
	select {
	case r.slots <- struct{}{}:
		return func() {
			<-r.slots
			r.inFlight.Add(-1)
		}, nil
	case <-ctx.Done():
		r.inFlight.Add(-1)
		return nil, ctx.Err()
	}
}

// trimToRuneBoundary drops a partial UTF-8 sequence from the end of a truncated
// body, which would otherwise turn a text response into base64.
func trimToRuneBoundary(b []byte) []byte {
	for i := 0; i < utf8.UTFMax && len(b) > 0; i++ {
		if r, size := utf8.DecodeLastRune(b); r != utf8.RuneError || size > 1 {
			break
		}
		b = b[:len(b)-1]
	}
	return b
}

// statusText returns the gateway's own reason phrase, or the standard one.
func statusText(res *http.Response) string {
	if _, phrase, ok := strings.Cut(res.Status, " "); ok && phrase != "" {
		return phrase
	}
	return http.StatusText(res.StatusCode)
}

// flattenHeaders lists the response headers, keeping repeated names and sorted
// for a stable display. They are returned as data, never set on the Platform
// API's own response, so a gateway cannot set a cookie on the console origin.
func flattenHeaders(h http.Header) []Header {
	out := make([]Header, 0, len(h))
	for name, values := range h {
		for _, v := range values {
			out = append(out, Header{Name: name, Value: v})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Value < out[j].Value
	})
	return out
}
