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

package gateway

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempCR(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

func TestParseResourceCR_ValidYAML(t *testing.T) {
	path := writeTempCR(t, "plan.yaml", `apiVersion: gateway.api-platform.wso2.com/v1
kind: SubscriptionPlan
metadata:
  name: bronze-1k-per-min
spec:
  planName: Bronze
  throttleLimitCount: 1000
  throttleLimitUnit: Min
`)

	cr, err := ParseResourceCR(path, "SubscriptionPlan")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cr.Metadata.Name != "bronze-1k-per-min" {
		t.Errorf("metadata.name = %q, want bronze-1k-per-min", cr.Metadata.Name)
	}
	if cr.Spec["planName"] != "Bronze" {
		t.Errorf("spec.planName = %v, want Bronze", cr.Spec["planName"])
	}
}

func TestParseResourceCR_ValidJSON(t *testing.T) {
	path := writeTempCR(t, "plan.json", `{"kind":"SubscriptionPlan","metadata":{"name":"gold"},"spec":{"planName":"Gold"}}`)

	cr, err := ParseResourceCR(path, "SubscriptionPlan")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cr.Metadata.Name != "gold" {
		t.Errorf("metadata.name = %q, want gold", cr.Metadata.Name)
	}
}

func TestParseResourceCR_NestedSpecIsStringKeyed(t *testing.T) {
	// yaml.v3 decodes nested maps as map[string]interface{}, which command
	// handlers rely on (e.g. spec.parentRef in the ApiKey CR).
	path := writeTempCR(t, "apikey.yaml", `kind: ApiKey
metadata:
  name: petstore-key-acme
spec:
  parentRef:
    kind: RestApi
    name: petstore-api-v1.0
`)

	cr, err := ParseResourceCR(path, "ApiKey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parentRef, ok := cr.Spec["parentRef"].(map[string]interface{})
	if !ok {
		t.Fatalf("spec.parentRef is %T, want map[string]interface{}", cr.Spec["parentRef"])
	}
	if parentRef["name"] != "petstore-api-v1.0" {
		t.Errorf("parentRef.name = %v, want petstore-api-v1.0", parentRef["name"])
	}
}

func TestParseResourceCR_Errors(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"wrong kind", "kind: Subscription\nmetadata:\n  name: x\nspec:\n  apiId: a\n"},
		{"missing kind", "metadata:\n  name: x\nspec:\n  planName: p\n"},
		{"missing name", "kind: SubscriptionPlan\nspec:\n  planName: p\n"},
		{"missing spec", "kind: SubscriptionPlan\nmetadata:\n  name: x\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTempCR(t, "cr.yaml", tc.content)
			if _, err := ParseResourceCR(path, "SubscriptionPlan"); err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestParseResourceCR_FileNotFound(t *testing.T) {
	if _, err := ParseResourceCR(filepath.Join(t.TempDir(), "nope.yaml"), "SubscriptionPlan"); err == nil {
		t.Error("expected error for missing file, got nil")
	}
}
