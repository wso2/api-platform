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
	"unicode"
	"unicode/utf8"
)

// This file implements the TOML subset the AI Workspace config uses, on the
// standard library alone. The full grammar is deliberately not supported: the
// config format is scalars in (possibly nested) tables — see flatten — so the
// parser accepts exactly that and fails closed on everything else. Rejecting an
// unsupported construct with a line-numbered error preserves integrity: a config
// can never be silently misread, only refused.
//
// Accepted per line (after comment/whitespace handling):
//   - [table] and [nested.table] headers with bare-key segments
//   - key = 'literal string'   (no escapes, as TOML defines)
//   - key = "basic string"     (TOML escape sequences)
//   - key = true | false | integer | float
//
// Rejected: arrays, inline tables, arrays of tables [[x]], multi-line strings,
// dotted or quoted keys, dates/times, and duplicate keys or table redefinitions.

// bareKeyValid reports whether s is a valid TOML bare key (A-Za-z0-9_-, non-empty).
func bareKeyValid(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

// tomlParser carries the document state: the root tree, the table the current
// header points at, and the set of explicitly declared headers and assigned keys
// (both used only for duplicate detection).
type tomlParser struct {
	root    map[string]any
	current map[string]any
	headers map[string]bool // explicitly declared [table] paths
	keys    map[string]bool // fully-qualified assigned keys
	prefix  string          // dotted path of the current table ("" at root)
}

// parseTOMLSubset parses data and returns the nested tree parseTOML used to get
// from go-toml: tables as map[string]any, scalars as string / bool / int64 / float64.
func parseTOMLSubset(data []byte) (map[string]any, error) {
	p := &tomlParser{
		root:    map[string]any{},
		headers: map[string]bool{},
		keys:    map[string]bool{},
	}
	p.current = p.root

	for n, line := range strings.Split(string(data), "\n") {
		if err := p.parseLine(strings.TrimSpace(strings.TrimSuffix(line, "\r"))); err != nil {
			return nil, fmt.Errorf("line %d: %w", n+1, err)
		}
	}
	return p.root, nil
}

func (p *tomlParser) parseLine(line string) error {
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}
	if strings.HasPrefix(line, "[") {
		return p.parseHeader(line)
	}
	return p.parseKeyValue(line)
}

// parseHeader handles [table] and [a.b] lines, creating intermediate tables.
func (p *tomlParser) parseHeader(line string) error {
	if strings.HasPrefix(line, "[[") {
		return fmt.Errorf("arrays of tables ([[...]]) are not supported")
	}
	end := strings.IndexByte(line, ']')
	if end < 0 {
		return fmt.Errorf("unterminated table header %q", line)
	}
	if rest := strings.TrimSpace(line[end+1:]); rest != "" && !strings.HasPrefix(rest, "#") {
		return fmt.Errorf("unexpected content after table header: %q", rest)
	}

	path := strings.TrimSpace(line[1:end])
	if p.headers[path] {
		return fmt.Errorf("table [%s] is defined twice", path)
	}
	table := p.root
	for _, seg := range strings.Split(path, ".") {
		seg = strings.TrimSpace(seg)
		if !bareKeyValid(seg) {
			return fmt.Errorf("invalid table name segment %q in [%s]", seg, path)
		}
		switch child := table[seg].(type) {
		case nil:
			next := map[string]any{}
			table[seg] = next
			table = next
		case map[string]any:
			table = child
		default:
			return fmt.Errorf("[%s] conflicts with key %q which already holds a value", path, seg)
		}
	}
	p.headers[path] = true
	p.current = table
	p.prefix = path
	return nil
}

// parseKeyValue handles a `key = value` line inside the current table.
func (p *tomlParser) parseKeyValue(line string) error {
	eq := strings.IndexByte(line, '=')
	if eq < 0 {
		return fmt.Errorf("expected key = value, got %q", line)
	}
	key := strings.TrimSpace(line[:eq])
	if !bareKeyValid(key) {
		return fmt.Errorf("invalid key %q (only bare keys are supported)", key)
	}

	qualified := key
	if p.prefix != "" {
		qualified = p.prefix + "." + key
	}
	if p.keys[qualified] {
		return fmt.Errorf("key %q is set twice", qualified)
	}
	if _, isTable := p.current[key].(map[string]any); isTable {
		return fmt.Errorf("key %q conflicts with table [%s]", key, qualified)
	}

	value, rest, err := parseValue(strings.TrimSpace(line[eq+1:]))
	if err != nil {
		return fmt.Errorf("value for key %q: %w", qualified, err)
	}
	if rest = strings.TrimSpace(rest); rest != "" && !strings.HasPrefix(rest, "#") {
		return fmt.Errorf("unexpected content after value for key %q: %q", qualified, rest)
	}

	p.keys[qualified] = true
	p.current[key] = value
	return nil
}

// parseValue parses one scalar at the start of s and returns it with the unparsed
// remainder (trailing comments are the caller's to validate).
func parseValue(s string) (any, string, error) {
	switch {
	case s == "":
		return nil, "", fmt.Errorf("missing value")
	case strings.HasPrefix(s, "'''"), strings.HasPrefix(s, `"""`):
		return nil, "", fmt.Errorf("multi-line strings are not supported")
	case s[0] == '\'':
		end := strings.IndexByte(s[1:], '\'')
		if end < 0 {
			return nil, "", fmt.Errorf("unterminated literal string")
		}
		return s[1 : 1+end], s[2+end:], nil
	case s[0] == '"':
		return parseBasicString(s)
	case s[0] == '[':
		return nil, "", fmt.Errorf("arrays are not supported")
	case s[0] == '{':
		return nil, "", fmt.Errorf("inline tables are not supported")
	}

	// Bare scalar: the token runs to the first whitespace or comment.
	token := s
	if i := strings.IndexAny(s, " \t#"); i >= 0 {
		token, s = s[:i], s[i:]
	} else {
		s = ""
	}
	switch token {
	case "true":
		return true, s, nil
	case "false":
		return false, s, nil
	}
	// TOML permits underscore separators between digits (1_000).
	numeric := strings.ReplaceAll(token, "_", "")
	if i, err := strconv.ParseInt(numeric, 10, 64); err == nil {
		return i, s, nil
	}
	if f, err := strconv.ParseFloat(numeric, 64); err == nil {
		return f, s, nil
	}
	return nil, "", fmt.Errorf("unsupported value %q (expected a string, boolean, or number)", token)
}

// parseBasicString decodes a double-quoted TOML basic string with its escape
// sequences, returning the value and the remainder after the closing quote.
func parseBasicString(s string) (string, string, error) {
	var b strings.Builder
	i := 1 // past the opening quote
	for i < len(s) {
		c := s[i]
		switch c {
		case '"':
			return b.String(), s[i+1:], nil
		case '\\':
			if i+1 >= len(s) {
				return "", "", fmt.Errorf("unterminated escape sequence")
			}
			i++
			switch e := s[i]; e {
			case 'b':
				b.WriteByte('\b')
			case 't':
				b.WriteByte('\t')
			case 'n':
				b.WriteByte('\n')
			case 'f':
				b.WriteByte('\f')
			case 'r':
				b.WriteByte('\r')
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			case 'u', 'U':
				width := 4
				if e == 'U' {
					width = 8
				}
				if i+width >= len(s) {
					return "", "", fmt.Errorf(`truncated \%c escape`, e)
				}
				code, err := strconv.ParseUint(s[i+1:i+1+width], 16, 32)
				if err != nil {
					return "", "", fmt.Errorf(`invalid \%c escape: %v`, e, err)
				}
				r := rune(code)
				if !utf8.ValidRune(r) || unicode.Is(unicode.Cs, r) {
					return "", "", fmt.Errorf(`\%c escape is not a valid Unicode scalar value`, e)
				}
				b.WriteRune(r)
				i += width
			default:
				return "", "", fmt.Errorf(`invalid escape sequence \%c`, e)
			}
		default:
			b.WriteByte(c)
		}
		i++
	}
	return "", "", fmt.Errorf("unterminated basic string")
}
