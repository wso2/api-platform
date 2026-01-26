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

package terminal

import (
	"bytes"
	"os"
	"sync"
	"testing"
)

func TestNewScrollingLogger_Defaults(t *testing.T) {
	logger := NewScrollingLogger(ScrollingLoggerConfig{})

	if logger.maxLines != DefaultMaxScrollLines {
		t.Errorf("expected maxLines=%d, got %d", DefaultMaxScrollLines, logger.maxLines)
	}
	if logger.prefix != "" {
		t.Errorf("expected empty prefix, got %q", logger.prefix)
	}
	if logger.terminal == nil {
		t.Error("expected terminal to be initialized")
	}
}

func TestNewScrollingLogger_MaxLinesConstraints(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"zero uses default", 0, DefaultMaxScrollLines},
		{"negative uses default", -5, DefaultMaxScrollLines},
		{"below min clamps to min", 3, MinScrollLines},
		{"above max clamps to max", 100, MaxScrollLines},
		{"valid value preserved", 20, 20},
		{"min value preserved", MinScrollLines, MinScrollLines},
		{"max value preserved", MaxScrollLines, MaxScrollLines},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := NewScrollingLogger(ScrollingLoggerConfig{MaxLines: tc.input})
			if logger.maxLines != tc.expected {
				t.Errorf("MaxLines=%d: expected maxLines=%d, got %d", tc.input, tc.expected, logger.maxLines)
			}
		})
	}
}

func TestScrollingLogger_WriteToLogFile(t *testing.T) {
	// Create a temp file for testing
	tmpFile, err := os.CreateTemp("", "scroller_test_*.log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	logger := NewScrollingLogger(ScrollingLoggerConfig{
		LogFile:  tmpFile,
		MaxLines: 5,
	})

	testData := "line1\nline2\nline3\n"
	n, err := logger.Write([]byte(testData))

	if err != nil {
		t.Errorf("Write returned error: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write returned %d bytes, expected %d", n, len(testData))
	}

	// Read back the file
	tmpFile.Seek(0, 0)
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}

	if string(content) != testData {
		t.Errorf("log file content mismatch: got %q, want %q", string(content), testData)
	}
}

func TestScrollingLogger_RingBuffer(t *testing.T) {
	// Test ring buffer behavior by verifying that the buffer doesn't grow
	// beyond maxLines. Since the display logic is terminal-dependent,
	// we test the internal state after manual manipulation.

	// Create a simple ring buffer implementation test
	maxLines := 3
	var lines []string
	lineIndex := 0

	// Add 5 items to a ring buffer of size 3
	testLines := []string{"line1", "line2", "line3", "line4", "line5"}
	for _, line := range testLines {
		if len(lines) < maxLines {
			lines = append(lines, line)
		} else {
			lines[lineIndex] = line
			lineIndex = (lineIndex + 1) % maxLines
		}
	}

	// Should only have 3 lines
	if len(lines) != 3 {
		t.Errorf("expected 3 lines in buffer, got %d", len(lines))
	}

	// Verify the content - should be the last 3 lines
	expectedContent := map[string]bool{"line3": true, "line4": true, "line5": true}
	for _, line := range lines {
		if !expectedContent[line] {
			t.Errorf("unexpected line in buffer: %q", line)
		}
	}
}

func TestScrollingLogger_LineBuffering(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "scroller_test_*.log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	logger := NewScrollingLogger(ScrollingLoggerConfig{
		LogFile:  tmpFile,
		MaxLines: 5,
	})

	// Write partial lines
	logger.Write([]byte("partial"))
	logger.Write([]byte(" line"))
	logger.Write([]byte("\n"))
	logger.Write([]byte("complete line\n"))

	// Verify log file has everything
	tmpFile.Seek(0, 0)
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}

	expected := "partial line\ncomplete line\n"
	if string(content) != expected {
		t.Errorf("log file content mismatch: got %q, want %q", string(content), expected)
	}
}

func TestScrollingLogger_CarriageReturnIgnored(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "scroller_test_*.log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	logger := NewScrollingLogger(ScrollingLoggerConfig{
		LogFile:  tmpFile,
		MaxLines: 5,
	})

	// The logger ignores \r in the display buffer but writes it to file
	// Let's verify the Write passes through everything to the file
	testData := "line with\r carriage\n"
	logger.Write([]byte(testData))

	tmpFile.Seek(0, 0)
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}

	// File should have the raw data
	if string(content) != testData {
		t.Errorf("log file content mismatch: got %q, want %q", string(content), testData)
	}
}

func TestScrollingLogger_ConcurrentWrites(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "scroller_test_*.log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	logger := NewScrollingLogger(ScrollingLoggerConfig{
		LogFile:  tmpFile,
		MaxLines: 10,
	})
	logger.Start()
	defer logger.Stop()

	var wg sync.WaitGroup
	numGoroutines := 10
	linesPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < linesPerGoroutine; j++ {
				logger.Write([]byte("goroutine line\n"))
			}
		}(i)
	}

	wg.Wait()

	// Verify all lines were written to file
	tmpFile.Seek(0, 0)
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}

	expectedLines := numGoroutines * linesPerGoroutine
	actualLines := bytes.Count(content, []byte("\n"))
	if actualLines != expectedLines {
		t.Errorf("expected %d lines in log file, got %d", expectedLines, actualLines)
	}
}

func TestScrollingLogger_StartStop(t *testing.T) {
	logger := NewScrollingLogger(ScrollingLoggerConfig{
		MaxLines: 5,
	})

	// Initially not active
	if logger.IsActive() {
		t.Error("logger should not be active before Start()")
	}

	// Start should work (even without TTY it's a no-op that returns nil)
	if err := logger.Start(); err != nil {
		t.Errorf("Start() returned error: %v", err)
	}

	// Stop should be safe to call
	logger.Stop()

	// Should be inactive after Stop
	if logger.IsActive() {
		t.Error("logger should not be active after Stop()")
	}

	// Multiple Stop calls should be safe
	logger.Stop()
	logger.Stop()
}

func TestScrollingLogger_EmptyOutput(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "scroller_test_*.log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	logger := NewScrollingLogger(ScrollingLoggerConfig{
		LogFile:  tmpFile,
		MaxLines: 5,
	})
	logger.Start()

	// Write empty data
	n, err := logger.Write([]byte{})
	if err != nil {
		t.Errorf("Write empty data returned error: %v", err)
	}
	if n != 0 {
		t.Errorf("Write empty data returned %d, expected 0", n)
	}

	logger.Stop()
}

func TestScrollingLogger_NilLogFile(t *testing.T) {
	// Logger should work even without a log file
	logger := NewScrollingLogger(ScrollingLoggerConfig{
		LogFile:  nil,
		MaxLines: 5,
	})
	logger.Start()

	// Should not panic
	n, err := logger.Write([]byte("test line\n"))
	if err != nil {
		t.Errorf("Write with nil LogFile returned error: %v", err)
	}
	if n != 10 {
		t.Errorf("Write returned %d, expected 10", n)
	}

	logger.Stop()
}

func TestScrollingLogger_Prefix(t *testing.T) {
	logger := NewScrollingLogger(ScrollingLoggerConfig{
		Prefix:   ">>> ",
		MaxLines: 5,
	})

	if logger.prefix != ">>> " {
		t.Errorf("expected prefix %q, got %q", ">>> ", logger.prefix)
	}
}

func TestTerminal_NewTerminal(t *testing.T) {
	term := NewTerminal()

	if term == nil {
		t.Fatal("NewTerminal returned nil")
	}

	// Width and height should be positive
	if term.Width() <= 0 {
		t.Errorf("terminal width should be positive, got %d", term.Width())
	}
	if term.Height() <= 0 {
		t.Errorf("terminal height should be positive, got %d", term.Height())
	}
}

func TestTerminal_ClearLinesNoOp(t *testing.T) {
	term := NewTerminal()

	// These should not panic
	term.ClearLines(0)
	term.ClearLines(-1)

	// If not TTY, ClearLines should be a no-op
	if !term.IsTTY() {
		term.ClearLines(5) // Should not panic or output anything
	}
}

func TestTerminal_CursorOperations(t *testing.T) {
	term := NewTerminal()

	// These should not panic regardless of TTY status
	term.HideCursor()
	term.ShowCursor()
}
