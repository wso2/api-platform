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
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cucumber/godog"
)

// ReporterConfig holds test reporter configuration
type ReporterConfig struct {
	// OutputDir is the directory where reports are saved
	OutputDir string

	// ReportName is the base name for report files
	ReportName string
}

// DefaultReporterConfig returns the default reporter configuration
func DefaultReporterConfig() *ReporterConfig {
	return &ReporterConfig{
		OutputDir:  "reports",
		ReportName: "integration-test-results",
	}
}

// TestReport represents the complete test report
type TestReport struct {
	// Metadata
	Name      string    `json:"name"`
	Timestamp time.Time `json:"timestamp"`
	Duration  string    `json:"duration"`

	// Summary
	Summary TestSummary `json:"summary"`

	// Scenarios
	Scenarios []ScenarioResult `json:"scenarios"`

	// Environment info
	Environment EnvironmentInfo `json:"environment"`
}

// TestSummary contains aggregate test statistics
type TestSummary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// ScenarioResult represents a single scenario's execution result
type ScenarioResult struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	URI         string       `json:"uri"`
	Tags        []string     `json:"tags,omitempty"`
	Status      string       `json:"status"`
	Duration    string       `json:"duration"`
	Error       string       `json:"error,omitempty"`
	ErrorDetail *ErrorDetail `json:"errorDetail,omitempty"`
	StartTime   time.Time    `json:"startTime"`
	EndTime     time.Time    `json:"endTime"`
}

// ErrorDetail contains detailed error information for failed scenarios
type ErrorDetail struct {
	Message    string `json:"message"`
	Type       string `json:"type,omitempty"`
	StackTrace string `json:"stackTrace,omitempty"`
}

// EnvironmentInfo contains test environment details
type EnvironmentInfo struct {
	GoVersion    string            `json:"goVersion"`
	Platform     string            `json:"platform"`
	DockerImages map[string]string `json:"dockerImages,omitempty"`
}

// TestReporter manages test report generation
type TestReporter struct {
	config    *ReporterConfig
	startTime time.Time
	scenarios []ScenarioResult
	current   *ScenarioResult
	mu        sync.Mutex
}

// NewTestReporter creates a new TestReporter
func NewTestReporter(config *ReporterConfig) *TestReporter {
	return &TestReporter{
		config:    config,
		scenarios: make([]ScenarioResult, 0),
	}
}

// Setup prepares the report output directory
func (r *TestReporter) Setup() error {
	log.Println("Setting up test reporter...")

	// Create reports directory
	if err := os.MkdirAll(r.config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create reports directory: %w", err)
	}

	r.startTime = time.Now()
	log.Printf("Reports directory ready at %s", r.config.OutputDir)
	return nil
}

// StartScenario records the start of a scenario
func (r *TestReporter) StartScenario(sc *godog.Scenario) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tags := make([]string, len(sc.Tags))
	for i, tag := range sc.Tags {
		tags[i] = tag.Name
	}

	r.current = &ScenarioResult{
		ID:        sc.Id,
		Name:      sc.Name,
		URI:       sc.Uri,
		Tags:      tags,
		StartTime: time.Now(),
		Status:    "running",
	}
}

// EndScenario records the completion of a scenario
func (r *TestReporter) EndScenario(sc *godog.Scenario, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.current == nil {
		// Scenario wasn't properly started, create a new result
		tags := make([]string, len(sc.Tags))
		for i, tag := range sc.Tags {
			tags[i] = tag.Name
		}
		r.current = &ScenarioResult{
			ID:        sc.Id,
			Name:      sc.Name,
			URI:       sc.Uri,
			Tags:      tags,
			StartTime: time.Now(),
		}
	}

	r.current.EndTime = time.Now()
	r.current.Duration = r.current.EndTime.Sub(r.current.StartTime).String()

	if err != nil {
		r.current.Status = "failed"
		r.current.Error = err.Error()
		r.current.ErrorDetail = &ErrorDetail{
			Message: err.Error(),
			Type:    fmt.Sprintf("%T", err),
		}
	} else {
		r.current.Status = "passed"
	}

	r.scenarios = append(r.scenarios, *r.current)
	r.current = nil
}

// GenerateReport generates the final test report
func (r *TestReporter) GenerateReport() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Calculate summary
	summary := TestSummary{
		Total: len(r.scenarios),
	}
	for _, s := range r.scenarios {
		switch s.Status {
		case "passed":
			summary.Passed++
		case "failed":
			summary.Failed++
		case "skipped":
			summary.Skipped++
		}
	}

	duration := time.Since(r.startTime)

	// Build the report
	report := TestReport{
		Name:      "Gateway Integration Tests",
		Timestamp: r.startTime,
		Duration:  duration.String(),
		Summary:   summary,
		Scenarios: r.scenarios,
		Environment: EnvironmentInfo{
			GoVersion: getGoVersion(),
			Platform:  getPlatform(),
		},
	}

	// Generate JSON report
	jsonPath := filepath.Join(r.config.OutputDir, r.config.ReportName+".json")
	if err := r.writeJSONReport(jsonPath, &report); err != nil {
		return err
	}

	// Print summary table to stdout
	r.printSummaryTable(&report)

	log.Printf("Test report saved to: %s", jsonPath)

	return nil
}

// printSummaryTable prints a formatted summary table to stdout
func (r *TestReporter) printSummaryTable(report *TestReport) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                       TEST RESULTS SUMMARY                               ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Suite: %-64s ║\n", report.Name)
	fmt.Printf("║  Duration: %-61s ║\n", formatDuration(parseDuration(report.Duration)))
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════╣")

	// Scenario results table
	fmt.Println("║  SCENARIOS                                                               ║")
	fmt.Println("╠────────────────────────────────────────────────────┬────────┬────────────╣")
	fmt.Println("║  Name                                              │ Status │  Duration  ║")
	fmt.Println("╠────────────────────────────────────────────────────┼────────┼────────────╣")

	for _, s := range report.Scenarios {
		name := truncateString(s.Name, 49)
		status := formatStatus(s.Status)
		duration := formatDuration(parseDuration(s.Duration))
		fmt.Printf("║  %-49s │ %-6s │ %10s ║\n", name, status, duration)

		// Print error if failed
		if s.Status == "failed" && s.Error != "" {
			errMsg := truncateString(s.Error, 68)
			fmt.Printf("║    └─ Error: %-59s ║\n", errMsg)
		}
	}

	fmt.Println("╠══════════════════════════════════════════════════════════════════════════╣")

	// Summary statistics
	fmt.Println("║  SUMMARY                                                                 ║")
	fmt.Println("╠─────────────┬────────────┬────────────┬────────────┬─────────────────────╣")
	fmt.Println("║    Total    │   Passed   │   Failed   │  Skipped   │       Result        ║")
	fmt.Println("╠─────────────┼────────────┼────────────┼────────────┼─────────────────────╣")

	result := "PASS"
	if report.Summary.Failed > 0 {
		result = "FAIL"
	}

	fmt.Printf("║     %3d     │     %3d    │     %3d    │     %3d    │        %-4s         ║\n",
		report.Summary.Total,
		report.Summary.Passed,
		report.Summary.Failed,
		report.Summary.Skipped,
		result)

	fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
}

// truncateString truncates a string to maxLen and adds ellipsis if needed
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// formatStatus returns a formatted status string
func formatStatus(status string) string {
	switch status {
	case "passed":
		return "PASS"
	case "failed":
		return "FAIL"
	case "skipped":
		return "SKIP"
	default:
		return status
	}
}

// parseDuration parses a duration string and returns the duration
func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d
}

// formatDuration formats a duration as seconds with 1 decimal place
func formatDuration(d time.Duration) string {
	seconds := d.Seconds()
	return fmt.Sprintf("%.1fs", seconds)
}

// writeJSONReport writes the report as JSON
func (r *TestReporter) writeJSONReport(path string, report *TestReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	return nil
}

// GetSummary returns the current test summary
func (r *TestReporter) GetSummary() TestSummary {
	r.mu.Lock()
	defer r.mu.Unlock()

	summary := TestSummary{
		Total: len(r.scenarios),
	}
	for _, s := range r.scenarios {
		switch s.Status {
		case "passed":
			summary.Passed++
		case "failed":
			summary.Failed++
		case "skipped":
			summary.Skipped++
		}
	}
	return summary
}

// getGoVersion returns the Go version
func getGoVersion() string {
	// This is a simplified version - in practice you might use runtime.Version()
	return "1.25.1"
}

// getPlatform returns the platform info
func getPlatform() string {
	// This is a simplified version
	return fmt.Sprintf("%s/%s", os.Getenv("GOOS"), os.Getenv("GOARCH"))
}
