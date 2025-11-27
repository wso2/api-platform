package manifest

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/policy-engine/policy-builder/pkg/types"
)

// Manifest represents the build manifest structure
type Manifest struct {
	BuildTimestamp string        `json:"buildTimestamp"`
	BuilderVersion string        `json:"builderVersion"`
	Images         ImageManifest `json:"images"`
	Policies       []PolicyInfo  `json:"policies"`
}

// ImageManifest contains the built image names
type ImageManifest struct {
	PolicyEngine      string `json:"policyEngine"`
	GatewayController string `json:"gatewayController"`
	Router            string `json:"router"`
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
	images ImageManifest,
) *Manifest {
	slog.Info("Creating build manifest")

	// Create manifest structure
	manifest := &Manifest{
		BuildTimestamp: time.Now().UTC().Format(time.RFC3339),
		BuilderVersion: builderVersion,
		Images:         images,
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
