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

package it

import (
	"context"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

// TemplateSteps provides step definitions for asserting that template
// expressions ({{ env "..." }}, {{ secret "..." }}, {{ default ... }}) are
// resolved at runtime but persisted unrendered in the API response and DB.
type TemplateSteps struct {
	state     *TestState
	httpSteps *steps.HTTPSteps
}

// NewTemplateSteps creates a new TemplateSteps instance.
func NewTemplateSteps(state *TestState, httpSteps *steps.HTTPSteps) *TemplateSteps {
	return &TemplateSteps{state: state, httpSteps: httpSteps}
}

// RegisterTemplateSteps registers all template-related Gherkin steps.
func RegisterTemplateSteps(ctx *godog.ScenarioContext, state *TestState, httpSteps *steps.HTTPSteps) {
	t := NewTemplateSteps(state, httpSteps)

	ctx.Step(`^the response body should contain template literal:$`, t.responseBodyShouldContainLiteral)
	ctx.Step(`^the stored RestApi configuration for "([^"]*)" should contain:$`, t.storedRestAPIShouldContain)
}

// responseBodyShouldContainLiteral checks that the last response body contains
// the supplied docstring verbatim. Because both REST responses and the DB store
// the body as JSON-encoded text, inner double quotes appear as `\"`. The check
// matches against either form so feature files can write the natural literal
// (e.g. `{{ secret "x" }}`) without manual escaping.
func (t *TemplateSteps) responseBodyShouldContainLiteral(literal *godog.DocString) error {
	body := string(t.httpSteps.LastBody())
	expected := strings.TrimSpace(literal.Content)
	if expected == "" {
		return fmt.Errorf("expected literal is empty")
	}
	if containsLiteralOrJSONEscaped(body, expected) {
		return nil
	}
	return fmt.Errorf("response body does not contain expected template literal\nexpected substring: %q\nactual body: %s", expected, body)
}

// containsLiteralOrJSONEscaped returns true if haystack contains needle either
// verbatim or with each unescaped `"` replaced by `\"` (the JSON-encoded form).
// We don't double-escape backslashes here because no current template literal
// in scope contains a literal backslash.
func containsLiteralOrJSONEscaped(haystack, needle string) bool {
	if strings.Contains(haystack, needle) {
		return true
	}
	jsonEscaped := strings.ReplaceAll(needle, `"`, `\"`)
	return jsonEscaped != needle && strings.Contains(haystack, jsonEscaped)
}

// storedRestAPIShouldContain queries the controller's SQLite DB via the
// it-db-reader sidecar and asserts the unrendered SourceConfiguration blob for
// the given RestApi handle contains the supplied literal. Used to verify that
// the persisted configuration retains template expressions verbatim.
func (t *TemplateSteps) storedRestAPIShouldContain(handle string, literal *godog.DocString) error {
	expected := strings.TrimSpace(literal.Content)
	if expected == "" {
		return fmt.Errorf("expected literal is empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultDBQueryTimeout)
	defer cancel()

	row, err := GetStoredRestAPISourceConfigurationWithRetry(ctx, handle)
	if err != nil {
		return fmt.Errorf("failed to read stored configuration for %q: %w", handle, err)
	}
	if !containsLiteralOrJSONEscaped(row, expected) {
		return fmt.Errorf("stored configuration for %q does not contain expected template literal\nexpected substring: %q\nstored row: %s", handle, expected, row)
	}
	return nil
}
