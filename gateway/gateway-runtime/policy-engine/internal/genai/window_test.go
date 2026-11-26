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

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWindow_KeepsASmallBodyWhole(t *testing.T) {
	w := NewWindow(16)
	w.Write([]byte("data: a\n\n"))
	w.Write([]byte("data: b\n\n"))
	assert.False(t, w.Truncated())
	assert.Equal(t, "data: a\n\ndata: b\n\n", string(w.Bytes()))
}

// A long stream keeps its first and last events whole, the middle dropped.
func TestWindow_HeadTailAtEventBoundaries(t *testing.T) {
	w := NewWindow(64)
	w.Write([]byte("data: {\"id\":\"first\"}\n\n"))
	for i := 0; i < 1000; i++ {
		w.Write([]byte("data: {\"delta\":\"middle\"}\n\n"))
	}
	w.Write([]byte("data: {\"usage\":\"last\"}\n\n"))

	assert.True(t, w.Truncated())
	got := string(w.Bytes())
	assert.True(t, strings.HasPrefix(got, "data: {\"id\":\"first\"}\n\n"))
	assert.True(t, strings.HasSuffix(got, "data: {\"usage\":\"last\"}\n\n"))
	assert.LessOrEqual(t, len(got), 2*64)
	for _, event := range strings.Split(strings.TrimSuffix(got, "\n\n"), "\n\n") {
		assert.True(t, strings.HasPrefix(event, "data: {") && strings.HasSuffix(event, "}"), "event %q was cut", event)
	}
}

// One write larger than both ends together.
func TestWindow_SingleLargeWrite(t *testing.T) {
	w := NewWindow(8)
	w.Write([]byte(strings.Repeat("x", 100)))
	assert.True(t, w.Truncated())
	assert.Len(t, w.Bytes(), 16)
}
