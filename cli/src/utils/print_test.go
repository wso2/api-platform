package utils

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout captures stdout output from fn.
func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestPrintTable_NormalCase(t *testing.T) {
	headers := []string{"NAME", "SERVER", "AUTH"}
	rows := [][]string{
		{"dev", "http://localhost:9090", "basic"},
		{"prod", "https://api.example.com", "oauth2"},
	}

	out := captureStdout(func() {
		PrintTable(headers, rows)
	})

	// Should not contain bordered table characters
	for _, ch := range []string{"+", "|", "---"} {
		if strings.Contains(out, ch) {
			t.Errorf("output should not contain %q, got:\n%s", ch, out)
		}
	}

	// Should contain all headers and data
	for _, h := range headers {
		if !strings.Contains(out, h) {
			t.Errorf("output should contain header %q, got:\n%s", h, out)
		}
	}
	for _, row := range rows {
		for _, cell := range row {
			if !strings.Contains(out, cell) {
				t.Errorf("output should contain cell %q, got:\n%s", cell, out)
			}
		}
	}

	// Verify column alignment: all lines should have same structure
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 { // 1 header + 2 data rows
		t.Errorf("expected 3 lines, got %d:\n%s", len(lines), out)
	}
}

func TestPrintTable_EmptyHeaders(t *testing.T) {
	out := captureStdout(func() {
		PrintTable([]string{}, [][]string{{"a", "b"}})
	})

	if out != "" {
		t.Errorf("expected no output for empty headers, got:\n%s", out)
	}
}

func TestPrintTable_RowsShorterThanHeaders(t *testing.T) {
	headers := []string{"NAME", "SERVER", "AUTH"}
	rows := [][]string{
		{"dev"}, // only 1 of 3 columns
	}

	out := captureStdout(func() {
		PrintTable(headers, rows)
	})

	if !strings.Contains(out, "dev") {
		t.Errorf("output should contain 'dev', got:\n%s", out)
	}

	// Should still have header and data line
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d:\n%s", len(lines), out)
	}
}

func TestPrintTable_NoRows(t *testing.T) {
	headers := []string{"NAME", "SERVER"}

	out := captureStdout(func() {
		PrintTable(headers, nil)
	})

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line (header only), got %d:\n%s", len(lines), out)
	}
	if !strings.Contains(out, "NAME") {
		t.Errorf("output should contain header 'NAME', got:\n%s", out)
	}
}
