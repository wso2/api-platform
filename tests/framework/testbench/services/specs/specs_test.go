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

package specs

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFixturesAreServedAsTheirOwnBytes(t *testing.T) {
	want, err := Fixture("petstore-v1.yaml")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New().Handler())
	defer server.Close()
	resp, err := http.Get(server.URL + "/specs/petstore-v1.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	got, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(got) != string(want) {
		t.Fatalf("GET /specs/petstore-v1.yaml = %d with %d bytes, want 200 with the fixture's %d", resp.StatusCode, len(got), len(want))
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/yaml" {
		t.Fatalf("Content-Type = %q, want application/yaml", ct)
	}
}

func TestUnknownAndTraversingNamesAreNotServed(t *testing.T) {
	for _, name := range []string{"missing.yaml", "../specs.go", ".hidden", "fixtures/petstore-v1.yaml"} {
		if _, err := Fixture(name); err == nil {
			t.Errorf("Fixture(%q) succeeded, want an error", name)
		}
	}
	server := httptest.NewServer(New().Handler())
	defer server.Close()
	resp, err := http.Get(server.URL + "/specs/missing.yaml")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /specs/missing.yaml = %d, want 404", resp.StatusCode)
	}
}

func TestGeneratedDocumentsHaveTheRequestedSize(t *testing.T) {
	for _, n := range []int{1024, 65537} {
		data, err := Generate(n)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) != n {
			t.Fatalf("Generate(%d) returned %d bytes", n, len(data))
		}
	}
	for _, n := range []int{10, maxGeneratedBytes + 1} {
		if _, err := Generate(n); err == nil {
			t.Errorf("Generate(%d) succeeded, want an error", n)
		}
	}
	server := httptest.NewServer(New().Handler())
	defer server.Close()
	resp, err := http.Get(server.URL + "/specs/large?bytes=4096")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	got, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || len(got) != 4096 {
		t.Fatalf("GET /specs/large?bytes=4096 = %d with %d bytes", resp.StatusCode, len(got))
	}
}
