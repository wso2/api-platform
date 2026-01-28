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

package coverage

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

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
	reportPrefix := c.config.GetReportPrefix()
	textReportPath := filepath.Join(c.config.OutputDir, reportPrefix+".txt")
	if err := c.generateTextReport(mergedDir, textReportPath); err != nil {
		log.Printf("Warning: Failed to generate text report: %v", err)
	}

	// Generate HTML report
	htmlReportPath := filepath.Join(c.config.OutputDir, "output", reportPrefix+".html")
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

	// Rewrite /build/ paths to be relative (works from source dir)
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

	// Run go tool cover from source directory so it can resolve module paths
	cmd := exec.Command("go", "tool", "cover", "-html="+absRewrittenFile, "-o="+absOutputPath)
	cmd.Dir = c.config.SourceDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go tool cover -html failed: %w", err)
	}

	log.Printf("HTML coverage report: %s", outputPath)
	return nil
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
	reportPrefix := c.config.GetReportPrefix()
	textFile := filepath.Join(c.config.OutputDir, reportPrefix+".txt")
	totalCoverage := c.getTotalCoverage(textFile)

	// Build coverage report
	report := CoverageReport{
		Timestamp:     time.Now().Format(time.RFC3339),
		TotalCoverage: totalCoverage,
		TotalPackages: len(packages),
		Packages:      packages,
	}

	// Write JSON report
	jsonPath := filepath.Join(c.config.OutputDir, "output", reportPrefix+"-report.json")
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

// printCoverageTable prints a formatted coverage summary table to stdout
func (c *CoverageCollector) printCoverageTable(report *CoverageReport) {
	if len(report.Packages) == 0 {
		fmt.Println("No coverage data available")
		return
	}

	fmt.Println()
	fmt.Println("+---------------------------------------------------------------------------+")
	fmt.Println("|                       CODE COVERAGE SUMMARY                               |")
	fmt.Println("+---------------------------------------------------------------------------+")

	// Package coverage table
	fmt.Println("|  PACKAGES                                                                 |")
	fmt.Println("+--------------------------------------------------------------+-----------+")
	fmt.Println("|  Package                                                     | Coverage  |")
	fmt.Println("+--------------------------------------------------------------+-----------+")

	for _, p := range report.Packages {
		pkg := TruncateString(p.Package, 60)
		fmt.Printf("| %-60s | %7.1f%%  |\n", pkg, p.Coverage)
	}

	fmt.Println("+---------------------------------------------------------------------------+")

	// Summary
	fmt.Println("|  SUMMARY                                                                  |")
	fmt.Println("+---------------------------------+----------------------------------------+")
	fmt.Println("|  Metric                         | Value                                  |")
	fmt.Println("+---------------------------------+----------------------------------------+")
	fmt.Printf("|  Total Packages                 | %38d |\n", report.TotalPackages)
	fmt.Printf("|  Total Coverage                 | %37.1f%% |\n", report.TotalCoverage)
	fmt.Println("+---------------------------------------------------------------------------+")
	fmt.Println()
}
