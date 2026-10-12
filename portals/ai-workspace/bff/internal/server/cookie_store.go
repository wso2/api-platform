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

package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ai-workspace-bff/internal/session"
)

// The session-state cookies carry the OIDC refresh/id tokens, the cached exchanged
// token and the selected org, sealed under a key every replica derives identically.
//
// stateChunkSize stays under the ~4 KB per-cookie ceiling browsers and proxies enforce;
// stateMaxChunks bounds the total.
const (
	stateChunkSize      = 3500
	stateMaxChunks      = 4
	stateChunkWarnCount = 2

	// nginx, Apache, Tomcat and CloudFront all default to 8 KB for a single header line,
	// shared with every other cookie on this host. 6 KB is where an operator should
	// hear about it, while they still have room to act.
	cookieHeaderWarnBytes = 6 << 10
	cookieWarnInterval    = 10 * time.Minute
)

var lastCookieWarn atomic.Int64

// warnIfHeaderLarge reports on what the NEXT request will carry, at most once per
// cookieWarnInterval — this is a property of the deployment, not of one request.
func warnIfHeaderLarge(r *http.Request, stateBytes int) {
	incoming := len(r.Header.Get("Cookie"))
	existingState := 0
	for _, c := range r.Cookies() {
		if strings.HasPrefix(c.Name, stateCookiePrefixOf(r)) {
			existingState += len(c.Value)
		}
	}
	projected := incoming - existingState + stateBytes
	if projected < cookieHeaderWarnBytes {
		return
	}

	now := time.Now().Unix()
	prev := lastCookieWarn.Load()
	if now-prev < int64(cookieWarnInterval/time.Second) || !lastCookieWarn.CompareAndSwap(prev, now) {
		return
	}
	slog.Warn("the session's Cookie header is approaching the size most proxies accept on a "+
		"single header line; raise the request-header buffers of any ingress in front of this "+
		"BFF (nginx: large_client_header_buffers) before it starts rejecting requests",
		"cookie_header_bytes", projected,
		"sealed_state_bytes", stateBytes,
		"typical_proxy_limit_bytes", 8<<10)
}

// stateCarrier is one request's view of the session-state cookies. The session.Store
// below reaches it through the request context, which is what let the process-local map
// be replaced without touching a single call site.
type stateCarrier struct {
	srv *Server
	r   *http.Request

	mu sync.Mutex
	// staged supersedes whatever arrived: a handler routinely writes and then re-reads
	// a session within one request (exchange caches a token, hydration reads it back).
	staged  *session.Session
	cleared bool

	// pending is emitted once by flush, so each cookie gets one authoritative value per
	// response — a login writes the session twice and two values for one name is
	// something an intermediary gets to choose between.
	//
	// dirty separates "this request rewrote the session" from "only read it". Without
	// it a read-only request reaches flush with nothing pending, which is
	// indistinguishable from a delete, and expires the cookies it just read.
	dirty    bool
	pending  []string
	pendingA int

	// present counts the chunk cookies that arrived; a shrinking record must expire the
	// indices it no longer occupies or a stale trailing chunk corrupts reassembly.
	present int
	flushed bool
}

func newStateCarrier(srv *Server, r *http.Request) *stateCarrier {
	c := &stateCarrier{srv: srv, r: r}
	for i := 0; i < stateMaxChunks; i++ {
		if _, err := r.Cookie(stateCookieName(srv.cfg.Cookie.StatePrefix, i)); err != nil {
			break
		}
		c.present = i + 1
	}
	return c
}

func stateCookieName(prefix string, i int) string { return prefix + strconv.Itoa(i) }

func stateCookiePrefixOf(*http.Request) string { return defaultStatePrefix }

// defaultStatePrefix mirrors config's stateCookiePrefix. Duplicated rather than
// imported because being wrong here costs an inaccurate log line and nothing else.
const defaultStatePrefix = "_ai_workspace_state_"

func (c *stateCarrier) get(id string) (*session.Session, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.staged != nil {
		if c.staged.AccessToken == id {
			cp := *c.staged
			return &cp, true
		}
		// Staged under a different token means it just rotated (doRefresh); fall
		// through and let Decode's binding check decide.
	} else if c.cleared {
		return nil, false
	}

	parts := make([]string, 0, c.present)
	for i := 0; i < c.present; i++ {
		ck, err := c.r.Cookie(stateCookieName(c.srv.cfg.Cookie.StatePrefix, i))
		if err != nil {
			return nil, false
		}
		parts = append(parts, ck.Value)
	}
	if len(parts) == 0 {
		return nil, false
	}
	return c.srv.stateCodec.Decode(parts, id)
}

func (c *stateCarrier) put(s *session.Session) error {
	chunks, err := c.srv.stateCodec.Encode(s)
	if err != nil {
		return err
	}
	stateBytes := 0
	for _, c := range chunks {
		stateBytes += len(c)
	}
	if len(chunks) > stateChunkWarnCount {
		slog.Warn("session state needs several cookies",
			"cookies", len(chunks), "bytes", stateBytes)
	}
	warnIfHeaderLarge(c.r, stateBytes)

	maxAge := 0
	if !s.AbsoluteExpiry.IsZero() {
		if d := time.Until(s.AbsoluteExpiry); d > 0 {
			maxAge = int(d.Seconds())
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	cp := *s
	c.staged = &cp
	c.pending = chunks
	c.pendingA = maxAge
	c.cleared = false
	c.dirty = true
	return nil
}

func (c *stateCarrier) delete(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// The refresh path puts the rotated session then deletes the pre-rotation one,
	// which here is the same storage slot. Deleting would discard the renewed session.
	if c.staged != nil && c.staged.AccessToken != id {
		return
	}
	c.staged = nil
	c.pending = nil
	c.pendingA = 0
	c.cleared = true
	c.dirty = true
}

// flush emits this request's single verdict on the state cookies, immediately before
// the response headers go out.
func (c *stateCarrier) flush(w http.ResponseWriter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.flushed {
		return
	}
	c.flushed = true
	// A read-only request says nothing about these cookies.
	if !c.dirty {
		return
	}

	for i, value := range c.pending {
		c.srv.writeStateCookie(w, i, value, c.pendingA)
	}
	// Expire whatever the browser holds beyond what this response sets — covers both a
	// shrunken record and an outright delete (pending nil).
	for i := len(c.pending); i < c.present; i++ {
		c.srv.expireStateCookie(w, i)
	}
}

// stateWriter defers the state cookies to just before the response starts, so each name
// is set once however many times a handler rewrote the session.
type stateWriter struct {
	http.ResponseWriter
	carrier *stateCarrier
}

func (w *stateWriter) WriteHeader(code int) {
	w.carrier.flush(w.ResponseWriter)
	w.ResponseWriter.WriteHeader(code)
}

func (w *stateWriter) Write(b []byte) (int, error) {
	w.carrier.flush(w.ResponseWriter)
	return w.ResponseWriter.Write(b)
}

// Flush commits the response: a streamed (SSE) proxied response flushes before Write.
func (w *stateWriter) Flush() {
	w.carrier.flush(w.ResponseWriter)
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer, which the reverse
// proxy relies on.
func (w *stateWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// cookieStore is a session.Store whose storage is the client's own cookies. Outside a
// request it reports an empty store rather than panicking — a miss costs a re-exchange,
// never a wrong session.
type cookieStore struct{}

type stateCarrierKey struct{}

func carrierFrom(ctx context.Context) *stateCarrier {
	c, _ := ctx.Value(stateCarrierKey{}).(*stateCarrier)
	return c
}

// errNoCarrier is returned rather than ignored so the one caller that treats
// persistence as load-bearing (login) fails loudly instead of handing out a session
// that cannot be renewed.
var errNoCarrier = errors.New("no session-state carrier on this request")

func (cookieStore) Put(ctx context.Context, s *session.Session) error {
	c := carrierFrom(ctx)
	if c == nil {
		return errNoCarrier
	}
	return c.put(s)
}

func (cookieStore) Get(ctx context.Context, id string) (*session.Session, bool, error) {
	c := carrierFrom(ctx)
	if c == nil {
		return nil, false, nil
	}
	s, ok := c.get(id)
	return s, ok, nil
}

func (cookieStore) Delete(ctx context.Context, id string) error {
	if c := carrierFrom(ctx); c != nil {
		c.delete(id)
	}
	return nil
}

// Touch must re-seal and re-set the cookies: the expiry it extends is inside the
// sealed record.
func (cookieStore) Touch(ctx context.Context, id string, extendTo time.Time) error {
	c := carrierFrom(ctx)
	if c == nil {
		return nil
	}
	s, ok := c.get(id)
	if !ok || !extendTo.After(s.AbsoluteExpiry) {
		return nil
	}
	s.AbsoluteExpiry = extendTo
	return c.put(s)
}

// Close is a no-op: nothing is held between requests.
func (cookieStore) Close() error { return nil }

// withSessionState gives every request a carrier. Installed for the whole mux: a route
// that quietly lacked one would degrade to "no session" with nothing pointing at why.
func (s *Server) withSessionState(next http.Handler) http.Handler {
	// No codec means file-based auth, which has no server-side session to seal.
	if s.stateCodec == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		carrier := newStateCarrier(s, r)
		ctx := context.WithValue(r.Context(), stateCarrierKey{}, carrier)
		sw := &stateWriter{ResponseWriter: w, carrier: carrier}
		next.ServeHTTP(sw, r.WithContext(ctx))
		// A handler that wrote nothing: net/http sends headers after this returns.
		carrier.flush(w)
	})
}

// writeStateCookie sets one chunk: HttpOnly so script can never read the sealed record,
// Path-scoped so a host serving several portals keeps them apart.
func (s *Server) writeStateCookie(w http.ResponseWriter, i int, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName(s.cfg.Cookie.StatePrefix, i),
		Value:    value,
		Path:     s.path("/"),
		HttpOnly: true,
		Secure:   s.cfg.Cookie.Secure,
		SameSite: sameSite(s.cfg.Cookie.SameSite),
		MaxAge:   maxAge,
	})
}

func (s *Server) expireStateCookie(w http.ResponseWriter, i int) {
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName(s.cfg.Cookie.StatePrefix, i),
		Value:    "",
		Path:     s.path("/"),
		HttpOnly: true,
		Secure:   s.cfg.Cookie.Secure,
		SameSite: sameSite(s.cfg.Cookie.SameSite),
		MaxAge:   -1,
	})
}
