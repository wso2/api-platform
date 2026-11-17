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

package featureusage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// Binding ties one REST API operation, named by its operationId in
// resources/openapi.yaml, to the taxonomy feature every call to it is an
// occurrence of, and says where each of the feature's dimensions comes from.
// The bindings live in bindings.go; this file is the engine that reads them.
//
// outcome is never bound: it always comes from the response status.
type Binding struct {
	// Op is the OpenAPI operationId, e.g. "UpdateRESTAPI".
	Op string
	// Feature is the taxonomy feature code, e.g. "apip.manage-apis.api-update".
	Feature string
	// Set holds dimensions whose value is fixed for this operation.
	Set Dims
	// From holds dimensions whose value is read from the request.
	From Sources
}

// Dims maps a dimension name to a fixed value: a string for an enum or string
// dimension, a bool for a boolean one.
type Dims map[string]any

// Sources maps a dimension name to where its value is read from.
type Sources map[string]Source

// platformAPIEmitter is how the taxonomy marks a feature Platform API emits.
// A binding may only name such a feature, and every such feature must be bound
// or listed in notBound, so a feature another producer emits (the console, the
// gateway, the CLI) is never double-counted from here.
const platformAPIEmitter = "Emitted by Platform API."

type sourceKind int

const (
	srcPath sourceKind = iota + 1
	srcForm
	srcFormJSON
	srcJSON
	srcPresent
)

func (k sourceKind) String() string {
	switch k {
	case srcPath:
		return "Path"
	case srcForm:
		return "Form"
	case srcFormJSON:
		return "FormJSON"
	case srcJSON:
		return "JSON"
	case srcPresent:
		return "Present"
	}
	return "unknown"
}

// Source says where one dimension's value is read from. Build it with Path,
// Form, FormJSON, JSON or Present, then optionally refine it with Map and
// Default. It is a closed set on purpose: a source can read only the
// matched route's path, a form the handler already parsed, or the JSON body,
// never a header, a token or the caller's identity.
//
// Values are read after the handler has returned, from what it read. A value
// is sent only if it is a member of the dimension's enum (or a bool, for a
// boolean dimension); anything else is left out, so free text never leaves
// the process.
type Source struct {
	kind    sourceKind
	name    string   // path parameter or form field
	pointer string   // JSON Pointer (RFC 6901) into a JSON document
	fields  []string // Present: form fields, in the order they are checked

	mapping map[string]string
	def     any
	hasDef  bool
}

// Path reads a path parameter of the matched route, e.g. Path("apiType") for
// /apis/{apiType}/{apiId}/thumbnail.
func Path(param string) Source { return Source{kind: srcPath, name: param} }

// Form reads a field of a multipart or urlencoded form the handler parsed.
// Surrounding white space is trimmed; an empty field counts as absent.
func Form(field string) Source { return Source{kind: srcForm, name: field} }

// FormJSON reads a form field holding a JSON document, and the value at
// pointer within it, e.g. FormJSON("metadata", "/schemaSource").
func FormJSON(field, pointer string) Source {
	return Source{kind: srcFormJSON, name: field, pointer: pointer}
}

// JSON reads the value at pointer in a JSON request body, e.g.
// JSON("/functionalityType"). Only the bytes the handler read are seen, up to
// maxCapturedBody; a larger body leaves the value unknown.
func JSON(pointer string) Source { return Source{kind: srcJSON, pointer: pointer} }

// Present yields the name of the first of fields that the parsed form carries
// (a non-empty value or an uploaded file), never what the field contains. Use
// it where the content is free text, such as a URL.
func Present(fields ...string) Source {
	return Source{kind: srcPresent, fields: append([]string(nil), fields...)}
}

// Map translates the value read into the taxonomy's spelling, e.g. "HowTo" to
// "how-to". Keys match regardless of case, as the API's own parsing of such
// values does ("howto" is a HowTo document). A value not in m is left out.
func (s Source) Map(m map[string]string) Source {
	s.mapping = m
	return s
}

// Default is the value used when the request was read but does not carry the
// field: the value the API itself assumes when the field is omitted. It is not
// used when the request could not be read at all (for example a call rejected
// before its body was parsed).
func (s Source) Default(v any) Source {
	s.def, s.hasDef = v, true
	return s
}

// readState says what reading a source found.
type readState int

const (
	// unavailable: what the source reads was never parsed, or could not be.
	unavailable readState = iota
	// absent: the request was read, but does not carry the field.
	absent
	// found: the request carries a value.
	found
)

// requestView is what a source may read once the handler has returned.
type requestView struct {
	pathValues map[string]string
	form       *formView // nil when the handler parsed no form
	body       []byte    // the JSON body the handler read; nil when not captured or incomplete
}

type formView struct {
	values map[string][]string
	files  map[string][]*multipart.FileHeader
}

// maxFormJSONBytes bounds a form field parsed as JSON by FormJSON. The handler
// has already bounded and read the form; this only keeps an outsized field
// from being parsed a second time.
const maxFormJSONBytes = 8 << 20

// resolve returns the dimension value s yields for v, or false when it yields
// none.
func (s Source) resolve(v *requestView) (any, bool) {
	raw, state := s.read(v)
	switch state {
	case unavailable:
		return nil, false
	case absent:
		if s.hasDef {
			return s.def, true
		}
		return nil, false
	}
	if s.mapping == nil {
		return raw, true
	}
	if str, ok := raw.(string); ok {
		if mapped, ok := s.mapping[str]; ok {
			return mapped, true
		}
		for key, mapped := range s.mapping {
			if strings.EqualFold(key, str) {
				return mapped, true
			}
		}
	}
	return nil, false
}

func (s Source) read(v *requestView) (any, readState) {
	switch s.kind {
	case srcPath:
		if val := v.pathValues[s.name]; val != "" {
			return val, found
		}
		return nil, unavailable
	case srcForm:
		if v.form == nil {
			return nil, unavailable
		}
		if val := firstValue(v.form.values[s.name]); val != "" {
			return val, found
		}
		return nil, absent
	case srcFormJSON:
		if v.form == nil {
			return nil, unavailable
		}
		doc := firstValue(v.form.values[s.name])
		if doc == "" || len(doc) > maxFormJSONBytes {
			return nil, unavailable
		}
		return jsonPointer([]byte(doc), s.pointer)
	case srcJSON:
		if v.body == nil {
			return nil, unavailable
		}
		return jsonPointer(v.body, s.pointer)
	case srcPresent:
		if v.form == nil {
			return nil, unavailable
		}
		for _, f := range s.fields {
			if len(v.form.files[f]) > 0 || firstValue(v.form.values[f]) != "" {
				return f, found
			}
		}
		return nil, absent
	}
	return nil, unavailable
}

func firstValue(vals []string) string {
	if len(vals) == 0 {
		return ""
	}
	return strings.TrimSpace(vals[0])
}

// jsonPointer returns the string or boolean at pointer in doc. A document that
// is not valid JSON is unavailable; a missing or null member is absent. Any
// other JSON type is returned as found and then fails the dimension check.
func jsonPointer(doc []byte, pointer string) (any, readState) {
	cur := json.RawMessage(bytes.TrimSpace(doc))
	if !json.Valid(cur) {
		return nil, unavailable
	}
	if pointer != "" {
		for _, token := range strings.Split(pointer[1:], "/") {
			token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
			var obj map[string]json.RawMessage
			if err := json.Unmarshal(cur, &obj); err == nil {
				next, ok := obj[token]
				if !ok {
					return nil, absent
				}
				cur = next
				continue
			}
			var arr []json.RawMessage
			if err := json.Unmarshal(cur, &arr); err == nil {
				i, err := strconv.Atoi(token)
				if err != nil || i < 0 || i >= len(arr) {
					return nil, absent
				}
				cur = arr[i]
				continue
			}
			return nil, absent // a scalar has no members
		}
	}
	var val any
	if err := json.Unmarshal(cur, &val); err != nil {
		return nil, unavailable
	}
	if val == nil {
		return nil, absent
	}
	if str, ok := val.(string); ok {
		return strings.TrimSpace(str), found
	}
	return val, found
}

// accepts reports whether value may be sent for the dimension: an enum member,
// a bool for a boolean dimension, or a code for a string dimension.
func (d Dimension) accepts(value any) bool {
	switch d.Type {
	case DimensionEnum:
		s, ok := value.(string)
		return ok && d.Values[s]
	case DimensionBoolean:
		_, ok := value.(bool)
		return ok
	case DimensionString:
		s, ok := value.(string)
		return ok && stringDimensionPattern.MatchString(s)
	}
	return false
}

// check verifies s can only ever yield values dim accepts from what it maps
// or defaults to, and that it is well formed. A raw value read
// without Map is checked against the enum at runtime.
func (s Source) check(dim Dimension) error {
	switch s.kind {
	case srcPath, srcForm:
		if s.name == "" {
			return fmt.Errorf("%s source names no parameter", s.kind)
		}
	case srcFormJSON:
		if s.name == "" {
			return errors.New("FormJSON source names no form field")
		}
		if !validPointer(s.pointer) {
			return fmt.Errorf("FormJSON source has an invalid JSON Pointer %q", s.pointer)
		}
	case srcJSON:
		if !validPointer(s.pointer) {
			return fmt.Errorf("JSON source has an invalid JSON Pointer %q", s.pointer)
		}
	case srcPresent:
		if len(s.fields) == 0 {
			return errors.New("Present source lists no fields")
		}
	default:
		return errors.New("source was not built with Path, Form, FormJSON, JSON or Present")
	}

	// Only an enum or a boolean is ever taken from the request: a string
	// dimension carries a code, and a request is not a trusted source of codes.
	switch dim.Type {
	case DimensionEnum:
	case DimensionBoolean:
		if s.kind != srcJSON && s.kind != srcFormJSON {
			return fmt.Errorf("boolean dimension can only be read from JSON, not from %s", s.kind)
		}
		if s.mapping != nil {
			return errors.New("boolean dimension cannot use Map")
		}
	default:
		return fmt.Errorf("a %s dimension cannot be read from the request; only enum and boolean dimensions can", dim.Type)
	}

	for raw, mapped := range s.mapping {
		if !dim.accepts(mapped) {
			return fmt.Errorf("Map sends %q for %q, which is not in the dimension's enum", mapped, raw)
		}
	}
	if s.hasDef && !dim.accepts(s.def) {
		return fmt.Errorf("Default %v is not a value the dimension accepts", s.def)
	}
	return nil
}

func validPointer(p string) bool { return p == "" || strings.HasPrefix(p, "/") }

// checkBindings verifies every binding against the taxonomy: a known feature
// Platform API emits and tracks from the API, declared dimensions only, fixed,
// mapped and default values its enums accept, a source for every required
// dimension, and each operation bound at most once.
func checkBindings(tax *Taxonomy, bs []Binding) error {
	seen := map[string]bool{}
	for _, b := range bs {
		fail := func(format string, args ...any) error {
			return fmt.Errorf("feature usage: binding for operation %q: %s", b.Op, fmt.Sprintf(format, args...))
		}
		if b.Op == "" {
			return fmt.Errorf("feature usage: a binding for %q names no operation", b.Feature)
		}
		if seen[b.Op] {
			return fail("the operation is bound more than once")
		}
		seen[b.Op] = true

		f, ok := tax.Feature(b.Feature)
		if !ok {
			return fail("feature %q is not in the taxonomy", b.Feature)
		}
		if f.EventName != f.Code {
			return fail("feature %q has event_name %q; expected it to equal the code", f.Code, f.EventName)
		}
		if !f.Platforms[PlatformAPI] {
			return fail("feature %q is not tracked from the API in the taxonomy", f.Code)
		}
		if !strings.Contains(f.Description, platformAPIEmitter) {
			return fail("feature %q is not marked %q in the taxonomy", f.Code, platformAPIEmitter)
		}

		for _, name := range sortedKeys(b.Set) {
			value := b.Set[name]
			dim, declared := f.Dimensions[name]
			switch {
			case !declared:
				return fail("Set dimension %q is not declared by %s", name, f.Code)
			case name == dimOutcome:
				return fail("outcome comes from the response status and cannot be bound")
			case !dim.accepts(value):
				return fail("Set dimension %q has a value the taxonomy does not accept", name)
			}
		}
		for _, name := range sortedKeys(b.From) {
			src := b.From[name]
			dim, declared := f.Dimensions[name]
			switch {
			case !declared:
				return fail("dimension %q is not declared by %s", name, f.Code)
			case name == dimOutcome:
				return fail("outcome comes from the response status and cannot be bound")
			}
			if _, both := b.Set[name]; both {
				return fail("dimension %q is both Set and read From the request", name)
			}
			if err := src.check(dim); err != nil {
				return fail("dimension %q: %s", name, err.Error())
			}
		}
		for _, name := range sortedKeys(f.Dimensions) {
			dim := f.Dimensions[name]
			_, set := b.Set[name]
			_, from := b.From[name]
			if dim.Required && name != dimOutcome && !set && !from {
				return fail("required dimension %q of %s has no value: add it to Set or From", name, f.Code)
			}
		}
	}
	return nil
}

// enabledFeatures is the set of features this process reports: exactly the
// ones bindings.go binds. A code missing from it is refused even if the
// taxonomy has it, so reporting a new feature is a reviewed binding, not a
// side effect of a taxonomy update.
var enabledFeatures = boundFeatures(bindings)

// boundFeatures returns the set of feature codes the bindings report.
func boundFeatures(bs []Binding) map[string]bool {
	out := make(map[string]bool, len(bs))
	for _, b := range bs {
		out[b.Feature] = true
	}
	return out
}

// pathParams returns the names of the {param} segments of a route path, in
// order. A "{name...}" wildcard is returned with its dots.
func pathParams(path string) []string {
	var names []string
	for _, seg := range strings.Split(path, "/") {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") && seg != "{$}" {
			names = append(names, seg[1:len(seg)-1])
		}
	}
	return names
}

// pathParamsFor extracts the {param} values of path, the escaped request path,
// against the route pattern path. It returns nil when the two do not line up
// segment for segment.
func pathParamsFor(pattern, path string) map[string]string {
	ps := strings.Split(pattern, "/")
	rs := strings.Split(path, "/")
	if len(ps) != len(rs) {
		return nil
	}
	out := map[string]string{}
	for i, seg := range ps {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			v, err := url.PathUnescape(rs[i])
			if err != nil {
				return nil
			}
			out[seg[1:len(seg)-1]] = v
		}
	}
	return out
}

// sortedKeys returns the keys of m in order, for deterministic errors and logs.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
