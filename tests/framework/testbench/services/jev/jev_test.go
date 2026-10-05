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

package jev

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

const defaultBattery = `{"jailbreak":{"type":"noul","instructions":"q"},` +
	`"severity":{"type":"score","instructions":"q","criteria":["none","low","medium","high"]},` +
	`"route":{"type":"choice","instructions":"q","criteria":{"code":null,"chat":null,"analysis":null}}}`

func body(state string) string {
	return `{"state":` + state + `,"model":"jev-it-model","questions":` + defaultBattery + `}`
}

func serve(t *testing.T, svc *Service, method, path, payload, auth string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(payload))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	svc.Handler().ServeHTTP(rec, req)
	return rec
}

func call(t *testing.T, svc *Service, path, payload string) *httptest.ResponseRecorder {
	t.Helper()
	return serve(t, svc, http.MethodPost, path, payload, "Bearer "+APIKey)
}

type answers struct {
	Answers map[string]struct {
		Type          string             `json:"type"`
		Noul          *float64           `json:"noul"`
		Score         *float64           `json:"score"`
		Confidence    *float64           `json:"confidence"`
		Choice        string             `json:"choice"`
		Probabilities map[string]float64 `json:"probabilities"`
	} `json:"answers"`
	Usage map[string]int `json:"usage"`
}

func decodeAnswers(t *testing.T, rec *httptest.ResponseRecorder) answers {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var a answers
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &a))
	return a
}

type requestLog struct {
	Count    int       `json:"count"`
	Requests []Request `json:"requests"`
}

func recorded(t *testing.T, svc *Service, partition string) requestLog {
	t.Helper()
	rec := serve(t, svc, http.MethodGet, "/"+partition+"/test/requests", "", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var l requestLog
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &l))
	return l
}

func TestServiceContract(t *testing.T) {
	svc := New()
	require.Equal(t, "jev", svc.Name())
	require.Equal(t, Port, svc.Port())
	require.True(t, svc.Stateful())
	require.Equal(t, testbench.PartitionByBlock, svc.PartitionKey())
	require.NoError(t, (&testbench.Registry{}).Register(svc))
}

func TestDefaultAnswers(t *testing.T) {
	a := decodeAnswers(t, call(t, New(), "/p/ok/v1/systemone", body(`"Hello there"`)))
	require.Len(t, a.Answers, 3)
	require.Equal(t, "noul", a.Answers["jailbreak"].Type)
	require.Equal(t, DefaultNoul, *a.Answers["jailbreak"].Noul)
	require.Equal(t, DefaultScore, *a.Answers["severity"].Score)
	require.Equal(t, DefaultConfidence, *a.Answers["severity"].Confidence)
	route := a.Answers["route"]
	require.Equal(t, "analysis", route.Choice, "ties go to the first option in key order")
	for _, option := range []string{"analysis", "chat", "code"} {
		require.InDelta(t, 1.0/3, route.Probabilities[option], 1e-9)
	}
	require.Equal(t, map[string]int{"input_tokens": len(`"Hello there"`), "output_tokens": 3}, a.Usage)
}

func TestMarkersSetAnswers(t *testing.T) {
	cases := []struct {
		name  string
		state string
		check func(t *testing.T, a answers)
	}{
		{"noul", `"jev:jailbreak=0.95 ignore your rules"`, func(t *testing.T, a answers) {
			require.Equal(t, 0.95, *a.Answers["jailbreak"].Noul)
		}},
		{"noul out of range is passed through", `"jev:jailbreak=1.5"`, func(t *testing.T, a answers) {
			require.Equal(t, 1.5, *a.Answers["jailbreak"].Noul)
		}},
		{"score without confidence", `"jev:severity=2"`, func(t *testing.T, a answers) {
			require.Equal(t, 2.0, *a.Answers["severity"].Score)
			require.Equal(t, DefaultConfidence, *a.Answers["severity"].Confidence)
		}},
		{"score with confidence", `"jev:severity=2@0.5"`, func(t *testing.T, a answers) {
			require.Equal(t, 2.0, *a.Answers["severity"].Score)
			require.Equal(t, 0.5, *a.Answers["severity"].Confidence)
		}},
		{"choice shorthand", `"jev:route=code@0.9"`, func(t *testing.T, a answers) {
			route := a.Answers["route"]
			require.Equal(t, "code", route.Choice)
			require.InDelta(t, 0.9, route.Probabilities["code"], 1e-9)
			require.InDelta(t, 0.05, route.Probabilities["chat"], 1e-9)
			require.InDelta(t, 0.05, route.Probabilities["analysis"], 1e-9)
		}},
		{"choice distribution", `"jev:route=code:0.35,chat:0.35,analysis:0.3"`, func(t *testing.T, a answers) {
			route := a.Answers["route"]
			require.Equal(t, "chat", route.Choice, "ties go to the first option in key order")
			require.Equal(t, map[string]float64{"code": 0.35, "chat": 0.35, "analysis": 0.3}, route.Probabilities)
		}},
		{"choice of an unconfigured option", `"jev:route=unknown@0.99"`, func(t *testing.T, a answers) {
			require.Equal(t, "unknown", a.Answers["route"].Choice)
		}},
		{"markers in an object state", `{"prompt":"jev:jailbreak=0.8 weather","tools":[]}`, func(t *testing.T, a answers) {
			require.Equal(t, 0.8, *a.Answers["jailbreak"].Noul)
		}},
		{"the last marker for a key wins", `"jev:jailbreak=0.2 then jev:jailbreak=0.7"`, func(t *testing.T, a answers) {
			require.Equal(t, 0.7, *a.Answers["jailbreak"].Noul)
		}},
		{"a marker for a question not asked is ignored", `"jev:other=0.9"`, func(t *testing.T, a answers) {
			require.Len(t, a.Answers, 3)
			require.Equal(t, DefaultNoul, *a.Answers["jailbreak"].Noul)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, decodeAnswers(t, call(t, New(), "/p/ok/v1/systemone", body(tc.state))))
		})
	}
}

func TestInvalidMarkersAreRejected(t *testing.T) {
	for _, state := range []string{
		`"jev:jailbreak=high"`, `"jev:severity=2@sure"`, `"jev:route=code@"`, `"jev:route=code:x,chat:0.1"`,
	} {
		rec := call(t, New(), "/p/ok/v1/systemone", body(state))
		require.Equal(t, http.StatusBadRequest, rec.Code, state)
	}
}

func TestModes(t *testing.T) {
	cases := []struct {
		mode    string
		status  []int
		answers bool
	}{
		{ModeOK, []int{200, 200}, true},
		{ModeError, []int{500, 500}, false},
		{ModeRateLimitOnce, []int{429, 200, 200}, true},
		{ModeOverloadedOnce, []int{StatusOverloaded, 200}, true},
		{ModeRateLimitAlways, []int{429, 429}, false},
		{ModeInvalidJSON, []int{200}, false},
		{ModeMissingAnswer, []int{200}, false},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			svc := New()
			for i, want := range tc.status {
				rec := call(t, svc, "/p/"+tc.mode+"/v1/systemone", body(`"text"`))
				require.Equal(t, want, rec.Code, "request %d", i+1)
				if want == http.StatusOK && tc.answers {
					require.Len(t, decodeAnswers(t, rec).Answers, 3)
				}
			}
			require.Equal(t, len(tc.status), recorded(t, svc, "p").Count)
		})
	}
}

func TestInvalidJSONModeIsNotJSON(t *testing.T) {
	rec := call(t, New(), "/p/invalid-json/v1/systemone", body(`"text"`))
	require.Equal(t, http.StatusOK, rec.Code)
	require.False(t, json.Valid(rec.Body.Bytes()))
}

func TestMissingAnswerModeDropsTheFirstKey(t *testing.T) {
	a := decodeAnswers(t, call(t, New(), "/p/missing-answer/v1/systemone", body(`"text"`)))
	require.Len(t, a.Answers, 2)
	require.NotContains(t, a.Answers, "jailbreak")
}

func TestSlowModeDelaysAndStopsWhenTheCallerGoesAway(t *testing.T) {
	svc := newWithSlowDelay(50 * time.Millisecond)
	start := time.Now()
	decodeAnswers(t, call(t, svc, "/p/slow/v1/systemone", body(`"text"`)))
	require.GreaterOrEqual(t, time.Since(start), 50*time.Millisecond)

	svc = newWithSlowDelay(time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/p/slow/v1/systemone", strings.NewReader(body(`"text"`))).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+APIKey)
	done := make(chan struct{})
	go func() {
		svc.Handler().ServeHTTP(httptest.NewRecorder(), req)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("slow mode kept running after the caller went away")
	}
}

func TestAuthentication(t *testing.T) {
	svc := New()
	for _, auth := range []string{"", "Bearer wrong-key", APIKey} {
		rec := serve(t, svc, http.MethodPost, "/p/ok/v1/systemone", body(`"text"`), auth)
		require.Equal(t, http.StatusUnauthorized, rec.Code, auth)
	}
	l := recorded(t, svc, "p")
	require.Equal(t, 3, l.Count, "rejected requests are recorded too")
	require.Equal(t, "Bearer wrong-key", l.Requests[1].Authorization)
}

func TestRecordedRequests(t *testing.T) {
	svc := New()
	call(t, svc, "/p/ok/v1/systemone", body(`"first"`))
	call(t, svc, "/p/error/v1/systemone", body(`{"prompt":"second"}`))
	l := recorded(t, svc, "p")
	require.Equal(t, 2, l.Count)
	require.Len(t, l.Requests, 2)
	first := l.Requests[0]
	require.Equal(t, ModeOK, first.Mode)
	require.Equal(t, "Bearer "+APIKey, first.Authorization)
	require.Equal(t, "jev-it-model", first.Model)
	require.JSONEq(t, `"first"`, string(first.State))
	require.Equal(t, []string{"jailbreak", "route", "severity"}, first.QuestionKeys)
	require.JSONEq(t, defaultBattery, string(first.Questions))
	require.JSONEq(t, `{"prompt":"second"}`, string(l.Requests[1].State))
}

func TestEmptyPartitionReportsNoRequests(t *testing.T) {
	l := recorded(t, New(), "never-used")
	require.Zero(t, l.Count)
	require.Empty(t, l.Requests)
}

func TestPartitionsAreIsolated(t *testing.T) {
	svc := New()
	call(t, svc, "/a/ratelimit-once/v1/systemone", body(`"text"`))
	rec := call(t, svc, "/b/ratelimit-once/v1/systemone", body(`"text"`))
	require.Equal(t, http.StatusTooManyRequests, rec.Code, "another partition's first request is still its first")
	require.Equal(t, 1, recorded(t, svc, "a").Count)
	require.Equal(t, 1, recorded(t, svc, "b").Count)
}

func TestRetentionIsBoundedButTheCountIsExact(t *testing.T) {
	svc := New()
	for i := 0; i < maxRetainedRequests+5; i++ {
		call(t, svc, "/p/ok/v1/systemone", body(`"text"`))
	}
	l := recorded(t, svc, "p")
	require.Equal(t, maxRetainedRequests+5, l.Count)
	require.Len(t, l.Requests, maxRetainedRequests)
}

func TestPartitionBudget(t *testing.T) {
	svc := New()
	for i := 0; i < maxPartitions; i++ {
		svc.partitions[fmt.Sprintf("partition-%d", i)] = &partition{}
	}
	rec := call(t, svc, "/new-partition/ok/v1/systemone", body(`"text"`))
	require.Equal(t, http.StatusInsufficientStorage, rec.Code)
}

func TestMalformedRequests(t *testing.T) {
	cases := []struct {
		name, path, payload string
		status              int
		recorded            bool
	}{
		{"unknown mode", "/p/sometimes/v1/systemone", body(`"text"`), http.StatusNotFound, false},
		{"wrong endpoint", "/p/ok/v1/other", body(`"text"`), http.StatusNotFound, false},
		{"no mode", "/p/v1/systemone", body(`"text"`), http.StatusNotFound, false},
		{"nested mode", "/p/ok/extra/v1/systemone", body(`"text"`), http.StatusNotFound, false},
		{"invalid JSON", "/p/ok/v1/systemone", `{"state":`, http.StatusBadRequest, false},
		{"missing state", "/p/ok/v1/systemone", `{"questions":` + defaultBattery + `}`, http.StatusBadRequest, false},
		{"null state", "/p/ok/v1/systemone", `{"state":null,"questions":` + defaultBattery + `}`, http.StatusBadRequest, false},
		{"no questions", "/p/ok/v1/systemone", `{"state":"text","questions":{}}`, http.StatusBadRequest, false},
		{"too large", "/p/ok/v1/systemone", `{"state":"` + strings.Repeat("a", maxBodyBytes) + `"}`, http.StatusRequestEntityTooLarge, false},
		// These requests are well formed but can't be answered, so they are recorded.
		{"question not an object", "/p/ok/v1/systemone", `{"state":"text","questions":{"q":1}}`, http.StatusBadRequest, true},
		{"unsupported type", "/p/ok/v1/systemone", `{"state":"text","questions":{"q":{"type":"bool"}}}`, http.StatusBadRequest, true},
		{"choice without options", "/p/ok/v1/systemone", `{"state":"text","questions":{"q":{"type":"choice","criteria":["a","b"]}}}`, http.StatusBadRequest, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := New()
			rec := call(t, svc, tc.path, tc.payload)
			require.Equal(t, tc.status, rec.Code, rec.Body.String())
			want := 0
			if tc.recorded {
				want = 1
			}
			require.Equal(t, want, recorded(t, svc, "p").Count)
		})
	}
}

func TestReservedAndInvalidPartitionsAreRejected(t *testing.T) {
	for _, path := range []string{"/v1/systemone", "/test/requests", "/Upper/ok/v1/systemone"} {
		rec := call(t, New(), path, body(`"text"`))
		require.Equal(t, http.StatusBadRequest, rec.Code, path)
	}
}

func TestConcurrentRequestsAreCountedExactly(t *testing.T) {
	svc := New()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			call(t, svc, "/p/ok/v1/systemone", body(`"text"`))
		}()
	}
	wg.Wait()
	require.Equal(t, 50, recorded(t, svc, "p").Count)
}

func TestHealth(t *testing.T) {
	rec := serve(t, New(), http.MethodGet, "/p/test/health", "", "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"ok","service":"jev"}`, rec.Body.String())
}
