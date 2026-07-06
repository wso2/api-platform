/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Verifier validates the HMAC-SHA256 signature of an incoming webhook request.
//
// The Developer Portal signs each request with a per-subscriber shared secret. The signature
// header has the form "t=<unix-seconds>,v1=<hex-hmac>" and the signed payload is
// "<timestamp>.<raw_body>". This matches the producer scheme documented in
// docs-local/platform-api-webhook.md.
type Verifier struct {
	secret    []byte
	tolerance time.Duration
}

// NewVerifier builds a Verifier. tolerance bounds how old a signed request may be (replay guard).
func NewVerifier(secret string, tolerance time.Duration) *Verifier {
	return &Verifier{secret: []byte(secret), tolerance: tolerance}
}

// Verify checks the signature header against the raw request body. It returns nil when the
// request is authentic and a sentinel error (ErrSignature*/ErrTimestampOutOfTolerance) otherwise.
// now is injected for testability; callers pass time.Now().
func (v *Verifier) Verify(header string, body []byte, now time.Time) error {
	if strings.TrimSpace(header) == "" {
		return ErrSignatureMissing
	}

	ts, sig, err := parseSignatureHeader(header)
	if err != nil {
		return err
	}

	// Reject stale (or future-dated) signatures to prevent replay.
	if v.tolerance > 0 {
		delta := now.Sub(time.Unix(ts, 0))
		if delta < 0 {
			delta = -delta
		}
		if delta > v.tolerance {
			return ErrTimestampOutOfTolerance
		}
	}

	signedPayload := strconv.FormatInt(ts, 10) + "." + string(body)
	mac := hmac.New(sha256.New, v.secret)
	mac.Write([]byte(signedPayload))
	expected := mac.Sum(nil)

	got, err := hex.DecodeString(sig)
	if err != nil {
		return ErrSignatureInvalid
	}
	if !hmac.Equal(expected, got) {
		return ErrSignatureInvalid
	}
	return nil
}

// parseSignatureHeader extracts the t and v1 components from a header like "t=123,v1=abcdef".
func parseSignatureHeader(header string) (timestamp int64, signature string, err error) {
	var tsStr, sigStr string
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch strings.TrimSpace(kv[0]) {
		case "t":
			tsStr = strings.TrimSpace(kv[1])
		case "v1":
			sigStr = strings.TrimSpace(kv[1])
		}
	}
	if tsStr == "" || sigStr == "" {
		return 0, "", fmt.Errorf("%w: malformed signature header", ErrSignatureInvalid)
	}
	ts, parseErr := strconv.ParseInt(tsStr, 10, 64)
	if parseErr != nil {
		return 0, "", fmt.Errorf("%w: invalid timestamp", ErrSignatureInvalid)
	}
	return ts, sigStr, nil
}
