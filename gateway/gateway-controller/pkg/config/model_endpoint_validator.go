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

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/modelgateway/inbound"
)

// ModelEndpointDefaultHandle is the handle reserved for the provisioned endpoint,
// whose context comes from controller.models.endpoint.context rather than its spec. Kept
// here as well as in pkg/models because this package cannot import that one's
// helpers without a cycle; a test pins the two together.
const ModelEndpointDefaultHandle = "wso2"

// transformerTypeNone attaches no transformer.
const transformerTypeNone = "none"

// providerIDParam is the transformer parameter the compiler injects.
const providerIDParam = "providerId"

// maxTransformerEntries bounds spec.transformers, matching the schema.
const maxTransformerEntries = 128

var (
	modelEndpointNamePattern  = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	modelEndpointVersion      = regexp.MustCompile(`^v\d+\.\d+$`)
	modelEndpointDisplayName  = regexp.MustCompile(`^[a-zA-Z0-9\-_\. ]+$`)
	modelEndpointVhostPattern = regexp.MustCompile(`^(\*\.|[a-zA-Z0-9](?:[a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
	// transformerPathPlaceholder matches every `{...}` in an upstream path, so an
	// unknown placeholder is refused instead of being sent upstream as text.
	transformerPathPlaceholder = regexp.MustCompile(`\{[^{}]*\}`)
	knownPathPlaceholder       = regexp.MustCompile(`^\{(upstreamModel|apiVersion(:[A-Za-z0-9._\-]+)?)\}$`)
	knownParamPlaceholder      = regexp.MustCompile(`\{apiVersion(:[^{}]*)?\}`)
	validParamDefault          = regexp.MustCompile(`^\{apiVersion(:[A-Za-z0-9._\-]+)?\}$`)
)

// ModelEndpointValidator validates the parts of a model endpoint that need nothing
// but the endpoint: its shape, names, inbound format and transformer entries.
// Everything that depends on other artifacts — the gateway-wide context and
// display-name uniqueness, the Models it serves — is checked by the deployment
// service, which can see them.
type ModelEndpointValidator struct{}

// NewModelEndpointValidator creates a model endpoint validator.
func NewModelEndpointValidator() *ModelEndpointValidator {
	return &ModelEndpointValidator{}
}

// Validate implements Validator.
func (v *ModelEndpointValidator) Validate(cfg interface{}) []ValidationError {
	switch e := cfg.(type) {
	case *api.ModelEndpointConfiguration:
		return v.validate(e)
	case api.ModelEndpointConfiguration:
		return v.validate(&e)
	default:
		return []ValidationError{{Field: "config", Message: "Unsupported configuration type for ModelEndpointValidator"}}
	}
}

func (v *ModelEndpointValidator) validate(e *api.ModelEndpointConfiguration) []ValidationError {
	if e == nil {
		return []ValidationError{{Field: "modelEndpoint", Message: "model endpoint cannot be nil"}}
	}
	var errs []ValidationError
	add := func(field, format string, args ...interface{}) {
		errs = append(errs, ValidationError{Field: field, Message: fmt.Sprintf(format, args...)})
	}

	if e.ApiVersion != api.ModelEndpointConfigurationApiVersionGatewayApiPlatformWso2Comv1 {
		add("apiVersion", "apiVersion must be 'gateway.api-platform.wso2.com/v1'")
	}
	if e.Kind != api.ModelEndpointConfigurationKindModelEndpoint {
		add("kind", "Kind must be 'ModelEndpoint'")
	}

	name := e.Metadata.Name
	switch {
	case name == "":
		add("metadata.name", "metadata.name is required")
	case len(name) > 253:
		add("metadata.name", "metadata.name must not exceed 253 characters")
	case !modelEndpointNamePattern.MatchString(name):
		add("metadata.name", "metadata.name must consist of lowercase alphanumeric characters, hyphens, or dots, "+
			"and must start and end with an alphanumeric character")
	}
	if e.Metadata.Labels != nil {
		errs = append(errs, ValidateLabels(*e.Metadata.Labels)...)
	}

	spec := e.Spec
	switch {
	case strings.TrimSpace(spec.DisplayName) == "":
		add("spec.displayName", "spec.displayName is required")
	case len(spec.DisplayName) > 253:
		add("spec.displayName", "spec.displayName must not exceed 253 characters")
	case !modelEndpointDisplayName.MatchString(spec.DisplayName):
		add("spec.displayName", "spec.displayName may contain only letters, numbers, spaces, hyphens, underscores and dots")
	}
	if !modelEndpointVersion.MatchString(spec.Version) {
		add("spec.version", "spec.version must be of the form vX.Y")
	}

	errs = append(errs, validateModelEndpointContext(name, spec.Context)...)

	if spec.Vhost != nil {
		vhost := strings.TrimSpace(*spec.Vhost)
		if vhost == "" || len(vhost) > 253 || !modelEndpointVhostPattern.MatchString(vhost) {
			add("spec.vhost", "spec.vhost must be a valid host name; a wildcard is allowed only as the left-most label")
		}
	}

	template := DefaultInboundTemplateOf(spec)
	if _, ok := inbound.Lookup(template); !ok {
		add("spec.inboundTemplate", "spec.inboundTemplate '%s' is not supported; it must be one of %s",
			template, strings.Join(inbound.Templates(), ", "))
	}

	errs = append(errs, validateEndpointPolicies("spec.policies", spec.Policies)...)
	errs = append(errs, validateEndpointPolicies("spec.faultPolicies", spec.FaultPolicies)...)
	errs = append(errs, ValidateModelEndpointTransformers("spec.transformers", spec.Transformers)...)
	errs = append(errs, validateResilienceTimeouts("spec.resilience", spec.Resilience)...)

	if spec.DeploymentState != nil {
		switch *spec.DeploymentState {
		case api.ModelEndpointConfigDataDeploymentStateDeployed, api.ModelEndpointConfigDataDeploymentStateUndeployed:
		default:
			add("spec.deploymentState", "spec.deploymentState must be 'deployed' or 'undeployed'")
		}
	}
	return errs
}

// DefaultInboundTemplateOf is an endpoint's inbound template: the one it names, or
// openai.
func DefaultInboundTemplateOf(spec api.ModelEndpointConfigData) string {
	if spec.InboundTemplate != nil && strings.TrimSpace(*spec.InboundTemplate) != "" {
		return strings.TrimSpace(*spec.InboundTemplate)
	}
	return inbound.FormatOpenAI
}

// validateModelEndpointContext validates an endpoint's context. It is required for
// every endpoint but the provisioned one, whose context is configuration.
func validateModelEndpointContext(handle string, context *string) []ValidationError {
	if context == nil || strings.TrimSpace(*context) == "" {
		if handle == ModelEndpointDefaultHandle {
			return nil
		}
		return []ValidationError{{Field: "spec.context", Message: "spec.context is required"}}
	}
	ctx := strings.TrimSpace(*context)
	if len(ctx) > 200 || !modelsContextPattern.MatchString(ctx) || strings.Contains(ctx, "//") ||
		strings.Contains(ctx, "/./") || strings.Contains(ctx, "/../") ||
		strings.HasSuffix(ctx, "/.") || strings.HasSuffix(ctx, "/..") {
		return []ValidationError{{Field: "spec.context", Message: "spec.context must be an absolute path with no " +
			"trailing slash, empty segment or dot segment"}}
	}
	if ctx == "/" {
		return []ValidationError{{Field: "spec.context", Message: "spec.context must not be '/': the endpoint " +
			"would shadow every other API"}}
	}
	return validateNotReservedGatewayPath("spec.context", ctx)
}

// validateEndpointPolicies checks the shape of a policy list. Whether each policy
// is loaded, and its parameters, is checked by the policy validator.
func validateEndpointPolicies(field string, policies *[]api.Policy) []ValidationError {
	if policies == nil {
		return nil
	}
	var errs []ValidationError
	for i, p := range *policies {
		f := fmt.Sprintf("%s[%d]", field, i)
		if strings.TrimSpace(p.Name) == "" {
			errs = append(errs, ValidationError{Field: f + ".name", Message: "policy name is required"})
		}
		if strings.TrimSpace(p.Version) == "" {
			errs = append(errs, ValidationError{Field: f + ".version", Message: "policy version is required"})
		} else if !majorVersionPattern.MatchString(p.Version) {
			errs = append(errs, ValidationError{Field: f + ".version", Message: "policy version must be major-only (e.g. v1)"})
		}
	}
	return errs
}

// ValidateModelEndpointTransformers validates spec.transformers: each entry has the
// LLM proxy transformer's type and version rules, names exactly one provider or
// template, uses only placeholders the compiler fills, and does not set the
// parameter the compiler injects. Two entries for the same provider, or the same
// template, are refused: which one applies would depend on list order.
func ValidateModelEndpointTransformers(field string, entries *[]api.ModelEndpointTransformer) []ValidationError {
	if entries == nil {
		return nil
	}
	var errs []ValidationError
	add := func(f, format string, args ...interface{}) {
		errs = append(errs, ValidationError{Field: f, Message: fmt.Sprintf(format, args...)})
	}
	if len(*entries) > maxTransformerEntries {
		add(field, "at most %d transformer entries are allowed", maxTransformerEntries)
		return errs
	}

	providers := map[string]int{}
	templates := map[string]int{}
	for i, e := range *entries {
		f := fmt.Sprintf("%s[%d]", field, i)
		provider := trimmedPtr(e.Provider)
		template := trimmedPtr(e.Template)
		switch {
		case provider == "" && template == "":
			add(f, "exactly one of provider or template is required")
		case provider != "" && template != "":
			add(f, "set provider or template, not both")
		case provider != "":
			if first, dup := providers[provider]; dup {
				add(f+".provider", "provider '%s' already has an entry (%s[%d])", provider, field, first)
			} else {
				providers[provider] = i
			}
		default:
			if first, dup := templates[template]; dup {
				add(f+".template", "template '%s' already has an entry (%s[%d])", template, field, first)
			} else {
				templates[template] = i
			}
		}

		typ := strings.TrimSpace(e.Type)
		version := trimmedPtr(e.Version)
		if typ == transformerTypeNone {
			if version != "" {
				add(f+".version", "a transformer of type 'none' takes no version")
			}
			if e.Params != nil && len(*e.Params) > 0 {
				add(f+".params", "a transformer of type 'none' takes no params")
			}
		} else {
			errs = append(errs, validateTransformerTypeVersion(f, typ, version)...)
		}

		if e.Params != nil {
			if _, set := (*e.Params)[providerIDParam]; set {
				add(f+".params."+providerIDParam, "%s is set by the gateway and cannot be configured", providerIDParam)
			}
			for k, val := range *e.Params {
				errs = append(errs, validateParamPlaceholders(f+".params."+k, val)...)
			}
		}

		if e.UpstreamPaths != nil {
			if len(*e.UpstreamPaths) > 16 {
				add(f+".upstreamPaths", "at most 16 upstream paths are allowed")
			}
			for j, p := range *e.UpstreamPaths {
				pf := fmt.Sprintf("%s.upstreamPaths[%d]", f, j)
				switch {
				case !strings.HasPrefix(p, "/") || len(p) > 512:
					add(pf, "an upstream path must start with '/' and not exceed 512 characters")
				case strings.ContainsAny(p, " \t\r\n?#"):
					add(pf, "an upstream path must not contain whitespace, a query or a fragment")
				default:
					for _, ph := range transformerPathPlaceholder.FindAllString(p, -1) {
						if !knownPathPlaceholder.MatchString(ph) {
							add(pf, "unknown placeholder %s; use {upstreamModel}, {apiVersion} or {apiVersion:<default>}", ph)
						}
					}
				}
			}
		}
	}
	return errs
}

// validateTransformerTypeVersion is the type and version rule shared by an LLM
// proxy's transformer and a model endpoint's transformer entries.
func validateTransformerTypeVersion(fieldPrefix, typ, version string) []ValidationError {
	var errors []ValidationError
	if typ == "" {
		errors = append(errors, ValidationError{Field: fieldPrefix + ".type", Message: "Transformer type is required"})
	}
	if version == "" {
		errors = append(errors, ValidationError{Field: fieldPrefix + ".version", Message: "Transformer version is required"})
	} else if !majorVersionPattern.MatchString(version) {
		errors = append(errors, ValidationError{Field: fieldPrefix + ".version",
			Message: "Transformer version must be major-only (e.g. v1)"})
	}
	return errors
}

// validateParamPlaceholders refuses an {apiVersion:<default>} whose default is
// empty or not a version-like token, wherever it appears in a parameter value.
func validateParamPlaceholders(field string, v interface{}) []ValidationError {
	switch val := v.(type) {
	case string:
		var errs []ValidationError
		for _, ph := range knownParamPlaceholder.FindAllString(val, -1) {
			if !validParamDefault.MatchString(ph) {
				errs = append(errs, ValidationError{Field: field, Message: fmt.Sprintf("placeholder %s needs a "+
					"non-empty default of letters, digits, '.', '_' or '-'", ph)})
			}
		}
		return errs
	case map[string]interface{}:
		var errs []ValidationError
		for k, inner := range val {
			errs = append(errs, validateParamPlaceholders(field+"."+k, inner)...)
		}
		return errs
	case []interface{}:
		var errs []ValidationError
		for i, inner := range val {
			errs = append(errs, validateParamPlaceholders(fmt.Sprintf("%s[%d]", field, i), inner)...)
		}
		return errs
	}
	return nil
}

// ModelEndpointWarnings returns advice about an endpoint that is valid but may not
// do what its author expects.
//
// An endpoint's policies are copied into every Model chain it serves, attached at
// API level. advanced-ratelimit nonetheless keys a quota on the route unless the
// quota extracts the API name, and every Model chain is its own route, so such a
// quota counts each Model separately rather than the endpoint as a whole.
func ModelEndpointWarnings(spec api.ModelEndpointConfigData) []string {
	if spec.Policies == nil {
		return nil
	}
	var out []string
	for i, p := range *spec.Policies {
		if p.Name != "advanced-ratelimit" || p.Params == nil {
			continue
		}
		quotas, _ := (*p.Params)["quotas"].([]interface{})
		for j, q := range quotas {
			quota, _ := q.(map[string]interface{})
			if quota == nil || quotaKeysOnAPI(quota) {
				continue
			}
			out = append(out, fmt.Sprintf("spec.policies[%d] (advanced-ratelimit) quota %d keys on the route, so it "+
				"counts each Model separately; add a keyExtraction of type 'apiname' to count the endpoint as a whole", i, j))
		}
	}
	return out
}

// quotaKeysOnAPI reports whether an advanced-ratelimit quota is shared across the
// routes of one API: its keyExtraction names the API and not the route.
func quotaKeysOnAPI(quota map[string]interface{}) bool {
	components, _ := quota["keyExtraction"].([]interface{})
	api, route := false, len(components) == 0
	for _, c := range components {
		comp, _ := c.(map[string]interface{})
		switch t, _ := comp["type"].(string); t {
		case "apiname":
			api = true
		case "routename":
			route = true
		}
	}
	return api && !route
}

func trimmedPtr(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}
