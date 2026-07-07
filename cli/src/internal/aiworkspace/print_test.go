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

package aiworkspace

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/wso2/api-platform/cli/test/testutil"
)

func responseWith(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusCreated,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{},
	}
}

func TestPrintApplyResult_StructuredOutput(t *testing.T) {
	body := `{"id":"claude-proxy2","organizationId":"org-1","projectId":"proj-1","createdAt":"2026-07-03T06:31:22Z","updatedAt":"2026-07-03T06:31:22Z","status":"deployed"}`
	resp := responseWith(body)

	out := testutil.CaptureStdout(t, func() {
		if err := PrintApplyResult(resp, "", "LlmProxy", "applied", "fallback-id", "fallback-proj"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{
		"Status: success",
		"Message: LlmProxy applied successfully",
		"ID: claude-proxy2",
		"Organization: org-1",
		"Project: proj-1",
		"Created At: 2026-07-03T06:31:22Z",
		"Updated At: 2026-07-03T06:31:22Z",
		"State: deployed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q\n---\n%s", want, out)
		}
	}
}

func TestPrintApplyResult_FallbackProjectAndNoOrgWhenAbsent(t *testing.T) {
	// Response omits organizationId and projectId: Project falls back to the
	// locally supplied --project-id, Organization is dropped entirely.
	resp := responseWith(`{"id":"p"}`)

	out := testutil.CaptureStdout(t, func() {
		if err := PrintApplyResult(resp, "", "LlmProxy", "updated", "fallback-id", "local-proj"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Project: local-proj") {
		t.Fatalf("expected fallback project, got:\n%s", out)
	}
	if strings.Contains(out, "Organization:") {
		t.Fatalf("expected no Organization line when absent, got:\n%s", out)
	}
}

func TestPrintApplyResult_UsesFallbackIDAndOmitsMissingFields(t *testing.T) {
	// Response without id/timestamps/status and no project fallback: id falls
	// back, optional lines dropped.
	resp := responseWith(`{}`)

	out := testutil.CaptureStdout(t, func() {
		if err := PrintApplyResult(resp, "", "LlmProvider", "updated", "fallback-id", ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Message: LlmProvider updated successfully") {
		t.Fatalf("expected updated message, got:\n%s", out)
	}
	if !strings.Contains(out, "ID: fallback-id") {
		t.Fatalf("expected fallback id, got:\n%s", out)
	}
	if strings.Contains(out, "Created At:") || strings.Contains(out, "State:") ||
		strings.Contains(out, "Organization:") || strings.Contains(out, "Project:") {
		t.Fatalf("expected no optional lines, got:\n%s", out)
	}
}

func TestPrintApplyResult_JSONOutputPassthrough(t *testing.T) {
	resp := responseWith(`{"id":"x","status":"deployed"}`)

	out := testutil.CaptureStdout(t, func() {
		if err := PrintApplyResult(resp, "json", "Mcp", "applied", "x", ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// json output mode prints the raw response, not the structured summary.
	if strings.Contains(out, "Status: success") {
		t.Fatalf("expected raw json output, got structured summary:\n%s", out)
	}
	if !strings.Contains(out, "\"status\": \"deployed\"") {
		t.Fatalf("expected pretty-printed json, got:\n%s", out)
	}
}
