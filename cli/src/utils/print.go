package utils

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// PrintTable prints a table with kubectl-style whitespace-aligned columns.
// Headers and each row should have the same number of columns; rows may be
// shorter and will be padded with empty strings.
func PrintTable(headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 3, ' ', 0)

	// Header row
	fmt.Fprintln(w, strings.Join(headers, "\t"))

	// Data rows
	cols := len(headers)
	for _, r := range rows {
		cells := make([]string, cols)
		for i := 0; i < cols; i++ {
			if i < len(r) {
				cells[i] = r[i]
			}
		}
		fmt.Fprintln(w, strings.Join(cells, "\t"))
	}

	w.Flush()
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
