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
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// CoverageConfig holds coverage collection configuration
type CoverageConfig struct {
	// OutputDir is the directory where coverage data is stored
	OutputDir string

	// Services lists the services to collect coverage from
	Services []string

	// SourceDir is the absolute path to the source code directory
	SourceDir string

	// ContainerPath is the path prefix used inside the container (e.g., "/build/")
	ContainerPath string

	// ModulePrefixes are the module prefixes to strip when displaying package names
	ModulePrefixes []string
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

// Cleanup removes temporary coverage files
func (c *CoverageCollector) Cleanup() {
	// Optionally clean up per-service directories after merge
	// For now, keep them for debugging
	log.Println("Coverage data preserved for analysis")
}
