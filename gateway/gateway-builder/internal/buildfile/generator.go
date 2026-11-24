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
	"regexp"
	"strings"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-builder/internal/discovery"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
	"golang.org/x/mod/modfile"
	"gopkg.in/yaml.v3"
)

// BuildInfo represents the build info structure
type BuildInfo struct {
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

// CreateBuildInfo creates a build info structure with build metadata
func CreateBuildInfo(
	builderVersion string,
	policies []*types.DiscoveredPolicy,
	outputDir string,
) *BuildInfo {
	slog.Info("Creating build info")

	info := &BuildInfo{
		BuildTimestamp: time.Now().UTC().Format(time.RFC3339),
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

func WriteBuildManifestWithVersions(buildFilePath string, discovered []*types.DiscoveredPolicy) error {
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
		Name       string `yaml:"name"`
		Version    string `yaml:"version,omitempty"`
		FilePath   string `yaml:"filePath,omitempty"`
		Gomodule   string `yaml:"gomodule,omitempty"`
		PipPackage string `yaml:"pipPackage,omitempty"`
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
		entry := lockEntry{Name: me.Name, FilePath: me.FilePath, Gomodule: me.Gomodule, PipPackage: me.PipPackage}

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

			// A policy can be built at more than one major (backend-jwt@v1 and
			// backend-jwt/v2@v2), so a candidate is taken only when it is the module
			// the entry names and its version satisfies the entry's selector. There is
			// no looser fallback: picking another candidate would record that
			// candidate's version against this entry.
			for _, c := range candidates {
				if candidateModulePath(c) != modPath {
					continue
				}
				if modVersion != "" && !versionSelectorMatches(modVersion, candidateVersion(c)) {
					continue
				}
				found = c
				break
			}
		} else if me.PipPackage != "" {
			for _, c := range candidates {
				if c.Runtime == "python" && c.IsPipPackage && c.OriginalPipSpec == me.PipPackage {
					found = c
					break
				}
			}
			if found == nil {
				for _, c := range candidates {
					if c.Runtime == "python" && c.IsPipPackage {
						found = c
						break
					}
				}
			}
		}

		if found == nil {
			return fmt.Errorf("failed to determine version for policy '%s'", me.Name)
		}

		entry.Version = found.Version
		if found.IsPipPackage && found.PipSpec != "" {
			entry.PipPackage = discovery.SanitizePipSpec(found.PipSpec)
		}
		lock.Policies = append(lock.Policies, entry)
	}

	outPath := filepath.Join(buildFileDir, "build-manifest.yaml")
	ydata, err := yaml.Marshal(&lock)
	if err != nil {
		return fmt.Errorf("failed to marshal lock YAML: %w", err)
	}

	slog.Info("Writing build manifest with versions", "path", outPath)
	if err := os.WriteFile(outPath, ydata, 0644); err != nil {
		return fmt.Errorf("failed to write build manifest file '%s': %w", outPath, err)
	}

	return nil
}

// candidateModulePath is the Go module path a discovered policy was resolved as:
// the path discovery recorded, else the module declared in its go.mod.
func candidateModulePath(c *types.DiscoveredPolicy) string {
	if c.GoModulePath != "" {
		return c.GoModulePath
	}
	if c.GoModPath == "" {
		return ""
	}
	data, err := os.ReadFile(c.GoModPath)
	if err != nil {
		return ""
	}
	mf, err := modfile.Parse(c.GoModPath, data, nil)
	if err != nil || mf.Module == nil {
		return ""
	}
	return mf.Module.Mod.Path
}

// candidateVersion is the version a discovered policy was resolved at: the module
// version for a gomodule entry, else the policy definition's version.
func candidateVersion(c *types.DiscoveredPolicy) string {
	if c.GoModuleVersion != "" {
		return c.GoModuleVersion
	}
	return c.Version
}

var semverSelector = regexp.MustCompile(`^\d+(\.\d+){0,2}$`)

// versionSelectorMatches reports whether version satisfies the version part of a
// gomodule entry. A full version must match exactly; a major-only (v2) or
// major.minor (v2.1) selector matches any version in that line, as `go get`
// resolves it. A selector that is not a version (latest, a branch, a commit) cannot
// be compared here, so it accepts the module's only resolution.
func versionSelectorMatches(selector, version string) bool {
	sel := strings.TrimPrefix(selector, "v")
	if !semverSelector.MatchString(sel) {
		return true
	}
	return strings.HasPrefix(strings.TrimPrefix(version, "v")+".", sel+".")
}
