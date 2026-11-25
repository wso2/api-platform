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

package capture

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func serveCapture(service *Service, method, path string, body string, headers http.Header) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reader)
	if headers != nil {
		req.Header = headers
	}
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, req)
	return recorder
}

func readCapturedInfo(t *testing.T, service *Service, block, path string) (requestInfo, int) {
	t.Helper()
	response := serveCapture(service, http.MethodGet, "/"+block+"/test/captured?path="+path, "", nil)
	if response.Code != http.StatusOK {
		return requestInfo{}, response.Code
	}
	var info requestInfo
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &info))
	return info, response.Code
}

func TestReflectRecordsAndEchoesTheRequest(t *testing.T) {
	service := New()
	response := serveCapture(service, http.MethodPost, "/block/echo", `{"email":"masked"}`, nil)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "application/json", response.Header().Get("Content-Type"))

	var echoed requestInfo
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &echoed))
	require.Equal(t, http.MethodPost, echoed.Method)
	require.Equal(t, "/echo", echoed.Path)
	require.Equal(t, `{"email":"masked"}`, echoed.Body)
}

func TestReflectRejectsOversizedRequestBody(t *testing.T) {
	service := New()
	response := serveCapture(service, http.MethodPost, "/block/echo",
		strings.Repeat("x", maxCaptureBodyBytes+1), nil)

	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
	require.Empty(t, service.partitions)
}

func TestCapturedRequestSurvivesIndependentlyOfTheEchoedResponse(t *testing.T) {
	service := New()
	response := serveCapture(service, http.MethodPost, "/block/echo", "masked-value", nil)
	require.Equal(t, http.StatusOK, response.Code)

	info, code := readCapturedInfo(t, service, "block", "/echo")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "masked-value", info.Body)
}

func TestReadCapturedReturnsNoContentWhenNothingWasCaptured(t *testing.T) {
	service := New()
	response := serveCapture(service, http.MethodGet, "/block/test/captured?path=/never-called", "", nil)
	require.Equal(t, http.StatusNoContent, response.Code)
}

func TestReadCapturedRequiresAPathParameter(t *testing.T) {
	service := New()
	response := serveCapture(service, http.MethodGet, "/block/test/captured", "", nil)
	require.Equal(t, http.StatusBadRequest, response.Code)
}

func TestPartitionsAreIsolatedByBlock(t *testing.T) {
	service := New()
	serveCapture(service, http.MethodPost, "/one/echo", "one-value", nil)
	serveCapture(service, http.MethodPost, "/two/echo", "two-value", nil)

	oneInfo, _ := readCapturedInfo(t, service, "one", "/echo")
	twoInfo, _ := readCapturedInfo(t, service, "two", "/echo")
	require.Equal(t, "one-value", oneInfo.Body)
	require.Equal(t, "two-value", twoInfo.Body)
}

func TestLaterRequestsOverwriteEarlierCapturesForTheSamePath(t *testing.T) {
	service := New()
	serveCapture(service, http.MethodPost, "/block/echo", "first", nil)
	serveCapture(service, http.MethodPost, "/block/echo", "second", nil)

	info, _ := readCapturedInfo(t, service, "block", "/echo")
	require.Equal(t, "second", info.Body)
}

func TestCaptureRetentionLimitsNewPathsButAllowsReplacement(t *testing.T) {
	service := NewWithRetentionLimit(2)
	serveCapture(service, http.MethodPost, "/block/first", "first", nil)
	serveCapture(service, http.MethodPost, "/block/second", "second", nil)
	serveCapture(service, http.MethodPost, "/block/third", "third", nil)

	_, code := readCapturedInfo(t, service, "block", "/third")
	require.Equal(t, http.StatusNoContent, code)

	serveCapture(service, http.MethodPost, "/block/first", "updated", nil)
	info, code := readCapturedInfo(t, service, "block", "/first")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "updated", info.Body)
	require.Len(t, service.partitions["block"].captured, 2)
}

func TestResetClearsCapturedRequestsForOneBlock(t *testing.T) {
	service := New()
	serveCapture(service, http.MethodPost, "/block/echo", "value", nil)

	response := serveCapture(service, http.MethodPost, "/block/test/reset", "", nil)
	require.Equal(t, http.StatusOK, response.Code)

	_, code := readCapturedInfo(t, service, "block", "/echo")
	require.Equal(t, http.StatusNoContent, code)
}

func TestServiceRejectsInvalidAndReservedPartitions(t *testing.T) {
	service := New()
	for _, path := range []string{"/test/echo", "/testbench/echo", "/v1/echo", "/Block/echo", "/block_name/echo"} {
		response := serveCapture(service, http.MethodPost, path, "value", nil)
		require.Equal(t, http.StatusBadRequest, response.Code, path)
	}
}

func TestServiceReturnsHealthForAValidPartition(t *testing.T) {
	response := serveCapture(New(), http.MethodGet, "/block/test/health", "", nil)
	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"status":"ok","service":"capture"}`, response.Body.String())
}

func TestServiceSupportsConcurrentCaptureAcrossDistinctPaths(t *testing.T) {
	service := New()
	const total = 64
	var wg sync.WaitGroup
	for i := range total {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			path := "/path-" + strconv.Itoa(i)
			response := serveCapture(service, http.MethodPost, "/block"+path, "value-"+strconv.Itoa(i), nil)
			if response.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", response.Code, http.StatusOK)
			}
		}(i)
	}
	wg.Wait()

	for i := range total {
		path := "/path-" + strconv.Itoa(i)
		info, code := readCapturedInfo(t, service, "block", path)
		require.Equal(t, http.StatusOK, code)
		require.Equal(t, "value-"+strconv.Itoa(i), info.Body)
	}
}

func TestCaptureRejectsBodiesBeyondThePartitionBudget(t *testing.T) {
	service := New()
	body := strings.Repeat("a", 4<<20)
	for i := range maxPartitionBodyBytes / len(body) {
		response := serveCapture(service, http.MethodPost, "/block/path"+strconv.Itoa(i), body, nil)
		require.Equal(t, http.StatusOK, response.Code)
	}

	response := serveCapture(service, http.MethodPost, "/block/overflow", body, nil)

	require.Equal(t, http.StatusInsufficientStorage, response.Code)
	_, code := readCapturedInfo(t, service, "block", "/overflow")
	require.Equal(t, http.StatusNoContent, code)
}

func TestCaptureReplacingAPathReturnsItsBytesToTheBudget(t *testing.T) {
	service := New()
	body := strings.Repeat("a", 4<<20)
	for range 20 {
		response := serveCapture(service, http.MethodPost, "/block/same", body, nil)
		require.Equal(t, http.StatusOK, response.Code)
	}

	require.Equal(t, len(body), service.partitions["block"].bytes)
}

func TestCaptureResetRestoresThePartitionBudget(t *testing.T) {
	service := New()
	body := strings.Repeat("a", 4<<20)
	for i := range maxPartitionBodyBytes / len(body) {
		serveCapture(service, http.MethodPost, "/block/path"+strconv.Itoa(i), body, nil)
	}
	require.Equal(t, http.StatusInsufficientStorage,
		serveCapture(service, http.MethodPost, "/block/overflow", body, nil).Code)

	serveCapture(service, http.MethodPost, "/block/test/reset", "", nil)

	require.Equal(t, http.StatusOK,
		serveCapture(service, http.MethodPost, "/block/after-reset", body, nil).Code)
}

func TestCaptureBudgetIsPerPartition(t *testing.T) {
	service := New()
	body := strings.Repeat("a", 4<<20)
	for i := range maxPartitionBodyBytes / len(body) {
		serveCapture(service, http.MethodPost, "/one/path"+strconv.Itoa(i), body, nil)
	}
	require.Equal(t, http.StatusInsufficientStorage,
		serveCapture(service, http.MethodPost, "/one/overflow", body, nil).Code)

	require.Equal(t, http.StatusOK,
		serveCapture(service, http.MethodPost, "/two/path", body, nil).Code)
}

func scriptResponse(t *testing.T, service *Service, block, path string, script any) *httptest.ResponseRecorder {
	t.Helper()
	document, err := json.Marshal(script)
	require.NoError(t, err)
	return serveCapture(service, http.MethodPut, "/"+block+"/test/responses?path="+path, string(document), nil)
}

func TestScriptedResponseAnswersWithItsStatusContentTypeAndBody(t *testing.T) {
	service := New()
	stored := scriptResponse(t, service, "block", "/orders",
		map[string]any{"status": 201, "contentType": "application/xml", "body": `<ack id="7">received</ack>`})
	require.Equal(t, http.StatusOK, stored.Code)
	require.JSONEq(t, `{"path":"/orders","status":201}`, stored.Body.String())

	response := serveCapture(service, http.MethodPost, "/block/orders", `<root><sku>W-1</sku></root>`, nil)

	require.Equal(t, http.StatusCreated, response.Code)
	require.Equal(t, "application/xml", response.Header().Get("Content-Type"))
	require.Equal(t, strconv.Itoa(len(`<ack id="7">received</ack>`)), response.Header().Get("Content-Length"))
	require.Equal(t, `<ack id="7">received</ack>`, response.Body.String())
}

func TestScriptedPathStillRecordsTheRequest(t *testing.T) {
	service := New()
	scriptResponse(t, service, "block", "/orders", map[string]any{"contentType": "application/xml", "body": "<ack/>"})
	headers := http.Header{"Content-Type": []string{"application/xml"}}

	serveCapture(service, http.MethodPost, "/block/orders", `<root><sku>W-1</sku></root>`, headers)

	info, code := readCapturedInfo(t, service, "block", "/orders")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, `<root><sku>W-1</sku></root>`, info.Body)
	require.Equal(t, []string{"application/xml"}, info.Headers["Content-Type"])
}

func TestScriptedStatusDefaultsTo200(t *testing.T) {
	service := New()
	scriptResponse(t, service, "block", "/ready", map[string]any{"contentType": "text/plain", "body": "ok"})

	response := serveCapture(service, http.MethodGet, "/block/ready", "", nil)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "ok", response.Body.String())
}

func TestScriptedResponseWithoutContentTypeSendsNone(t *testing.T) {
	service := New()
	scriptResponse(t, service, "block", "/raw", map[string]any{"body": "<a>1</a>"})

	response := serveCapture(service, http.MethodGet, "/block/raw", "", nil)

	require.Equal(t, http.StatusOK, response.Code)
	require.Empty(t, response.Header().Get("Content-Type"))
	require.Equal(t, "<a>1</a>", response.Body.String())
}

func TestScriptedEmptyBodyKeepsItsContentType(t *testing.T) {
	service := New()
	scriptResponse(t, service, "block", "/empty", map[string]any{"contentType": "text/plain"})

	response := serveCapture(service, http.MethodGet, "/block/empty", "", nil)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "text/plain", response.Header().Get("Content-Type"))
	require.Equal(t, "0", response.Header().Get("Content-Length"))
	require.Empty(t, response.Body.String())
}

func TestScriptedResponseKeepsMalformedBodiesVerbatim(t *testing.T) {
	service := New()
	scriptResponse(t, service, "block", "/users",
		map[string]any{"contentType": "application/json", "body": `{"user": {"id": "u1", }`})

	response := serveCapture(service, http.MethodGet, "/block/users", "", nil)

	require.Equal(t, `{"user": {"id": "u1", }`, response.Body.String())
}

func TestScriptingOnePathLeavesOtherPathsReflecting(t *testing.T) {
	service := New()
	scriptResponse(t, service, "block", "/scripted", map[string]any{"contentType": "application/xml", "body": "<ack/>"})

	response := serveCapture(service, http.MethodPost, "/block/other", "value", nil)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "application/json", response.Header().Get("Content-Type"))
	var echoed requestInfo
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &echoed))
	require.Equal(t, "/other", echoed.Path)
	require.Equal(t, "value", echoed.Body)
}

func TestScriptMatchesTheExactPathOnly(t *testing.T) {
	service := New()
	scriptResponse(t, service, "block", "/orders", map[string]any{"contentType": "application/xml", "body": "<ack/>"})

	for _, path := range []string{"/orders/1", "/order", "/Orders"} {
		response := serveCapture(service, http.MethodGet, "/block"+path, "", nil)
		require.Equal(t, "application/json", response.Header().Get("Content-Type"), path)
	}
}

func TestStoringAScriptAgainReplacesIt(t *testing.T) {
	service := New()
	scriptResponse(t, service, "block", "/orders", map[string]any{"contentType": "application/xml", "body": "<first/>"})
	scriptResponse(t, service, "block", "/orders", map[string]any{"status": 202, "contentType": "application/json", "body": "{}"})

	response := serveCapture(service, http.MethodGet, "/block/orders", "", nil)

	require.Equal(t, http.StatusAccepted, response.Code)
	require.Equal(t, "application/json", response.Header().Get("Content-Type"))
	require.Equal(t, "{}", response.Body.String())
	require.Equal(t, 2, service.partitions["block"].scriptBytes)
}

func TestScriptsAreIsolatedByBlock(t *testing.T) {
	service := New()
	scriptResponse(t, service, "one", "/orders", map[string]any{"contentType": "application/xml", "body": "<one/>"})

	scripted := serveCapture(service, http.MethodGet, "/one/orders", "", nil)
	reflected := serveCapture(service, http.MethodGet, "/two/orders", "", nil)

	require.Equal(t, "<one/>", scripted.Body.String())
	require.Equal(t, "application/json", reflected.Header().Get("Content-Type"))
}

func TestResetClearsScriptsForOneBlock(t *testing.T) {
	service := New()
	scriptResponse(t, service, "one", "/orders", map[string]any{"contentType": "application/xml", "body": "<one/>"})
	scriptResponse(t, service, "two", "/orders", map[string]any{"contentType": "application/xml", "body": "<two/>"})

	serveCapture(service, http.MethodPost, "/one/test/reset", "", nil)

	require.Equal(t, "application/json", serveCapture(service, http.MethodGet, "/one/orders", "", nil).Header().Get("Content-Type"))
	require.Equal(t, "<two/>", serveCapture(service, http.MethodGet, "/two/orders", "", nil).Body.String())
}

func TestStoreResponseRejectsInvalidPaths(t *testing.T) {
	service := New()
	for _, query := range []string{"", "?path=", "?path=orders", "?path=/test", "?path=/test/captured", "?path=/test/responses"} {
		response := serveCapture(service, http.MethodPut, "/block/test/responses"+query, `{"body":"x"}`, nil)
		require.Equal(t, http.StatusBadRequest, response.Code, query)
	}
	require.Empty(t, service.partitions)
}

func TestStoreResponseRejectsInvalidDocuments(t *testing.T) {
	service := New()
	for name, document := range map[string]string{
		"not JSON":           `<ack/>`,
		"empty":              ``,
		"unknown field":      `{"status":200,"headers":{}}`,
		"two documents":      `{"status":200}{"status":201}`,
		"status below range": `{"status":199}`,
		"status above range": `{"status":600}`,
		"negative status":    `{"status":-1}`,
		"string status":      `{"status":"200"}`,
		"body with 204":      `{"status":204,"body":"x"}`,
		"body with 304":      `{"status":304,"body":"x"}`,
		"newline in type":    `{"contentType":"text/plain\r\nX-Injected: 1"}`,
		"non-string body":    `{"body":{"a":1}}`,
		"null document":      `null`,
		"array document":     `[]`,
		"trailing garbage":   `{"status":200} x`,
	} {
		response := serveCapture(service, http.MethodPut, "/block/test/responses?path=/orders", document, nil)
		require.Equal(t, http.StatusBadRequest, response.Code, name)
	}
	require.Empty(t, service.partitions)
}

func TestStoreResponseAcceptsA204WithoutABody(t *testing.T) {
	service := New()
	require.Equal(t, http.StatusOK, scriptResponse(t, service, "block", "/none", map[string]any{"status": 204}).Code)

	response := serveCapture(service, http.MethodGet, "/block/none", "", nil)

	require.Equal(t, http.StatusNoContent, response.Code)
	require.Empty(t, response.Body.String())
}

func TestStoreResponseRejectsOversizedDocuments(t *testing.T) {
	service := New()
	document := `{"body":"` + strings.Repeat("a", maxScriptDocumentBytes) + `"}`

	response := serveCapture(service, http.MethodPut, "/block/test/responses?path=/orders", document, nil)

	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
	require.Equal(t, "application/json", serveCapture(service, http.MethodGet, "/block/orders", "", nil).Header().Get("Content-Type"))
}

func TestScriptCountIsLimitedPerBlockButAllowsReplacement(t *testing.T) {
	service := NewWithRetentionLimit(2)
	require.Equal(t, http.StatusOK, scriptResponse(t, service, "block", "/one", map[string]any{"body": "1"}).Code)
	require.Equal(t, http.StatusOK, scriptResponse(t, service, "block", "/two", map[string]any{"body": "2"}).Code)

	require.Equal(t, http.StatusInsufficientStorage, scriptResponse(t, service, "block", "/three", map[string]any{"body": "3"}).Code)
	require.Equal(t, http.StatusOK, scriptResponse(t, service, "block", "/one", map[string]any{"body": "updated"}).Code)
	require.Equal(t, http.StatusOK, scriptResponse(t, service, "other", "/three", map[string]any{"body": "3"}).Code)
	require.Equal(t, "updated", serveCapture(service, http.MethodGet, "/block/one", "", nil).Body.String())
}

func TestScriptBytesAreLimitedPerBlock(t *testing.T) {
	service := New()
	body := strings.Repeat("a", maxScriptDocumentBytes-64)
	stored := 0
	for i := 0; ; i++ {
		response := scriptResponse(t, service, "block", "/path"+strconv.Itoa(i), map[string]any{"body": body})
		if response.Code == http.StatusInsufficientStorage {
			break
		}
		require.Equal(t, http.StatusOK, response.Code)
		stored++
		require.Less(t, stored, maxPartitionBodyBytes/len(body)+1, "the byte budget was never enforced")
	}
	require.Equal(t, maxPartitionBodyBytes/len(body), stored)

	serveCapture(service, http.MethodPost, "/block/test/reset", "", nil)
	require.Equal(t, http.StatusOK, scriptResponse(t, service, "block", "/after-reset", map[string]any{"body": body}).Code)
}

func TestScriptedResponsesAreSafeUnderConcurrentUse(t *testing.T) {
	service := New()
	const total = 32
	var wg sync.WaitGroup
	for i := range total {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			path := "/path-" + strconv.Itoa(i)
			want := "<n>" + strconv.Itoa(i) + "</n>"
			if code := scriptResponse(t, service, "block", path, map[string]any{"contentType": "application/xml", "body": want}).Code; code != http.StatusOK {
				t.Errorf("store status = %d", code)
				return
			}
			if got := serveCapture(service, http.MethodPost, "/block"+path, "value", nil).Body.String(); got != want {
				t.Errorf("answer for %s = %q, want %q", path, got, want)
			}
		}(i)
	}
	wg.Wait()
}

func TestScriptedResponseWithoutContentTypeIsNotSniffedByTheServer(t *testing.T) {
	service := New()
	scriptResponse(t, service, "block", "/raw", map[string]any{"body": "<html><body>x</body></html>"})
	server := httptest.NewServer(service.Handler())
	defer server.Close()

	response, err := http.Get(server.URL + "/block/raw")
	require.NoError(t, err)
	defer func() { _ = response.Body.Close() }()

	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Empty(t, response.Header.Get("Content-Type"))
}

func TestOtherMethodsOnTheResponsesPathStillReflect(t *testing.T) {
	service := New()
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodPatch} {
		response := serveCapture(service, method, "/block/test/responses", "value-"+method, nil)
		require.Equal(t, http.StatusOK, response.Code, method)
		require.Equal(t, "application/json", response.Header().Get("Content-Type"), method)

		info, code := readCapturedInfo(t, service, "block", "/test/responses")
		require.Equal(t, http.StatusOK, code, method)
		require.Equal(t, method, info.Method)
		require.Equal(t, "value-"+method, info.Body)
	}
	require.Empty(t, service.partitions["block"].scripts)
}
