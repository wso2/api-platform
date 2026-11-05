/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package platformgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
)

// llmErrorsUseOpenAIFormat reports whether a Gateway release returns gateway-produced errors on
// LLM APIs in the OpenAI error envelope. Releases up to and including 1.2.0 return each policy's
// own body; source builds and later releases return the envelope.
func llmErrorsUseOpenAIFormat(version string) bool {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	version, _, _ = strings.Cut(version, "-")
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return true
	}
	var n [3]int
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil || v < 0 {
			return true
		}
		n[i] = v
	}
	return n[0] > 1 || (n[0] == 1 && (n[1] > 2 || (n[1] == 2 && n[2] > 0)))
}

// rateLimitRejectionError returns nil when body is the rate-limit rejection the release
// returns on an LLM API, and an error describing the mismatch otherwise.
func rateLimitRejectionError(version string, body []byte) error {
	if !llmErrorsUseOpenAIFormat(version) {
		if !bytes.Contains(body, []byte("Rate limit exceeded")) {
			return fmt.Errorf("expected the rate-limit policy's own body mentioning %q", "Rate limit exceeded")
		}
		return nil
	}
	var envelope struct {
		Error *struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Error == nil {
		return fmt.Errorf("expected the OpenAI error envelope")
	}
	if envelope.Error.Type != "rate_limit_error" {
		return fmt.Errorf("expected error.type %q, got %q", "rate_limit_error", envelope.Error.Type)
	}
	return nil
}

// responseIsRateLimitRejection asserts that the published response is a 429 rate-limit
// rejection in the representation the Gateway release under test returns on an LLM API.
func (s *Steps) responseIsRateLimitRejection(ctx context.Context) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		return fmt.Errorf("expected status %d: %s", http.StatusTooManyRequests, resp.Describe())
	}
	if err := rateLimitRejectionError(gatewayVersion(s.topo), resp.Body); err != nil {
		return fmt.Errorf("%w: %s", err, resp.Describe())
	}
	return nil
}
