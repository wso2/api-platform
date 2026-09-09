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
