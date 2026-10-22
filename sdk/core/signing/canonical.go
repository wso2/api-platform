/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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

package signing

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// maxCanonicalDepth bounds nesting so a hostile document cannot exhaust the
// stack; real JSON documents that get signed are a handful of levels deep.
const maxCanonicalDepth = 64

// CanonicalJSON returns the RFC 8785 JSON Canonicalization Scheme (JCS) form
// of doc: object members sorted by UTF-16 code units, no insignificant
// whitespace, ECMAScript number formatting and minimal string escaping. Two
// semantically equal JSON documents canonicalize to the same bytes, which is
// what a detached signature over a JSON document needs.
//
// doc must be I-JSON (RFC 7493): valid UTF-8, no duplicate member names, and
// numbers representable as IEEE 754 doubles. Anything else is an error rather
// than a silently different canonical form.
func CanonicalJSON(doc []byte) ([]byte, error) {
	if !utf8.Valid(doc) {
		return nil, errors.New("canonical JSON: input is not valid UTF-8")
	}
	dec := json.NewDecoder(bytes.NewReader(doc))
	dec.UseNumber()

	var out bytes.Buffer
	if err := canonicalValue(dec, &out, 0); err != nil {
		return nil, fmt.Errorf("canonical JSON: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("canonical JSON: trailing data after document")
	}
	return out.Bytes(), nil
}

func canonicalValue(dec *json.Decoder, out *bytes.Buffer, depth int) error {
	if depth > maxCanonicalDepth {
		return errors.New("document nested too deeply")
	}
	tok, err := dec.Token()
	if err != nil {
		return err
	}

	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			return canonicalObject(dec, out, depth)
		case '[':
			return canonicalArray(dec, out, depth)
		}
		return fmt.Errorf("unexpected %q", t)
	case string:
		writeCanonicalString(out, t)
	case json.Number:
		s, err := canonicalNumber(t)
		if err != nil {
			return err
		}
		out.WriteString(s)
	case bool:
		out.WriteString(strconv.FormatBool(t))
	case nil:
		out.WriteString("null")
	}
	return nil
}

func canonicalObject(dec *json.Decoder, out *bytes.Buffer, depth int) error {
	type member struct {
		key   string
		value []byte
	}
	var members []member
	seen := map[string]bool{}

	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		key := tok.(string) // the decoder only yields string keys inside an object
		if seen[key] {
			return fmt.Errorf("duplicate member name %q", key)
		}
		seen[key] = true

		var value bytes.Buffer
		if err := canonicalValue(dec, &value, depth+1); err != nil {
			return err
		}
		members = append(members, member{key, value.Bytes()})
	}
	if _, err := dec.Token(); err != nil { // '}'
		return err
	}

	slices.SortFunc(members, func(a, b member) int {
		return slices.Compare(utf16.Encode([]rune(a.key)), utf16.Encode([]rune(b.key)))
	})

	out.WriteByte('{')
	for i, m := range members {
		if i > 0 {
			out.WriteByte(',')
		}
		writeCanonicalString(out, m.key)
		out.WriteByte(':')
		out.Write(m.value)
	}
	out.WriteByte('}')
	return nil
}

func canonicalArray(dec *json.Decoder, out *bytes.Buffer, depth int) error {
	out.WriteByte('[')
	for i := 0; dec.More(); i++ {
		if i > 0 {
			out.WriteByte(',')
		}
		if err := canonicalValue(dec, out, depth+1); err != nil {
			return err
		}
	}
	out.WriteByte(']')
	_, err := dec.Token() // ']'
	return err
}

// writeCanonicalString escapes as ECMAScript JSON.stringify does (RFC 8785
// §3.2.2.2): only '"', '\\' and control characters, the common ones in their
// short form and the rest as lowercase \u00xx. Everything else is literal.
func writeCanonicalString(out *bytes.Buffer, s string) {
	const hex = "0123456789abcdef"
	out.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if c < 0x20 {
				out.WriteString(`\u00`)
				out.WriteByte(hex[c>>4])
				out.WriteByte(hex[c&0xf])
			} else {
				out.WriteByte(c)
			}
		}
	}
	out.WriteByte('"')
}

// canonicalNumber formats n as ECMAScript Number.prototype.toString does for
// the nearest IEEE 754 double (RFC 8785 §3.2.2.3).
func canonicalNumber(n json.Number) (string, error) {
	f, err := strconv.ParseFloat(string(n), 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return "", fmt.Errorf("number %s is not representable as an IEEE 754 double", n)
	}
	if f == 0 {
		return "0", nil // also -0
	}

	sign := ""
	if f < 0 {
		sign, f = "-", -f
	}

	// Shortest round-tripping digits d1..dk and exponent e, with value
	// 0.d1..dk × 10^e — i.e. ECMAScript's k digits and n = e.
	sci := strconv.FormatFloat(f, 'e', -1, 64) // "d.ddde±xx"
	mantissa, expStr, _ := strings.Cut(sci, "e")
	digits := strings.Replace(mantissa, ".", "", 1)
	exp, _ := strconv.Atoi(expStr)
	k, pos := len(digits), exp+1

	switch {
	case k <= pos && pos <= 21:
		return sign + digits + strings.Repeat("0", pos-k), nil
	case 0 < pos && pos <= 21:
		return sign + digits[:pos] + "." + digits[pos:], nil
	case -6 < pos && pos <= 0:
		return sign + "0." + strings.Repeat("0", -pos) + digits, nil
	}

	e := pos - 1
	expSign := "+"
	if e < 0 {
		expSign, e = "-", -e
	}
	m := digits[:1]
	if k > 1 {
		m += "." + digits[1:]
	}
	return sign + m + "e" + expSign + strconv.Itoa(e), nil
}
