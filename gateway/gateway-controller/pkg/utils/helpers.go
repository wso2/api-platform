package utils

import (
	"fmt"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
)

// ExtractNameVersion returns the name and version from an API configuration
// Supports both HTTP REST APIs and async/websub kinds.
func ExtractNameVersion(cfg api.APIConfiguration) (string, string, error) {
	if cfg.Kind == api.APIConfigurationKindHttprest {
		d, err := cfg.Spec.AsAPIConfigData()
		if err != nil {
			return "", "", fmt.Errorf("failed to parse http/rest api config data: %w", err)
		}
		return d.Name, d.Version, nil
	}
	if cfg.Kind == api.APIConfigurationKindAsyncwebsub {
		d, err := cfg.Spec.AsWebhookAPIData()
		if err != nil {
			return "", "", fmt.Errorf("failed to parse async/websub api config data: %w", err)
		}
		return d.Name, d.Version, nil
	}
	return "", "", fmt.Errorf("unsupported api kind: %s", cfg.Kind)
}
