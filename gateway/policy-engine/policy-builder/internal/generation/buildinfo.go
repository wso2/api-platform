package generation

import (
	"bytes"
	"fmt"
	"text/template"
	"time"

	"github.com/policy-engine/policy-builder/pkg/types"
	"github.com/policy-engine/policy-builder/templates"
)

// GenerateBuildInfo generates the build_info.go file with metadata
func GenerateBuildInfo(policies []*types.DiscoveredPolicy, builderVersion string) (string, error) {
	// Create policy info list
	policyInfos := make([]types.PolicyInfo, 0, len(policies))
	for _, policy := range policies {
		policyInfos = append(policyInfos, types.PolicyInfo{
			Name:    policy.Name,
			Version: policy.Version,
			Path:    policy.Path,
		})
	}

	// Parse embedded template
	tmpl, err := template.New("build_info").Parse(templates.BuildInfoTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	data := struct {
		Timestamp      string
		BuilderVersion string
		Policies       []types.PolicyInfo
	}{
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		BuilderVersion: builderVersion,
		Policies:       policyInfos,
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}
