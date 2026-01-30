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

// MergeAndGenerateReport generates separate coverage reports for each service
func (c *CoverageCollector) MergeAndGenerateReport() error {
	log.Println("Generating coverage reports...")
	time.Sleep(2 * time.Second)

	reportPrefix := c.config.GetReportPrefix()

	for _, service := range c.config.Services {
		sourceDir := c.config.GetSourceDir(service)
		if sourceDir == "" {
			log.Printf("Warning: No source directory configured for service %s, skipping", service)
			continue
		}

		serviceDir := filepath.Join(c.config.OutputDir, service)
		if _, err := os.Stat(serviceDir); err != nil {
			log.Printf("No coverage directory for %s", service)
			continue
		}

		entries, _ := os.ReadDir(serviceDir)
		if len(entries) == 0 {
			log.Printf("No coverage data found for %s", service)
			continue
		}

		log.Printf("Found coverage data for %s", service)
		servicePrefix := fmt.Sprintf("%s-%s", reportPrefix, service)

		// Step 1: Generate raw text report to temp file
		rawTextFile := filepath.Join(c.config.OutputDir, "coverage_raw_temp.txt")
		if err := c.generateTextReport(serviceDir, rawTextFile); err != nil {
			log.Printf("Warning: Failed to generate text report for %s: %v", service, err)
			continue
		}

		// Step 2: Rewrite paths directly to final text report location
		textReportPath := filepath.Join(c.config.OutputDir, "output", "txt", servicePrefix+".txt")
		if err := c.rewriteCoveragePaths(rawTextFile, textReportPath, sourceDir); err != nil {
			log.Printf("Warning: Failed to rewrite coverage paths for %s: %v", service, err)
			os.Remove(rawTextFile)
			continue
		}
		os.Remove(rawTextFile)
		log.Printf("Text coverage report: %s", textReportPath)

		// Step 3: Generate HTML report using final text report
		htmlReportPath := filepath.Join(c.config.OutputDir, "output", servicePrefix+".html")
		if err := c.generateHTMLReportFromTextFile(textReportPath, htmlReportPath, sourceDir); err != nil {
			log.Printf("Warning: Failed to generate HTML report for %s: %v", service, err)
		}

		// Get per-package coverage and generate JSON report
		cmd := exec.Command("go", "tool", "covdata", "percent", "-i="+serviceDir)
		output, err := cmd.Output()
		if err != nil {
			log.Printf("Warning: Failed to get coverage percent for %s: %v", service, err)
			continue
		}

		// Step 4: Get total coverage using final text report (already has rewritten paths)
		packages := c.parseCoverageOutput(string(output))
		totalCoverage := c.getTotalCoverageFromFile(textReportPath, sourceDir)
		report := CoverageReport{
			Timestamp:     time.Now().Format(time.RFC3339),
			TotalCoverage: totalCoverage,
			TotalPackages: len(packages),
			Packages:      packages,
		}

		jsonPath := filepath.Join(c.config.OutputDir, "output", servicePrefix+"-report.json")
		if err := c.writeCoverageJSON(jsonPath, &report); err != nil {
			log.Printf("Warning: Failed to write JSON report for %s: %v", service, err)
		}

		// Print summary for this service
		fmt.Printf("\n=== Coverage for %s ===\n", service)
		c.printCoverageTable(&report)
	}

	log.Printf("Coverage reports generated in %s", c.config.OutputDir)
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

	return nil
}

// generateHTMLReportFromTextFile generates an HTML coverage report from a pre-rewritten text file
func (c *CoverageCollector) generateHTMLReportFromTextFile(textFile, outputPath, sourceDir string) error {
	// Get absolute paths for the command
	absTextFile, err := filepath.Abs(textFile)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for text file: %w", err)
	}
	absOutputPath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for output: %w", err)
	}

	// Run go tool cover from source directory so it can resolve module paths
	cmd := exec.Command("go", "tool", "cover", "-html="+absTextFile, "-o="+absOutputPath)
	cmd.Dir = sourceDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go tool cover -html failed: %w", err)
	}

	log.Printf("HTML coverage report: %s", outputPath)
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
