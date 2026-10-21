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
	"fmt"

	sdksigning "github.com/wso2/api-platform/sdk/core/signing"
)

// Sign returns the raw signature over data made with the active key, in the
// JWS encoding for its algorithm (RFC 7518 §3; r||s for ECDSA).
func (kr *Keyring) Sign(data []byte) ([]byte, sdksigning.KeyInfo, error) {
	if kr == nil || kr.active == nil {
		return nil, sdksigning.KeyInfo{}, sdksigning.ErrNotConfigured
	}
	sig, err := kr.active.sign(data)
	if err != nil {
		return nil, sdksigning.KeyInfo{}, fmt.Errorf("signing failed: %w", err)
	}
	return sig, kr.info, nil
}

// SignJWS returns a compact-serialised JWS (RFC 7515 §7.1) over payload.
func (kr *Keyring) SignJWS(payload []byte, opts sdksigning.Options) (string, sdksigning.KeyInfo, error) {
	protected, sig, info, err := kr.signJWS(payload, opts)
	if err != nil {
		return "", sdksigning.KeyInfo{}, err
	}
	return protected + "." + b64.EncodeToString(payload) + "." + sig, info, nil
}

// SignDetachedJWS signs payload and returns the signature without it
// (RFC 7515 Appendix F).
func (kr *Keyring) SignDetachedJWS(payload []byte, opts sdksigning.Options) (sdksigning.DetachedJWS, sdksigning.KeyInfo, error) {
	protected, sig, info, err := kr.signJWS(payload, opts)
	if err != nil {
		return sdksigning.DetachedJWS{}, sdksigning.KeyInfo{}, err
	}
	return sdksigning.DetachedJWS{Protected: protected, Signature: sig}, info, nil
}

// signJWS returns the encoded protected header and signature for payload.
//
// The SDK option builders already refuse reserved headers; they are checked
// again here because Options is a plain struct a caller can fill in directly.
// alg and kid are always taken from the key that signs, and jku only ever from
// the operator-configured URL.
func (kr *Keyring) signJWS(payload []byte, o sdksigning.Options) (protected, sig string, info sdksigning.KeyInfo, err error) {
	if kr == nil || kr.active == nil {
		return "", "", sdksigning.KeyInfo{}, sdksigning.ErrNotConfigured
	}

	header := make(map[string]any, len(o.Headers)+4)
	for k, v := range o.Headers {
		if sdksigning.ReservedHeaders[k] || k == "typ" {
			return "", "", sdksigning.KeyInfo{}, fmt.Errorf("header %q is set by the gateway signing service and cannot be overridden", k)
		}
		header[k] = v
	}
	header["alg"] = string(kr.info.Algorithm)
	header["kid"] = kr.info.Kid
	if o.Type != "" {
		header["typ"] = o.Type
	}
	if o.JKU && kr.jwksURL != "" {
		header["jku"] = kr.jwksURL
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", "", sdksigning.KeyInfo{}, fmt.Errorf("marshal JWS header: %w", err)
	}
	protected = b64.EncodeToString(headerJSON)

	signingInput := make([]byte, 0, len(protected)+1+b64.EncodedLen(len(payload)))
	signingInput = append(signingInput, protected...)
	signingInput = append(signingInput, '.')
	signingInput = b64.AppendEncode(signingInput, payload)

	raw, err := kr.active.sign(signingInput)
	if err != nil {
		return "", "", sdksigning.KeyInfo{}, fmt.Errorf("signing failed: %w", err)
	}
	return protected, b64.EncodeToString(raw), kr.info, nil
}
