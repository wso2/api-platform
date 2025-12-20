package packaging

import (
	"fmt"
	"strings"
	"time"

	"github.com/policy-engine/gateway-builder/pkg/types"
)

// GenerateDockerLabels creates Docker labels for the runtime image
func GenerateDockerLabels(metadata *types.PackagingMetadata) map[string]string {
	labels := make(map[string]string)

	// Standard OCI labels
	labels["org.opencontainers.image.created"] = metadata.BuildTimestamp.Format(time.RFC3339)
	labels["org.opencontainers.image.title"] = "API Platform Policy Engine"
	labels["org.opencontainers.image.description"] = "API Platform Policy Engine with compiled policies"
	labels["org.opencontainers.image.vendor"] = "WSO2"

	// Builder metadata
	labels["build.timestamp"] = metadata.BuildTimestamp.Format(time.RFC3339)

	// Policy list
	if len(metadata.Policies) > 0 {
		policyList := formatPolicyList(metadata.Policies)
		labels["build.policies"] = policyList
		labels["build.policy-count"] = fmt.Sprintf("%d", len(metadata.Policies))
	}

	// Add any custom labels
	for key, value := range metadata.Labels {
		labels[key] = value
	}

	return labels
}

// formatPolicyList creates a comma-separated list of policies for Docker labels
func formatPolicyList(policies []types.PolicyInfo) string {
	var parts []string
	for _, p := range policies {
		parts = append(parts, fmt.Sprintf("%s@%s", p.Name, p.Version))
	}
	return strings.Join(parts, ",")
}
