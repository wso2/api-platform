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

package templateengine

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// TemplateParseError wraps a template parse failure (e.g. unknown function, syntax error).
// Its Error() strips Go's internal "template: <name>:<line>: " prefix so callers receive
// just the description (e.g. `function "secret2" not defined`).
type TemplateParseError struct {
	Cause error
}

func (e *TemplateParseError) Error() string {
	// Template parse errors have format: "template: <name>:<line>: <description>"
	parts := strings.SplitN(e.Cause.Error(), ": ", 3)
	if len(parts) == 3 {
		return parts[2]
	}
	return e.Cause.Error()
}

func (e *TemplateParseError) Unwrap() error { return e.Cause }

// render executes Go template expressions in a raw string and returns the rendered result.
func render(raw []byte, funcMap template.FuncMap) ([]byte, error) {
	tmpl, err := template.New("artifact").Funcs(funcMap).Parse(string(raw))
	if err != nil {
		return nil, &TemplateParseError{Cause: err}
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		return nil, fmt.Errorf("template execution error: %w", err)
	}
	return buf.Bytes(), nil
}
