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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// buildJWKS renders the gateway JWK Set (RFC 7517 §5) served at the gateway
// JWKS endpoint, and its strong entity tag. The keys are published in
// configuration order. The body is generated once per keyring, so the ETag is
// stable for as long as the configured keys are.
func buildJWKS(keys []PublicJWK) (body []byte, etag string, err error) {
	if keys == nil {
		keys = []PublicJWK{}
	}
	body, err = json.Marshal(struct {
		Keys []PublicJWK `json:"keys"`
	}{keys})
	if err != nil {
		return nil, "", err
	}
	return body, jwksETag(body), nil
}

// jwksETag is the quoted strong entity tag of a JWKS body: the first 128 bits
// of its SHA-256, hex-encoded.
func jwksETag(body []byte) string {
	sum := sha256.Sum256(body)
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}
