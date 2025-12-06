package compilation

import (
	"fmt"
	"runtime"
	"time"

	"github.com/policy-engine/gateway-builder/pkg/types"
)

// BuildOptions creates compilation options for the policy engine binary
func BuildOptions(outputPath string, buildMetadata *types.BuildMetadata) *types.CompilationOptions {
	// Generate ldflags for build metadata injection
	ldflags := generateLDFlags(buildMetadata)

	return &types.CompilationOptions{
		OutputPath: outputPath,
		EnableUPX:  false, // Disabled by default for compatibility
		LDFlags:    ldflags,
		BuildTags:  []string{},
		CGOEnabled: false, // Static binary
		TargetOS:   "linux",
		TargetArch: runtime.GOARCH, // Use native architecture
	}
}

// generateLDFlags creates ldflags string for embedding build metadata
func generateLDFlags(metadata *types.BuildMetadata) string {
	ldflags := "-s -w" // Strip debug info and symbol table

	// Add version information (matching policy-engine main.go variables)
	ldflags += fmt.Sprintf(" -X main.Version=%s", metadata.Version)
	ldflags += fmt.Sprintf(" -X main.GitCommit=%s", metadata.GitCommit)

	// Add build timestamp as BuildDate
	timestamp := metadata.Timestamp.Format(time.RFC3339)
	ldflags += fmt.Sprintf(" -X main.BuildDate=%s", timestamp)

	return ldflags
}
