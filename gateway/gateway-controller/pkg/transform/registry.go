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
	"errors"
	"fmt"
	"slices"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// ErrUnsupportedKind reports that no transformer handles an artifact's kind.
// Exported so a caller — and the test that checks the kind lists below agree
// with the dispatch switch — can tell "this kind is not routed through the RDC
// path" apart from "this artifact is malformed".
var ErrUnsupportedKind = errors.New("unsupported kind for runtime config")

// registryKinds is every artifact kind Transform handles. It must agree with the
// switch in Transform; TestRegistryKindsMatchDispatch checks that it does.
var registryKinds = []string{
	models.KindRestApi,
	models.KindWebSubApi,
	models.KindMcp,
	models.KindLlmProvider,
	models.KindLlmProxy,
	models.KindAgent,
}

// envoyTranslatorExcludedKinds are the kinds that must NOT be wired into the
// Envoy xDS translator even though the Registry can transform them.
//
// WebSubApi is deliberately excluded: it keeps using the async-specific legacy
// translation path. Everything else has to go through the RDC path, because the
// policy engine resolves a chain by the Envoy route name — if Envoy builds a
// route by the legacy path while the policy resources are keyed from an RDC, the
// two names disagree and every request on that route 500s with "policy chain not
// found".
var envoyTranslatorExcludedKinds = []string{models.KindWebSubApi}

// Kinds returns every artifact kind the Registry can transform.
func Kinds() []string {
	return slices.Clone(registryKinds)
}

// EnvoyTranslatorKinds returns the kinds to register with the Envoy xDS
// translator.
//
// It exists so the wiring in cmd/controller/main.go is derived rather than
// hand-listed. A hand-written map beside a hand-written dispatch switch is two
// lists that have to be kept in step, and the failure when they drift is silent
// — a new kind's policy chains come from the RDC while its Envoy routes come
// from the legacy path, under a different route name. Deriving the map means a
// kind added to registryKinds is wired everywhere or excluded on purpose.
func EnvoyTranslatorKinds() []string {
	kinds := make([]string, 0, len(registryKinds))
	for _, kind := range registryKinds {
		if !slices.Contains(envoyTranslatorExcludedKinds, kind) {
			kinds = append(kinds, kind)
		}
	}
	return kinds
}

// Transform converts a StoredConfig to a RuntimeDeployConfig using the appropriate transformer.
func (r *Registry) Transform(cfg *models.StoredConfig) (*models.RuntimeDeployConfig, error) {
	switch cfg.Kind {
	case models.KindRestApi, models.KindWebSubApi, models.KindMcp:
		return r.restT.Transform(cfg)
	case models.KindLlmProvider, models.KindLlmProxy:
		return r.llmT.Transform(cfg)
	case models.KindAgent:
		return r.agentT.Transform(cfg)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedKind, cfg.Kind)
	}
}

// Registry dispatches StoredConfig → RuntimeDeployConfig by API kind.
type Registry struct {
	restT  *RestAPITransformer
	llmT   *LLMTransformer
	agentT *AgentTransformer
}

// NewRegistry creates a new transformer Registry.
func NewRegistry(restT *RestAPITransformer, llmT *LLMTransformer, agentT *AgentTransformer) *Registry {
	return &Registry{restT: restT, llmT: llmT, agentT: agentT}
}
