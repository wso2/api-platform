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
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// rewriteCoveragePaths rewrites container paths to local absolute paths
// and filters out entries for files that don't exist in the source tree
func (c *CoverageCollector) rewriteCoveragePaths(inputPath, outputPath, sourceDir string) error {
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
	skippedFiles := make(map[string]bool)

	for scanner.Scan() {
		line := scanner.Text()

		// Always write the mode line
		if strings.HasPrefix(line, "mode:") {
			fmt.Fprintln(output, line)
			continue
		}

		// Replace container path with absolute path to source directory
		newLine := strings.Replace(line, c.config.ContainerPath, sourceDir+"/", 1)

		// Extract the file path from the coverage line (format: path:line.col,line.col count hits)
		if colonIdx := strings.Index(newLine, ":"); colonIdx > 0 {
			filePath := newLine[:colonIdx]

			// Check if this file exists in the source directory
			// First check if the file is referenced with the module path
			localPath := filePath
			for _, prefix := range c.config.ModulePrefixes {
				if strings.HasPrefix(filePath, prefix) {
					// Convert module path to local path
					relativePath := strings.TrimPrefix(filePath, prefix)
					localPath = filepath.Join(sourceDir, relativePath)
					// Rewrite the output line to use the local path
					newLine = localPath + newLine[colonIdx:]
					filePath = localPath
					break
				}
			}

			// Check if file exists
			if _, err := os.Stat(localPath); os.IsNotExist(err) {
				// Track skipped files (only log once per file)
				if !skippedFiles[filePath] {
					skippedFiles[filePath] = true
				}
				continue // Skip this line - file doesn't exist in source tree
			}
		}

		fmt.Fprintln(output, newLine)
	}

	if len(skippedFiles) > 0 {
		log.Printf("Skipped %d generated files not in source tree", len(skippedFiles))
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	return nil
}

// getTotalCoverageFromFile calculates total coverage from a text coverage file with rewritten paths
func (c *CoverageCollector) getTotalCoverageFromFile(textFile, sourceDir string) float64 {
	absTextFile, err := filepath.Abs(textFile)
	if err != nil {
		return 0
	}

	// Run go tool cover -func to get total
	cmd := exec.Command("go", "tool", "cover", "-func="+absTextFile)
	cmd.Dir = sourceDir
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
	// Remove common prefixes using configured module prefixes
	for _, prefix := range c.config.ModulePrefixes {
		if strings.HasPrefix(pkg, prefix) {
			return strings.TrimPrefix(pkg, prefix)
		}
	}
	return pkg
}

// TruncateString truncates a string to maxLen and adds ellipsis if needed
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
