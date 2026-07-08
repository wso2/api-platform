/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package apperror

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// codeShape is the format every catalog code must follow: uppercase
// SCREAMING_SNAKE_CASE, stable and client-visible.
var codeShape = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// fmtVerb matches fmt verbs in message templates (ignoring literal %%).
var fmtVerb = regexp.MustCompile(`%[^%]`)

// messageArity declares, for every catalog entry whose MessageFmt contains
// fmt verbs, how many arguments call sites must pass. Adding a verb to a
// template without updating this table fails the test — the arity is part of
// the entry's contract, mirroring the argument list of a Java ExceptionCodes
// enum constant.
var messageArity = map[string]int{
	CodeOf(ValidationFailed):                      1,
	CodeOf(RESTAPIExists):                         1,
	CodeOf(RESTAPIDeploymentValidationFailed):     1,
	CodeOf(LLMProviderDeploymentValidationFailed): 1,
	CodeOf(LLMProxyDeploymentValidationFailed):    1,
	CodeOf(MCPProxyDeploymentValidationFailed):    1,
	CodeOf(DeploymentNotActive):                   1,
	CodeOf(ArtifactReadOnly):                      1,
	CodeOf(ArtifactRuntimeImmutable):              1,
	CodeOf(ArtifactDeployed):                      1,
	CodeOf(TooManyRequests):                       1,
}

// CodeOf makes the arity table read as a set of catalog references rather
// than duplicated string literals.
func CodeOf(d Def) string { return d.Code }

// internalMarkers are substrings that must never appear in a client-facing
// message template (error-handling.md: zero internal details).
var internalMarkers = []string{"sql", "pq:", "goroutine", ".go:", "/internal/", "stack", "panic"}

func TestCatalogIntegrity(t *testing.T) {
	if len(allDefs) == 0 {
		t.Fatal("catalog is empty — def() registration broken?")
	}

	seen := make(map[string]Def, len(allDefs))
	for _, d := range allDefs {
		if !codeShape.MatchString(d.Code) {
			t.Errorf("%s: code is not SCREAMING_SNAKE_CASE", d.Code)
		}
		if prev, dup := seen[d.Code]; dup {
			t.Errorf("%s: declared twice (statuses %d and %d) — codes must be unique", d.Code, prev.HTTPStatus, d.HTTPStatus)
		}
		seen[d.Code] = d

		if d.HTTPStatus < 400 || d.HTTPStatus > 599 {
			t.Errorf("%s: status %d is not a 4xx/5xx error status", d.Code, d.HTTPStatus)
		}
		if http.StatusText(d.HTTPStatus) == "" {
			t.Errorf("%s: status %d is not a registered HTTP status", d.Code, d.HTTPStatus)
		}

		if strings.TrimSpace(d.MessageFmt) == "" {
			t.Errorf("%s: empty message template", d.Code)
		}
		lower := strings.ToLower(d.MessageFmt)
		for _, marker := range internalMarkers {
			if strings.Contains(lower, marker) {
				t.Errorf("%s: message template contains internal marker %q: %q", d.Code, marker, d.MessageFmt)
			}
		}

		verbs := len(fmtVerb.FindAllString(strings.ReplaceAll(d.MessageFmt, "%%", ""), -1))
		want, declared := messageArity[d.Code]
		if verbs > 0 && !declared {
			t.Errorf("%s: template has %d fmt verb(s) but no entry in messageArity", d.Code, verbs)
		}
		if declared && verbs != want {
			t.Errorf("%s: messageArity declares %d arg(s) but template has %d verb(s)", d.Code, want, verbs)
		}
	}
}

// TestUnauthorizedIsUnified pins the unified-auth-failure rule: exactly one
// 401 entry exists and its message is the standard generic one, so no auth
// path can leak why authentication failed.
func TestUnauthorizedIsUnified(t *testing.T) {
	var unauthorized []Def
	for _, d := range allDefs {
		if d.HTTPStatus == http.StatusUnauthorized {
			unauthorized = append(unauthorized, d)
		}
	}
	if len(unauthorized) != 1 {
		t.Fatalf("expected exactly one 401 catalog entry, found %d", len(unauthorized))
	}
	if unauthorized[0].MessageFmt != "Invalid or expired credentials." {
		t.Errorf("401 message must be the standard generic one, got %q", unauthorized[0].MessageFmt)
	}
	if strings.Contains(unauthorized[0].MessageFmt, "%") {
		t.Error("401 message must not be parameterizable")
	}
}
