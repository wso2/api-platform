package utils

import (
	"fmt"
	"strings"
)

// PrintTable prints a table with ASCII borders. Headers and each row
// should have the same number of columns; rows may be shorter and will be
// padded.
func PrintTable(headers []string, rows [][]string) {
	cols := len(headers)
	if cols == 0 {
		return
	}

	// compute column widths
	widths := make([]int, cols)
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, r := range rows {
		for i := 0; i < cols && i < len(r); i++ {
			if len(r[i]) > widths[i] {
				widths[i] = len(r[i])
			}
		}
	}

	// helper to build separator like +-----+------+
	buildSep := func() string {
		var b strings.Builder
		b.WriteString("+")
		for _, w := range widths {
			b.WriteString(strings.Repeat("-", w+2))
			b.WriteString("+")
		}
		return b.String()
	}

	pad := func(s string, w int) string {
		if len(s) >= w {
			return s
		}
		return s + strings.Repeat(" ", w-len(s))
	}

	sep := buildSep()
	fmt.Println(sep)

	// header
	var hb strings.Builder
	hb.WriteString("|")
	for i, h := range headers {
		hb.WriteString(" ")
		hb.WriteString(pad(h, widths[i]))
		hb.WriteString(" |")
	}
	fmt.Println(hb.String())
	fmt.Println(sep)

	// rows
	for _, r := range rows {
		var rb strings.Builder
		rb.WriteString("|")
		for i := 0; i < cols; i++ {
			var cell string
			if i < len(r) {
				cell = r[i]
			} else {
				cell = ""
			}
			rb.WriteString(" ")
			rb.WriteString(pad(cell, widths[i]))
			rb.WriteString(" |")
		}
		fmt.Println(rb.String())
		fmt.Println(sep)
	}
}

// PrintBoxedMessage prints a message in a box with borders
func PrintBoxedMessage(lines []string) {
	if len(lines) == 0 {
		return
	}

	// Find the maximum line length
	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	// Add padding
	width := maxLen + 2

	// Top border
	fmt.Println("╔" + strings.Repeat("═", width) + "╗")

	// Content lines
	for _, line := range lines {
		padding := width - len(line) - 1
		fmt.Printf("║ %s%s║\n", line, strings.Repeat(" ", padding))
	}

	// Bottom border
	fmt.Println("╚" + strings.Repeat("═", width) + "╝")
}
