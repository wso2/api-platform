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

package utils

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout captures stdout output from fn.
func captureStdout(fn func()) (string, error) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w

	defer func() {
		os.Stdout = old
		r.Close()
	}()

	fn()

	// Close write end before reading so io.Copy can detect EOF
	w.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func TestPrintTable_NormalCase(t *testing.T) {
	headers := []string{"NAME", "SERVER", "AUTH"}
	rows := [][]string{
		{"dev", "http://localhost:9090", "basic"},
		{"prod", "https://api.example.com", "oauth2"},
	}

	out, err := captureStdout(func() {
		PrintTable(headers, rows)
	})
	if err != nil {
		t.Fatalf("failed to capture stdout: %v", err)
	}

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
	out, err := captureStdout(func() {
		PrintTable([]string{}, [][]string{{"a", "b"}})
	})
	if err != nil {
		t.Fatalf("failed to capture stdout: %v", err)
	}

	if out != "" {
		t.Errorf("expected no output for empty headers, got:\n%s", out)
	}
}

func TestPrintTable_RowsShorterThanHeaders(t *testing.T) {
	headers := []string{"NAME", "SERVER", "AUTH"}
	rows := [][]string{
		{"dev"}, // only 1 of 3 columns
	}

	out, err := captureStdout(func() {
		PrintTable(headers, rows)
	})
	if err != nil {
		t.Fatalf("failed to capture stdout: %v", err)
	}

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

	out, err := captureStdout(func() {
		PrintTable(headers, nil)
	})
	if err != nil {
		t.Fatalf("failed to capture stdout: %v", err)
	}

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line (header only), got %d:\n%s", len(lines), out)
	}
	if !strings.Contains(out, "NAME") {
		t.Errorf("output should contain header 'NAME', got:\n%s", out)
	}
}
