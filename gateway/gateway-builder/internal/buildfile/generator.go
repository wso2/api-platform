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

package buildfile

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
	"golang.org/x/mod/modfile"
	"gopkg.in/yaml.v3"
)

// BuildInfo represents the build info structure
type BuildInfo struct {
	BuildTimestamp  string       `json:"buildTimestamp"`
	BuilderVersion string       `json:"builderVersion"`
	OutputDir      string       `json:"outputDir"`
	Policies       []PolicyInfo `json:"policies"`
}

// PolicyInfo contains policy name and version
type PolicyInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// CreateBuildInfo creates a build info structure with build metadata
func CreateBuildInfo(
	builderVersion string,
	policies []*types.DiscoveredPolicy,
	outputDir string,
) *BuildInfo {
	slog.Info("Creating build info")

	info := &BuildInfo{
		BuildTimestamp:  time.Now().UTC().Format(time.RFC3339),
		BuilderVersion: builderVersion,
		OutputDir:      outputDir,
		Policies:       make([]PolicyInfo, 0, len(policies)),
	}

	for _, p := range policies {
		info.Policies = append(info.Policies, PolicyInfo{
			Name:    p.Name,
			Version: p.Version,
		})
	}

	slog.Info("Successfully created build info",
		"policyCount", len(policies))

	return info
}

// ToJSON converts the build info to a JSON string
func (m *BuildInfo) ToJSON() (string, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal build info to JSON: %w", err)
	}
	return string(data), nil
}

// WriteToFile writes the build info to a JSON file
func (m *BuildInfo) WriteToFile(path string) error {
	jsonData, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal build info to JSON: %w", err)
	}

	slog.Info("Writing build info to file", "path", path)

	if err := os.WriteFile(path, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write build info file: %w", err)
	}

	return nil
}

func WriteBuildLockWithVersions(buildFilePath string, discovered []*types.DiscoveredPolicy) error {
	data, err := os.ReadFile(buildFilePath)
	if err != nil {
		return fmt.Errorf("failed to read build file '%s': %w", buildFilePath, err)
	}

	var bf types.BuildFile
	if err := yaml.Unmarshal(data, &bf); err != nil {
		return fmt.Errorf("failed to parse build file YAML: %w", err)
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
		Version:  bf.Version,
		Policies: make([]lockEntry, 0, len(bf.Policies)),
	}

	buildFileDir := filepath.Dir(buildFilePath)

	for _, me := range bf.Policies {
		entry := lockEntry{Name: me.Name, FilePath: me.FilePath, Gomodule: me.Gomodule}

		candidates := discoveredByName[me.Name]
		var found *types.DiscoveredPolicy
		if len(candidates) == 1 {
			found = candidates[0]
		} else if me.FilePath != "" {
			relPath := filepath.Join(buildFileDir, me.FilePath)
			relAbs, _ := filepath.Abs(relPath)
			for _, c := range candidates {
				cAbs, _ := filepath.Abs(c.Path)
				if cAbs == relAbs {
					found = c
					break
				}
			}
		} else if me.Gomodule != "" {
			// me.Gomodule may be in the form "module/path" or "module/path@version"
			modPath := me.Gomodule
			modVersion := ""
			if strings.Contains(me.Gomodule, "@") {
				parts := strings.SplitN(me.Gomodule, "@", 2)
				modPath = parts[0]
				modVersion = parts[1]
			}

			for _, c := range candidates {
				// Prefer matching declared module path in candidate's go.mod
				if c.GoModPath != "" {
					if data, err := os.ReadFile(c.GoModPath); err == nil {
						if mf, err := modfile.Parse(c.GoModPath, data, nil); err == nil && mf.Module != nil {
							if mf.Module.Mod.Path == modPath {
								// If version specified, ensure it matches discovered policy version (normalize 'v' prefix)
								if modVersion == "" || strings.TrimPrefix(modVersion, "v") == strings.TrimPrefix(c.Version, "v") {
									found = c
									break
								}
							}
						}
					}
				}

				// Fallback to path comparison (support file paths as well)
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

	outPath := filepath.Join(buildFileDir, "build-lock.yaml")
	ydata, err := yaml.Marshal(&lock)
	if err != nil {
		return fmt.Errorf("failed to marshal lock YAML: %w", err)
	}

	slog.Info("Writing build lock with versions", "path", outPath)
	if err := os.WriteFile(outPath, ydata, 0600); err != nil {
		return fmt.Errorf("failed to write build lock file '%s': %w", outPath, err)
	}

	return nil
}
