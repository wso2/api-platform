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

// Package terminal provides TTY detection and ANSI terminal control operations
// for implementing scrolling log displays.
package terminal

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// ANSI escape codes for terminal control
const (
	ansiClearLine   = "\x1b[2K"   // Clear entire line
	ansiCursorUp    = "\x1b[%dA"  // Move cursor up N lines
	ansiHideCursor  = "\x1b[?25l" // Hide cursor
	ansiShowCursor  = "\x1b[?25h" // Show cursor
	ansiCarriageRet = "\r"        // Return to start of line
	AnsiGray        = "\x1b[90m"  // Gray text color
	AnsiReset       = "\x1b[0m"   // Reset text formatting
)

// Default terminal dimensions when detection fails
const (
	defaultWidth  = 80
	defaultHeight = 24
)

// Terminal handles TTY detection and ANSI operations
type Terminal struct {
	fd     int
	isTTY  bool
	width  int
	height int
}

// NewTerminal creates a Terminal for stdout with auto-detected properties
func NewTerminal() *Terminal {
	fd := int(os.Stdout.Fd())
	isTTY := term.IsTerminal(fd)
	width, height := defaultWidth, defaultHeight

	if isTTY {
		if w, h, err := term.GetSize(fd); err == nil {
			width, height = w, h
		}
	}

	return &Terminal{
		fd:     fd,
		isTTY:  isTTY,
		width:  width,
		height: height,
	}
}

// IsTTY returns true if stdout is a terminal
func (t *Terminal) IsTTY() bool {
	return t.isTTY
}

// Width returns the terminal width in columns
func (t *Terminal) Width() int {
	return t.width
}

// Height returns the terminal height in rows
func (t *Terminal) Height() int {
	return t.height
}

// ClearLines clears N lines above the cursor position
func (t *Terminal) ClearLines(n int) {
	if !t.isTTY || n <= 0 {
		return
	}
	for i := 0; i < n; i++ {
		fmt.Printf(ansiCursorUp, 1)
		fmt.Print(ansiClearLine)
		fmt.Print(ansiCarriageRet)
	}
}

// HideCursor hides the terminal cursor
func (t *Terminal) HideCursor() {
	if t.isTTY {
		fmt.Print(ansiHideCursor)
	}
}

// ShowCursor shows the terminal cursor
func (t *Terminal) ShowCursor() {
	if t.isTTY {
		fmt.Print(ansiShowCursor)
	}
}
