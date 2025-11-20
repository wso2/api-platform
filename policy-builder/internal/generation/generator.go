package generation

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/envoy-policy-engine/policy-builder/pkg/errors"
	"github.com/envoy-policy-engine/policy-builder/pkg/types"
)

const BuilderVersion = "v1.0.0"

// GenerateCode orchestrates all code generation tasks
func GenerateCode(srcDir string, policies []*types.DiscoveredPolicy) error {
	// Generated files go in cmd/policy-engine (main package)
	mainPkgDir := filepath.Join(srcDir, "cmd", "policy-engine")

	// Generate plugin_registry.go
	registryCode, err := GeneratePluginRegistry(policies, srcDir)
	if err != nil {
		return errors.NewGenerationError("failed to generate plugin registry", err)
	}

	registryPath := filepath.Join(mainPkgDir, "plugin_registry.go")
	if err := os.WriteFile(registryPath, []byte(registryCode), 0644); err != nil {
		return errors.NewGenerationError("failed to write plugin_registry.go", err)
	}

	fmt.Printf("✓ Generated plugin_registry.go with %d policies\n", len(policies))

	// Generate build_info.go
	buildInfoCode, err := GenerateBuildInfo(policies, BuilderVersion)
	if err != nil {
		return errors.NewGenerationError("failed to generate build info", err)
	}

	buildInfoPath := filepath.Join(mainPkgDir, "build_info.go")
	if err := os.WriteFile(buildInfoPath, []byte(buildInfoCode), 0644); err != nil {
		return errors.NewGenerationError("failed to write build_info.go", err)
	}

	fmt.Printf("✓ Generated build_info.go\n")

	// Update go.mod with replace directives
	if err := GenerateGoModReplaces(srcDir, policies); err != nil {
		return errors.NewGenerationError("failed to update go.mod", err)
	}

	fmt.Printf("✓ Updated go.mod with %d replace directives\n", len(policies))

	return nil
}
