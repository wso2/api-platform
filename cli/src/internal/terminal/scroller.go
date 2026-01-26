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
	"fmt"
	"os"
	"strings"
	"sync"
)

// Default configuration values
const (
	DefaultMaxScrollLines = 12
	MinScrollLines        = 5
	MaxScrollLines        = 50
)

// ScrollingLoggerConfig holds configuration for the scrolling logger
type ScrollingLoggerConfig struct {
	LogFile  *os.File // Required: where to write complete logs
	MaxLines int      // Default: 12, min: 5, max: 50
	Prefix   string   // Optional prefix for each displayed line (e.g., "  ")
}

// ScrollingLogger displays scrolling logs in the terminal while writing
// the complete output to a log file. It implements io.Writer for seamless
// integration with exec.Command.
type ScrollingLogger struct {
	terminal *Terminal
	logFile  *os.File
	maxLines int
	prefix   string

	lines       []string        // Ring buffer for visible lines
	lineIndex   int             // Next write position in ring buffer
	displayedN  int             // Lines currently displayed on screen
	currentLine strings.Builder // Buffer for incomplete lines

	mu     sync.Mutex
	active bool
}

// NewScrollingLogger creates a new scrolling logger.
// Auto-detects TTY - if not a terminal, falls back to file-only logging.
func NewScrollingLogger(cfg ScrollingLoggerConfig) *ScrollingLogger {
	maxLines := cfg.MaxLines
	if maxLines <= 0 {
		maxLines = DefaultMaxScrollLines
	}
	if maxLines < MinScrollLines {
		maxLines = MinScrollLines
	}
	if maxLines > MaxScrollLines {
		maxLines = MaxScrollLines
	}

	return &ScrollingLogger{
		terminal: NewTerminal(),
		logFile:  cfg.LogFile,
		maxLines: maxLines,
		prefix:   cfg.Prefix,
		lines:    make([]string, 0, maxLines),
	}
}

// Start prepares the scrolling display.
// Returns nil on success or if not a TTY (graceful fallback).
func (s *ScrollingLogger) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.terminal.IsTTY() {
		return nil // Silent fallback for non-TTY
	}
	s.terminal.HideCursor()
	s.active = true
	return nil
}

// Stop cleans up the display and ensures the cursor is visible.
// Safe to call multiple times.
func (s *ScrollingLogger) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return
	}

	// Flush any remaining partial line
	if s.currentLine.Len() > 0 {
		s.addLineInternal(s.currentLine.String())
		s.currentLine.Reset()
	}

	s.active = false
	s.terminal.ShowCursor()
}

// Write implements io.Writer for use with exec.Command.
// All data is written to the log file. If TTY is active, the scrolling
// display is updated in real-time.
func (s *ScrollingLogger) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Always write to log file (complete output)
	if s.logFile != nil {
		if _, err := s.logFile.Write(p); err != nil {
			// Log file write error - continue anyway to not fail the build
			// Terminal display is best-effort
		}
	}

	// If not TTY or not active, skip terminal display
	if !s.terminal.IsTTY() || !s.active {
		return len(p), nil
	}

	// Process byte by byte for line buffering
	for _, b := range p {
		if b == '\n' {
			s.addLineInternal(s.currentLine.String())
			s.currentLine.Reset()
		} else if b != '\r' { // Ignore carriage returns
			s.currentLine.WriteByte(b)
		}
	}

	return len(p), nil
}

// addLineInternal adds a line to the ring buffer and redraws.
// Must be called with mutex held.
func (s *ScrollingLogger) addLineInternal(line string) {
	// Truncate long lines to fit terminal width
	maxWidth := s.terminal.Width() - len(s.prefix) - 3
	if maxWidth > 0 && len(line) > maxWidth {
		line = line[:maxWidth] + "..."
	}

	// Add to ring buffer
	if len(s.lines) < s.maxLines {
		s.lines = append(s.lines, line)
	} else {
		s.lines[s.lineIndex] = line
		s.lineIndex = (s.lineIndex + 1) % s.maxLines
	}

	s.redraw()
}

// redraw clears previously displayed lines and redraws the visible window.
// Must be called with mutex held.
func (s *ScrollingLogger) redraw() {
	// Clear previously displayed lines
	if s.displayedN > 0 {
		s.terminal.ClearLines(s.displayedN)
	}

	// Print lines in order (oldest first)
	n := len(s.lines)
	if n == 0 {
		s.displayedN = 0
		return
	}

	// When buffer is full, lineIndex points to oldest line
	// When buffer is not full, lines are in order starting from 0
	if len(s.lines) < s.maxLines {
		// Buffer not full yet - lines are in order
		for i := 0; i < n; i++ {
			fmt.Printf("%s%s%s%s\n", AnsiGray, s.prefix, s.lines[i], AnsiReset)
		}
	} else {
		// Buffer is full - need to read in ring order
		for i := 0; i < n; i++ {
			idx := (s.lineIndex + i) % s.maxLines
			fmt.Printf("%s%s%s%s\n", AnsiGray, s.prefix, s.lines[idx], AnsiReset)
		}
	}

	s.displayedN = n
}

// GetDisplayedLines returns the number of lines currently shown.
// Useful for testing.
func (s *ScrollingLogger) GetDisplayedLines() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.displayedN
}

// IsActive returns true if the scrolling display is active.
// Useful for testing.
func (s *ScrollingLogger) IsActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

// ClearDisplay clears the currently displayed scrolling lines from the terminal.
// This is useful to clean up before printing final status messages.
func (s *ScrollingLogger) ClearDisplay() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.displayedN > 0 {
		s.terminal.ClearLines(s.displayedN)
		s.displayedN = 0
	}
}
