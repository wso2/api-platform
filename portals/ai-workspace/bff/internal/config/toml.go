/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package config

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// decodeTOML decodes the subset of TOML the AI Workspace configuration is written in,
// using only the standard library. The supported grammar is:
//
//	# comment                          comments, whole-line or trailing
//	key = 'literal'                    literal string, taken verbatim (backslashes kept)
//	key = "basic\n"                    basic string, standard escapes expanded
//	key = true | 42 | 1.5              bare boolean, integer, float
//	[table]                            table header
//	[table.sub]                        nested table header
//	a.b = "v"                          dotted key, equivalent to b under table a
//
// Everything else in the TOML specification — arrays, inline tables, multi-line strings,
// arrays of tables, dates — is rejected with an error naming the line, rather than being
// parsed loosely. The config keys are all scalars (see settings.flatten, which drops
// anything that is not a scalar or a table), so an unsupported construct in the file is a
// mistake worth failing on, not something to skip silently.
//
// Values are returned as map[string]any holding string, bool, int64 or float64 leaves and
// map[string]any sub-tables — the shape configinterpolate.Expand walks.
func decodeTOML(src string) (map[string]any, error) {
	root := map[string]any{}
	table := root // the table that bare keys currently land in

	for i, raw := range strings.Split(src, "\n") {
		lineNo := i + 1
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") {
			t, err := openTable(root, line)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNo, err)
			}
			table = t
			continue
		}

		if err := setKey(table, line); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
	}
	return root, nil
}

// openTable handles a [table] / [table.sub] header, creating the table if needed and
// returning it as the destination for the keys that follow.
func openTable(root map[string]any, line string) (map[string]any, error) {
	if strings.HasPrefix(line, "[[") {
		return nil, fmt.Errorf("arrays of tables are not supported: %s", line)
	}
	end := strings.IndexByte(line, ']')
	if end < 0 {
		return nil, fmt.Errorf("unterminated table header: %s", line)
	}
	if rest := strings.TrimSpace(line[end+1:]); rest != "" && !strings.HasPrefix(rest, "#") {
		return nil, fmt.Errorf("unexpected text after table header: %s", rest)
	}

	path, err := splitKeyPath(line[1:end])
	if err != nil {
		return nil, err
	}
	return descend(root, path)
}

// setKey handles a `key = value` line, assigning into table (or into a sub-table of it
// when the key is dotted).
func setKey(table map[string]any, line string) error {
	eq := strings.IndexByte(line, '=')
	if eq < 0 {
		return fmt.Errorf("expected 'key = value': %s", line)
	}

	path, err := splitKeyPath(line[:eq])
	if err != nil {
		return err
	}
	dst, err := descend(table, path[:len(path)-1])
	if err != nil {
		return err
	}
	key := path[len(path)-1]
	if _, exists := dst[key]; exists {
		return fmt.Errorf("duplicate key %q", key)
	}

	val, err := parseValue(strings.TrimSpace(line[eq+1:]))
	if err != nil {
		return fmt.Errorf("key %q: %w", key, err)
	}
	dst[key] = val
	return nil
}

// splitKeyPath splits a dotted key or table name into its parts, accepting the quoted
// form ("my key") alongside bare keys.
func splitKeyPath(s string) ([]string, error) {
	var path []string
	for _, part := range strings.Split(s, ".") {
		part = strings.TrimSpace(part)
		if len(part) >= 2 && (part[0] == '"' || part[0] == '\'') && part[len(part)-1] == part[0] {
			part = part[1 : len(part)-1]
		}
		if part == "" {
			return nil, fmt.Errorf("empty key in %q", strings.TrimSpace(s))
		}
		path = append(path, part)
	}
	if len(path) == 0 {
		return nil, fmt.Errorf("empty key")
	}
	return path, nil
}

// descend walks path from tbl, creating the intermediate tables that do not exist yet.
// A path that runs through a key already holding a scalar is an error rather than an
// overwrite.
func descend(tbl map[string]any, path []string) (map[string]any, error) {
	for _, part := range path {
		switch child := tbl[part].(type) {
		case nil:
			next := map[string]any{}
			tbl[part] = next
			tbl = next
		case map[string]any:
			tbl = child
		default:
			return nil, fmt.Errorf("key %q is a value, not a table", part)
		}
	}
	return tbl, nil
}

// parseValue decodes the right-hand side of a key/value line: a quoted string, or a bare
// scalar followed by an optional trailing comment.
func parseValue(s string) (any, error) {
	if s == "" {
		return nil, fmt.Errorf("missing value")
	}

	switch s[0] {
	case '\'':
		if strings.HasPrefix(s, "'''") {
			return nil, fmt.Errorf("multi-line strings are not supported")
		}
		// Literal string: no escape processing, so a backslash stays a backslash — the
		// {{ env }} tokens rely on this to carry \" inside their JSON defaults.
		end := strings.IndexByte(s[1:], '\'')
		if end < 0 {
			return nil, fmt.Errorf("unterminated literal string")
		}
		return s[1 : 1+end], trailing(s[end+2:])

	case '"':
		if strings.HasPrefix(s, `"""`) {
			return nil, fmt.Errorf("multi-line strings are not supported")
		}
		val, n, err := parseBasicString(s)
		if err != nil {
			return nil, err
		}
		return val, trailing(s[n:])

	case '[':
		return nil, fmt.Errorf("arrays are not supported")
	case '{':
		return nil, fmt.Errorf("inline tables are not supported")
	}

	// Bare scalar: a trailing comment can only start at an unquoted '#'.
	bare := s
	if hash := strings.IndexByte(bare, '#'); hash >= 0 {
		bare = bare[:hash]
	}
	bare = strings.TrimSpace(bare)

	switch bare {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	if i, err := strconv.ParseInt(bare, 10, 64); err == nil {
		return i, nil
	}
	if f, err := strconv.ParseFloat(bare, 64); err == nil {
		return f, nil
	}
	return nil, fmt.Errorf("unsupported value %q — strings must be quoted", bare)
}

// parseBasicString decodes a double-quoted string starting at s[0], expanding the TOML
// escape sequences. It returns the value and the number of bytes consumed, including
// both quotes.
func parseBasicString(s string) (string, int, error) {
	var b strings.Builder
	for i := 1; i < len(s); {
		c := s[i]
		switch c {
		case '"':
			return b.String(), i + 1, nil
		case '\\':
			val, n, err := unescape(s[i:])
			if err != nil {
				return "", 0, err
			}
			b.WriteString(val)
			i += n
		default:
			b.WriteByte(c)
			i++
		}
	}
	return "", 0, fmt.Errorf("unterminated string")
}

// unescape decodes the escape sequence at the start of s (which begins with a
// backslash), returning the replacement and the number of bytes consumed.
func unescape(s string) (string, int, error) {
	if len(s) < 2 {
		return "", 0, fmt.Errorf("dangling escape at end of string")
	}
	switch s[1] {
	case 'b':
		return "\b", 2, nil
	case 't':
		return "\t", 2, nil
	case 'n':
		return "\n", 2, nil
	case 'f':
		return "\f", 2, nil
	case 'r':
		return "\r", 2, nil
	case '"':
		return `"`, 2, nil
	case '\\':
		return `\`, 2, nil
	case 'u', 'U':
		width := 4
		if s[1] == 'U' {
			width = 8
		}
		if len(s) < 2+width {
			return "", 0, fmt.Errorf("truncated \\%c escape", s[1])
		}
		code, err := strconv.ParseUint(s[2:2+width], 16, 32)
		if err != nil {
			return "", 0, fmt.Errorf("invalid \\%c escape: %s", s[1], s[2:2+width])
		}
		r := rune(code)
		if !utf8.ValidRune(r) {
			return "", 0, fmt.Errorf("invalid unicode code point U+%X", code)
		}
		return string(r), 2 + width, nil
	}
	return "", 0, fmt.Errorf("unknown escape \\%c", s[1])
}

// trailing rejects anything but whitespace and a comment after a value.
func trailing(s string) error {
	if rest := strings.TrimSpace(s); rest != "" && !strings.HasPrefix(rest, "#") {
		return fmt.Errorf("unexpected text after value: %s", rest)
	}
	return nil
}
