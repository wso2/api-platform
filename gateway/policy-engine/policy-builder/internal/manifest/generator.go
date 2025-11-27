package manifest

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/policy-engine/policy-builder/pkg/types"
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
