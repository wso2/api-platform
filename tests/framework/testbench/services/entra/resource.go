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

package entra

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Audiences of the two protected resources.
const (
	// FoundryAudience is the audience /foundry accepts, as an Azure AI Foundry project does.
	FoundryAudience = "https://ai.azure.com"
	// CognitiveAudience is the audience /cognitive accepts, as Azure OpenAI does.
	CognitiveAudience = "https://cognitiveservices.azure.com"
)

// Response headers through which a protected resource reports the token it received.
const (
	// HeaderToken carries the bearer token received. It is absent when the request had none.
	HeaderToken = "X-Testbench-Entra-Token"
	// HeaderTenant carries the received token's tenant, when the token is one the service issued.
	HeaderTenant = "X-Testbench-Entra-Tid"
	// HeaderAudience carries the received token's audience, when the token is one the service issued.
	HeaderAudience = "X-Testbench-Entra-Aud"
	// HeaderAppID carries the received token's appid (the client ID, or system-assigned), when the
	// token is one the service issued.
	HeaderAppID = "X-Testbench-Entra-Appid"
)

// resource serves a resource protected by tokens for audience: any method, any path below it.
func resource(audience string) func(string, http.ResponseWriter, *http.Request) {
	return func(_ string, w http.ResponseWriter, r *http.Request) {
		token, hasBearer := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		var claims Claims
		valid := false
		if hasBearer {
			w.Header().Set(HeaderToken, token)
			if claims, valid = parseToken(token); valid {
				w.Header().Set(HeaderTenant, claims.Tid)
				w.Header().Set(HeaderAudience, claims.Aud)
				w.Header().Set(HeaderAppID, claims.AppID)
			}
		}
		if !valid {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{
				"code": "401", "message": "missing or invalid bearer token (testbench)",
			}})
			return
		}
		if claims.Aud != audience {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{
				"code":    "401",
				"message": fmt.Sprintf("token audience %q is not valid for this resource (testbench)", claims.Aud),
			}})
			return
		}

		headers := map[string]string{}
		for name, values := range r.Header {
			headers[name] = strings.Join(values, ", ")
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":      "chatcmpl-testbench",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "testbench-entra",
			"choices": []any{map[string]any{
				"index":         0,
				"message":       map[string]string{"role": "assistant", "content": "hello from an Entra-protected testbench resource"},
				"finish_reason": "stop",
			}},
			"usage":   map[string]int{"prompt_tokens": 5, "completion_tokens": 5, "total_tokens": 10},
			"headers": headers,
			"token":   claims,
		})
	}
}
