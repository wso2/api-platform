/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package devportalwebhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const replayWindowSeconds = 300

var (
	// ErrSignatureInvalid is returned when the HMAC digest does not match or the header is malformed.
	ErrSignatureInvalid = errors.New("devportal webhook: invalid signature")
	// ErrReplayDetected is returned when the request timestamp is outside the replay window.
	ErrReplayDetected = errors.New("devportal webhook: request outside replay window")
)

// VerifySignature validates the X-Devportal-Signature header against the raw request body.
//
// Header format: "t=<unix_ts>,v1=<hex_hmac>"
// Signed message: "<unix_ts>.<raw_body>"
// Algorithm: HMAC-SHA256 with the shared secret.
// Replay window: 300 seconds in either direction.
func VerifySignature(secret []byte, header string, body []byte, now time.Time) error {
	ts, sig, err := parseSignatureHeader(header)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSignatureInvalid, err)
	}

	delta := now.Unix() - ts
	if delta < 0 {
		delta = -delta
	}
	if delta > replayWindowSeconds {
		return ErrReplayDetected
	}

	message := strconv.FormatInt(ts, 10) + "." + string(body)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(message))
	expected := mac.Sum(nil)

	received, err := hex.DecodeString(sig)
	if err != nil {
		return fmt.Errorf("%w: v1 field is not valid hex", ErrSignatureInvalid)
	}

	if !hmac.Equal(expected, received) {
		return ErrSignatureInvalid
	}
	return nil
}

// parseSignatureHeader splits "t=<ts>,v1=<hex>" into (timestamp, hexSig, error).
func parseSignatureHeader(header string) (int64, string, error) {
	if header == "" {
		return 0, "", fmt.Errorf("missing X-Devportal-Signature header")
	}

	parts := make(map[string]string)
	for _, segment := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(segment), "=", 2)
		if len(kv) == 2 {
			parts[kv[0]] = kv[1]
		}
	}

	tsStr, ok := parts["t"]
	if !ok || tsStr == "" {
		return 0, "", fmt.Errorf("missing t field in signature header")
	}
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid t field: %v", err)
	}

	v1, ok := parts["v1"]
	if !ok || v1 == "" {
		return 0, "", fmt.Errorf("missing v1 field in signature header")
	}

	return ts, v1, nil
}
