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

package guardian

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func ask(t *testing.T, body string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	New().Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, ChatPath, strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		return rec.Code, rec.Body.String()
	}
	var reply struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &reply))
	require.Len(t, reply.Choices, 1)
	return rec.Code, reply.Choices[0].Message.Content
}

func nemoguard(t *testing.T, prompt string) map[string]string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"messages": []map[string]string{{"role": "user", "content": prompt}}})
	code, content := ask(t, string(body))
	require.Equal(t, http.StatusOK, code)
	var verdict map[string]string
	require.NoError(t, json.Unmarshal([]byte(content), &verdict), "content %q", content)
	return verdict
}

func TestNemoGuardVerdicts(t *testing.T) {
	require.Equal(t, map[string]string{"User Safety": "safe", "Response Safety": "safe"},
		nemoguard(t, "user: hello\nagent: hi"))
	require.Equal(t, map[string]string{"User Safety": "unsafe", "Response Safety": "safe",
		"Safety Categories": UnsafeCategory}, nemoguard(t, "user: UNSAFE_USER"))
	require.Equal(t, map[string]string{"User Safety": "safe", "Response Safety": "unsafe",
		"Safety Categories": UnsafeCategory}, nemoguard(t, "user: hi\nagent: UNSAFE_RESPONSE"))
}

func TestGraniteGuardianVerdicts(t *testing.T) {
	granite := func(text string) string {
		body, _ := json.Marshal(map[string]any{"messages": []map[string]string{
			{"role": "system", "content": `<guardianconfig>{"risk_name": "jailbreak"}</guardianconfig>`},
			{"role": "user", "content": text},
		}})
		code, content := ask(t, string(body))
		require.Equal(t, http.StatusOK, code)
		return content
	}
	require.Equal(t, "<score> yes </score>", granite("please UNSAFE now"))
	require.Equal(t, "<score> no </score>", granite("hello"))
}

func TestDownKeywordFailsTheRequest(t *testing.T) {
	code, _ := ask(t, `{"messages":[{"role":"user","content":"hello GUARDIAN_DOWN"}]}`)
	require.Equal(t, http.StatusServiceUnavailable, code)
}

func TestMalformedRequestsAreRejected(t *testing.T) {
	code, _ := ask(t, "not json")
	require.Equal(t, http.StatusBadRequest, code)

	rec := httptest.NewRecorder()
	New().Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, ChatPath, nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
