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
	"strings"
	"time"
	"unicode/utf8"

	"ai-workspace-bff/internal/secure"
)

// stateVersion is the envelope format. Bump it for any change to the envelope shape or
// the JWT segment encoding: an old record is then refused cleanly (one re-login) rather
// than decoding into the wrong shape.
const stateVersion = 4

// ErrStateTooLarge means the session will not fit the cookie budget even after shedding
// its optional parts.
var ErrStateTooLarge = errors.New("session state exceeds the cookie budget")

// stateEnvelope is the wire form of a Session as carried by the browser. Field names
// are short because every byte is multiplied by base64 and paid on every request.
//
// The access token itself is not here — it already travels in its own cookie pair.
// Bind holds a hash of it, so a record salvaged from another session, or kept across a
// rotation, decrypts fine and is still rejected.
// Only what cannot be recomputed from what the request already carries. The display
// User and the access expiry are deliberately absent: both are exact functions of the
// access token and id_token, and a stored derivation can disagree with its source.
type stateEnvelope struct {
	V    int                `json:"v"`
	Bind string             `json:"b"`
	RT   string             `json:"rt,omitempty"`
	IT   *storedJWT         `json:"it,omitempty"`
	XExp int64              `json:"xe,omitempty"`
	Org  string             `json:"oh,omitempty"`
	ODis bool               `json:"od,omitempty"`
	Ex   *exchangedEnvelope `json:"ex,omitempty"`
}

type exchangedEnvelope struct {
	Token  *storedJWT `json:"t"`
	Exp    int64      `json:"e"`
	Scopes []string   `json:"s,omitempty"`
	FP     string     `json:"f,omitempty"`
	Org    string     `json:"o,omitempty"`
	OrgObj *Org       `json:"g,omitempty"`
}

// storedJWT keeps a JWT as decoded segments rather than the compact string, because
// base64 hides its contents from the compressor: the ~3 KB `scope` claim is highly
// repetitive text that flate can only reach once decoded. Worth ~30% of the record.
//
// splitJWT falls back to the original string unless the decode round-trips
// byte-identically — the token is forwarded to the Platform API, where an altered
// segment would fail signature verification.
type storedJWT struct {
	Raw string `json:"r,omitempty"` // set only when not a round-trippable 3-segment JWT

	Header  string `json:"h,omitempty"` // decoded JSON text
	Payload string `json:"p,omitempty"` // decoded JSON text — the compressible part
	Sig     string `json:"s,omitempty"` // left base64; binary
}

// splitJWT converts a compact JWT into its stored form, or nil for an absent token.
func splitJWT(token string) *storedJWT {
	if token == "" {
		return nil
	}
	seg := strings.Split(token, ".")
	if len(seg) != 3 {
		return &storedJWT{Raw: token}
	}
	decoded := make([]string, 2)
	for i := 0; i < 2; i++ { // header and payload only; the signature is binary
		raw, err := base64.RawURLEncoding.DecodeString(seg[i])
		if err != nil || !utf8.Valid(raw) ||
			base64.RawURLEncoding.EncodeToString(raw) != seg[i] {
			return &storedJWT{Raw: token}
		}
		decoded[i] = string(raw)
	}
	return &storedJWT{Header: decoded[0], Payload: decoded[1], Sig: seg[2]}
}

// join reverses splitJWT.
func (j *storedJWT) join() string {
	switch {
	case j == nil:
		return ""
	case j.Raw != "":
		return j.Raw
	default:
		return base64.RawURLEncoding.EncodeToString([]byte(j.Header)) + "." +
			base64.RawURLEncoding.EncodeToString([]byte(j.Payload)) + "." + j.Sig
	}
}

// CookieCodec encodes a Session into sealed cookie-sized chunks and back. It holds no
// state of its own, so every replica with the same key decodes every other replica's
// records.
type CookieCodec struct {
	sealer    *secure.Sealer
	mapping   ClaimMapping
	ChunkSize int
	MaxChunks int
}

// NewCookieCodec builds a codec. chunkSize is the largest value a single cookie may
// carry and maxChunks how many such cookies the session may occupy — together they
// are the budget Encode sheds optional fields to stay inside.
// mapping is the login claim mapping. Decode rebuilds the display User with it rather
// than reading a stored copy, so it must be the same mapping the login path uses.
func NewCookieCodec(sealer *secure.Sealer, chunkSize, maxChunks int, mapping ClaimMapping) *CookieCodec {
	return &CookieCodec{sealer: sealer, mapping: mapping, ChunkSize: chunkSize, MaxChunks: maxChunks}
}

// Bind ties a record to one access token. Truncated because it only has to distinguish
// this session's token from another's, and every byte is paid on every request.
func Bind(accessToken string) string {
	sum := sha256.Sum256([]byte(accessToken))
	return base64.RawURLEncoding.EncodeToString(sum[:12])
}

// Encode seals the session into at most MaxChunks values. Over budget it sheds rather
// than fails: the id_token first (costs the logout hint and the display claims), then
// the cached exchange (costs a re-exchange per request). The refresh token is never
// shed — without it the session cannot be renewed at all.
func (c *CookieCodec) Encode(s *Session) ([]string, error) {
	env := stateEnvelope{
		V:    stateVersion,
		Bind: Bind(s.AccessToken),
		RT:   s.RefreshToken,
		IT:   splitJWT(s.IDToken),
		Org:  s.OrgHandle,
		ODis: s.OrgDiscovered,
	}
	if !s.AbsoluteExpiry.IsZero() {
		env.XExp = s.AbsoluteExpiry.Unix()
	}
	if s.Exchanged.Token != "" {
		env.Ex = &exchangedEnvelope{
			Token:  splitJWT(s.Exchanged.Token),
			Scopes: s.Exchanged.Scopes,
			FP:     s.Exchanged.ConfigFingerprint,
			Org:    s.Exchanged.OrgHandle,
			OrgObj: s.Exchanged.Org,
		}
		if !s.Exchanged.Expiry.IsZero() {
			env.Ex.Exp = s.Exchanged.Expiry.Unix()
		}
	}

	// Each step drops strictly more than the last, so this terminates.
	for step := 0; step < 3; step++ {
		switch step {
		case 1:
			env.IT = nil
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
	// Compressed before sealing, never after: ciphertext is incompressible.
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

// maxStatePlaintext bounds what Decode will inflate. Only reachable by a record this
// BFF sealed (the GCM tag is checked first), but the ceiling costs nothing.
const maxStatePlaintext = 1 << 20

// Decode reverses Encode, returning ok=false for every way a record can fail to apply
// — absent, wrong key, tampered, stale format, wrong access token, expired. All mean
// the same thing to the caller, and Store.Get can report nothing else.
func (c *CookieCodec) Decode(parts []string, accessToken string) (*Session, bool) {
	joined := joinChunks(parts)
	if joined == "" {
		return nil, false
	}
	plain, err := c.sealer.Open(joined)
	if err != nil {
		return nil, false
	}
	zr := flate.NewReader(bytes.NewReader(plain))
	raw, err := io.ReadAll(io.LimitReader(zr, maxStatePlaintext+1))
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

	idToken := env.IT.join()
	atClaims := DecodeJWTClaims(accessToken)
	s := &Session{
		ID:           accessToken,
		Mode:         ModeOIDC, // the only mode that uses the store
		AccessToken:  accessToken,
		RefreshToken: env.RT,
		IDToken:      idToken,
		// Recomputed from the two tokens, exactly as the login path built it.
		User:          UserFromClaims(atClaims, DecodeJWTClaims(idToken), c.mapping),
		AccessExpiry:  ExpiryFromClaims(atClaims),
		OrgHandle:     env.Org,
		OrgDiscovered: env.ODis,
	}
	if env.XExp != 0 {
		s.AbsoluteExpiry = time.Unix(env.XExp, 0)
	}
	if env.Ex != nil {
		s.Exchanged = ExchangedToken{
			Token:             env.Ex.Token.join(),
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

// chunk splits a sealed value into fixed-size pieces, reassembled in order by
// joinChunks. No piece is independently meaningful.
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

// joinChunks fails closed on a gap. A missing middle cookie would otherwise produce a
// shorter value that fails the GCM tag anyway, but with a far less obvious cause.
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
