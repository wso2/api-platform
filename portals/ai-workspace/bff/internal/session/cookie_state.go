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

package session

import (
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"time"

	"ai-workspace-bff/internal/secure"
)

// stateVersion is the envelope format version. A record written by an older BFF is
// rejected rather than guessed at, which costs one re-login at upgrade and never a
// mis-decoded session.
const stateVersion = 1

// ErrStateTooLarge means the session could not be made to fit the configured cookie
// budget even after shedding its optional parts. Callers treat it as "cannot persist"
// — the user stays logged in on the token cookie, at the cost of re-exchanging.
var ErrStateTooLarge = errors.New("session state exceeds the cookie budget")

// stateEnvelope is the wire form of a Session as carried by the browser. Field names
// are short because every byte is multiplied by base64 and paid on every request.
//
// The access token is deliberately NOT in here: it already travels in its own cookie
// pair, and repeating it would roughly double the session's cookie footprint. Bind
// holds a hash of it instead, so a record can be proven to belong to the token
// presented with it — a state cookie salvaged from another session, or kept across a
// token rotation, decrypts fine and is still rejected.
type stateEnvelope struct {
	V    int                `json:"v"`
	Bind string             `json:"b"`
	Mode string             `json:"m,omitempty"`
	RT   string             `json:"rt,omitempty"`
	IT   string             `json:"it,omitempty"`
	AExp int64              `json:"ae,omitempty"`
	XExp int64              `json:"xe,omitempty"`
	User User               `json:"u"`
	Org  string             `json:"oh,omitempty"`
	ODis bool               `json:"od,omitempty"`
	Ex   *exchangedEnvelope `json:"ex,omitempty"`
}

type exchangedEnvelope struct {
	Token  string   `json:"t"`
	Exp    int64    `json:"e"`
	Scopes []string `json:"s,omitempty"`
	FP     string   `json:"f,omitempty"`
	Org    string   `json:"o,omitempty"`
	OrgObj *Org     `json:"g,omitempty"`
}

// CookieCodec encodes a Session into sealed cookie-sized chunks and back. It holds no
// state of its own, so every replica with the same key decodes every other replica's
// records.
type CookieCodec struct {
	sealer    *secure.Sealer
	ChunkSize int
	MaxChunks int
}

// NewCookieCodec builds a codec. chunkSize is the largest value a single cookie may
// carry and maxChunks how many such cookies the session may occupy — together they
// are the budget Encode sheds optional fields to stay inside.
func NewCookieCodec(sealer *secure.Sealer, chunkSize, maxChunks int) *CookieCodec {
	return &CookieCodec{sealer: sealer, ChunkSize: chunkSize, MaxChunks: maxChunks}
}

// Bind is the value a state record carries to tie itself to one access token. A
// truncated SHA-256 rather than the token: it only ever has to distinguish this
// session's token from another's, and the full digest costs bytes on every request.
func Bind(accessToken string) string {
	sum := sha256.Sum256([]byte(accessToken))
	return base64.RawURLEncoding.EncodeToString(sum[:12])
}

// Encode seals the session and splits it into at most MaxChunks values.
//
// Over budget, it sheds in a deliberate order rather than failing: the id_token first
// (needed only to hint the IDP at logout, which degrades to a plain end-session call),
// then the cached exchanged token (costs a re-exchange per request, which is slow but
// correct). The refresh token is never shed — without it the session cannot be renewed
// and the user is logged out mid-work.
func (c *CookieCodec) Encode(s *Session) ([]string, error) {
	env := stateEnvelope{
		V:    stateVersion,
		Bind: Bind(s.AccessToken),
		Mode: s.Mode,
		RT:   s.RefreshToken,
		IT:   s.IDToken,
		User: s.User,
		Org:  s.OrgHandle,
		ODis: s.OrgDiscovered,
	}
	if !s.AccessExpiry.IsZero() {
		env.AExp = s.AccessExpiry.Unix()
	}
	if !s.AbsoluteExpiry.IsZero() {
		env.XExp = s.AbsoluteExpiry.Unix()
	}
	if s.Exchanged.Token != "" {
		env.Ex = &exchangedEnvelope{
			Token:  s.Exchanged.Token,
			Scopes: s.Exchanged.Scopes,
			FP:     s.Exchanged.ConfigFingerprint,
			Org:    s.Exchanged.OrgHandle,
			OrgObj: s.Exchanged.Org,
		}
		if !s.Exchanged.Expiry.IsZero() {
			env.Ex.Exp = s.Exchanged.Expiry.Unix()
		}
	}

	// Each step drops strictly more than the last, so the loop always terminates.
	for step := 0; step < 3; step++ {
		switch step {
		case 1:
			env.IT = ""
		case 2:
			env.Ex = nil
		}
		chunks, err := c.sealChunks(env)
		if err != nil {
			return nil, err
		}
		if len(chunks) <= c.MaxChunks {
			return chunks, nil
		}
	}
	return nil, ErrStateTooLarge
}

func (c *CookieCodec) sealChunks(env stateEnvelope) ([]string, error) {
	raw, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	// Compressed before sealing, never after: ciphertext is incompressible. JWTs are
	// base64 text with repeated claim names, so this reliably pays for itself.
	var buf bytes.Buffer
	zw, err := flate.NewWriter(&buf, flate.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	sealed, err := c.sealer.Seal(buf.Bytes())
	if err != nil {
		return nil, err
	}
	return chunk(sealed, c.ChunkSize), nil
}

// maxStatePlaintext bounds what Decode will inflate, so a crafted cookie cannot be a
// decompression bomb. It can only ever be reached by a record this BFF itself sealed
// — the GCM tag is checked first — but the ceiling costs nothing and removes the
// question. Generous against the real ceiling (MaxChunks * ChunkSize of ciphertext).
const maxStatePlaintext = 1 << 20

// Decode reverses Encode. It returns ok=false — never an error — for every way a
// record can fail to apply: absent, sealed under another key, tampered with, written
// by an older format, bound to a different access token, or past its absolute expiry.
// All of them mean the same thing to the caller (no session state here), and a session
// store's Get has no way to report anything else.
func (c *CookieCodec) Decode(parts []string, accessToken string) (*Session, bool) {
	joined := joinChunks(parts)
	if joined == "" {
		return nil, false
	}
	plain, err := c.sealer.Open(joined)
	if err != nil {
		return nil, false
	}
	raw, err := io.ReadAll(io.LimitReader(flate.NewReader(bytes.NewReader(plain)), maxStatePlaintext+1))
	if err != nil || len(raw) > maxStatePlaintext {
		return nil, false
	}
	var env stateEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, false
	}
	if env.V != stateVersion || env.Bind != Bind(accessToken) {
		return nil, false
	}

	s := &Session{
		ID:            accessToken,
		Mode:          env.Mode,
		AccessToken:   accessToken,
		RefreshToken:  env.RT,
		IDToken:       env.IT,
		User:          env.User,
		OrgHandle:     env.Org,
		OrgDiscovered: env.ODis,
	}
	if env.AExp != 0 {
		s.AccessExpiry = time.Unix(env.AExp, 0)
	}
	if env.XExp != 0 {
		s.AbsoluteExpiry = time.Unix(env.XExp, 0)
	}
	if env.Ex != nil {
		s.Exchanged = ExchangedToken{
			Token:             env.Ex.Token,
			Scopes:            env.Ex.Scopes,
			ConfigFingerprint: env.Ex.FP,
			OrgHandle:         env.Ex.Org,
			Org:               env.Ex.OrgObj,
		}
		if env.Ex.Exp != 0 {
			s.Exchanged.Expiry = time.Unix(env.Ex.Exp, 0)
		}
	}
	if s.Expired(time.Now()) {
		return nil, false
	}
	return s, true
}

// chunk splits a sealed value into fixed-size pieces. Splitting at an arbitrary byte
// offset is safe because the pieces are only ever reassembled in order by joinChunks
// — no piece is independently meaningful.
func chunk(s string, size int) []string {
	if size <= 0 {
		return []string{s}
	}
	out := make([]string, 0, (len(s)+size-1)/size)
	for len(s) > size {
		out = append(out, s[:size])
		s = s[size:]
	}
	return append(out, s)
}

// joinChunks concatenates the pieces, failing closed on a gap: a missing middle
// cookie (dropped by a proxy, or trimmed by a browser at its per-domain limit) would
// otherwise silently produce a shorter value that fails the GCM tag anyway, but with a
// far less obvious cause.
func joinChunks(parts []string) string {
	var b bytes.Buffer
	for _, p := range parts {
		if p == "" {
			return ""
		}
		b.WriteString(p)
	}
	return b.String()
}
