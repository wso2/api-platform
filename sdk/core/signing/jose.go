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

package signing

import (
	"encoding/json"
	"errors"
	"fmt"
)

// reservedHeaders are set only by this package. alg and kid always describe
// the key that actually signed; jku is set only through WithJKU so it can only
// ever point at the operator-configured JWKS; crit and b64 would change how a
// verifier must process the token, which no caller of this package needs.
var reservedHeaders = map[string]bool{"alg": true, "kid": true, "jku": true, "crit": true, "b64": true}

type options struct {
	typ     string
	headers map[string]any
	jku     bool
	err     error
}

// Option customises the protected header of a JWS.
type Option func(*options)

// WithType sets the "typ" header, e.g. "JWT" (the SignJWT default) or an
// application-specific media type.
func WithType(typ string) Option {
	return func(o *options) { o.typ = typ }
}

// WithHeader adds a protected header parameter. alg, kid, jku, crit and b64
// are reserved; setting one makes the signing call fail.
func WithHeader(name string, value any) Option {
	return func(o *options) {
		if reservedHeaders[name] || name == "typ" {
			if name == "typ" {
				o.err = errors.New(`use WithType to set the "typ" header`)
			} else {
				o.err = fmt.Errorf("header %q is set by the gateway signing service and cannot be overridden", name)
			}
			return
		}
		if o.headers == nil {
			o.headers = map[string]any{}
		}
		o.headers[name] = value
	}
}

// WithJKU adds the "jku" header set to the operator-configured public JWKS
// URL ([signing] public_jwks_url). It is a no-op when none is configured.
func WithJKU() Option {
	return func(o *options) { o.jku = true }
}

// Sign returns the raw signature over data made with the active gateway key,
// and the key that made it. The encoding is the JWS one for the key's
// algorithm (RFC 7518 §3; r||s for ECDSA). Use it for formats that are not
// JOSE; prefer the JWS functions otherwise.
func Sign(data []byte) ([]byte, KeyInfo, error) {
	kr, err := activeKeyring()
	if err != nil {
		return nil, KeyInfo{}, err
	}
	sig, err := kr.active.sign(data)
	if err != nil {
		return nil, KeyInfo{}, fmt.Errorf("signing failed: %w", err)
	}
	return sig, kr.info, nil
}

// SignJWS returns a compact-serialised JWS (RFC 7515 §7.1) over payload.
func SignJWS(payload []byte, opts ...Option) (string, KeyInfo, error) {
	protected, sig, info, err := signJWS(payload, opts)
	if err != nil {
		return "", KeyInfo{}, err
	}
	return protected + "." + b64.EncodeToString(payload) + "." + sig, info, nil
}

// SignJWT marshals claims to JSON and returns them as a signed compact JWT
// (RFC 7519) with typ "JWT" unless WithType says otherwise. Claim validation
// (exp, iat, iss, ...) is the caller's responsibility.
func SignJWT(claims map[string]any, opts ...Option) (string, KeyInfo, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", KeyInfo{}, fmt.Errorf("marshal JWT claims: %w", err)
	}
	return SignJWS(payload, append([]Option{WithType("JWT")}, opts...)...)
}

// DetachedJWS is a JWS whose payload is carried separately (RFC 7515
// Appendix F). Its JSON form is the A2A AgentCardSignature object.
type DetachedJWS struct {
	Protected string         `json:"protected"`
	Signature string         `json:"signature"`
	Header    map[string]any `json:"header,omitempty"`
}

// SignDetachedJWS signs payload and returns the signature without it. The
// verifier reconstructs the signing input from its own copy of the payload,
// so the payload must be byte-identical on both sides; for a JSON document,
// sign CanonicalJSON(document).
func SignDetachedJWS(payload []byte, opts ...Option) (DetachedJWS, KeyInfo, error) {
	protected, sig, info, err := signJWS(payload, opts)
	if err != nil {
		return DetachedJWS{}, KeyInfo{}, err
	}
	return DetachedJWS{Protected: protected, Signature: sig}, info, nil
}

// signJWS returns the encoded protected header and signature for payload.
func signJWS(payload []byte, opts []Option) (protected, sig string, info KeyInfo, err error) {
	kr, err := activeKeyring()
	if err != nil {
		return "", "", KeyInfo{}, err
	}

	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if o.err != nil {
		return "", "", KeyInfo{}, o.err
	}

	header := make(map[string]any, len(o.headers)+4)
	for k, v := range o.headers {
		header[k] = v
	}
	header["alg"] = string(kr.info.Algorithm)
	header["kid"] = kr.info.Kid
	if o.typ != "" {
		header["typ"] = o.typ
	}
	if o.jku && kr.jwksURL != "" {
		header["jku"] = kr.jwksURL
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", "", KeyInfo{}, fmt.Errorf("marshal JWS header: %w", err)
	}
	protected = b64.EncodeToString(headerJSON)

	signingInput := make([]byte, 0, len(protected)+1+b64.EncodedLen(len(payload)))
	signingInput = append(signingInput, protected...)
	signingInput = append(signingInput, '.')
	signingInput = b64.AppendEncode(signingInput, payload)

	raw, err := kr.active.sign(signingInput)
	if err != nil {
		return "", "", KeyInfo{}, fmt.Errorf("signing failed: %w", err)
	}
	return protected, b64.EncodeToString(raw), kr.info, nil
}
