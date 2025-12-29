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
	"time"

	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
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
