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

// Setup prepares the coverage output directories
func (c *CoverageCollector) Setup() error {
	log.Println("Setting up coverage collection directories...")

	// Create main coverage directory
	if err := os.MkdirAll(c.config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create coverage directory: %w", err)
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
	htmlReportPath := filepath.Join(c.config.OutputDir, "coverage.html")
	if err := c.generateHTMLReport(mergedDir, htmlReportPath); err != nil {
		log.Printf("Warning: Failed to generate HTML report: %v", err)
	}

	// Calculate and log coverage percentage
	if err := c.logCoveragePercentage(mergedDir); err != nil {
		log.Printf("Warning: Failed to calculate coverage percentage: %v", err)
	}

	log.Printf("Coverage reports generated in %s", c.config.OutputDir)
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
	defer os.Remove(textFile)

	// Rewrite /build/ paths to be relative (works from gateway-controller dir)
	rewrittenFile := filepath.Join(c.config.OutputDir, "coverage_rewritten.txt")
	if err := c.rewriteCoveragePaths(textFile, rewrittenFile); err != nil {
		return err
	}
	defer os.Remove(rewrittenFile)

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

// logCoveragePercentage calculates and logs the coverage percentage
func (c *CoverageCollector) logCoveragePercentage(coverDir string) error {
	cmd := exec.Command("go", "tool", "covdata", "percent", "-i="+coverDir)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("go tool covdata percent failed: %w", err)
	}

	log.Printf("Coverage summary:\n%s", string(output))
	return nil
}

// Cleanup removes temporary coverage files
func (c *CoverageCollector) Cleanup() {
	// Optionally clean up per-service directories after merge
	// For now, keep them for debugging
	log.Println("Coverage data preserved for analysis")
}
