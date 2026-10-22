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
	"encoding/json"
	"os"
	"testing"
)

// Reading and writing a file without changes must reproduce it exactly, so a
// sync diff shows only what changed. Checked against the shipped file too.
func TestPricingFileRoundTrip(t *testing.T) {
	for _, path := range []string{
		"testdata/model_prices_sample.json",
		"../../gateway/configs/llm-pricing/model_prices.json",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		f, err := parsePricingFile(data)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		got, err := f.bytes()
		if err != nil {
			t.Fatalf("encoding %s: %v", path, err)
		}
		if string(got) != string(data) {
			t.Errorf("%s did not round-trip byte for byte", path)
		}
	}
}

func TestParsePricingFile_Rejects(t *testing.T) {
	for _, data := range []string{`[]`, `{"a": {}, "a": {}}`, `{"a": `} {
		if _, err := parsePricingFile([]byte(data)); err == nil {
			t.Errorf("parsePricingFile(%q) succeeded; want an error", data)
		}
	}
}

func TestFormatRate(t *testing.T) {
	for v, want := range map[float64]string{
		4e-06:    "4e-06",
		1.25e-07: "1.25e-07",
		0.00025:  "0.00025",
		1.5:      "1.5",
		2:        "2.0",
	} {
		if got := formatRate(v); got != want {
			t.Errorf("formatRate(%v) = %q; want %q", v, got, want)
		}
	}
}

func TestEncodeEntry_SortsAndIndents(t *testing.T) {
	fields := map[string]json.RawMessage{
		"provider":                      json.RawMessage(`"azure"`),
		"input_cost_per_token":          json.RawMessage(`4e-06`),
		"search_context_cost_per_query": json.RawMessage(`{"search_context_size_low": 0.01}`),
	}
	got, err := encodeEntry(fields)
	if err != nil {
		t.Fatalf("encodeEntry: %v", err)
	}
	want := "{\n" +
		"    \"input_cost_per_token\": 4e-06,\n" +
		"    \"provider\": \"azure\",\n" +
		"    \"search_context_cost_per_query\": {\n" +
		"      \"search_context_size_low\": 0.01\n" +
		"    }\n" +
		"  }"
	if string(got) != want {
		t.Errorf("encodeEntry =\n%s\nwant\n%s", got, want)
	}
}

func TestKeyFold(t *testing.T) {
	f := &pricingFile{keys: []string{"azure_ai/Llama-3.3-70B-Instruct"}}
	if got, ok := f.keyFold("azure_ai/llama-3.3-70b-instruct"); !ok || got != "azure_ai/Llama-3.3-70B-Instruct" {
		t.Errorf("keyFold = %q, %v; want the existing spelling", got, ok)
	}
}
