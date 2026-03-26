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

package transform

import (
	"fmt"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// Registry dispatches StoredConfig → RuntimeDeployConfig by API kind.
type Registry struct {
	restT *RestAPITransformer
	llmT  *LLMTransformer
}

// NewRegistry creates a new transformer Registry.
func NewRegistry(restT *RestAPITransformer, llmT *LLMTransformer) *Registry {
	return &Registry{restT: restT, llmT: llmT}
}

// Transform converts a StoredConfig to a RuntimeDeployConfig using the appropriate transformer.
func (r *Registry) Transform(cfg *models.StoredConfig) (*models.RuntimeDeployConfig, error) {
	switch cfg.Kind {
	case "RestApi", "WebSubApi", "Mcp":
		return r.restT.Transform(cfg)
	case "LlmProvider", "LlmProxy":
		return r.llmT.Transform(cfg)
	default:
		return nil, fmt.Errorf("unsupported kind for runtime config: %s", cfg.Kind)
	}
}
