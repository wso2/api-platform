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
	"sync"
	"time"

	"ai-workspace-bff/internal/session"
)

// The session-state cookies carry what used to live in the BFF's process memory: the
// OIDC refresh/id tokens, the cached exchanged token, and the selected organization.
// Sealed (AES-256-GCM) under a key every replica derives identically, so a request
// that lands on a replica which has never seen this user still finds its state.
//
// stateChunkSize stays under the ~4 KB per-cookie ceiling browsers and intermediate
// proxies enforce; stateMaxChunks bounds the total so a session can never grow into a
// request header no proxy will accept. A session needing more than
// stateChunkWarnCount cookies is logged once per write — it still works, but it is
// the point at which an ingress's header-buffer limits start to matter.
const (
	stateChunkSize      = 3500
	stateMaxChunks      = 4
	stateChunkWarnCount = 2
)

// stateCarrier is one request's view of the session-state cookies: it decodes them on
// demand and writes replacements onto this request's response.
//
// Request-scoped rather than a process-wide map, because that is the whole point — it
// holds nothing between requests. The session.Store implementation below reaches it
// through the request context, so every existing store call site keeps working
// unchanged.
type stateCarrier struct {
	srv *Server
	r   *http.Request

	mu sync.Mutex
	// staged is the state written during this request, which supersedes whatever
	// arrived on it. Needed because a handler routinely writes and then re-reads a
	// session within one request (exchange caches a token, hydration reads it back).
	staged  *session.Session
	cleared bool
	// pending is what the response will carry, replaced in place by each write and
	// emitted once by flush. One authoritative Set-Cookie per name per response: a
	// login writes the session twice (the refresh state, then the exchanged token
	// cached onto it), and emitting both would leave two values for the same cookie
	// on the wire for any intermediary to pick between.
	//
	// dirty is what separates "this request rewrote the session" from "this request
	// only read it". Without it, a read-only request (GET /api/session, or a proxied
	// call that hit the exchanged-token cache and so wrote nothing) would reach flush
	// with nothing pending and expire the very cookies it had just read — deleting
	// the session's refresh token and cached exchange on every such request.
	dirty    bool
	pending  []string
	pendingA int
	// present counts the chunk cookies that arrived. A shrinking record must expire
	// the indices it no longer occupies, or a stale trailing chunk corrupts the next
	// request's reassembly.
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

func (c *stateCarrier) get(id string) (*session.Session, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.staged != nil {
		if c.staged.AccessToken == id {
			cp := *c.staged
			return &cp, true
		}
		// A state staged for a different token means the token just rotated under
		// us (doRefresh). The record on the request still belongs to the old token,
		// so fall through and let Decode's binding check decide.
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
	if len(chunks) > stateChunkWarnCount {
		slog.Warn("session state needs several cookies; make sure any ingress in front of "+
			"this BFF accepts request headers of this size",
			"cookies", len(chunks), "bytes", len(chunks[0])*(len(chunks)-1)+len(chunks[len(chunks)-1]))
	}

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

	// A state written earlier in this request for a DIFFERENT token must survive: the
	// refresh path puts the rotated session and then deletes the pre-rotation one,
	// which in a cookie world is the same storage slot. Deleting here would throw away
	// the session that was just renewed.
	if c.staged != nil && c.staged.AccessToken != id {
		return
	}
	c.staged = nil
	c.pending = nil
	c.pendingA = 0
	c.cleared = true
	c.dirty = true
}

// flush emits this request's single verdict on the state cookies: the record staged
// by the last write, or an expiry of everything that arrived when the session was
// deleted (or shrank to fewer chunks). Called once, immediately before the response
// headers go out.
func (c *stateCarrier) flush(w http.ResponseWriter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.flushed {
		return
	}
	c.flushed = true
	// A request that only read the session says nothing about these cookies: leave
	// the browser holding exactly what it sent.
	if !c.dirty {
		return
	}

	for i, value := range c.pending {
		c.srv.writeStateCookie(w, i, value, c.pendingA)
	}
	// Everything the browser already holds beyond what this response sets. Covers both
	// a record that got smaller and a session that was deleted outright (pending nil).
	for i := len(c.pending); i < c.present; i++ {
		c.srv.expireStateCookie(w, i)
	}
}

// stateWriter defers the state cookies to the last moment before the response starts,
// so each name is set exactly once however many times a handler rewrote the session.
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

// Flush commits the response, so the cookies must be on it by then — a streamed
// (SSE) proxied response flushes before it ever calls Write.
func (w *stateWriter) Flush() {
	w.carrier.flush(w.ResponseWriter)
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer's own
// Flush/Hijack/deadline support, which the reverse proxy relies on.
func (w *stateWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// cookieStore is a session.Store whose storage is the client's own cookies. Every
// method resolves the current request's carrier from the context; outside a request
// (or in a code path that never went through withSessionState) it reports an empty
// store rather than panicking — a miss costs a re-exchange, never a wrong session.
type cookieStore struct{}

type stateCarrierKey struct{}

func carrierFrom(ctx context.Context) *stateCarrier {
	c, _ := ctx.Value(stateCarrierKey{}).(*stateCarrier)
	return c
}

// errNoCarrier means a store write was attempted outside an HTTP request. Returned
// rather than ignored, so the one caller that treats persistence as load-bearing
// (login) fails loudly instead of handing out a session that cannot be renewed.
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

// Touch extends the record's absolute expiry by rewriting it. Unlike the in-memory
// store it must re-seal and re-set the cookies, since the expiry it is extending is
// part of the sealed record.
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

// Close is a no-op: there is nothing held between requests to release.
func (cookieStore) Close() error { return nil }

// withSessionState gives every request a carrier for the session-state cookies.
// Installed for the whole mux rather than per route: the store is reached from
// handlers, composite handlers and the proxy path alike, and a route that quietly
// lacked a carrier would degrade to "no session" with nothing pointing at why.
func (s *Server) withSessionState(next http.Handler) http.Handler {
	// No codec means the memory store (or file-based auth) is in use and there is
	// nothing for a carrier to seal — skip it entirely rather than attach one that
	// would have no codec to call.
	if s.stateCodec == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		carrier := newStateCarrier(s, r)
		ctx := context.WithValue(r.Context(), stateCarrierKey{}, carrier)
		sw := &stateWriter{ResponseWriter: w, carrier: carrier}
		next.ServeHTTP(sw, r.WithContext(ctx))
		// A handler that returned without writing anything: net/http sends the
		// headers after this returns, so the cookies still make it onto the response.
		carrier.flush(w)
	})
}

// writeStateCookie sets one chunk. Same attributes as the session cookie pair —
// HttpOnly so script can never read the sealed record, and Path-scoped to this app so
// a host serving several portals does not ship this one's state to the others.
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

// clearOrphanStateCookies expires the sealed state cookies on a BFF that is NOT using
// the cookie store. They can only be leftovers from a deployment that has since been
// switched to store = "memory", and nothing would ever clear them otherwise — a
// browser keeps a cookie until something expires it by name. With the cookie store in
// use, the carrier owns these cookies and clears them itself on delete (see flush), so
// writing them here as well would put two values for the same name on one response.
func (s *Server) clearOrphanStateCookies(w http.ResponseWriter) {
	if s.stateCodec != nil {
		return
	}
	for i := 0; i < stateMaxChunks; i++ {
		s.expireStateCookie(w, i)
	}
}
