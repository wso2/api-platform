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

package model

import (
	"bytes"
	"encoding/json"
)

// ─── the model a response reports ────────────────────────────────────────────
//
// A client asks for a Model by its name and is answered in the Model's name. The
// provider's response names the model it ran — the target's upstream model, or a
// dated variant of it — and passed through unchanged that would tell every client
// which provider and model a Model is made of, and answer a request for `auto`
// with `gpt-4o-mini`. The route job replaces it on the way out, where the response
// has already been turned back into the endpoint's format, and records the
// provider's own value for the analytics collector.

// responseModelPaths are where each format's responses report the model: a member
// of the response object, or of an object inside it. Anthropic's streamed
// message_start event carries it one level down, in `message`; Gemini calls it
// modelVersion.
var responseModelPaths = map[string][][]string{
	FormatOpenAI:    {{"model"}},
	FormatAnthropic: {{"model"}, {"message", "model"}},
	FormatGemini:    {{"modelVersion"}},
}

// maxHeldResponseBytes bounds what the rewriter holds back from a streamed
// response: one JSON object, or one event-stream line, while it waits for its end.
// It is the kernel's own ceiling on an accumulated response chunk
// (maxStreamAccumulatorSize in the policy engine), so the rewriter never holds
// more than the kernel would have handed over at once. Past it the rest of the
// response is forwarded as it arrives, still reporting the provider's model —
// the response itself is never cut or changed otherwise.
const maxHeldResponseBytes = 10 * 1024 * 1024

// modelRewriter replaces the model one endpoint format's responses report.
type modelRewriter struct {
	paths   [][]string
	encoded []byte // the Model's name, as a JSON string
}

func newModelRewriter(format, modelName string) modelRewriter {
	encoded, err := json.Marshal(modelName)
	if err != nil {
		// Unreachable for a string.
		encoded = nil
	}
	return modelRewriter{paths: responseModelPaths[format], encoded: encoded}
}

// object returns obj with the model it reports replaced, and the first value it
// replaced. ok is false when obj is not a JSON object or reports no model as a
// string, and obj is then to be forwarded as it is.
//
// The value is spliced in, as the request's model is, so every other byte of the
// response reaches the client exactly as the provider sent it.
func (r modelRewriter) object(obj []byte) (out []byte, reported string, ok bool) {
	if len(r.encoded) == 0 || !isJSONObject(obj) {
		return nil, "", false
	}
	out = obj
	for _, path := range r.paths {
		rewritten, old, replaced := replaceStringAtPath(out, path, r.encoded)
		if !replaced {
			continue
		}
		out = rewritten
		if reported == "" {
			reported = old
		}
		ok = true
	}
	return out, reported, ok
}

// isJSONObject reports whether b is one valid JSON value, and that value an
// object.
func isJSONObject(b []byte) bool {
	trimmed := bytes.TrimLeft(b, " \t\r\n")
	return len(trimmed) > 0 && trimmed[0] == '{' && json.Valid(b)
}

// replaceStringAtPath replaces every string value at path in the JSON object obj
// — every occurrence, as the request rewrite does — and returns the first value it
// replaced. A member that is absent or not a string is left alone; nothing is
// added.
func replaceStringAtPath(obj []byte, path []string, encoded []byte) ([]byte, string, bool) {
	if len(path) == 0 {
		return nil, "", false
	}
	type splice struct {
		start, end int64
		value      []byte
	}
	var splices []splice
	reported := ""
	err := walkTopLevelMembers(obj, func(key string, start, end int64, value json.RawMessage) {
		if key != path[0] {
			return
		}
		if len(path) == 1 {
			// Only a string: encoding/json reads a null into a string without error.
			var old string
			if len(value) == 0 || value[0] != '"' || json.Unmarshal(value, &old) != nil {
				return
			}
			if len(splices) == 0 {
				reported = old
			}
			splices = append(splices, splice{start, end, encoded})
			return
		}
		if !isJSONObject(value) {
			return
		}
		inner, old, ok := replaceStringAtPath(value, path[1:], encoded)
		if !ok {
			return
		}
		if len(splices) == 0 {
			reported = old
		}
		splices = append(splices, splice{start, end, inner})
	})
	if err != nil || len(splices) == 0 {
		return nil, "", false
	}

	var out bytes.Buffer
	out.Grow(len(obj) + len(encoded)*len(splices))
	var cursor int64
	for _, s := range splices {
		out.Write(obj[cursor:s.start])
		out.Write(s.value)
		cursor = s.end
	}
	out.Write(obj[cursor:])
	return out.Bytes(), reported, true
}

// ─── a response in pieces ────────────────────────────────────────────────────

// responseShape is what a response body turned out to be, decided from its first
// bytes.
type responseShape int

const (
	shapeUndecided   responseShape = iota
	shapeJSON                      // JSON objects, or arrays of them (Gemini's streamGenerateContent)
	shapeEventStream               // Server-Sent Events, one JSON object per data line
	shapePassthrough               // anything else, or past the hold limit: forwarded as it arrives
)

// eventStreamFields are the field names an event-stream line may open with, and
// ":" for a comment line.
var eventStreamFields = [][]byte{[]byte("data:"), []byte("event:"), []byte("id:"), []byte("retry:"), []byte(":")}

var dataField = []byte("data:")

// responseModelStream rewrites the model in one response delivered in any number
// of pieces: a buffered body is one piece, a streamed one as many as the kernel
// flushes. It holds back only what it cannot yet rewrite — an unfinished object,
// an unfinished event-stream line — and forwards everything else at once, so a
// streamed completion still streams.
//
// Each piece goes through feed, and the output of feed is what replaces it: the
// bytes forwarded for that piece, possibly none.
type responseModelStream struct {
	rewriter modelRewriter
	shape    responseShape

	// pending is what has been received and not yet forwarded.
	pending []byte

	// The JSON scan. scanned is how much of pending has been read; held, where the
	// object being held starts in pending (-1 for none); depth, the nesting at the
	// end of what was read; arrayAtTop, whether the value at the top is an array,
	// whose object elements are then held and rewritten one at a time.
	scanned    int
	held       int
	depth      int
	arrayAtTop bool
	inString   bool
	escaped    bool

	// reported is the first model the provider's response named.
	reported string
}

func newResponseModelStream(rewriter modelRewriter) *responseModelStream {
	return &responseModelStream{rewriter: rewriter, held: -1}
}

// feed takes the next piece of the response, end marking the last, and returns
// the bytes to forward in its place. The result is never nil, so a caller can tell
// "forward nothing yet" from "forward the piece unchanged".
func (s *responseModelStream) feed(piece []byte, end bool) []byte {
	out := make([]byte, 0, len(piece)+len(s.rewriter.encoded))
	if s.shape == shapePassthrough {
		return append(out, piece...)
	}
	s.pending = append(s.pending, piece...)

	if s.shape == shapeUndecided {
		s.shape = decideShape(s.pending, end)
	}
	switch s.shape {
	case shapeJSON:
		out = s.scanJSON(out)
	case shapeEventStream:
		out = s.scanEventStream(out, end)
	case shapePassthrough:
		out = append(out, s.pending...)
		s.pending = nil
	}

	if end || len(s.pending) > maxHeldResponseBytes {
		// The end of the response, or more held than the limit allows: what is
		// held is forwarded as it is — an unfinished object cannot be rewritten,
		// and is the provider's to have sent.
		out = append(out, s.pending...)
		s.pending = nil
		s.shape = shapePassthrough
	}
	return out
}

// decideShape reads the first bytes that are not whitespace. Too few to decide
// leaves the response undecided, unless it has ended.
func decideShape(pending []byte, end bool) responseShape {
	rest := bytes.TrimLeft(pending, " \t\r\n")
	if len(rest) == 0 {
		if end {
			return shapePassthrough
		}
		return shapeUndecided
	}
	if rest[0] == '{' || rest[0] == '[' {
		return shapeJSON
	}
	for _, field := range eventStreamFields {
		if bytes.HasPrefix(rest, field) {
			return shapeEventStream
		}
		if !end && len(rest) < len(field) && bytes.HasPrefix(field, rest) {
			return shapeUndecided
		}
	}
	return shapePassthrough
}

// scanJSON reads pending from where the last piece left off. An object at the top,
// or an object element of an array at the top, is held from its opening brace and
// forwarded rewritten once its closing brace arrives; every other byte is
// forwarded as soon as it is read. Several values one after another — newline-
// delimited JSON — are each handled the same way.
func (s *responseModelStream) scanJSON(out []byte) []byte {
	p := s.pending
	forwarded := 0
	for i := s.scanned; i < len(p); i++ {
		c := p[i]
		if s.inString {
			switch {
			case s.escaped:
				s.escaped = false
			case c == '\\':
				s.escaped = true
			case c == '"':
				s.inString = false
			}
			continue
		}
		switch c {
		case '"':
			s.inString = true
		case '{', '[':
			if s.depth == 0 {
				s.arrayAtTop = c == '['
			}
			if c == '{' && (s.depth == 0 || (s.depth == 1 && s.arrayAtTop)) {
				s.held = i
			}
			s.depth++
		case '}', ']':
			s.depth--
			if s.depth < 0 {
				// Not JSON after all; nothing more is read.
				s.shape = shapePassthrough
				out = append(out, p[forwarded:]...)
				s.pending, s.scanned, s.held, s.depth = nil, 0, -1, 0
				return out
			}
			if c == '}' && s.held >= 0 && (s.depth == 0 || (s.depth == 1 && s.arrayAtTop)) {
				out = append(out, p[forwarded:s.held]...)
				out = append(out, s.rewrite(p[s.held:i+1])...)
				forwarded = i + 1
				s.held = -1
			}
		}
	}

	keep := len(p)
	if s.held >= 0 {
		keep = s.held
	}
	out = append(out, p[forwarded:keep]...)
	// Resliced, not copied: an object arriving in many small pieces would otherwise
	// be copied once per piece.
	s.pending = p[keep:]
	if s.held >= 0 {
		s.held = 0
	}
	s.scanned = len(s.pending)
	return out
}

// scanEventStream forwards every complete line of pending, a data line carrying a
// JSON object rewritten, and holds the unfinished last one. A line ends at "\n";
// a stream that ends its lines with a bare "\r" is held until its end and then
// forwarded unchanged. A data value split over several data lines is not one JSON
// object on any line, and is forwarded unchanged as well — none of the formats
// this endpoint speaks sends one.
func (s *responseModelStream) scanEventStream(out []byte, end bool) []byte {
	p := s.pending
	start := 0
	// What is held has no line end in it, so the search starts with the new bytes.
	search := s.scanned
	for {
		nl := bytes.IndexByte(p[search:], '\n')
		if nl < 0 {
			break
		}
		lineEnd := search + nl + 1
		out = s.rewriteEventLine(out, p[start:lineEnd])
		start, search = lineEnd, lineEnd
	}
	if end && start < len(p) {
		out = s.rewriteEventLine(out, p[start:])
		start = len(p)
	}
	s.pending = p[start:]
	s.scanned = len(s.pending)
	return out
}

// rewriteEventLine forwards one event-stream line, with the model replaced when it
// is a data line whose value is a JSON object reporting one.
func (s *responseModelStream) rewriteEventLine(out, line []byte) []byte {
	content := bytes.TrimSuffix(line, []byte("\n"))
	content = bytes.TrimSuffix(content, []byte("\r"))
	if !bytes.HasPrefix(content, dataField) {
		return append(out, line...)
	}
	value := content[len(dataField):]
	lead := 0
	if len(value) > 0 && value[0] == ' ' {
		lead = 1
	}
	rewritten, reported, ok := s.rewriter.object(value[lead:])
	if !ok {
		return append(out, line...)
	}
	s.note(reported)
	out = append(out, dataField...)
	out = append(out, value[:lead]...)
	out = append(out, rewritten...)
	return append(out, line[len(content):]...)
}

// rewrite is one held object, rewritten when it reports a model.
func (s *responseModelStream) rewrite(obj []byte) []byte {
	rewritten, reported, ok := s.rewriter.object(obj)
	if !ok {
		return obj
	}
	s.note(reported)
	return rewritten
}

func (s *responseModelStream) note(reported string) {
	if s.reported == "" {
		s.reported = reported
	}
}
