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
	"strconv"
	"strings"

	"cel.dev/cel-go/cel"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
)

// inlineTemplateSetting names the inline template in startup messages.
const inlineTemplateSetting = "traffic_logging.template"

// maxTemplateDepth is the deepest nesting of objects and arrays a template may use.
const maxTemplateDepth = 64

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
}

// templateMember is one key of a template object and its value.
type templateMember struct {
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
}

// newLogTemplate parses and compiles the template in logCfg. It returns an error only for a
// template that cannot be used at all; an expression that does not compile is written as null.
func newLogTemplate(logCfg *config.TrafficLoggingConfig, maskedHeaders map[string]bool) (*logTemplate, error) {
	source := inlineTemplateSetting
	root, err := parseTemplate(source, logCfg.Template)
	if err != nil {
		return nil, err
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
	}
	t.compile(env, root)
	return t, nil
}

// compile compiles every expression under node. An expression that does not compile becomes
// a fixed null, reported once with the field and the reason.
func (t *logTemplate) compile(env *cel.Env, node *templateNode) {
	switch node.kind {
	case objectNode:
		for _, m := range node.members {
			t.compile(env, m.value)
		}
	case arrayNode:
		for _, e := range node.elements {
			t.compile(env, e)
		}
	case expressionNode:
		ast, program, err := compileExpression(env, node.expression)
		if err != nil {
			slog.Warn(fmt.Sprintf("%s: field %q is written as null: %v", t.source, node.path, err))
			*node = templateNode{kind: literalNode, path: node.path, literal: []byte("null")}
			return
		}
		node.program = program
		for name := range referencedGlobalPropertyVariables(ast) {
			t.referencedVars[name] = true
		}
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
	evalCtx := buildGlobalPropertyEvalCtx(event, t.referencedVars, t.maskedHeaders)
	var buf bytes.Buffer
	t.write(&buf, t.root, evalCtx)
	return buf.Bytes()
}

// write appends node's JSON to buf and reports whether it wrote anything. Only an expression
// with no value for this request writes nothing; objects and arrays are always written.
func (t *logTemplate) write(buf *bytes.Buffer, node *templateNode, evalCtx map[string]interface{}) bool {
	switch node.kind {
	case objectNode:
		buf.WriteByte('{')
		first := true
		for _, m := range node.members {
			if t.writeMember(buf, first, m.encodedKey, m.value, evalCtx) {
				first = false
			}
		}
		buf.WriteByte('}')
	case arrayNode:
		buf.WriteByte('[')
		first := true
		for _, e := range node.elements {
			if t.writeMember(buf, first, nil, e, evalCtx) {
				first = false
			}
		}
		buf.WriteByte(']')
	case literalNode:
		buf.Write(node.literal)
	case expressionNode:
		value, err := node.evaluate(evalCtx)
		if err != nil {
			slog.Debug(t.source+": field left out of this line", "field", node.path, "error", err)
			return false
		}
		buf.Write(value)
	}
	return true
}

// writeMember appends one object member (encodedKey set) or array element (encodedKey nil),
// after a comma unless it is the first, and undoes the write when the value has no value.
func (t *logTemplate) writeMember(buf *bytes.Buffer, first bool, encodedKey []byte, value *templateNode, evalCtx map[string]interface{}) bool {
	mark := buf.Len()
	if !first {
		buf.WriteByte(',')
	}
	buf.Write(encodedKey)
	if t.write(buf, value, evalCtx) {
		return true
	}
	buf.Truncate(mark)
	return false
}

// evaluate returns the JSON of an expression node's result for one request.
func (n *templateNode) evaluate(evalCtx map[string]interface{}) ([]byte, error) {
	out, _, err := n.program.Eval(evalCtx)
	if err != nil {
		return nil, err
	}
	value, err := globalPropertyCELToGoValue(out)
	if err != nil {
		return nil, err
	}
	return json.Marshal(value)
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
		node.members = append(node.members, templateMember{encodedKey: append(encodedKey, ':'), value: value})
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
		return &templateNode{kind: literalNode, path: path, literal: []byte("null")}, nil
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
