package generation

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/envoy-policy-engine/builder/pkg/types"
	"golang.org/x/mod/modfile"
)

// GenerateGoModReplaces adds replace directives to go.mod for local policies
func GenerateGoModReplaces(srcDir string, policies []*types.DiscoveredPolicy) error {
	// Ensure srcDir is absolute
	absSrcDir, err := filepath.Abs(srcDir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute srcDir: %w", err)
	}
	srcDir = absSrcDir

	goModPath := filepath.Join(srcDir, "go.mod")

	// Read existing go.mod
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return fmt.Errorf("failed to read go.mod: %w", err)
	}

	// Parse go.mod
	modFile, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return fmt.Errorf("failed to parse go.mod: %w", err)
	}

	// Add replace directives for each policy
	for _, policy := range policies {
		importPath := generateImportPath(policy)
		relativePath, err := filepath.Rel(srcDir, policy.Path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", policy.Name, err)
		}

		slog.Debug("adding replace directive",
			"policy", policy.Name,
			"version", policy.Version,
			"importPath", importPath,
			"relativePath", relativePath)

		// Add replace directive
		if err := modFile.AddReplace(importPath, "", relativePath, ""); err != nil {
			// If replace already exists, that's okay
			if !strings.Contains(err.Error(), "already") {
				return fmt.Errorf("failed to add replace directive: %w", err)
			}
		}
	}

	// Format and write back
	formattedData, err := modFile.Format()
	if err != nil {
		return fmt.Errorf("failed to format go.mod: %w", err)
	}

	if err := os.WriteFile(goModPath, formattedData, 0644); err != nil {
		return fmt.Errorf("failed to write go.mod: %w", err)
	}

	return nil
}
