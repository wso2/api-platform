package compilation

import (
	"fmt"
	"time"

	"github.com/policy-engine/policy-builder/pkg/types"
)

// BuildOptions creates compilation options for the policy engine binary
func BuildOptions(outputPath string, buildMetadata *types.BuildMetadata) *types.CompilationOptions {
	// Generate ldflags for build metadata injection
	ldflags := generateLDFlags(buildMetadata)

	return &types.CompilationOptions{
		OutputPath:      outputPath,
		EnableUPX:       false, // Disabled by default for compatibility
		LDFlags:         ldflags,
		BuildTags:       []string{},
		CGOEnabled:      false, // Static binary
		TargetOS:        "linux",
		TargetArch:      "amd64",
	}
}

// generateLDFlags creates ldflags string for embedding build metadata
func generateLDFlags(metadata *types.BuildMetadata) string {
	// Format: -s -w -X 'main.buildTimestamp=...' -X 'main.builderVersion=...'
	ldflags := "-s -w" // Strip debug info and symbol table

	// Add build timestamp
	timestamp := metadata.Timestamp.Format(time.RFC3339)
	ldflags += fmt.Sprintf(" -X 'main.buildTimestamp=%s'", timestamp)

	// Add builder version
	ldflags += fmt.Sprintf(" -X 'main.builderVersion=%s'", metadata.BuilderVersion)

	// Add policy count
	ldflags += fmt.Sprintf(" -X 'main.policyCount=%d'", len(metadata.Policies))

	return ldflags
}
