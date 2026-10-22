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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func copySample(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("testdata/model_prices_sample.json")
	if err != nil {
		t.Fatalf("reading sample: %v", err)
	}
	path := filepath.Join(t.TempDir(), "model_prices.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing sample copy: %v", err)
	}
	return path
}

// -check reports an out-of-date file without touching it; a sync then writes
// it, after which -check passes.
func TestRun_CheckThenSync(t *testing.T) {
	path := copySample(t)
	before, _ := os.ReadFile(path)
	args := []string{"-file", path, "-meters", "testdata/meters.json"}

	err := run(context.Background(), append([]string{"-check"}, args...), io.Discard)
	if !errors.Is(err, errOutOfDate) {
		t.Fatalf("-check = %v; want errOutOfDate", err)
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Fatal("-check must not write the file")
	}

	if err := run(context.Background(), args, io.Discard); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if err := run(context.Background(), append([]string{"-check"}, args...), io.Discard); err != nil {
		t.Errorf("-check after sync = %v; want up to date", err)
	}
}

func TestRun_RequiresFile(t *testing.T) {
	if err := run(context.Background(), []string{"-meters", "testdata/meters.json"}, io.Discard); err == nil {
		t.Error("run without -file must fail")
	}
}

// fetchMeters must page through NextPageLink and ask for one region at a time.
func TestFetchMeters_FollowsPagination(t *testing.T) {
	var filters []string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filters = append(filters, r.URL.Query().Get("$filter"))
		page := retailPage{Items: []meter{{SkuName: "first"}}, NextPageLink: srv.URL + "/page2"}
		if r.URL.Path == "/page2" {
			page = retailPage{Items: []meter{{SkuName: "second"}}}
		}
		if err := json.NewEncoder(w).Encode(page); err != nil {
			t.Errorf("encoding page: %v", err)
		}
	}))
	defer srv.Close()

	got, err := fetchMeters(context.Background(), srv.Client(), srv.URL, []string{"eastus2"})
	if err != nil {
		t.Fatalf("fetchMeters: %v", err)
	}
	if len(got) != 2 || got[1].SkuName != "second" {
		t.Errorf("fetched %+v; want both pages", got)
	}
	if !strings.Contains(filters[0], "armRegionName eq 'eastus2'") || !strings.Contains(filters[0], "serviceName eq 'Foundry Models'") {
		t.Errorf("filter = %q; want the service and region scoped", filters[0])
	}
}

func TestFetchMeters_PropagatesHTTPErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "throttled", http.StatusTooManyRequests)
	}))
	defer srv.Close()
	if _, err := fetchMeters(context.Background(), srv.Client(), srv.URL, []string{"eastus2"}); err == nil {
		t.Error("a non-200 page must fail the fetch")
	}
}
