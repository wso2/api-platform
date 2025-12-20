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
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CoverageConfig holds coverage collection configuration
type CoverageConfig struct {
	// OutputDir is the directory where coverage data is stored
	OutputDir string

	// Services lists the services to collect coverage from
	Services []string
}

// DefaultCoverageConfig returns the default coverage configuration
func DefaultCoverageConfig() *CoverageConfig {
	return &CoverageConfig{
		OutputDir: "coverage",
		Services:  []string{"gateway-controller"},
	}
}

// CoverageCollector manages coverage data collection and report generation
type CoverageCollector struct {
	config *CoverageConfig
}

// NewCoverageCollector creates a new CoverageCollector
func NewCoverageCollector(config *CoverageConfig) *CoverageCollector {
	return &CoverageCollector{
		config: config,
	}
}

// cleanCoverageDirectory removes existing files in the coverage directory (preserving .gitkeep)
func (c *CoverageCollector) cleanCoverageDirectory() error {
	entries, err := os.ReadDir(c.config.OutputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist, nothing to clean
		}
		return fmt.Errorf("failed to read coverage directory: %w", err)
	}

	for _, entry := range entries {
		// Preserve .gitkeep file
		if entry.Name() == ".gitkeep" {
			continue
		}
		path := filepath.Join(c.config.OutputDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("failed to remove %s: %w", path, err)
		}
	}

	log.Println("Cleaned existing coverage data")
	return nil
}

// Setup prepares the coverage output directories
func (c *CoverageCollector) Setup() error {
	log.Println("Setting up coverage collection directories...")

	// Delete existing coverage folder contents
	if err := c.cleanCoverageDirectory(); err != nil {
		log.Printf("Warning: Failed to clean coverage directory: %v", err)
	}

	// Create main coverage directory
	if err := os.MkdirAll(c.config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create coverage directory: %w", err)
	}

	// Create output directory for reports
	outputDir := filepath.Join(c.config.OutputDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create coverage output directory: %w", err)
	}

	// Create per-service directories
	for _, service := range c.config.Services {
		serviceDir := filepath.Join(c.config.OutputDir, service)
		if err := os.MkdirAll(serviceDir, 0755); err != nil {
			return fmt.Errorf("failed to create coverage directory for %s: %w", service, err)
		}
	}

	log.Printf("Coverage directories created at %s", c.config.OutputDir)
	return nil
}

// MergeAndGenerateReport merges coverage data and generates reports
func (c *CoverageCollector) MergeAndGenerateReport() error {
	log.Println("Merging coverage data and generating reports...")

	// Wait a moment for coverage files to be flushed
	time.Sleep(2 * time.Second)

	// Merge coverage data from all services
	mergedDir := filepath.Join(c.config.OutputDir, "merged")
	if err := os.MkdirAll(mergedDir, 0755); err != nil {
		return fmt.Errorf("failed to create merged coverage directory: %w", err)
	}

	// Collect all coverage directories
	var coverDirs []string
	for _, service := range c.config.Services {
		serviceDir := filepath.Join(c.config.OutputDir, service)
		if _, err := os.Stat(serviceDir); err == nil {
			// Check if directory has any coverage files
			entries, _ := os.ReadDir(serviceDir)
			if len(entries) > 0 {
				coverDirs = append(coverDirs, serviceDir)
				log.Printf("Found coverage data for %s", service)
			} else {
				log.Printf("No coverage data found for %s", service)
			}
		}
	}

	if len(coverDirs) == 0 {
		log.Println("No coverage data found to merge")
		return nil
	}

	// Use go tool covdata to merge coverage files
	if err := c.mergeCoverageData(coverDirs, mergedDir); err != nil {
		log.Printf("Warning: Failed to merge coverage data: %v", err)
		// Continue with individual service coverage if merge fails
	}

	// Generate text report
	textReportPath := filepath.Join(c.config.OutputDir, "coverage.txt")
	if err := c.generateTextReport(mergedDir, textReportPath); err != nil {
		log.Printf("Warning: Failed to generate text report: %v", err)
	}

	// Generate HTML report
	htmlReportPath := filepath.Join(c.config.OutputDir, "output", "coverage.html")
	if err := c.generateHTMLReport(mergedDir, htmlReportPath); err != nil {
		log.Printf("Warning: Failed to generate HTML report: %v", err)
	}

	// Calculate and log coverage percentage
	if err := c.logCoveragePercentage(mergedDir); err != nil {
		log.Printf("Warning: Failed to calculate coverage percentage: %v", err)
	}

	log.Printf("Coverage reports generated in %s/output", c.config.OutputDir)
	return nil
}

// mergeCoverageData merges coverage data from multiple directories
func (c *CoverageCollector) mergeCoverageData(inputDirs []string, outputDir string) error {
	args := []string{"tool", "covdata", "merge"}
	for _, dir := range inputDirs {
		args = append(args, "-i="+dir)
	}
	args = append(args, "-o="+outputDir)

	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go tool covdata merge failed: %w", err)
	}

	return nil
}

// generateTextReport generates a text coverage report
func (c *CoverageCollector) generateTextReport(coverDir, outputPath string) error {
	// Convert to text format
	cmd := exec.Command("go", "tool", "covdata", "textfmt",
		"-i="+coverDir,
		"-o="+outputPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go tool covdata textfmt failed: %w", err)
	}

	log.Printf("Text coverage report: %s", outputPath)
	return nil
}

// generateHTMLReport generates an HTML coverage report
func (c *CoverageCollector) generateHTMLReport(coverDir, outputPath string) error {
	// First convert to text format
	textFile := filepath.Join(c.config.OutputDir, "coverage_temp.txt")
	if err := c.generateTextReport(coverDir, textFile); err != nil {
		return err
	}

	// Rewrite /build/ paths to be relative (works from gateway-controller dir)
	rewrittenFile := filepath.Join(c.config.OutputDir, "coverage_rewritten.txt")
	if err := c.rewriteCoveragePaths(textFile, rewrittenFile); err != nil {
		return err
	}

	// Get absolute paths for the command
	absRewrittenFile, err := filepath.Abs(rewrittenFile)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for text file: %w", err)
	}
	absOutputPath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for output: %w", err)
	}

	// Get gateway-controller directory (where go.mod is)
	controllerDir, err := filepath.Abs("../gateway-controller")
	if err != nil {
		return fmt.Errorf("failed to get gateway-controller path: %w", err)
	}

	// Run go tool cover from gateway-controller directory so it can resolve module paths
	cmd := exec.Command("go", "tool", "cover", "-html="+absRewrittenFile, "-o="+absOutputPath)
	cmd.Dir = controllerDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go tool cover -html failed: %w", err)
	}

	log.Printf("HTML coverage report: %s", outputPath)
	return nil
}

// rewriteCoveragePaths rewrites container paths (/build/) to local absolute paths
func (c *CoverageCollector) rewriteCoveragePaths(inputPath, outputPath string) error {
	// Get absolute path to gateway-controller source
	controllerDir, err := filepath.Abs("../gateway-controller")
	if err != nil {
		return fmt.Errorf("failed to get gateway-controller path: %w", err)
	}

	input, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer input.Close()

	output, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer output.Close()

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		// Replace /build/ with absolute path to gateway-controller
		line = strings.Replace(line, "/build/", controllerDir+"/", 1)
		fmt.Fprintln(output, line)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	return nil
}

// CoverageReport represents the complete coverage report
type CoverageReport struct {
	Timestamp     string            `json:"timestamp"`
	TotalCoverage float64           `json:"totalCoverage"`
	TotalPackages int               `json:"totalPackages"`
	Packages      []PackageCoverage `json:"packages"`
}

// PackageCoverage represents coverage for a single package
type PackageCoverage struct {
	Package  string  `json:"package"`
	Coverage float64 `json:"coverage"`
}

// logCoveragePercentage calculates and logs the coverage percentage
func (c *CoverageCollector) logCoveragePercentage(coverDir string) error {
	// Get per-package coverage
	cmd := exec.Command("go", "tool", "covdata", "percent", "-i="+coverDir)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("go tool covdata percent failed: %w", err)
	}

	// Parse coverage output
	packages := c.parseCoverageOutput(string(output))

	// Get total coverage from the text coverage file
	textFile := filepath.Join(c.config.OutputDir, "coverage.txt")
	totalCoverage := c.getTotalCoverage(textFile)

	// Build coverage report
	report := CoverageReport{
		Timestamp:     time.Now().Format(time.RFC3339),
		TotalCoverage: totalCoverage,
		TotalPackages: len(packages),
		Packages:      packages,
	}

	// Write JSON report
	jsonPath := filepath.Join(c.config.OutputDir, "output", "coverage-report.json")
	if err := c.writeCoverageJSON(jsonPath, &report); err != nil {
		return err
	}

	// Print summary table from JSON
	c.printCoverageTable(&report)

	log.Printf("Coverage report saved to: %s", jsonPath)
	return nil
}

// writeCoverageJSON writes the coverage report as JSON
func (c *CoverageCollector) writeCoverageJSON(path string, report *CoverageReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal coverage report: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write coverage report: %w", err)
	}

	return nil
}

// getTotalCoverage calculates total coverage from the coverage.txt file
func (c *CoverageCollector) getTotalCoverage(textFile string) float64 {
	// Get gateway-controller directory (where go.mod is)
	controllerDir, err := filepath.Abs("../gateway-controller")
	if err != nil {
		return 0
	}

	absTextFile, err := filepath.Abs(textFile)
	if err != nil {
		return 0
	}

	// Rewrite paths for go tool cover
	rewrittenFile := filepath.Join(c.config.OutputDir, "coverage_total_temp.txt")
	if err := c.rewriteCoveragePaths(absTextFile, rewrittenFile); err != nil {
		return 0
	}
	defer os.Remove(rewrittenFile)

	absRewrittenFile, err := filepath.Abs(rewrittenFile)
	if err != nil {
		return 0
	}

	// Run go tool cover -func to get total
	cmd := exec.Command("go", "tool", "cover", "-func="+absRewrittenFile)
	cmd.Dir = controllerDir
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	// Parse the last line which contains total: XX.X%
	lines := strings.Split(string(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "total:") {
			// Extract percentage from "total:			XX.X%"
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				pctStr := strings.TrimSuffix(parts[len(parts)-1], "%")
				var pct float64
				fmt.Sscanf(pctStr, "%f", &pct)
				return pct
			}
		}
	}
	return 0
}

// printCoverageTable prints a formatted coverage summary table to stdout
func (c *CoverageCollector) printCoverageTable(report *CoverageReport) {
	if len(report.Packages) == 0 {
		fmt.Println("No coverage data available")
		return
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                       CODE COVERAGE SUMMARY                              ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════╣")

	// Package coverage table
	fmt.Println("║  PACKAGES                                                                ║")
	fmt.Println("╠──────────────────────────────────────────────────────────────┬───────────╣")
	fmt.Println("║  Package                                                     │ Coverage  ║")
	fmt.Println("╠──────────────────────────────────────────────────────────────┼───────────╣")

	for _, p := range report.Packages {
		pkg := truncateCoverageString(p.Package, 60)
		fmt.Printf("║ %-60s │ %7.1f%%  ║\n", pkg, p.Coverage)
	}

	fmt.Println("╠══════════════════════════════════════════════════════════════════════════╣")

	// Summary
	fmt.Println("║  SUMMARY                                                                 ║")
	fmt.Println("╠─────────────────────────────────┬────────────────────────────────────────╣")
	fmt.Println("║  Metric                         │ Value                                  ║")
	fmt.Println("╠─────────────────────────────────┼────────────────────────────────────────╣")
	fmt.Printf("║  Total Packages                 │ %38d ║\n", report.TotalPackages)
	fmt.Printf("║  Total Coverage                 │ %37.1f%% ║\n", report.TotalCoverage)
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
}

// parseCoverageOutput parses the output from go tool covdata percent
func (c *CoverageCollector) parseCoverageOutput(output string) []PackageCoverage {
	var packages []PackageCoverage
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Format: "package/path\t\tcoverage: XX.X% of statements"
		// Find the coverage percentage using regex-like parsing
		coverageIdx := strings.Index(line, "coverage:")
		if coverageIdx == -1 {
			// No coverage info (e.g., package with no statements)
			continue
		}

		// Extract package name (everything before "coverage:")
		// Handle case where multiple packages are on same line (pkg without statements + pkg with statements)
		pkgPart := strings.TrimSpace(line[:coverageIdx])

		// Find the last package path (in case multiple are concatenated)
		// Look for the last occurrence of "github.com/" or similar package prefix
		pkg := pkgPart
		if lastIdx := strings.LastIndex(pkgPart, "github.com/"); lastIdx > 0 {
			pkg = strings.TrimSpace(pkgPart[lastIdx:])
		}

		// Extract percentage from "coverage: XX.X% of statements"
		coverPart := line[coverageIdx:]
		coverPart = strings.TrimPrefix(coverPart, "coverage:")
		coverPart = strings.TrimSpace(coverPart)
		coverPart = strings.TrimSuffix(coverPart, "% of statements")
		coverPart = strings.TrimSuffix(coverPart, " of statements")
		coverPart = strings.TrimSuffix(coverPart, "%")
		coverPart = strings.TrimSpace(coverPart)

		var pct float64
		fmt.Sscanf(coverPart, "%f", &pct)

		// Shorten package name by removing common prefix
		pkg = c.shortenPackageName(pkg)

		packages = append(packages, PackageCoverage{
			Package:  pkg,
			Coverage: pct,
		})
	}

	return packages
}

// shortenPackageName removes common prefixes to make package names more readable
func (c *CoverageCollector) shortenPackageName(pkg string) string {
	// Remove common prefixes
	prefixes := []string{
		"github.com/wso2/api-platform/gateway/gateway-controller/",
		"github.com/wso2/api-platform/gateway/",
		"github.com/wso2/api-platform/",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(pkg, prefix) {
			return strings.TrimPrefix(pkg, prefix)
		}
	}
	return pkg
}

// truncateCoverageString truncates a string to maxLen and adds ellipsis if needed
func truncateCoverageString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// Cleanup removes temporary coverage files
func (c *CoverageCollector) Cleanup() {
	// Optionally clean up per-service directories after merge
	// For now, keep them for debugging
	log.Println("Coverage data preserved for analysis")
}
