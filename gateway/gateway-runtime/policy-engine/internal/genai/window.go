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

package genai

import "bytes"

// DefaultWindowBytes is how much of each end of a response a Window keeps.
const DefaultWindowBytes = 256 << 10

// Window keeps the start and the end of a streamed body, so a long stream costs
// bounded memory while the parts the span needs survive: the first events carry the
// response id and model, the last ones the finish reason and the token usage.
type Window struct {
	limit   int
	head    []byte
	tail    []byte
	dropped bool
}

// NewWindow returns a window keeping up to limit bytes at each end.
func NewWindow(limit int) *Window {
	return &Window{limit: limit}
}

// Write appends p to the body.
func (w *Window) Write(p []byte) {
	if room := w.limit - len(w.head); room > 0 {
		n := min(room, len(p))
		w.head = append(w.head, p[:n]...)
		p = p[n:]
	}
	if len(p) == 0 {
		return
	}
	w.tail = append(w.tail, p...)
	// Compact at twice the limit, so the copy is amortised over many writes.
	if len(w.tail) > 2*w.limit {
		w.tail = append(w.tail[:0:0], w.tail[len(w.tail)-w.limit:]...)
		w.dropped = true
	}
}

// Truncated reports whether bytes in the middle were dropped.
func (w *Window) Truncated() bool {
	return w.dropped || len(w.tail) > w.limit
}

// Bytes returns the body, or when the middle was dropped, the head and the tail cut
// back to whole server-sent events, so each part still parses event by event.
func (w *Window) Bytes() []byte {
	if !w.Truncated() {
		return append(append([]byte(nil), w.head...), w.tail...)
	}
	head := w.head
	if i := bytes.LastIndex(head, eventBoundary); i >= 0 {
		head = head[:i+len(eventBoundary)]
	}
	tail := w.tail[len(w.tail)-min(len(w.tail), w.limit):]
	if i := bytes.Index(tail, eventBoundary); i >= 0 {
		tail = tail[i+len(eventBoundary):]
	}
	return append(append([]byte(nil), head...), tail...)
}

// eventBoundary ends a server-sent event.
var eventBoundary = []byte("\n\n")
