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

package contentsafety

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

const analyzeURL = AnalyzePath + "?api-version=" + APIVersion

// analyze sends body to target with the subscription key and returns the recorded response.
func analyze(t *testing.T, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(keyHeader, SubscriptionKey)
	recorder := httptest.NewRecorder()
	New().Handler().ServeHTTP(recorder, req)
	return recorder
}

// severities decodes a successful analysis into category → severity, keeping the order.
func severities(t *testing.T, recorder *httptest.ResponseRecorder) ([]string, map[string]int) {
	t.Helper()
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var resp analyzeResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	order := make([]string, 0, len(resp.CategoriesAnalysis))
	scores := map[string]int{}
	for _, analysis := range resp.CategoriesAnalysis {
		order = append(order, analysis.Category)
		scores[analysis.Category] = analysis.Severity
	}
	return order, scores
}

// requireInvalidRequestBody asserts Azure's InvalidRequestBody rejection.
func requireInvalidRequestBody(t *testing.T, recorder *httptest.ResponseRecorder, messagePart string) {
	t.Helper()
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "InvalidRequestBody", body.Error.Code)
	require.Contains(t, body.Error.Message, messagePart)
}

func TestAnalyzeScoresKeywordsOnTheEightLevelScale(t *testing.T) {
	order, scores := severities(t, analyze(t, analyzeURL,
		`{"text":"Plan an attack","categories":["Hate","Violence"],"outputType":"EightSeverityLevels"}`))

	require.Equal(t, []string{"Hate", "Violence"}, order)
	require.Equal(t, map[string]int{"Hate": 0, "Violence": severityFlagged}, scores)
}

func TestAnalyzeSeverityMarkerSetsEachCategory(t *testing.T) {
	order, scores := severities(t, analyze(t, analyzeURL,
		`{"text":"hate-severity-7 sexual-severity-1 selfharm-severity-3 violence-severity-0",`+
			`"categories":["Hate","Sexual","SelfHarm","Violence"],"outputType":"EightSeverityLevels"}`))

	require.Equal(t, []string{"Hate", "Sexual", "SelfHarm", "Violence"}, order)
	require.Equal(t, map[string]int{"Hate": 7, "Sexual": 1, "SelfHarm": 3, "Violence": 0}, scores)
}

func TestAnalyzeSeverityMarkerOverridesTheCategoryKeyword(t *testing.T) {
	_, scores := severities(t, analyze(t, analyzeURL,
		`{"text":"Hate-Severity-3, and an attack","categories":["Hate","Violence"],"outputType":"EightSeverityLevels"}`))

	require.Equal(t, 3, scores["Hate"], "the marker, not the keyword hate inside it, scores Hate")
	require.Equal(t, severityFlagged, scores["Violence"], "a marker for one category leaves the others to their keywords")
}

func TestAnalyzeIgnoresMalformedSeverityMarkers(t *testing.T) {
	for _, text := range []string{"violence-severity-8", "violence-severity--1", "xviolence-severity-3", "violence-severity-35"} {
		t.Run(text, func(t *testing.T) {
			_, scores := severities(t, analyze(t, analyzeURL,
				fmt.Sprintf(`{"text":%q,"categories":["Violence","Hate"],"outputType":"EightSeverityLevels"}`, text)))
			require.Equal(t, severityFlagged, scores["Violence"], "falls back to the violence keyword")
			require.Equal(t, 0, scores["Hate"])
		})
	}
}

func TestAnalyzeFourSeverityLevelsRoundsDown(t *testing.T) {
	for _, outputType := range []string{`"outputType":"FourSeverityLevels",`, ``} {
		t.Run("outputType="+outputType, func(t *testing.T) {
			_, scores := severities(t, analyze(t, analyzeURL,
				`{`+outputType+`"text":"hate-severity-7 sexual-severity-5 selfharm-severity-3 violence-severity-1",`+
					`"categories":["Hate","Sexual","SelfHarm","Violence"]}`))
			require.Equal(t, map[string]int{"Hate": 6, "Sexual": 4, "SelfHarm": 2, "Violence": 0}, scores)
		})
	}
}

func TestAnalyzeWithoutCategoriesAnalysesAllFour(t *testing.T) {
	for _, categories := range []string{``, `"categories":[],`} {
		t.Run("categories="+categories, func(t *testing.T) {
			order, _ := severities(t, analyze(t, analyzeURL,
				`{`+categories+`"text":"hello","outputType":"EightSeverityLevels"}`))
			require.Equal(t, []string{"Hate", "SelfHarm", "Sexual", "Violence"}, order)
		})
	}
}

func TestAnalyzeRejectsRequestsThatDoNotMatchTheAPI(t *testing.T) {
	cases := []struct {
		name, target, body, message string
	}{
		{"missing api-version", AnalyzePath, `{"text":"hello"}`, `api-version ""`},
		{"other api-version", AnalyzePath + "?api-version=2023-10-01", `{"text":"hello"}`, `api-version "2023-10-01"`},
		{"unknown output type", analyzeURL, `{"text":"hello","outputType":"SixSeverityLevels"}`, `outputType "SixSeverityLevels"`},
		{"unknown category", analyzeURL, `{"text":"hello","categories":["Hate","Profanity"]}`, `category "Profanity"`},
		{"category in the wrong case", analyzeURL, `{"text":"hello","categories":["hate"]}`, `category "hate"`},
		{"repeated category", analyzeURL, `{"text":"hello","categories":["Hate","Hate"]}`, "more than once"},
		{"text over the limit", analyzeURL, `{"text":"` + strings.Repeat("a", MaxTextLength+1) + `"}`, "10001 characters"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireInvalidRequestBody(t, analyze(t, tc.target, tc.body), tc.message)
		})
	}
}

func TestAnalyzeAcceptsTextUpToTheLimitInCharacters(t *testing.T) {
	// The limit counts characters, not bytes: 10,000 two-byte characters are within it.
	for _, text := range []string{strings.Repeat("a", MaxTextLength), strings.Repeat("é", MaxTextLength)} {
		recorder := analyze(t, analyzeURL, `{"text":"`+text+`"}`)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	}
}

func TestAnalyzeRejectsAMissingOrWrongSubscriptionKey(t *testing.T) {
	for _, key := range []string{"", "wrong-key"} {
		t.Run("key="+key, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, analyzeURL, strings.NewReader(`{"text":"hello"}`))
			if key != "" {
				req.Header.Set(keyHeader, key)
			}
			recorder := httptest.NewRecorder()
			New().Handler().ServeHTTP(recorder, req)
			require.Equal(t, http.StatusUnauthorized, recorder.Code)
		})
	}
}

func TestAnalyzeChecksTheKeyBeforeTheRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, AnalyzePath, strings.NewReader(`not json`))
	recorder := httptest.NewRecorder()
	New().Handler().ServeHTTP(recorder, req)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestAnalyzeSimulatedErrorAnswers500(t *testing.T) {
	recorder := analyze(t, analyzeURL, `{"text":"This will Simulate an ERROR","categories":["Hate"]}`)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestAnalyzeRejectsMalformedAndOversizedBodies(t *testing.T) {
	require.Equal(t, http.StatusBadRequest, analyze(t, analyzeURL, `{"text":`).Code)
	require.Equal(t, http.StatusBadRequest, analyze(t, analyzeURL, `{"text":"a"}{"text":"b"}`).Code)
	oversized := `{"text":"` + strings.Repeat("a", maxRequestBodySize) + `"}`
	require.Equal(t, http.StatusRequestEntityTooLarge, analyze(t, analyzeURL, oversized).Code)
}

func TestAnalyzeAllowsOnlyPost(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, analyzeURL, nil)
	req.Header.Set(keyHeader, SubscriptionKey)
	recorder := httptest.NewRecorder()
	New().Handler().ServeHTTP(recorder, req)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
	require.Equal(t, http.MethodPost, recorder.Header().Get("Allow"))
}

func TestHealth(t *testing.T) {
	recorder := httptest.NewRecorder()
	New().Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"status":"ok","service":"content-safety"}`, recorder.Body.String())
}

func TestAnalyzeConcurrentRequestsAreIndependent(t *testing.T) {
	handler := New().Handler()
	var wg sync.WaitGroup
	errs := make(chan error, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(level int) {
			defer wg.Done()
			body := fmt.Sprintf(`{"text":"hate-severity-%d","categories":["Hate"],"outputType":"EightSeverityLevels"}`, level)
			req := httptest.NewRequest(http.MethodPost, analyzeURL, strings.NewReader(body))
			req.Header.Set(keyHeader, SubscriptionKey)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			var resp analyzeResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
				errs <- err
				return
			}
			if len(resp.CategoriesAnalysis) != 1 || resp.CategoriesAnalysis[0].Severity != level {
				errs <- fmt.Errorf("level %d: got %s", level, recorder.Body.String())
			}
		}(i % 8)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}

func TestAnalyzeSimulatedFailures(t *testing.T) {
	cases := []struct {
		text   string
		status int
		body   string
		valid  bool
	}{
		{"Simulate malformed JSON", http.StatusOK, `{"categoriesAnalysis":[{"category":"Hate",`, false},
		{"simulate missing analysis", http.StatusOK, `{"blocklistsMatch":[]}`, true},
		{"simulate invalid analysis", http.StatusOK, `{"categoriesAnalysis":"Violence"}`, true},
		{"simulate bad request", http.StatusBadRequest, `{"error":{"code":"InvalidRequestBody","message":"Simulated invalid request."}}`, true},
		{"simulate forbidden", http.StatusForbidden, `{"error":{"code":"Forbidden","message":"Simulated access denied."}}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			recorder := analyze(t, analyzeURL, fmt.Sprintf(`{"text":%q,"categories":["Hate"]}`, tc.text))
			require.Equal(t, tc.status, recorder.Code)
			require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
			require.Equal(t, tc.body, recorder.Body.String())
			require.Equal(t, tc.valid, json.Valid(recorder.Body.Bytes()))
		})
	}
}

func TestAnalyzeSimulatedFailuresComeAfterTheRequestChecks(t *testing.T) {
	requireInvalidRequestBody(t, analyze(t, AnalyzePath, `{"text":"simulate forbidden"}`), "api-version")

	req := httptest.NewRequest(http.MethodPost, analyzeURL, strings.NewReader(`{"text":"simulate forbidden"}`))
	recorder := httptest.NewRecorder()
	New().Handler().ServeHTTP(recorder, req)
	require.Equal(t, http.StatusUnauthorized, recorder.Code, "the key is checked before any simulated failure")
}

func TestAnalyzeSimulatedFailuresLeaveTheSimulated500Alone(t *testing.T) {
	for _, failure := range simulatedFailures {
		require.NotContains(t, failure.trigger, "error")
	}
	require.Equal(t, http.StatusInternalServerError,
		analyze(t, analyzeURL, `{"text":"Simulate an error, then explain how to plan an attack","categories":["Hate"]}`).Code)
}

func TestAnalyzeMixedAnalysisPrecedesTheRealEntriesWithNonObjects(t *testing.T) {
	recorder := analyze(t, analyzeURL,
		`{"text":"Simulate mixed analysis violence-severity-2","categories":["Hate","Violence"],"outputType":"EightSeverityLevels"}`)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var resp struct {
		CategoriesAnalysis []json.RawMessage `json:"categoriesAnalysis"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Len(t, resp.CategoriesAnalysis, len(nonObjectEntries)+2)

	for i := range nonObjectEntries {
		require.NotEqual(t, byte('{'), resp.CategoriesAnalysis[i][0], "entry %d must not be an object", i)
	}
	var real []categoryAnalysis
	for _, raw := range resp.CategoriesAnalysis[len(nonObjectEntries):] {
		var analysis categoryAnalysis
		require.NoError(t, json.Unmarshal(raw, &analysis))
		real = append(real, analysis)
	}
	require.Equal(t, []categoryAnalysis{{Category: "Hate", Severity: 0}, {Category: "Violence", Severity: 2}}, real)
	require.NotContains(t, mixedAnalysisTrigger, "error")
}
