/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package it

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ANSI color codes
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorGray   = "\033[90m"
	ColorBold   = "\033[1m"
)

// TestResult represents the result of a single test
type TestResult struct {
	TestID    string
	TestName  string
	Status    string // PASS, FAIL, SKIP
	Duration  time.Duration
	LogFile   string
	Error     string
	Stdout    string
	Stderr    string
	Command   []string
	ExitCode  int
	Timestamp time.Time
}

// TestReporter handles test logging and reporting
type TestReporter struct {
	logsDir     string
	results     []TestResult
	currentTest *TestResult
	mu          sync.Mutex
}

// NewTestReporter creates a new test reporter
func NewTestReporter(logsDir string) *TestReporter {
	return &TestReporter{
		logsDir: logsDir,
		results: make([]TestResult, 0),
	}
}

// Setup initializes the reporter and creates the logs directory
func (r *TestReporter) Setup() error {
	if err := os.MkdirAll(r.logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}
	return nil
}

// StartTest begins tracking a new test
func (r *TestReporter) StartTest(testID, testName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.currentTest = &TestResult{
		TestID:    testID,
		TestName:  testName,
		Timestamp: time.Now(),
	}

	// Print simple running status
	fmt.Printf("Running %s...\n", testName)
}

// EndTest completes the current test and writes the log file
func (r *TestReporter) EndTest(state *TestState, passed bool, errorMsg string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.currentTest == nil {
		return
	}

	r.currentTest.Duration = state.GetDuration()
	r.currentTest.Stdout = state.GetStdout()
	r.currentTest.Stderr = state.GetStderr()
	r.currentTest.Command = state.LastCommand
	r.currentTest.ExitCode = state.GetExitCode()

	if passed {
		r.currentTest.Status = "PASS"
	} else {
		r.currentTest.Status = "FAIL"
		r.currentTest.Error = errorMsg
	}

	// Generate log filename
	logFileName := r.generateLogFileName(r.currentTest.TestID, r.currentTest.TestName)
	r.currentTest.LogFile = logFileName

	// Write log file
	r.writeTestLog(r.currentTest)

	// Add to results
	r.results = append(r.results, *r.currentTest)
	r.currentTest = nil
}

// SkipTest marks a test as skipped
func (r *TestReporter) SkipTest(testID, testName, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := TestResult{
		TestID:    testID,
		TestName:  testName,
		Status:    "SKIP",
		Error:     reason,
		Timestamp: time.Now(),
	}

	r.results = append(r.results, result)
}

// generateLogFileName creates a log file name from test ID and name
func (r *TestReporter) generateLogFileName(testID, testName string) string {
	// Sanitize the test name for filename
	safeName := strings.ReplaceAll(testName, " ", "-")
	safeName = strings.ReplaceAll(safeName, "/", "-")
	safeName = strings.ToLower(safeName)

	return fmt.Sprintf("%s-%s.log", testID, safeName)
}

// writeTestLog writes the test log to a file in a simple, readable format
func (r *TestReporter) writeTestLog(result *TestResult) {
	logPath := filepath.Join(r.logsDir, result.LogFile)

	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("Test: %s - %s\n", result.TestID, result.TestName))
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n", result.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Duration: %s\n", result.Duration.String()))
	sb.WriteString(fmt.Sprintf("Status: %s %s\n", r.getStatusSymbol(result.Status), result.Status))
	sb.WriteString("\n")

	if len(result.Command) > 0 {
		sb.WriteString(fmt.Sprintf("Command: ap %s\n", strings.Join(result.Command, " ")))
		sb.WriteString(fmt.Sprintf("Exit Code: %d\n", result.ExitCode))
		sb.WriteString("\n")
	}

	sb.WriteString("--- STDOUT ---\n")
	if result.Stdout != "" {
		sb.WriteString(result.Stdout)
		if !strings.HasSuffix(result.Stdout, "\n") {
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("(empty)\n")
	}

	sb.WriteString("\n--- STDERR ---\n")
	if result.Stderr != "" {
		sb.WriteString(result.Stderr)
		if !strings.HasSuffix(result.Stderr, "\n") {
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("(empty)\n")
	}

	if result.Error != "" {
		sb.WriteString("\n--- ERROR ---\n")
		sb.WriteString(result.Error)
		sb.WriteString("\n")
	}

	os.WriteFile(logPath, []byte(sb.String()), 0644)
}

// getStatusSymbol returns a symbol for the status
func (r *TestReporter) getStatusSymbol(status string) string {
	switch status {
	case "PASS":
		return "✓"
	case "FAIL":
		return "✗"
	case "SKIP":
		return "⊘"
	default:
		return "?"
	}
}

// LogPhase1 logs a Phase 1 infrastructure message
func (r *TestReporter) LogPhase1(component, message string) {
	fmt.Printf("  %s[%s]%s %s\n", ColorBlue, component, ColorReset, message)
}

// LogPhase1Detail logs a detailed Phase 1 step (indented)
func (r *TestReporter) LogPhase1Detail(detail string) {
	fmt.Printf("    %s→%s %s\n", ColorCyan, ColorReset, detail)
}

// LogPhase1Pass logs a Phase 1 success
func (r *TestReporter) LogPhase1Pass(component, message string) {
	fmt.Printf("  %s[%s]%s %s %s✓%s\n", ColorBlue, component, ColorReset, message, ColorGreen, ColorReset)
}

// LogPhase1Fail logs a Phase 1 failure
func (r *TestReporter) LogPhase1Fail(component, message, details string) {
	fmt.Printf("  %s[%s]%s %s %s✗ FAIL%s\n", ColorBlue, component, ColorReset, message, ColorRed, ColorReset)
	if details != "" {
		fmt.Printf("    %sError: %s%s\n", ColorRed, details, ColorReset)
	}
}

// LogWaiting logs a waiting/progress message with spinner
func (r *TestReporter) LogWaiting(message string) {
	fmt.Printf("    %s⏳%s %s\n", ColorYellow, ColorReset, message)
}

// LogAction logs an action being performed
func (r *TestReporter) LogAction(action string) {
	fmt.Printf("    %s▶%s %s...\n", ColorPurple, ColorReset, action)
}

// LogSuccess logs a success message
func (r *TestReporter) LogSuccess(message string) {
	fmt.Printf("    %s✓%s %s\n", ColorGreen, ColorReset, message)
}

// LogInfo logs an info message
func (r *TestReporter) LogInfo(message string) {
	fmt.Printf("    %sℹ%s %s\n", ColorBlue, ColorReset, message)
}

// LogTest logs a test result (called after test completion)
func (r *TestReporter) LogTest(testID, testName string, passed bool, logFile string) {
	// This is now handled in PrintResultsTable
	// Keep for backwards compatibility but don't print
}

// PrintSummary prints the test summary with results table
func (r *TestReporter) PrintSummary() {
	r.mu.Lock()
	defer r.mu.Unlock()

	passed := 0
	failed := 0
	skipped := 0
	var totalDuration time.Duration

	for _, result := range r.results {
		switch result.Status {
		case "PASS":
			passed++
		case "FAIL":
			failed++
		case "SKIP":
			skipped++
		}
		totalDuration += result.Duration
	}

	total := passed + failed + skipped

	// Print Results Table
	fmt.Println()
	fmt.Printf("%s╔══════════════════════════════════════════════════════════════════════════════════╗%s\n", ColorBold, ColorReset)
	fmt.Printf("%s║                              TEST RESULTS                                        ║%s\n", ColorBold, ColorReset)
	fmt.Printf("%s╠════════════╦═══════════════════════════════════════════════════════╦═════════════╣%s\n", ColorBold, ColorReset)
	fmt.Printf("%s║   TEST ID  ║                      TEST NAME                        ║   STATUS    ║%s\n", ColorBold, ColorReset)
	fmt.Printf("%s╠════════════╬═══════════════════════════════════════════════════════╬═════════════╣%s\n", ColorBold, ColorReset)

	for _, result := range r.results {
		statusColor := ColorGreen
		statusSymbol := "✓ PASS"
		if result.Status == "FAIL" {
			statusColor = ColorRed
			statusSymbol = "✗ FAIL"
		} else if result.Status == "SKIP" {
			statusColor = ColorYellow
			statusSymbol = "⊘ SKIP"
		}

		// Truncate test name if too long
		testName := result.TestName
		if len(testName) > 53 {
			testName = testName[:50] + "..."
		}

		fmt.Printf("║ %-10s ║ %-53s ║ %s%-11s%s ║\n",
			result.TestID, testName, statusColor, statusSymbol, ColorReset)
	}

	fmt.Printf("%s╚════════════╩═══════════════════════════════════════════════════════╩═════════════╝%s\n", ColorBold, ColorReset)

	// Print Summary Box
	fmt.Println()
	fmt.Printf("%s┌─────────────────────────────────────┐%s\n", ColorBold, ColorReset)
	fmt.Printf("%s│             SUMMARY                 │%s\n", ColorBold, ColorReset)
	fmt.Printf("%s├─────────────────────────────────────┤%s\n", ColorBold, ColorReset)
	fmt.Printf("│  Total:    %-24d │\n", total)
	fmt.Printf("│  %sPassed:   %-24d%s │\n", ColorGreen, passed, ColorReset)
	if failed > 0 {
		fmt.Printf("│  %sFailed:   %-24d%s │\n", ColorRed, failed, ColorReset)
	} else {
		fmt.Printf("│  Failed:   %-24d │\n", failed)
	}
	if skipped > 0 {
		fmt.Printf("│  %sSkipped:  %-24d%s │\n", ColorYellow, skipped, ColorReset)
	} else {
		fmt.Printf("│  Skipped:  %-24d │\n", skipped)
	}
	fmt.Printf("│  Duration: %-24s │\n", totalDuration.Round(time.Second))
	fmt.Printf("%s└─────────────────────────────────────┘%s\n", ColorBold, ColorReset)

	fmt.Printf("\n%sLogs directory: %s%s\n", ColorCyan, r.logsDir, ColorReset)
}

// GetResults returns all test results
func (r *TestReporter) GetResults() []TestResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.results
}

// HasFailures returns true if any tests failed
func (r *TestReporter) HasFailures() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, result := range r.results {
		if result.Status == "FAIL" {
			return true
		}
	}
	return false
}
