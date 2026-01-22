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

package manifest

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
	"gopkg.in/yaml.v3"
)

// Manifest represents the build manifest structure
type Manifest struct {
	BuildTimestamp string       `json:"buildTimestamp"`
	BuilderVersion string       `json:"builderVersion"`
	OutputDir      string       `json:"outputDir"`
	Policies       []PolicyInfo `json:"policies"`
}

// PolicyInfo contains policy name and version
type PolicyInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// CreateManifest creates a manifest structure with build metadata
func CreateManifest(
	builderVersion string,
	policies []*types.DiscoveredPolicy,
	outputDir string,
) *Manifest {
	slog.Info("Creating build manifest")

	// Create manifest structure
	manifest := &Manifest{
		BuildTimestamp: time.Now().UTC().Format(time.RFC3339),
		BuilderVersion: builderVersion,
		OutputDir:      outputDir,
		Policies:       make([]PolicyInfo, 0, len(policies)),
	}

	// Add policies
	for _, p := range policies {
		manifest.Policies = append(manifest.Policies, PolicyInfo{
			Name:    p.Name,
			Version: p.Version,
		})
	}

	slog.Info("Successfully created build manifest",
		"policyCount", len(policies))

	return manifest
}

// ToJSON converts the manifest to a JSON string
func (m *Manifest) ToJSON() (string, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal manifest to JSON: %w", err)
	}
	return string(data), nil
}

// WriteToFile writes the manifest to a JSON file
func (m *Manifest) WriteToFile(path string) error {
	jsonData, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest to JSON: %w", err)
	}

	slog.Info("Writing build manifest to file", "path", path)

	if err := os.WriteFile(path, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write manifest file: %w", err)
	}

	return nil
}

func WriteManifestLockWithVersions(manifestFilePath string, discovered []*types.DiscoveredPolicy) error {
	data, err := os.ReadFile(manifestFilePath)
	if err != nil {
		return fmt.Errorf("failed to read manifest file '%s': %w", manifestFilePath, err)
	}

	var manifest types.PolicyManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest YAML: %w", err)
	}

	discoveredByName := make(map[string][]*types.DiscoveredPolicy)
	for _, p := range discovered {
		discoveredByName[p.Name] = append(discoveredByName[p.Name], p)
	}

	type lockEntry struct {
		Name     string `yaml:"name"`
		Version  string `yaml:"version,omitempty"`
		FilePath string `yaml:"filePath,omitempty"`
		Gomodule string `yaml:"gomodule,omitempty"`
	}

	lock := struct {
		Version  string      `yaml:"version"`
		Policies []lockEntry `yaml:"policies"`
	}{
		Version:  manifest.Version,
		Policies: make([]lockEntry, 0, len(manifest.Policies)),
	}

	manifestDir := filepath.Dir(manifestFilePath)

	for _, me := range manifest.Policies {
		entry := lockEntry{Name: me.Name, FilePath: me.FilePath, Gomodule: me.Gomodule}

		candidates := discoveredByName[me.Name]
		var found *types.DiscoveredPolicy
		if len(candidates) == 1 {
			found = candidates[0]
		} else if me.FilePath != "" {
			relPath := filepath.Join(manifestDir, me.FilePath)
			relAbs, _ := filepath.Abs(relPath)
			for _, c := range candidates {
				cAbs, _ := filepath.Abs(c.Path)
				if cAbs == relAbs {
					found = c
					break
				}
			}
		} else if me.Gomodule != "" {
			for _, c := range candidates {
				cAbs, _ := filepath.Abs(c.Path)
				if c.Path != "" && (c.Path == me.Gomodule || cAbs == c.Path) {
					found = c
					break
				}
			}
		}

		if found == nil {
			return fmt.Errorf("failed to determine version for policy '%s'", me.Name)
		}

		entry.Version = found.Version
		lock.Policies = append(lock.Policies, entry)
	}

	outPath := filepath.Join(manifestDir, "policy-manifest-lock.yaml")
	ydata, err := yaml.Marshal(&lock)
	if err != nil {
		return fmt.Errorf("failed to marshal lock YAML: %w", err)
	}

	slog.Info("Writing policy lock with versions", "path", outPath)
	if err := os.WriteFile(outPath, ydata, 0644); err != nil {
		return fmt.Errorf("failed to write policy lock file '%s': %w", outPath, err)
	}

	return nil
}
