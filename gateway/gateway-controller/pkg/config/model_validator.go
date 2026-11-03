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

package config

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// MaxModelNameBytes bounds a Model's modelName. It is the same ceiling the policy
// engine's llm-model-dispatch resolver enforces when it prepares the model
// endpoint's route: a longer name there makes the whole route fail to prepare, so
// a Model carrying one must be refused here, where it names only itself.
const MaxModelNameBytes = 256

// maxModelEndpoints bounds spec.endpoints, matching the schema.
const maxModelEndpoints = 64

// ModelValidator validates the parts of a Model that need nothing but the Model:
// shape, names, and the internal consistency of its targets. Everything that
// depends on other artifacts — the providers it targets, their templates, the
// routing policy — is resolved by pkg/modelgateway, which both the deployment
// service and the compiler run.
type ModelValidator struct {
	metadataNameRegex *regexp.Regexp
	versionRegex      *regexp.Regexp
	displayNameRegex  *regexp.Regexp
	targetNameRegex   *regexp.Regexp
}

// NewModelValidator creates a Model validator.
func NewModelValidator() *ModelValidator {
	return &ModelValidator{
		metadataNameRegex: regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`),
		versionRegex:      regexp.MustCompile(`^v\d+\.\d+$`),
		displayNameRegex:  regexp.MustCompile(`^[a-zA-Z0-9\-_\. ]+$`),
		targetNameRegex:   regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`),
	}
}

// Validate implements Validator.
func (v *ModelValidator) Validate(cfg interface{}) []ValidationError {
	switch m := cfg.(type) {
	case *api.ModelConfiguration:
		return v.validateModel(m)
	case api.ModelConfiguration:
		return v.validateModel(&m)
	default:
		return []ValidationError{{Field: "config", Message: "Unsupported configuration type for ModelValidator"}}
	}
}

func (v *ModelValidator) validateModel(m *api.ModelConfiguration) []ValidationError {
	if m == nil {
		return []ValidationError{{Field: "model", Message: "model cannot be nil"}}
	}
	var errs []ValidationError
	add := func(field, format string, args ...interface{}) {
		errs = append(errs, ValidationError{Field: field, Message: fmt.Sprintf(format, args...)})
	}

	if m.ApiVersion != api.ModelConfigurationApiVersion("gateway.api-platform.wso2.com/v1") {
		add("apiVersion", "apiVersion must be 'gateway.api-platform.wso2.com/v1'")
	}
	if m.Kind != api.ModelConfigurationKind("Model") {
		add("kind", "Kind must be 'Model'")
	}

	switch name := m.Metadata.Name; {
	case name == "":
		add("metadata.name", "metadata.name is required")
	case len(name) > 253:
		add("metadata.name", "metadata.name must not exceed 253 characters")
	case !v.metadataNameRegex.MatchString(name):
		add("metadata.name", "metadata.name must consist of lowercase alphanumeric characters, hyphens, or dots, "+
			"and must start and end with an alphanumeric character")
	}
	if m.Metadata.Labels != nil {
		errs = append(errs, ValidateLabels(*m.Metadata.Labels)...)
	}

	spec := m.Spec
	switch {
	case strings.TrimSpace(spec.DisplayName) == "":
		add("spec.displayName", "spec.displayName is required")
	case len(spec.DisplayName) > 100:
		add("spec.displayName", "spec.displayName must not exceed 100 characters")
	case !v.displayNameRegex.MatchString(spec.DisplayName):
		add("spec.displayName", "spec.displayName may contain only letters, numbers, spaces, hyphens, underscores and dots")
	}
	if !v.versionRegex.MatchString(spec.Version) {
		add("spec.version", "spec.version must be of the form vX.Y")
	}

	// The model name is what clients send. It is matched byte for byte, so
	// anything that could make two spellings look alike to a person is refused:
	// surrounding whitespace, control characters, and invalid UTF-8.
	switch name := spec.ModelName; {
	case name == "":
		add("spec.modelName", "spec.modelName is required")
	case len(name) > MaxModelNameBytes:
		add("spec.modelName", "spec.modelName must not exceed %d bytes", MaxModelNameBytes)
	case !utf8.ValidString(name):
		add("spec.modelName", "spec.modelName must be valid UTF-8")
	case strings.TrimSpace(name) != name:
		add("spec.modelName", "spec.modelName must not begin or end with whitespace")
	case strings.IndexFunc(name, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0:
		add("spec.modelName", "spec.modelName must not contain control characters")
	}

	// The endpoints are named, never implied: a Model always says where it is
	// served, so it behaves the same on a gateway with or without the provisioned
	// endpoint. That each one exists, and can serve a deployed Model, is checked
	// by the deployment service, which has the store.
	if len(spec.Endpoints) == 0 {
		add("spec.endpoints", "a Model must name at least one model endpoint in spec.endpoints")
	}
	if len(spec.Endpoints) > maxModelEndpoints {
		add("spec.endpoints", "a Model may name at most %d endpoints", maxModelEndpoints)
	}
	seenEndpoints := make(map[string]int, len(spec.Endpoints))
	for i, h := range spec.Endpoints {
		field := fmt.Sprintf("spec.endpoints[%d]", i)
		switch {
		case h == "" || len(h) > 253 || !v.metadataNameRegex.MatchString(h):
			add(field, "endpoint must be the handle of a model endpoint")
		default:
			if first, dup := seenEndpoints[h]; dup {
				add(field, "endpoint '%s' is already named by spec.endpoints[%d]", h, first)
			} else {
				seenEndpoints[h] = i
			}
		}
	}

	if len(spec.Targets) == 0 {
		add("spec.targets", "a Model needs at least one target")
	}
	names := make(map[string]int, len(spec.Targets))
	pairs := make(map[string]int, len(spec.Targets))
	for i, t := range spec.Targets {
		field := fmt.Sprintf("spec.targets[%d]", i)
		switch {
		case t.Name == "":
			add(field+".name", "target name is required")
		case len(t.Name) > 100:
			add(field+".name", "target name must not exceed 100 characters")
		case !v.targetNameRegex.MatchString(t.Name):
			add(field+".name", "target name may contain only letters, numbers, hyphens and underscores")
		default:
			if first, dup := names[t.Name]; dup {
				add(field+".name", "target name '%s' is already used by spec.targets[%d]", t.Name, first)
			} else {
				names[t.Name] = i
			}
		}

		if t.Provider.Id == "" {
			add(field+".provider.id", "provider id is required")
		} else if !v.metadataNameRegex.MatchString(t.Provider.Id) {
			add(field+".provider.id", "provider id must be the handle of an LLM provider")
		}

		switch m := t.UpstreamModel; {
		case strings.TrimSpace(m) == "":
			add(field+".upstreamModel", "upstreamModel is required")
		case len(m) > MaxModelNameBytes:
			add(field+".upstreamModel", "upstreamModel must not exceed %d bytes", MaxModelNameBytes)
		case strings.TrimSpace(m) != m || strings.IndexFunc(m, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0:
			add(field+".upstreamModel", "upstreamModel must not contain surrounding whitespace or control characters")
		}

		// A target is identified by its provider and upstream model together: two
		// targets with the same pair would be one target under two names, and a
		// router choosing one of them could not be told apart from choosing the other.
		if t.Provider.Id != "" && t.UpstreamModel != "" {
			key := t.Provider.Id + "\x00" + t.UpstreamModel
			if first, dup := pairs[key]; dup {
				add(field, "target uses provider '%s' with upstreamModel '%s', which spec.targets[%d] already does",
					t.Provider.Id, t.UpstreamModel, first)
			} else {
				pairs[key] = i
			}
		}

		if t.Auth != nil {
			errs = append(errs, validateUpstreamAuthFields(field+".auth", upstreamAuthFields{
				authType:      string(t.Auth.Type),
				header:        t.Auth.Header,
				value:         t.Auth.Value,
				policyName:    t.Auth.PolicyName,
				policyVersion: t.Auth.PolicyVersion,
				policyParams:  t.Auth.PolicyParams,
			})...)
		}
	}

	if spec.Routing != nil {
		if strings.TrimSpace(spec.Routing.Policy.Name) == "" {
			add("spec.routing.policy.name", "routing policy name is required")
		}
		if !majorVersionPattern.MatchString(spec.Routing.Policy.Version) {
			add("spec.routing.policy.version", "routing policy version must be major-only (e.g. v1)")
		}
	} else if len(spec.Targets) > 1 {
		add("spec.routing", "a Model with more than one target needs a routing policy to choose between them")
	}

	if spec.DeploymentState != nil {
		switch *spec.DeploymentState {
		case api.ModelConfigDataDeploymentStateDeployed, api.ModelConfigDataDeploymentStateUndeployed:
		default:
			add("spec.deploymentState", "spec.deploymentState must be 'deployed' or 'undeployed'")
		}
	}

	return errs
}
