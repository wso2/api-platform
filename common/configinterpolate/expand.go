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

package configinterpolate

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"text/template"
)

// DefaultMaxFileBytes is the ceiling applied to a single {{ file }} read when
// Options.MaxFileBytes is not set. Secret files (tokens, keys, passwords) are far
// smaller than this; the cap guards against accidentally slurping a huge file.
const DefaultMaxFileBytes int64 = 1 << 20 // 1 MiB

// Options configures an Expand pass. The zero value is usable: file access is
// rejected (empty allowlist) and the default size cap applies.
type Options struct {
	// FileAllowlist is the set of absolute directories a {{ file }} path may live
	// under. Empty means file interpolation is not permitted. Callers supply their
	// own per-component defaults (see ResolveAllowlist).
	FileAllowlist []string
	// MaxFileBytes caps a single {{ file }} read. <= 0 uses DefaultMaxFileBytes.
	MaxFileBytes int64
}

// Stats reports how many references were resolved, for boot logging. It carries
// counts only — never resolved values.
type Stats struct {
	EnvRefs  int // number of {{ env }} invocations
	FileRefs int // number of {{ file }} invocations
	Fields   int // number of string leaves that contained a "{{" token
}

// Expand walks every string leaf of raw, renders any Go text/template tokens it
// finds, and returns a new map with the resolved values. The input map is not
// mutated. It fails closed: a missing required env var, a disallowed/missing/
// oversize file, or a template parse error aborts the whole pass with an error.
//
// A leaf that contains no "{{" token is returned unchanged, so a config with no
// tokens round-trips structurally identical (backward-compatible no-op).
func Expand(raw map[string]any, opts Options) (map[string]any, Stats, error) {
	if opts.MaxFileBytes <= 0 {
		opts.MaxFileBytes = DefaultMaxFileBytes
	}
	stats := &Stats{}
	funcMap := buildFuncMap(&opts, stats)

	out, err := walkValue(raw, funcMap, stats, "")
	if err != nil {
		return nil, Stats{}, err
	}
	// The top-level value is always a map[string]any.
	expanded, _ := out.(map[string]any)
	return expanded, *stats, nil
}

// walkValue recursively renders template expressions in string leaves of a
// map[string]any / []any / string tree. Non-string scalars are returned as-is.
// Rendering at the leaf level (rather than over the whole serialized document)
// keeps escaping simple and reaches into arrays-of-tables for free.
func walkValue(val any, funcMap template.FuncMap, stats *Stats, path string) (any, error) {
	switch v := val.(type) {
	case string:
		if !strings.Contains(v, "{{") {
			return v, nil
		}
		stats.Fields++
		rendered, err := render(v, funcMap)
		if err != nil {
			return nil, &ExecError{Field: path, Cause: err}
		}
		return rendered, nil

	case map[string]any:
		result := make(map[string]any, len(v))
		for key, child := range v {
			r, err := walkValue(child, funcMap, stats, joinKey(path, key))
			if err != nil {
				return nil, err
			}
			result[key] = r
		}
		return result, nil

	case []any:
		result := make([]any, len(v))
		for i, child := range v {
			r, err := walkValue(child, funcMap, stats, fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return nil, err
			}
			result[i] = r
		}
		return result, nil

	default:
		// Numbers, booleans, nil — nothing to render.
		return val, nil
	}
}

func joinKey(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// render executes Go template expressions in a single string. Parse failures are
// wrapped in *ParseError; execution failures are unwrapped from text/template's
// ExecError so the caller sees the underlying func error (e.g. the sterile
// `required env var "X" is not found`) rather than the verbose template wrapper.
func render(raw string, funcMap template.FuncMap) (string, error) {
	tmpl, err := template.New("config").Funcs(funcMap).Parse(raw)
	if err != nil {
		return "", &ParseError{Cause: err}
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		if execErr, ok := errors.AsType[template.ExecError](err); ok {
			return "", execErr.Err
		}
		return "", err
	}
	return buf.String(), nil
}
