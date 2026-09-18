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

package testproxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/wso2/api-platform/httpkit/httpclient"
	"github.com/wso2/api-platform/httpkit/netguard"
)

// ErrBusy is returned when the relay is already at its concurrency ceiling and
// its pending queue is full. Surfaced as 503 rather than queued indefinitely:
// an unbounded queue in front of a bounded pool is only delayed memory growth.
var ErrBusy = errors.New("test-console relay is at capacity")

// RelayOptions configures the relay. Everything here has a validated,
// non-zero default in the config package; New does not invent one, so a
// zero-valued option is a programming error rather than a silent "unbounded".
type RelayOptions struct {
	RequestTimeout   time.Duration
	MaxRequestBytes  int64
	MaxResponseBytes int64
	MaxConcurrent    int
	MaxPending       int

	CAFile        string
	TLSSkipVerify bool
	DenyCIDRs     []string
	AllowCIDRs    []string
}

// Relay sends one sanitized request to a resolved gateway target and reads a
// bounded response back.
//
// It holds no reference to the session, the session store, or any token. The
// only headers it sets are the ones handed to Do, which is what makes it
// structurally impossible for this path to leak the caller's Platform API
// bearer token to a tenant-controlled gateway, rather than merely unlikely.
type Relay struct {
	client *http.Client
	opts   RelayOptions

	// inFlight counts running *and* waiting calls, so the ceiling below bounds
	// the queue as well as the pool.
	inFlight atomic.Int64
	slots    chan struct{}
}

// NewRelay builds the relay and its guarded outbound client.
func NewRelay(opts RelayOptions) (*Relay, error) {
	if opts.RequestTimeout <= 0 || opts.MaxResponseBytes <= 0 || opts.MaxConcurrent <= 0 {
		return nil, fmt.Errorf("testproxy: RequestTimeout, MaxResponseBytes and MaxConcurrent must all be positive")
	}

	policy, err := dialPolicy(opts)
	if err != nil {
		return nil, err
	}

	cfg := httpclient.DefaultConfig()
	cfg.Timeouts.Overall = opts.RequestTimeout
	// The relay does its own bounded read so it can *truncate* a large response
	// instead of failing it. httpkit's ceiling stays on as a hard backstop, set
	// above the relay's own limit so the relay's friendlier behaviour is what
	// callers normally meet.
	cfg.Timeouts.MaxResponseBytes = opts.MaxResponseBytes * 2
	cfg.SSRF = httpclient.SSRFConfig{Enabled: true, Policy: policy}
	cfg.TLS = httpclient.TLSConfig{
		MinVersion: "TLS1_2",
		MaxVersion: "TLS1_3",
		// Prefer the FIPS 203 ML-KEM-768 hybrid group, keeping the classical
		// curves after it so a gateway build that does not offer the hybrid
		// group still completes its handshake instead of failing closed.
		CurvePreferences:               "X25519MLKEM768,X25519,P-256,P-384",
		RootCAFile:                     opts.CAFile,
		InsecureSkipVerify:             opts.TLSSkipVerify,
		InsecureSkipVerifyAcknowledged: opts.TLSSkipVerify,
	}

	client, err := httpclient.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("testproxy: build relay client: %w", err)
	}
	// Never follow a redirect. A 3xx is a result the tester wants to see, and
	// chasing it would send the caller's headers, the test key among them, to
	// a location the resolved target chose rather than the one authorized here.
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return &Relay{
		client: client,
		opts:   opts,
		slots:  make(chan struct{}, opts.MaxConcurrent),
	}, nil
}

// dialPolicy builds the netguard policy for gateway dials.
//
// PermitPrivateBlockMetadata is the right stance here and the strict
// PublicOnly preset is not: a gateway legitimately lives on a ClusterIP or an
// RFC 1918 address in most deployments, so refusing private space would break
// the ordinary case. What stays refused regardless is the set that is never a
// gateway but is a standard SSRF target, link-local (where 169.254.169.254
// lives), the unspecified address, and multicast/broadcast. Operators narrow
// it further with deny_cidrs, and AllowCIDRs exists only as an explicit,
// deliberately-approved carve-out.
//
// The real containment is upstream of this: Resolver only ever yields a URL
// Platform API named as this API's deployed gateway. This policy is the second
// layer, and it is what holds if that first one is ever weakened.
func dialPolicy(opts RelayOptions) (netguard.Policy, error) {
	policy := netguard.PermitPrivateBlockMetadata()
	policy.AllowedSchemes = []string{"http", "https"}

	deny, err := parseCIDRs(opts.DenyCIDRs)
	if err != nil {
		return netguard.Policy{}, fmt.Errorf("testproxy: deny_cidrs: %w", err)
	}
	allow, err := parseCIDRs(opts.AllowCIDRs)
	if err != nil {
		return netguard.Policy{}, fmt.Errorf("testproxy: allow_cidrs: %w", err)
	}
	policy.DenyCIDRs = deny
	policy.AllowCIDRs = allow
	return policy, nil
}

func parseCIDRs(in []string) ([]*net.IPNet, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]*net.IPNet, 0, len(in))
	for _, raw := range in {
		_, n, err := net.ParseCIDR(raw)
		if err != nil {
			return nil, fmt.Errorf("%q is not a valid CIDR: %w", raw, err)
		}
		out = append(out, n)
	}
	return out, nil
}

// Do relays one request and returns the gateway's response as data.
//
// The returned error means the relay itself failed. A gateway that answered at
// all, with any status, is a successful relay and comes back in Relayed.
func (r *Relay) Do(ctx context.Context, target Target, env Envelope) (Relayed, error) {
	method, err := NormalizeMethod(env.Method)
	if err != nil {
		return Relayed{}, err
	}
	headers, err := SanitizeHeaders(env.Headers)
	if err != nil {
		return Relayed{}, err
	}
	rawQuery, err := SanitizeQuery(env.Query)
	if err != nil {
		return Relayed{}, err
	}
	decodedPath, escapedPath, err := ResolvePath(target.URL.Path, env.Path)
	if err != nil {
		return Relayed{}, err
	}
	body, err := env.DecodeBody()
	if err != nil {
		return Relayed{}, err
	}
	if r.opts.MaxRequestBytes > 0 && int64(len(body)) > r.opts.MaxRequestBytes {
		return Relayed{}, ErrBodyTooLarge
	}

	u := *target.URL
	u.Path = decodedPath
	// RawPath is only consulted by Go when it is a valid alternative encoding
	// of Path; setting it when the two agree would be noise.
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

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return Relayed{}, err
	}
	// The sanitized list is the complete set of headers on the wire. Assigning
	// rather than merging is deliberate: nothing this BFF holds may reach a
	// gateway by default, so there is no base set to merge into.
	req.Header = headers
	req.ContentLength = int64(len(body))

	started := time.Now()
	res, err := r.client.Do(req)
	if err != nil {
		return Relayed{}, err
	}
	defer res.Body.Close()

	// Bound the read one byte past the ceiling so hitting it is detectable.
	// This counts *decompressed* bytes: Go's transport transparently inflates a
	// gzip response it negotiated itself, so the ceiling applies to what the
	// relay would actually hold in memory, not to the compressed size.
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

// acquire takes a concurrency slot, rejecting immediately once running plus
// waiting calls would exceed the pool and its bounded queue together.
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

// trimToRuneBoundary drops a partial UTF-8 sequence from the end of a
// truncated body.
//
// Without it, cutting a text response at an arbitrary byte has a good chance of
// landing mid-rune, which makes the whole body invalid UTF-8 — and EncodeBody
// would then hand the console base64 for what is plainly text. Dropping up to
// three trailing bytes keeps a truncated JSON or XML response readable, which
// is the entire point of truncating rather than failing.
func trimToRuneBoundary(b []byte) []byte {
	for i := 0; i < utf8.UTFMax && len(b) > 0; i++ {
		if r, size := utf8.DecodeLastRune(b); r != utf8.RuneError || size > 1 {
			break
		}
		b = b[:len(b)-1]
	}
	return b
}

// statusText returns the gateway's own reason phrase, falling back to the
// canonical one when the response carries none.
func statusText(res *http.Response) string {
	if _, phrase, ok := cutStatusLine(res.Status); ok && phrase != "" {
		return phrase
	}
	return http.StatusText(res.StatusCode)
}

// cutStatusLine splits "404 Not Found" into code and reason phrase.
func cutStatusLine(status string) (code, phrase string, ok bool) {
	for i := 0; i < len(status); i++ {
		if status[i] == ' ' {
			return status[:i], status[i+1:], true
		}
	}
	return status, "", false
}

// flattenHeaders renders the response headers as an ordered list.
//
// A list, and sorted, for two reasons: a response may repeat a header name
// (Set-Cookie routinely does) and a map would drop all but one, and a stable
// order keeps the console's rendering from reshuffling between identical calls.
//
// These travel as inert JSON strings. None of them is ever written onto the
// BFF's own response, so a gateway cannot set a cookie on the portal origin or
// talk the browser into rendering its body as HTML there.
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
