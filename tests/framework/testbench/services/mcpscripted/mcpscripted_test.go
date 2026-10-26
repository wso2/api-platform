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

package mcpscripted

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

func serve(t *testing.T, svc *Service, method, path, payload string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	svc.Handler().ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(payload)))
	return rec
}

// callTool sends tools/call for the tool with the given arguments to partition p.
func callTool(t *testing.T, svc *Service, p, tool, arguments string) *httptest.ResponseRecorder {
	t.Helper()
	return serve(t, svc, http.MethodPost, "/"+p+Path,
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"`+tool+`","arguments":`+arguments+`}}`)
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) response {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	var r response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &r))
	require.Equal(t, "2.0", r.JSONRPC)
	return r
}

type requestLog struct {
	Count    int       `json:"count"`
	Requests []Request `json:"requests"`
}

func recorded(t *testing.T, svc *Service, p string) requestLog {
	t.Helper()
	rec := serve(t, svc, http.MethodGet, "/"+p+"/test/requests", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var l requestLog
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &l))
	return l
}

func TestServiceContract(t *testing.T) {
	svc := New()
	require.Equal(t, "mcp-scripted", svc.Name())
	require.Equal(t, Port, svc.Port())
	require.True(t, svc.Stateful())
	require.Equal(t, testbench.PartitionByBlock, svc.PartitionKey())
	require.NoError(t, (&testbench.Registry{}).Register(svc))
}

func TestInitializeReturnsASessionAndTheRequestedVersion(t *testing.T) {
	rec := serve(t, New(), http.MethodPost, "/p/mcp",
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`)
	r := decode(t, rec)
	require.Equal(t, SessionID, rec.Header().Get("Mcp-Session-Id"))
	require.JSONEq(t, `1`, string(r.ID))
	require.JSONEq(t, `{"protocolVersion":"2025-03-26","capabilities":{"tools":{}},"serverInfo":{"name":"mcp-scripted","version":"1.0.0"}}`, string(r.Result))

	r = decode(t, serve(t, New(), http.MethodPost, "/p/mcp", `{"jsonrpc":"2.0","id":2,"method":"initialize","params":{}}`))
	require.Contains(t, string(r.Result), `"protocolVersion":"2025-06-18"`)
}

func TestNotificationsAreAccepted(t *testing.T) {
	rec := serve(t, New(), http.MethodPost, "/p/mcp", `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Empty(t, rec.Body.String())
}

func TestPingAndToolsList(t *testing.T) {
	svc := New()
	require.JSONEq(t, `{}`, string(decode(t, serve(t, svc, http.MethodPost, "/p/mcp", `{"jsonrpc":"2.0","id":"a","method":"ping"}`)).Result))
	r := decode(t, serve(t, svc, http.MethodPost, "/p/mcp", `{"jsonrpc":"2.0","id":3,"method":"tools/list"}`))
	var list struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(r.Result, &list))
	require.Len(t, list.Tools, 1)
	require.Equal(t, "respond", list.Tools[0].Name)
}

func TestUnknownMethodIsAJSONRPCError(t *testing.T) {
	r := decode(t, serve(t, New(), http.MethodPost, "/p/mcp", `{"jsonrpc":"2.0","id":4,"method":"resources/list"}`))
	require.NotNil(t, r.Error)
	require.Equal(t, -32601, r.Error.Code)
}

func TestShapes(t *testing.T) {
	cases := map[string]string{
		ShapeText:       `{"content":[{"type":"text","text":"hello"}]}`,
		ShapeStructured: `{"content":[],"structuredContent":{"text":"hello"}}`,
		ShapeResource:   `{"content":[{"type":"resource","resource":{"uri":"test://result","mimeType":"text/plain","text":"hello"}}]}`,
		ShapeImage:      `{"content":[{"type":"image","data":"` + onePixelPNG + `","mimeType":"image/png"}]}`,
	}
	for shape, want := range cases {
		t.Run(shape, func(t *testing.T) {
			r := decode(t, callTool(t, New(), "p", "any-tool", `{"text":"hello","shape":"`+shape+`"}`))
			require.JSONEq(t, `7`, string(r.ID))
			require.Nil(t, r.Error)
			require.JSONEq(t, want, string(r.Result))
		})
	}
}

func TestTextDefaultsAndRepeats(t *testing.T) {
	svc := New()
	for name, args := range map[string]string{"no arguments": `null`, "empty arguments": `{}`} {
		r := decode(t, callTool(t, svc, "p", "t", args))
		require.JSONEq(t, `{"content":[{"type":"text","text":"ok"}]}`, string(r.Result), name)
	}
	r := decode(t, callTool(t, svc, "p", "t", `{"text":"ab","repeat":3}`))
	require.JSONEq(t, `{"content":[{"type":"text","text":"ababab"}]}`, string(r.Result))
	r = decode(t, callTool(t, svc, "p", "t", `{"text":""}`))
	require.JSONEq(t, `{"content":[{"type":"text","text":""}]}`, string(r.Result), "an empty text is kept, not replaced by the default")
}

func TestRepeatedTextFillsTheBudgetExactly(t *testing.T) {
	r := decode(t, callTool(t, New(), "p", "t", fmt.Sprintf(`{"text":"ab","repeat":%d}`, MaxResultBytes/2)))
	require.Nil(t, r.Error)
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	require.NoError(t, json.Unmarshal(r.Result, &result))
	require.Len(t, result.Content[0].Text, MaxResultBytes)
}

func TestRPCErrorShape(t *testing.T) {
	r := decode(t, callTool(t, New(), "p", "t", `{"text":"tool failed","shape":"rpc-error"}`))
	require.Nil(t, r.Result)
	require.NotNil(t, r.Error)
	require.Equal(t, -32000, r.Error.Code)
	require.Equal(t, "tool failed", r.Error.Message)
}

func TestPlainTextShapes(t *testing.T) {
	rec := callTool(t, New(), "p", "t", `{"text":"upstream broke","shape":"http-error"}`)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
	require.Equal(t, "upstream broke", rec.Body.String())

	rec = callTool(t, New(), "p", "t", `{"text":"not a message","shape":"not-jsonrpc"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
	require.Equal(t, "not a message", rec.Body.String())
}

// eventData returns the data of each server-sent event in the body, checking each event's form.
func eventData(t *testing.T, rec *httptest.ResponseRecorder) []string {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	var data []string
	for _, event := range strings.Split(strings.TrimSuffix(rec.Body.String(), "\n\n"), "\n\n") {
		lines := strings.Split(event, "\n")
		require.Len(t, lines, 2, event)
		require.Equal(t, "event: message", lines[0])
		d, ok := strings.CutPrefix(lines[1], "data: ")
		require.True(t, ok, event)
		data = append(data, d)
	}
	return data
}

func TestEventsShapeSendsProgressForTheRequestsToken(t *testing.T) {
	for name, token := range map[string]string{"a string token": `"tok-1"`, "a number token": `9`} {
		rec := serve(t, New(), http.MethodPost, "/p"+Path,
			`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"t","arguments":{"text":"done","shape":"events"},"_meta":{"progressToken":`+token+`}}}`)
		data := eventData(t, rec)
		require.Len(t, data, 3, name)
		for i := range 2 {
			require.JSONEq(t, fmt.Sprintf(`{"jsonrpc":"2.0","method":"notifications/progress","params":{"progressToken":%s,"progress":%d,"total":2}}`, token, i+1), data[i], name)
		}
		require.JSONEq(t, `{"jsonrpc":"2.0","id":7,"result":{"content":[{"type":"text","text":"done"}]}}`, data[2], name)
	}
}

func TestEventsShapeWithoutAProgressTokenSendsOnlyTheResult(t *testing.T) {
	data := eventData(t, callTool(t, New(), "p", "t", `{"text":"done","shape":"events"}`))
	require.Len(t, data, 1)
	require.JSONEq(t, `{"jsonrpc":"2.0","id":7,"result":{"content":[{"type":"text","text":"done"}]}}`, data[0])
}

func TestInvalidRequests(t *testing.T) {
	svc := New()
	rec := serve(t, svc, http.MethodPost, "/p/mcp", `{not json`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var r response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &r))
	require.Equal(t, -32700, r.Error.Code)
	require.JSONEq(t, `null`, string(r.ID))

	for name, args := range map[string]string{
		"arguments not an object": `"text"`,
		"unknown shape":           `{"shape":"video"}`,
		"negative repeat":         `{"repeat":-1}`,
		"repeat over the budget":  fmt.Sprintf(`{"text":"ab","repeat":%d}`, MaxResultBytes/2+1),
		"a repeat that overflows": `{"text":"ab","repeat":9223372036854775807}`,
		"text over the budget":    fmt.Sprintf(`{"text":"%s"}`, strings.Repeat("a", MaxResultBytes+1)),
		"text not a string":       `{"text":5}`,
	} {
		r := decode(t, callTool(t, svc, "p", "t", args))
		require.NotNil(t, r.Error, name)
		require.Equal(t, -32602, r.Error.Code, name)
	}
}

func TestRequestsAreRecordedPerPartition(t *testing.T) {
	svc := New()
	serve(t, svc, http.MethodPost, "/a/mcp", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	callTool(t, svc, "a", "lookup", `{"text":"hi"}`)
	callTool(t, svc, "b", "other", `{}`)

	a := recorded(t, svc, "a")
	require.Equal(t, 2, a.Count)
	require.Equal(t, "initialize", a.Requests[0].Method)
	require.Empty(t, a.Requests[0].Tool)
	require.Equal(t, "tools/call", a.Requests[1].Method)
	require.Equal(t, "lookup", a.Requests[1].Tool)
	require.JSONEq(t, `{"text":"hi"}`, string(a.Requests[1].Arguments))

	require.Equal(t, 1, recorded(t, svc, "b").Count)
	empty := recorded(t, svc, "never-used")
	require.Zero(t, empty.Count)
	require.Empty(t, empty.Requests)
}

func TestRetentionIsBoundedButTheCountIsExact(t *testing.T) {
	svc := New()
	for range maxRetainedRequests + 5 {
		callTool(t, svc, "p", "t", `{}`)
	}
	l := recorded(t, svc, "p")
	require.Equal(t, maxRetainedRequests+5, l.Count)
	require.Len(t, l.Requests, maxRetainedRequests)
}

func TestPartitionBudget(t *testing.T) {
	svc := New()
	for i := range maxPartitions {
		svc.partitions[fmt.Sprintf("p%d", i)] = &partition{}
	}
	rec := callTool(t, svc, "one-too-many", "t", `{}`)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Equal(t, http.StatusOK, callTool(t, svc, "p0", "t", `{}`).Code, "an existing partition still works")
}

func TestPathsNeedAPartitionAndTheRightMethod(t *testing.T) {
	svc := New()
	require.Equal(t, http.StatusNotFound, serve(t, svc, http.MethodPost, "/mcp", `{}`).Code, "/mcp alone is a partition named mcp with no route left")
	require.Zero(t, recorded(t, svc, "mcp").Count)
	require.Equal(t, http.StatusMethodNotAllowed, serve(t, svc, http.MethodGet, "/p/mcp", "").Code)
	rec := serve(t, svc, http.MethodGet, "/p/test/health", "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"ok","service":"mcp-scripted"}`, rec.Body.String())
}

func TestConcurrentRequestsAreCountedExactly(t *testing.T) {
	svc := New()
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() { callTool(t, svc, "p", "t", `{}`) })
	}
	wg.Wait()
	require.Equal(t, 50, recorded(t, svc, "p").Count)
}
