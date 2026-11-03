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
	"fmt"

	commonconstants "github.com/wso2/api-platform/common/constants"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

// SoapApi is wired into DeployAPIConfiguration through the same extension points as
// GraphQLApi (see graphql_deployment.go): it self-registers its deploy parser, config
// validator and vhost-sentinel resolver here rather than adding cases to
// api_deployment.go's core switches.
func init() {
	RegisterKindDeployParser(soapApiKind, parseSoapAPIDeployment)
	RegisterKindConfigValidator(soapApiKind, validateSoapAPIConfig)
	RegisterKindVhostSentinelResolver(fmt.Sprintf("%T", api.SoapAPI{}), resolveSoapAPIVhostSentinels)
}

const soapApiKind = string(api.SoapAPIKindSoapApi)

// soapSpecRules supplies the spec-level rules SoapApi shares with RestApi
// (displayName, version, context).
var soapSpecRules = config.NewAPIValidator()

// soapPolicyValidator validates spec.policies references. It is nil until
// SetSoapPolicyValidator is called from main.go, for the same reason as
// graphQLPolicyValidator: the policy definitions are not loaded yet when init() runs.
var soapPolicyValidator *config.PolicyValidator

// SetSoapPolicyValidator wires the shared policy validator into SoapApi config
// validation. Call once from main.go after config.NewPolicyValidator is constructed.
func SetSoapPolicyValidator(pv *config.PolicyValidator) {
	soapPolicyValidator = pv
}

// parseSoapAPIDeployment is the KindDeployParser for SoapApi: the whole request body is
// parsed directly into api.SoapAPI (the deployable shape).
func parseSoapAPIDeployment(parser *config.Parser, data []byte, contentType string) (any, string, string, string, error) {
	var soapConfig api.SoapAPI
	if err := parser.Parse(data, contentType, &soapConfig); err != nil {
		return nil, "", "", "", fmt.Errorf("failed to unmarshal SOAP API configuration: %w", err)
	}
	handle := soapConfig.Metadata.Name
	kind := string(soapConfig.Kind)
	annotationArtifactID := annotationValue(soapConfig.Metadata.Annotations, commonconstants.AnnotationArtifactID)
	return soapConfig, handle, kind, annotationArtifactID, nil
}

// validateSoapAPIConfig is the KindConfigValidator for SoapApi.
func validateSoapAPIConfig(cfg any) (apiName, apiVersion string, validationErrors []config.ValidationError) {
	soapConfig, ok := cfg.(api.SoapAPI)
	if !ok {
		return "", "", []config.ValidationError{{
			Field:   "config",
			Message: fmt.Sprintf("unexpected configuration type %T for SoapApi", cfg),
		}}
	}

	var errors []config.ValidationError

	if soapConfig.Kind != api.SoapAPIKindSoapApi {
		errors = append(errors, config.ValidationError{
			Field:   "kind",
			Message: "Unsupported kind (must be 'SoapApi')",
		})
	}

	errors = append(errors, config.ValidateMetadata(&soapConfig.Metadata)...)

	spec := soapConfig.Spec
	errors = append(errors, soapSpecRules.ValidateDisplayName(spec.DisplayName)...)
	errors = append(errors, soapSpecRules.ValidateVersion(spec.Version)...)
	errors = append(errors, soapSpecRules.ValidateContext(spec.Context)...)

	errors = append(errors, validateDirectUpstream(soapApiKind, "main", &spec.Upstream.Main)...)
	if spec.Upstream.Sandbox != nil {
		errors = append(errors, validateDirectUpstream(soapApiKind, "sandbox", spec.Upstream.Sandbox)...)
	}

	if soapPolicyValidator != nil {
		errors = append(errors, soapPolicyValidator.ValidateSoapAPIPolicies(&soapConfig)...)
	}

	return spec.DisplayName, spec.Version, errors
}

// resolveSoapAPIVhostSentinels freezes a SoapApi's vhosts to concrete hostnames, the
// same way resolveVhostSentinels does for RestApi: omitted vhosts take the current
// gateway defaults, and the gateway-default sentinel is replaced by its value.
func resolveSoapAPIVhostSentinels(cfg any, routerCfg *config.RouterConfig) (any, error) {
	c, ok := cfg.(api.SoapAPI)
	if !ok {
		return cfg, nil
	}
	if c.Spec.Vhosts == nil {
		c.Spec.Vhosts = &struct {
			Main    string  `json:"main" yaml:"main"`
			Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: routerCfg.VHosts.Main.Default,
		}
		if sandboxDefault := routerCfg.VHosts.Sandbox.Default; sandboxDefault != "" {
			c.Spec.Vhosts.Sandbox = &sandboxDefault
		}
		return c, nil
	}
	if c.Spec.Vhosts.Main == constants.VHostGatewayDefault {
		c.Spec.Vhosts.Main = routerCfg.VHosts.Main.Default
	}
	if c.Spec.Vhosts.Sandbox != nil && *c.Spec.Vhosts.Sandbox == constants.VHostGatewayDefault {
		resolved := routerCfg.VHosts.Sandbox.Default
		if resolved != "" {
			c.Spec.Vhosts.Sandbox = &resolved
		} else {
			c.Spec.Vhosts.Sandbox = nil
		}
	}
	return c, nil
}
