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

package bedrock

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"strings"
)

// Credentials a Converse call must carry, one of Bedrock's two schemes:
//
//   - SigV4, signed with the access key pair and verified as Bedrock verifies it, so a scenario
//     proves the caller signed the host, path and body it actually sent.
//   - A Bedrock API key, sent as the only Authorization header, `Bearer <key>`, so a scenario
//     proves the caller replaced any Authorization header of its own instead of adding one.
const (
	ConverseAccessKeyID     = "AKIAITBEDROCKMOCK000"
	ConverseSecretAccessKey = "itBedrockMockSecretKey00000000000000000000"
	ConverseAPIKey          = "itBedrockMockApiKey000000000000000000000"

	maxConverseBodyBytes = 1 << 20
)

// conversePath matches the Converse operation, /model/{modelId}/converse.
var conversePath = regexp.MustCompile(`^/model/([^/]+)/converse$`)

type converseRequest struct {
	Messages []struct {
		Role    string `json:"role"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	} `json:"messages"`
}

// converse answers a Converse call whose credential verifies with a reply naming the model and
// the last user text, and refuses any other with Bedrock's 403.
func (s *Service) converse(w http.ResponseWriter, r *http.Request, modelID string) {
	w.Header().Set("Content-Type", "application/json")
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxConverseBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, `{"message":"request body too large"}`, http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, `{"message":"failed to read request body"}`, http.StatusBadRequest)
		return
	}
	if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		if reason := verifyAPIKey(r); reason != "" {
			slog.Warn("bedrock converse: API key rejected", "reason", reason)
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"Authentication failed: the API key is not valid."}`))
			return
		}
	} else if reason := verifySigV4(r, body); reason != "" {
		slog.Warn("bedrock converse: signature rejected", "reason", reason)
		w.Header().Set("X-Amzn-ErrorType", "SignatureDoesNotMatch")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"The request signature we calculated does not match the signature you provided."}`))
		return
	}

	var req converseRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, `{"message":"invalid JSON"}`, http.StatusBadRequest)
		return
	}
	text := ""
	for _, m := range req.Messages {
		if m.Role == "user" && len(m.Content) > 0 {
			text = m.Content[len(m.Content)-1].Text
		}
	}
	writeJSON(w, map[string]any{
		"output": map[string]any{"message": map[string]any{
			"role":    "assistant",
			"content": []map[string]string{{"text": fmt.Sprintf("Hello from mock Bedrock (%s): %s", modelID, text)}},
		}},
		"stopReason": "end_turn",
		"usage":      map[string]int{"inputTokens": 12, "outputTokens": 6, "totalTokens": 18},
		"metrics":    map[string]int{"latencyMs": 42},
	})
}

// verifyAPIKey checks a Bedrock API key: exactly one Authorization header, carrying the service's
// key. It returns why the request is refused, or "" when it is not.
func verifyAPIKey(r *http.Request) string {
	if n := len(r.Header.Values("Authorization")); n != 1 {
		return fmt.Sprintf("%d Authorization headers, want 1", n)
	}
	if r.Header.Get("Authorization") != "Bearer "+ConverseAPIKey {
		return "unknown API key"
	}
	return ""
}

// verifySigV4 recomputes the request's AWS Signature Version 4 from what was received — the raw
// path, the Host header, the signed headers and the body — and returns why it does not match,
// or "" when it does.
func verifySigV4(r *http.Request, body []byte) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 ") {
		return "missing or non-SigV4 Authorization header"
	}
	fields := map[string]string{}
	for _, part := range strings.Split(strings.TrimPrefix(auth, "AWS4-HMAC-SHA256 "), ",") {
		if k, v, ok := strings.Cut(strings.TrimSpace(part), "="); ok {
			fields[k] = v
		}
	}
	scope := strings.SplitN(fields["Credential"], "/", 2)
	if len(scope) != 2 || scope[0] != ConverseAccessKeyID {
		return "unknown access key"
	}
	scopeParts := strings.Split(scope[1], "/") // date/region/service/aws4_request
	if len(scopeParts) != 4 || scopeParts[2] != "bedrock" {
		return "credential scope is not for the bedrock service"
	}
	payloadHash := sha256.Sum256(body)
	if hex.EncodeToString(payloadHash[:]) != r.Header.Get("X-Amz-Content-Sha256") {
		return "body does not match x-amz-content-sha256"
	}
	want := sigV4Signature(r, fields["SignedHeaders"], scope[1], hex.EncodeToString(payloadHash[:]))
	if want != fields["Signature"] {
		return fmt.Sprintf("signature does not match %s %s (host %s)", r.Method, r.RequestURI, r.Host)
	}
	return ""
}

// sigV4Signature computes the signature of a received request over the given signed headers and
// credential scope (date/region/service/aws4_request).
func sigV4Signature(r *http.Request, signedHeaders, credentialScope, payloadHash string) string {
	signed := strings.Split(signedHeaders, ";")
	sort.Strings(signed)
	var headers strings.Builder
	for _, name := range signed {
		value := r.Header.Get(name)
		if name == "host" {
			value = r.Host
		}
		headers.WriteString(name + ":" + strings.TrimSpace(value) + "\n")
	}
	requestURI := r.RequestURI
	if requestURI == "" {
		requestURI = r.URL.RequestURI()
	}
	path, query, _ := strings.Cut(requestURI, "?")
	canonical := strings.Join([]string{r.Method, escapeSigV4Path(path), query, headers.String(),
		strings.Join(signed, ";"), payloadHash}, "\n")
	canonicalHash := sha256.Sum256([]byte(canonical))
	stringToSign := strings.Join([]string{"AWS4-HMAC-SHA256", r.Header.Get("X-Amz-Date"), credentialScope,
		hex.EncodeToString(canonicalHash[:])}, "\n")

	scopeParts := strings.Split(credentialScope, "/")
	key := hmacSHA256([]byte("AWS4"+ConverseSecretAccessKey), scopeParts[0])
	for _, part := range scopeParts[1:] {
		key = hmacSHA256(key, part)
	}
	return hex.EncodeToString(hmacSHA256(key, stringToSign))
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

// escapeSigV4Path encodes a received path as SigV4's canonical URI does for every service but S3:
// each byte outside RFC 3986's unreserved set, except '/', is percent-encoded.
func escapeSigV4Path(p string) string {
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c == '/' || c == '-' || c == '_' || c == '.' || c == '~' ||
			(c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
