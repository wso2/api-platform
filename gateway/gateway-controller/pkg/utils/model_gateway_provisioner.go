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

package utils

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	versionutil "github.com/wso2/api-platform/common/version"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// modelEndpointSeedYAML holds the transformer entries the provisioned endpoint
// starts with. Data, not code: the knowledge of which policy transforms OpenAI
// requests for which template lives in one reviewable file.
//
//go:embed model_endpoint_seed.yaml
var modelEndpointSeedYAML []byte

// ModelEndpointSeedTransformers returns the seed entries as written, without
// versions.
func ModelEndpointSeedTransformers() ([]api.ModelEndpointTransformer, error) {
	var seed struct {
		Transformers []api.ModelEndpointTransformer `yaml:"transformers"`
	}
	if err := yaml.Unmarshal(modelEndpointSeedYAML, &seed); err != nil {
		return nil, fmt.Errorf("model endpoint seed transformers are unreadable: %w", err)
	}
	return seed.Transformers, nil
}

// CheckModelEndpointStartup verifies, before anything is served, that provisioning
// the default endpoint takes over nothing an operator already owns.
//
// It fails closed for that one case: when provisioning would put `wso2` at a
// context it does not serve today — on first start, or after
// controller.models.endpoint.context changed — and another artifact already serves that
// context, startup is refused, naming the artifact (GO-AUTH-011: verified locally,
// on the store as loaded from the database).
//
// An endpoint already serving its context was checked when it was written. Another
// artifact written at that context since, through an API that does not check model
// endpoints, is logged as a conflict for the operator to resolve, never a reason
// to refuse a restart: otherwise one write could leave the gateway unable to start.
// With the feature off nothing is served and nothing is checked.
func CheckModelEndpointStartup(store *storage.ConfigStore, cfg config.ModelsConfig, logger *slog.Logger) error {
	if !cfg.Enabled || store == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	all := store.GetAll()

	if cfg.ProvisionsDefaultEndpoint() {
		context := cfg.EffectiveContext()
		defaultUUID := models.ModelEndpointUUID(models.DefaultModelEndpointHandle)
		var vhost *string
		served := ""
		if existing, err := store.GetByKindAndHandle(models.KindModelEndpoint, models.DefaultModelEndpointHandle); err == nil && existing != nil {
			defaultUUID = existing.UUID
			if ep, ok := existing.SourceConfiguration.(api.ModelEndpointConfiguration); ok {
				vhost = ep.Spec.Vhost
				served = trimmedOrEmpty(ep.Spec.Context)
			}
		}
		if other := conflictingArtifact(all, defaultUUID, context, vhost); other != nil {
			if served == "" || normalizedContext(served) != normalizedContext(context) {
				return fmt.Errorf("controller.models.endpoint.context '%s' is already used by %s '%s'; choose another context "+
					"for the default model endpoint", context, other.Kind, other.Handle)
			}
			warnModelEndpointContextShared(logger, models.DefaultModelEndpointHandle, context, other)
		}
	}

	for _, existing := range all {
		if existing.Kind != models.KindModelEndpoint {
			continue
		}
		if existing.Handle == models.DefaultModelEndpointHandle && cfg.ProvisionsDefaultEndpoint() {
			continue // checked above, at the configured context
		}
		ep, ok := existing.SourceConfiguration.(api.ModelEndpointConfiguration)
		if !ok || ep.Spec.Context == nil {
			continue
		}
		if other := conflictingArtifact(all, existing.UUID, *ep.Spec.Context, ep.Spec.Vhost); other != nil {
			warnModelEndpointContextShared(logger, existing.Handle, strings.TrimSpace(*ep.Spec.Context), other)
		}
	}
	return nil
}

// warnModelEndpointContextShared reports a model endpoint whose context another
// artifact also serves: requests there may reach either, so one should move.
func warnModelEndpointContextShared(logger *slog.Logger, handle, context string, other *models.StoredConfig) {
	logger.Warn("A model endpoint shares its context with another artifact; requests may reach either. "+
		"Move one of them to another context or vhost",
		slog.String("model_endpoint", handle), slog.String("context", context),
		slog.String("other_kind", other.Kind), slog.String("other", other.Handle))
}

// ProvisionModelEndpoint makes sure the default model endpoint exists when the
// gateway provisions it: ModelEndpoint `wso2`, at its deterministic UUID, at the
// configured context.
//
// It is an idempotent upsert, run on every start by every replica:
//
//   - Missing, the endpoint is created with api-key-auth as its only policy, so it
//     starts out authenticated, and the seed transformer entries whose policies
//     this build has loaded.
//   - Present, it is left as the operator has shaped it — its policies,
//     transformers, display name and resilience are theirs — except for the
//     context, which is configuration and is moved to the configured value if it
//     differs.
//
// Replicas starting together all derive the same UUID, so a create that loses the
// race finds the winner's row and treats it as present.
func (s *LLMDeploymentService) ProvisionModelEndpoint(logger *slog.Logger) error {
	if !s.modelsConfig.ProvisionsDefaultEndpoint() {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}

	existing, err := s.db.GetConfigByKindAndHandle(models.KindModelEndpoint, models.DefaultModelEndpointHandle)
	if err != nil && !storage.IsNotFoundError(err) {
		return fmt.Errorf("failed to look up the default model endpoint: %w", err)
	}

	if existing == nil {
		endpoint, err := s.newDefaultModelEndpoint(logger)
		if err != nil {
			return err
		}
		result, err := s.deployDefaultModelEndpoint(endpoint, "", logger)
		if err != nil {
			if storage.IsConflictError(err) {
				logger.Info("Default model endpoint was provisioned concurrently by another replica")
				return nil
			}
			return fmt.Errorf("failed to provision the default model endpoint: %w", err)
		}
		logger.Info("Default model endpoint provisioned",
			slog.String("handle", result.StoredConfig.Handle),
			slog.String("uuid", result.StoredConfig.UUID),
			slog.String("context", s.modelsConfig.EffectiveContext()))
		return nil
	}

	stored, ok := existing.SourceConfiguration.(api.ModelEndpointConfiguration)
	if !ok {
		return fmt.Errorf("the default model endpoint has unexpected configuration %T", existing.SourceConfiguration)
	}
	configured := s.modelsConfig.EffectiveContext()
	if stored.Spec.Context != nil && strings.TrimSpace(*stored.Spec.Context) == configured {
		return nil
	}
	stored.Spec.Context = &configured
	if _, err := s.deployDefaultModelEndpoint(stored, existing.UUID, logger); err != nil {
		return fmt.Errorf("failed to move the default model endpoint to context '%s': %w", configured, err)
	}
	logger.Info("Default model endpoint moved to the configured context", slog.String("context", configured))
	return nil
}

// newDefaultModelEndpoint builds the default endpoint as first provisioned.
func (s *LLMDeploymentService) newDefaultModelEndpoint(logger *slog.Logger) (api.ModelEndpointConfiguration, error) {
	auth, err := s.defaultEndpointAuthPolicy()
	if err != nil {
		// Without it the endpoint would be provisioned open. Refuse instead.
		return api.ModelEndpointConfiguration{}, fmt.Errorf("cannot provision the default model endpoint: %w", err)
	}
	seed, err := ModelEndpointSeedTransformers()
	if err != nil {
		return api.ModelEndpointConfiguration{}, err
	}
	transformers := make([]api.ModelEndpointTransformer, 0, len(seed))
	for _, entry := range seed {
		version, err := s.loadedMajorVersion(entry.Type)
		if err != nil {
			// A build without one of the transformers still provisions; the
			// providers on that template are not served until an entry is added.
			logger.Warn("Default model endpoint: transformer is not loaded in this gateway build; its entry is left out",
				slog.String("transformer", entry.Type), slog.String("template", trimmedOrEmpty(entry.Template)))
			continue
		}
		entry.Version = &version
		transformers = append(transformers, entry)
	}

	context := s.modelsConfig.EffectiveContext()
	return api.ModelEndpointConfiguration{
		ApiVersion: api.ModelEndpointConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.ModelEndpointConfigurationKindModelEndpoint,
		Metadata:   api.Metadata{Name: models.DefaultModelEndpointHandle},
		Spec: api.ModelEndpointConfigData{
			DisplayName:  models.DefaultModelEndpointDisplayName,
			Version:      models.DefaultModelEndpointVersion,
			Context:      &context,
			Policies:     &[]api.Policy{auth},
			Transformers: &transformers,
		},
	}, nil
}

func (s *LLMDeploymentService) deployDefaultModelEndpoint(endpoint api.ModelEndpointConfiguration, id string,
	logger *slog.Logger) (*APIDeploymentResult, error) {
	data, err := json.Marshal(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the default model endpoint: %w", err)
	}
	params := LLMDeploymentParams{
		Data:         data,
		ContentType:  "application/json",
		Origin:       models.OriginGatewayAPI,
		Logger:       logger,
		Provisioning: true,
	}
	if id != "" {
		params.ID = id
		params.IsUpdate = true
		now := time.Now().Truncate(time.Millisecond)
		params.DeployedAt = &now
	}
	return s.DeployModelEndpointConfiguration(params)
}

// loadedMajorVersion is the major version of the loaded policy with this name.
func (s *LLMDeploymentService) loadedMajorVersion(name string) (string, error) {
	if s.policyValidator == nil {
		return "", fmt.Errorf("no policy definitions are loaded")
	}
	def, err := LookupPolicyDefinition(s.policyValidator.Definitions(), name, "")
	if err != nil {
		return "", err
	}
	return versionutil.MajorVersion(def.Version), nil
}
