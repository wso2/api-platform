package policyengine

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/policy-engine/policy-builder/pkg/errors"
	"github.com/policy-engine/policy-builder/pkg/types"
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
