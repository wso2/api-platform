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

package policyengine

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/policy-engine/gateway-builder/pkg/errors"
	"github.com/policy-engine/gateway-builder/pkg/types"
)

const BuilderVersion = "v1.0.0"

// GenerateCode orchestrates all code generation tasks
func GenerateCode(srcDir string, policies []*types.DiscoveredPolicy) error {
	slog.Debug("Starting code generation",
		"srcDir", srcDir,
		"policyCount", len(policies),
		"phase", "generation")

	// Generated files go in cmd/policy-engine (main package)
	mainPkgDir := filepath.Join(srcDir, "cmd", "policy-engine")
	slog.Debug("Code generation target", "mainPkgDir", mainPkgDir, "phase", "generation")

	// Generate plugin_registry.go
	registryCode, err := GeneratePluginRegistry(policies, srcDir)
	if err != nil {
		return errors.NewGenerationError("failed to generate plugin registry", err)
	}

	registryPath := filepath.Join(mainPkgDir, "plugin_registry.go")
	if err := os.WriteFile(registryPath, []byte(registryCode), 0644); err != nil {
		return errors.NewGenerationError("failed to write plugin_registry.go", err)
	}

	slog.Info("Generated plugin_registry.go",
		"policies", len(policies),
		"path", registryPath,
		"phase", "generation")

	// Generate build_info.go
	buildInfoCode, err := GenerateBuildInfo(policies, BuilderVersion)
	if err != nil {
		return errors.NewGenerationError("failed to generate build info", err)
	}

	buildInfoPath := filepath.Join(mainPkgDir, "build_info.go")
	if err := os.WriteFile(buildInfoPath, []byte(buildInfoCode), 0644); err != nil {
		return errors.NewGenerationError("failed to write build_info.go", err)
	}

	slog.Info("Generated build_info.go",
		"path", buildInfoPath,
		"phase", "generation")

	// Update go.mod with replace directives
	if err := GenerateGoModReplaces(srcDir, policies); err != nil {
		return errors.NewGenerationError("failed to update go.mod", err)
	}

	slog.Info("Updated go.mod with replace directives",
		"count", len(policies),
		"phase", "generation")

	return nil
}
