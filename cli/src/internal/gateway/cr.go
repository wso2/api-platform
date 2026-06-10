/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package gateway

import (
	"fmt"
	"os"
	"strings"

	"github.com/wso2/api-platform/cli/utils"
	"gopkg.in/yaml.v3"
)

// CRMetadata is the metadata block of a gateway custom resource.
type CRMetadata struct {
	Name string `yaml:"name"`
}

// ResourceCR is the minimal custom-resource shape shared by the gateway
// management resources (SubscriptionPlan, Subscription, ApiKey, ...). It mirrors
// the operator CRs under gateway/examples/*.yaml: an apiVersion/kind/metadata
// envelope wrapping a free-form spec. The CLI validates the envelope locally and
// forwards the spec to the management API.
type ResourceCR struct {
	APIVersion string                 `yaml:"apiVersion"`
	Kind       string                 `yaml:"kind"`
	Metadata   CRMetadata             `yaml:"metadata"`
	Spec       map[string]interface{} `yaml:"spec"`
}

// ParseResourceCR reads a CR file (YAML or JSON), validates that its kind
// matches expectedKind and that metadata.name and a spec are present, and
// returns the parsed resource. For multi-document YAML only the first document
// is considered (e.g. an accompanying Secret is ignored — secret references are
// an operator-only concept and are not resolved by the CLI).
func ParseResourceCR(filePath, expectedKind string) (*ResourceCR, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("resource file not found: %s", filePath)
		}
		return nil, fmt.Errorf("failed to read resource file: %w", err)
	}

	// Accept JSON input by converting it to YAML first.
	content, err = utils.ConvertJSONToYAMLIfNeeded(content)
	if err != nil {
		return nil, fmt.Errorf("failed to process file content: %w", err)
	}

	var cr ResourceCR
	if err := yaml.Unmarshal(content, &cr); err != nil {
		return nil, fmt.Errorf("invalid resource YAML: %w", err)
	}

	if strings.TrimSpace(cr.Kind) == "" {
		return nil, fmt.Errorf("missing 'kind': expected %s", expectedKind)
	}
	if cr.Kind != expectedKind {
		return nil, fmt.Errorf("unsupported kind %q: expected %s", cr.Kind, expectedKind)
	}
	if strings.TrimSpace(cr.Metadata.Name) == "" {
		return nil, fmt.Errorf("invalid %s: metadata.name is required", expectedKind)
	}
	if len(cr.Spec) == 0 {
		return nil, fmt.Errorf("invalid %s: spec is required", expectedKind)
	}

	return &cr, nil
}
