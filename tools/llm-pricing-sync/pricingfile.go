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
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// pricingFile is model_prices.json held so that writing it back reproduces
// every untouched entry byte for byte, in its original order.
type pricingFile struct {
	keys            []string
	entries         map[string]json.RawMessage
	trailingNewline bool
}

func parsePricingFile(data []byte) (*pricingFile, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, fmt.Errorf("pricing file must be a JSON object: %v", err)
	}
	f := &pricingFile{entries: map[string]json.RawMessage{}, trailingNewline: bytes.HasSuffix(data, []byte("\n"))}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("reading pricing key: %w", err)
		}
		key, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected token %v where a pricing key belongs", tok)
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, fmt.Errorf("reading pricing entry %q: %w", key, err)
		}
		if _, dup := f.entries[key]; dup {
			return nil, fmt.Errorf("duplicate pricing key %q", key)
		}
		f.keys = append(f.keys, key)
		f.entries[key] = raw
	}
	return f, nil
}

// bytes writes the file in the layout it was read in: two-space indented
// top level, entries verbatim.
func (f *pricingFile) bytes() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("{\n")
	for i, key := range f.keys {
		name, err := json.Marshal(key)
		if err != nil {
			return nil, fmt.Errorf("encoding key %q: %w", key, err)
		}
		buf.WriteString("  ")
		buf.Write(name)
		buf.WriteString(": ")
		buf.Write(f.entries[key])
		if i < len(f.keys)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("}")
	if f.trailingNewline {
		buf.WriteString("\n")
	}
	return buf.Bytes(), nil
}

// keyFold returns the existing spelling of key, compared case-insensitively
// as azure-llm-cost does, so an update never adds a second entry for a model.
func (f *pricingFile) keyFold(key string) (string, bool) {
	for _, k := range f.keys {
		if strings.EqualFold(k, key) {
			return k, true
		}
	}
	return "", false
}

// insert adds a new key in sorted position when the file is sorted, else last.
func (f *pricingFile) insert(key string, raw json.RawMessage) {
	f.entries[key] = raw
	if !sort.StringsAreSorted(f.keys) {
		f.keys = append(f.keys, key)
		return
	}
	i := sort.SearchStrings(f.keys, key)
	f.keys = append(f.keys, "")
	copy(f.keys[i+1:], f.keys[i:])
	f.keys[i] = key
}

func (f *pricingFile) remove(key string) {
	delete(f.entries, key)
	for i, k := range f.keys {
		if k == key {
			f.keys = append(f.keys[:i], f.keys[i+1:]...)
			return
		}
	}
}

// formatRate writes a rate the way the pricing file already spells them
// (Python float repr: 4e-06, 0.00025, 1.5).
func formatRate(v float64) string {
	s := strconv.FormatFloat(v, 'g', -1, 64)
	if !strings.ContainsAny(s, ".e") {
		s += ".0"
	}
	return s
}

// encodeEntry writes an entry's fields sorted by name, indented as the file
// indents entries, re-indenting nested objects to match.
func encodeEntry(fields map[string]json.RawMessage) (json.RawMessage, error) {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)

	var buf bytes.Buffer
	buf.WriteString("{\n")
	for i, name := range names {
		quoted, err := json.Marshal(name)
		if err != nil {
			return nil, fmt.Errorf("encoding field name %q: %w", name, err)
		}
		var value bytes.Buffer
		if err := json.Indent(&value, fields[name], "    ", "  "); err != nil {
			return nil, fmt.Errorf("re-indenting field %q: %w", name, err)
		}
		buf.WriteString("    ")
		buf.Write(quoted)
		buf.WriteString(": ")
		buf.Write(value.Bytes())
		if i < len(names)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("  }")
	return buf.Bytes(), nil
}
