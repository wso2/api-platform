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

package publishers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"cel.dev/cel-go/cel"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
)

// inlineTemplateSetting and fileTemplateSetting name the two template settings in startup
// messages.
const (
	inlineTemplateSetting = "traffic_logging.template"
	fileTemplateSetting   = "traffic_logging.template_file"
)

// maxTemplateDepth is the deepest nesting of objects and arrays a template may use.
const maxTemplateDepth = 64

// nullJSON is written for a literal null, an expression that does not compile, and an
// expression with no value for a request.
var nullJSON = []byte("null")

// capturedVariables lists, in message order, the variables whose data exists only when the
// collector captures it, with the setting that switches capture on and the data it carries.
var capturedVariables = []struct {
	variable string
	setting  string
	data     string
	captured func(*config.CollectorConfig) bool
}{
	{"request.header", "collector.request_headers", "request headers", func(c *config.CollectorConfig) bool { return c.RequestHeaders }},
	{"response.header", "collector.response_headers", "response headers", func(c *config.CollectorConfig) bool { return c.ResponseHeaders }},
	{"request.body", "collector.request_body", "request body", func(c *config.CollectorConfig) bool { return c.RequestBody }},
	{"response.body", "collector.response_body", "response body", func(c *config.CollectorConfig) bool { return c.ResponseBody }},
}

// templateNodeKind says which kind of JSON value a template node holds.
type templateNodeKind int

const (
	objectNode templateNodeKind = iota
	arrayNode
	literalNode
	expressionNode
)

// templateNode is one value of a parsed traffic log template.
type templateNode struct {
	kind templateNodeKind
	// path names the node in startup messages, for example http.request.method or tags[1].
	path string
	// members are an object's keys and values, in template order.
	members []templateMember
	// elements are an array's values, in template order.
	elements []*templateNode
	// literal is the JSON a literal node writes, encoded once at startup.
	literal []byte
	// expression is the text after "$ctx:" of an expression node; program is its compiled form.
	expression string
	program    cel.Program
	// excludePaths are the exclude_fields paths that go on inside an expression's value, split
	// on ".". They are removed from each request's value before it is written.
	excludePaths [][]string
	// headerVariable is request.header or response.header when the expression is exactly that
	// variable and exclude_fields names headers inside it; excludedHeaders holds those header
	// names, lower-cased.
	headerVariable  string
	excludedHeaders map[string]bool
	// hasExclusions is set on an expression that exclude_fields filters per request and on every
	// object above one, so all other nodes are written without an exclusion check.
	hasExclusions bool
}

// templateMember is one key of a template object and its value.
type templateMember struct {
	key string
	// encodedKey is the key as JSON followed by a colon, encoded once at startup.
	encodedKey []byte
	value      *templateNode
}

// logTemplate writes traffic log lines in the layout of the operator's template. It is
// read-only once built, so concurrent Publish calls can share it.
type logTemplate struct {
	// source is the setting the template came from, used in messages.
	source string
	root   *templateNode
	// referencedVars is the union of the variables the template's expressions read, so the
	// evaluation context only fills those.
	referencedVars map[string]bool
	// maskedHeaders are redacted in request.header and response.header, as in the default line.
	maskedHeaders map[string]bool
	// maxPayloadSize cuts request.body and response.body, as in the default line.
	maxPayloadSize int
}

// newLogTemplate parses and compiles the template in logCfg. It returns an error only for a
// template that cannot be used at all; an expression that does not compile is written as null.
// collectorCfg says which headers and bodies the collector captures.
func newLogTemplate(logCfg *config.TrafficLoggingConfig, collectorCfg *config.CollectorConfig, maskedHeaders map[string]bool) (*logTemplate, error) {
	if logCfg.Template != "" && logCfg.TemplateFile != "" {
		return nil, fmt.Errorf("%s, %s: both are set; set only one", inlineTemplateSetting, fileTemplateSetting)
	}
	source := inlineTemplateSetting
	root, err := parseTemplate(source, logCfg.Template)
	if err != nil {
		return nil, err
	}
	// Excluded fields are removed before compiling, so they are never compiled or reported.
	for _, entry := range logCfg.ExcludeFields {
		excludeTemplatePath(root, strings.Split(entry, "."))
	}
	env, err := createGlobalPropertyEnv()
	if err != nil {
		return nil, fmt.Errorf("%s: cannot create the expression environment: %w", source, err)
	}

	t := &logTemplate{
		source:         source,
		root:           root,
		referencedVars: make(map[string]bool),
		maskedHeaders:  maskedHeaders,
		maxPayloadSize: logCfg.MaxPayloadSize,
	}
	t.compile(env, collectorCfg, root)
	markExclusions(root)
	return t, nil
}

// excludeTemplatePath applies one exclude_fields path to an object node. A path that ends at a
// key removes it; one that goes on inside an expression's value is kept on that node for
// each request; any other path has no effect. It reports whether the removal emptied node.
func excludeTemplatePath(node *templateNode, path []string) bool {
	i := -1
	_, rest, ok := splitLongestKey(path, func(k string) bool {
		i = slices.IndexFunc(node.members, func(m templateMember) bool { return m.key == k })
		return i >= 0
	})
	if !ok {
		return false
	}
	child := node.members[i].value
	switch {
	case len(rest) == 0: // the path ends at this key, removed below
	case child.kind == objectNode:
		if !excludeTemplatePath(child, rest) {
			return false
		}
	case child.kind == expressionNode:
		if variable := strings.TrimSpace(child.expression); variable == "request.header" || variable == "response.header" {
			// Header values are text, so the rest of the path is one header name.
			if child.excludedHeaders == nil {
				child.excludedHeaders = make(map[string]bool)
			}
			child.headerVariable = variable
			child.excludedHeaders[strings.ToLower(strings.Join(rest, "."))] = true
		} else {
			child.excludePaths = append(child.excludePaths, rest)
		}
		return false
	default: // a literal or an array
		return false
	}
	node.members = slices.Delete(node.members, i, i+1)
	return len(node.members) == 0
}

// splitLongestKey returns the longest run of path's leading segments, joined with ".", that
// has accepts, and the segments after it, so a key that contains dots can be named.
func splitLongestKey(path []string, has func(string) bool) (key string, rest []string, ok bool) {
	for n := len(path); n > 0; n-- {
		if key := strings.Join(path[:n], "."); has(key) {
			return key, path[n:], true
		}
	}
	return "", nil, false
}

// markExclusions sets hasExclusions under node and reports whether node has it. Exclusion
// paths never reach into arrays, so arrays are not visited.
func markExclusions(node *templateNode) bool {
	switch node.kind {
	case objectNode:
		for _, m := range node.members {
			if markExclusions(m.value) {
				node.hasExclusions = true
			}
		}
	case expressionNode:
		node.hasExclusions = len(node.excludePaths) > 0 || len(node.excludedHeaders) > 0
	}
	return node.hasExclusions
}

// compile compiles every expression under node. An expression that does not compile becomes
// a fixed null, reported once with the field and the reason.
func (t *logTemplate) compile(env *cel.Env, collectorCfg *config.CollectorConfig, node *templateNode) {
	switch node.kind {
	case objectNode:
		for _, m := range node.members {
			t.compile(env, collectorCfg, m.value)
		}
	case arrayNode:
		for _, e := range node.elements {
			t.compile(env, collectorCfg, e)
		}
	case expressionNode:
		ast, program, err := compileExpression(env, node.expression)
		if err != nil {
			slog.Warn(fmt.Sprintf("%s: field %q is written as null: %v", t.source, node.path, err))
			*node = templateNode{kind: literalNode, path: node.path, literal: nullJSON}
			return
		}
		node.program = program
		reads := referencedGlobalPropertyVariables(ast)
		for name := range reads {
			t.referencedVars[name] = true
		}
		t.warnIfUncaptured(node.path, reads, collectorCfg)
	}
}

// warnIfUncaptured logs one WARNING for a field that reads data the collector does not capture,
// listing every reason. The field is kept.
func (t *logTemplate) warnIfUncaptured(path string, reads map[string]bool, collectorCfg *config.CollectorConfig) {
	var reasons []string
	for _, v := range capturedVariables {
		if reads[v.variable] && !v.captured(collectorCfg) {
			reasons = append(reasons, fmt.Sprintf("reads %s but %s is false; no %s will be logged", v.variable, v.setting, v.data))
		}
	}
	if len(reasons) > 0 {
		slog.Warn(fmt.Sprintf("%s: field %q %s", t.source, path, strings.Join(reasons, "; ")))
	}
}

// compileExpression compiles one "$ctx:" expression. The error joins the compiler's messages
// on one line.
func compileExpression(env *cel.Env, expression string) (*cel.Ast, cel.Program, error) {
	ast, issues := env.Compile(expression)
	if issues.Err() != nil {
		messages := make([]string, 0, len(issues.Errors()))
		for _, e := range issues.Errors() {
			messages = append(messages, e.Message)
		}
		return nil, nil, errors.New(strings.Join(messages, "; "))
	}
	program, err := env.Program(ast)
	if err != nil {
		return nil, nil, err
	}
	return ast, program, nil
}

// render returns one line for event in the template's layout.
func (t *logTemplate) render(event *dto.Event) []byte {
	evalCtx := buildGlobalPropertyEvalCtx(event, t.referencedVars, t.maskedHeaders, t.maxPayloadSize)
	var buf bytes.Buffer
	if !t.write(&buf, t.root, evalCtx) {
		buf.WriteString("{}") // exclude_fields emptied every field for this request
	}
	return buf.Bytes()
}

// write appends node's JSON to buf. An expression with no value for this request is written
// as null, so every line has the same fields and array positions are kept. It reports false,
// with nothing to keep, when exclude_fields emptied the node for this request.
func (t *logTemplate) write(buf *bytes.Buffer, node *templateNode, evalCtx map[string]interface{}) bool {
	switch node.kind {
	case objectNode:
		if node.hasExclusions {
			return t.writeFilteredObject(buf, node, evalCtx)
		}
		buf.WriteByte('{')
		for i, m := range node.members {
			if i > 0 {
				buf.WriteByte(',')
			}
			buf.Write(m.encodedKey)
			t.write(buf, m.value, evalCtx)
		}
		buf.WriteByte('}')
	case arrayNode:
		buf.WriteByte('[')
		for i, e := range node.elements {
			if i > 0 {
				buf.WriteByte(',')
			}
			t.write(buf, e, evalCtx)
		}
		buf.WriteByte(']')
	case literalNode:
		buf.Write(node.literal)
	case expressionNode:
		value, emptied, err := node.evaluate(evalCtx)
		if emptied {
			return false
		}
		if err != nil {
			slog.Debug("Traffic log template field has no value for this request; writing null",
				"source", t.source, "field", node.path, "error", err)
			value = nullJSON
		}
		buf.Write(value)
	}
	return true
}

// writeFilteredObject writes an object some of whose members exclude_fields may empty for this
// request, leaving those members out. It reports false when every member was left out.
func (t *logTemplate) writeFilteredObject(buf *bytes.Buffer, node *templateNode, evalCtx map[string]interface{}) bool {
	start := buf.Len()
	buf.WriteByte('{')
	written := 0
	for _, m := range node.members {
		mark := buf.Len()
		if written > 0 {
			buf.WriteByte(',')
		}
		buf.Write(m.encodedKey)
		if !t.write(buf, m.value, evalCtx) {
			buf.Truncate(mark)
			continue
		}
		written++
	}
	buf.WriteByte('}')
	if written == 0 {
		buf.Truncate(start)
		return false
	}
	return true
}

// evaluate returns the JSON of an expression node's result for one request, after removing the
// keys exclude_fields names inside it. emptied is true when that removal left nothing to write.
func (n *templateNode) evaluate(evalCtx map[string]interface{}) (value []byte, emptied bool, err error) {
	if n.headerVariable != "" {
		// The masked header map in evalCtx is shared, so it is filtered into a copy.
		headers, _ := evalCtx[n.headerVariable].(map[string]string)
		if len(headers) == 0 {
			value, err = json.Marshal(headers)
			return value, false, err
		}
		kept := filterAndMaskHeaders(headers, nil, n.excludedHeaders)
		if kept == nil {
			return nil, true, nil
		}
		value, err = json.Marshal(kept)
		return value, false, err
	}
	out, _, err := n.program.Eval(evalCtx)
	if err != nil {
		return nil, false, err
	}
	result, err := globalPropertyCELToGoValue(out)
	if err != nil {
		return nil, false, err
	}
	// result is a fresh copy, so removing keys from it never changes the event.
	for _, path := range n.excludePaths {
		if removeValuePath(result, path) {
			return nil, true, nil
		}
	}
	value, err = json.Marshal(result)
	return value, false, err
}

// removeValuePath removes the key at path inside value, matching keys exactly and using the
// longest run of segments that names a key at each level. A map the removal empties is removed
// too, as in the default line. It reports whether the removal emptied value itself; a value
// that is not a map, or was already empty, is left as it is.
func removeValuePath(value interface{}, path []string) bool {
	m, ok := value.(map[string]interface{})
	if !ok || len(m) == 0 {
		return false
	}
	var child interface{}
	key, rest, ok := splitLongestKey(path, func(k string) bool {
		var found bool
		child, found = m[k]
		return found
	})
	if !ok {
		return false
	}
	if len(rest) > 0 && !removeValuePath(child, rest) {
		return false
	}
	delete(m, key)
	return len(m) == 0
}

// templateParser reads a template's JSON tokens in order, so the parsed tree keeps the
// template's key order.
type templateParser struct {
	source string
	text   string
	dec    *json.Decoder
}

// parseTemplate parses the template text. The error is the startup message for a template
// that cannot be used.
func parseTemplate(source, text string) (*templateNode, error) {
	p := &templateParser{source: source, text: text, dec: json.NewDecoder(strings.NewReader(text))}
	p.dec.UseNumber()

	tok, err := p.token()
	if err != nil {
		return nil, err
	}
	if tok != json.Delim('{') {
		return nil, fmt.Errorf("%s: the template must be a JSON object, found %s", source, jsonKind(tok))
	}
	root, err := p.parseObject("", 1)
	if err != nil {
		return nil, err
	}
	if err := p.checkNothingFollows(); err != nil {
		return nil, err
	}
	return root, nil
}

// parseObject reads the members of an object whose opening brace was just read.
func (p *templateParser) parseObject(path string, depth int) (*templateNode, error) {
	if depth > maxTemplateDepth {
		return nil, fmt.Errorf("%s: nesting deeper than %d levels at %q", p.source, maxTemplateDepth, path)
	}
	node := &templateNode{kind: objectNode, path: path}
	seen := make(map[string]bool)
	for p.dec.More() {
		tok, err := p.token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, p.invalidJSON()
		}
		if seen[key] {
			return nil, fmt.Errorf("%s: key %q appears more than once %s", p.source, key, objectLocation(path))
		}
		seen[key] = true

		encodedKey, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		value, err := p.parseValue(memberPath(path, key), depth)
		if err != nil {
			return nil, err
		}
		node.members = append(node.members, templateMember{key: key, encodedKey: append(encodedKey, ':'), value: value})
	}
	if _, err := p.token(); err != nil { // the closing brace
		return nil, err
	}
	return node, nil
}

// parseArray reads the elements of an array whose opening bracket was just read.
func (p *templateParser) parseArray(path string, depth int) (*templateNode, error) {
	if depth > maxTemplateDepth {
		return nil, fmt.Errorf("%s: nesting deeper than %d levels at %q", p.source, maxTemplateDepth, path)
	}
	node := &templateNode{kind: arrayNode, path: path}
	for p.dec.More() {
		value, err := p.parseValue(fmt.Sprintf("%s[%d]", path, len(node.elements)), depth)
		if err != nil {
			return nil, err
		}
		node.elements = append(node.elements, value)
	}
	if _, err := p.token(); err != nil { // the closing bracket
		return nil, err
	}
	return node, nil
}

// parseValue reads one value inside an object or array at depth.
func (p *templateParser) parseValue(path string, depth int) (*templateNode, error) {
	tok, err := p.token()
	if err != nil {
		return nil, err
	}
	switch v := tok.(type) {
	case json.Delim:
		if v == '{' {
			return p.parseObject(path, depth+1)
		}
		return p.parseArray(path, depth+1)
	case string:
		if expression, ok := strings.CutPrefix(v, globalPropertyCtxPrefix); ok {
			return &templateNode{kind: expressionNode, path: path, expression: expression}, nil
		}
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return &templateNode{kind: literalNode, path: path, literal: encoded}, nil
	case json.Number:
		return &templateNode{kind: literalNode, path: path, literal: []byte(v)}, nil
	case bool:
		return &templateNode{kind: literalNode, path: path, literal: []byte(strconv.FormatBool(v))}, nil
	default: // nil, the JSON null
		return &templateNode{kind: literalNode, path: path, literal: nullJSON}, nil
	}
}

// token reads the next JSON token, turning a syntax error into the "invalid JSON" message.
func (p *templateParser) token() (json.Token, error) {
	tok, err := p.dec.Token()
	if err != nil {
		return nil, p.invalidJSON()
	}
	return tok, nil
}

// invalidJSON returns the "invalid JSON" message with the line and column of the mistake. The
// token reader counts positions from the start of the value it was reading, so the text is
// decoded again as one value to find the position in the whole template.
func (p *templateParser) invalidJSON() error {
	offset, reason := len(p.text), "unexpected end of JSON input"
	var raw json.RawMessage
	var syntaxErr *json.SyntaxError
	if errors.As(json.NewDecoder(strings.NewReader(p.text)).Decode(&raw), &syntaxErr) {
		offset, reason = int(syntaxErr.Offset)-1, syntaxErr.Error()
	}
	line, column := lineAndColumn(p.text, offset)
	return fmt.Errorf("%s: invalid JSON at line %d, column %d: %s", p.source, line, column, reason)
}

// checkNothingFollows reports content after the top-level object.
func (p *templateParser) checkNothingFollows() error {
	rest := strings.TrimLeft(p.text[p.dec.InputOffset():], " \t\r\n")
	if rest == "" {
		return nil
	}
	line, column := lineAndColumn(p.text, len(p.text)-len(rest))
	return fmt.Errorf("%s: unexpected content after the template object at line %d, column %d", p.source, line, column)
}

// lineAndColumn returns the 1-based line and column of the byte at offset in text.
func lineAndColumn(text string, offset int) (line, column int) {
	offset = max(0, min(offset, len(text)))
	before := text[:offset]
	return strings.Count(before, "\n") + 1, offset - strings.LastIndexByte(before, '\n')
}

// memberPath returns the path of key inside the object at parent. A key containing ".", "["
// or "]" is quoted, so the path stays unambiguous.
func memberPath(parent, key string) string {
	switch {
	case strings.ContainsAny(key, ".[]"):
		return parent + "[" + strconv.Quote(key) + "]"
	case parent == "":
		return key
	default:
		return parent + "." + key
	}
}

// objectLocation names the object at path in messages.
func objectLocation(path string) string {
	if path == "" {
		return "at the top level"
	}
	return fmt.Sprintf("in %q", path)
}

// jsonKind names the JSON kind of a token read by json.Decoder.Token.
func jsonKind(tok json.Token) string {
	switch v := tok.(type) {
	case json.Delim:
		if v == '[' {
			return "an array"
		}
		return "an object"
	case string:
		return "a string"
	case json.Number:
		return "a number"
	case bool:
		return "a boolean"
	default:
		return "null"
	}
}
