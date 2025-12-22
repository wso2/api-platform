package utils

import (
	"fmt"
	"os"
	"text/tabwriter"
)

// PrintTable prints a simple aligned table to stdout. Headers and each row
// should have the same number of columns.
func PrintTable(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)

	// Print header
	for i, h := range headers {
		if i == len(headers)-1 {
			fmt.Fprintf(w, "%s\n", h)
		} else {
			fmt.Fprintf(w, "%s\t", h)
		}
	}

	// Print rows
	for _, row := range rows {
		for i, col := range row {
			if i == len(row)-1 {
				fmt.Fprintf(w, "%s\n", col)
			} else {
				fmt.Fprintf(w, "%s\t", col)
			}
		}
	}

	_ = w.Flush()
}
