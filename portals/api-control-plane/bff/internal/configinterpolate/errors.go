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

// Package configinterpolate resolves Go text/template expressions embedded in a
// configuration map before it is unmarshalled into typed structs. It supports two
// fail-closed source functions — env and file — so operators can inject values
// (especially secrets) into individual config fields, including elements of
// arrays-of-tables that flat environment-variable overrides cannot reach.
//
// The package is transport-agnostic: it operates purely on map[string]any and has
// no dependency on koanf or any config library, so it can be shared across the
// gateway-controller, policy-engine, and platform-api config loaders.
package configinterpolate

import (
	"fmt"
	"strings"
)

// ParseError wraps a template parse failure (e.g. unknown function, syntax error).
// Its Error() strips Go's internal "template: <name>:<line>: " prefix so callers
// receive just the description (e.g. `function "secret" not defined`).
type ParseError struct {
	Cause error
}

func (e *ParseError) Error() string {
	// Template parse errors have format: "template: <name>:<line>: <description>".
	parts := strings.SplitN(e.Cause.Error(), ": ", 3)
	if len(parts) == 3 {
		return parts[2]
	}
	return e.Cause.Error()
}

func (e *ParseError) Unwrap() error { return e.Cause }

// ExecError wraps a rendering failure for a single config field, carrying the
// dotted key path so the operator can see which field failed. The Cause is the
// sterile underlying reason (e.g. `required env var "TOK" is not found`) with no
// resolved value, allowlist contents, or size limit embedded.
type ExecError struct {
	Field string
	Cause error
}

func (e *ExecError) Error() string {
	return fmt.Sprintf("config interpolation failed at %q: %v", e.Field, e.Cause)
}

func (e *ExecError) Unwrap() error { return e.Cause }
