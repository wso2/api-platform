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
	"net/url"
	"strings"

	commonconstants "github.com/wso2/api-platform/common/constants"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

// GraphQLApi is not known to api_deployment.go's core switch (it is only handled there
// for "RestApi") — it is wired in generically via the RegisterKindDeployParser /
// RegisterKindConfigValidator extension points that api_deployment.go already exposes
// for kinds not known to core (the same mechanism an event-gateway-controller binary
// uses for WebSubApi/WebBrokerApi). Unlike those, GraphQLApi is compiled directly into
// this binary (not gated behind a build tag), so it self-registers here via init()
// rather than from a separate module's Init().
func init() {
	RegisterKindDeployParser(graphQLApiKind, parseGraphQLAPIDeployment)
	RegisterKindConfigValidator(graphQLApiKind, validateGraphQLAPIConfig)
}

const graphQLApiKind = string(api.GraphQLAPIKindGraphQLApi)

// parseGraphQLAPIDeployment is the KindDeployParser for GraphQLApi. It mirrors the
// "RestApi" case in DeployAPIConfiguration's own switch: the whole request body is
// parsed directly into api.GraphQLAPI (the deployable shape), and identifiers that
// live outside the spec block (kind, metadata.name, artifact-id annotation) are
// extracted for the caller.
func parseGraphQLAPIDeployment(parser *config.Parser, data []byte, contentType string) (any, string, string, string, error) {
	var graphqlConfig api.GraphQLAPI
	if err := parser.Parse(data, contentType, &graphqlConfig); err != nil {
		return nil, "", "", "", fmt.Errorf("failed to unmarshal GraphQL API configuration: %w", err)
	}
	handle := graphqlConfig.Metadata.Name
	kind := string(graphqlConfig.Kind)
	annotationArtifactID := annotationValue(graphqlConfig.Metadata.Annotations, commonconstants.AnnotationArtifactID)
	return graphqlConfig, handle, kind, annotationArtifactID, nil
}

// validateGraphQLAPIConfig is the KindConfigValidator for GraphQLApi. It performs the
// same class of structural validation config.APIValidator applies to RestAPI (kind,
// metadata, upstream url/ref) — duplicated here in miniature rather than extending
// APIValidator's private RestAPI-specific methods, since a GraphQLApi has no
// operations/upstreamDefinitions to validate against. config.ValidateMetadata is
// reused as-is since it is already kind-agnostic (operates on *api.Metadata alone).
func validateGraphQLAPIConfig(cfg any) (apiName, apiVersion string, validationErrors []config.ValidationError) {
	graphqlConfig, ok := cfg.(api.GraphQLAPI)
	if !ok {
		return "", "", []config.ValidationError{{
			Field:   "config",
			Message: fmt.Sprintf("unexpected configuration type %T for GraphQLApi", cfg),
		}}
	}

	var errors []config.ValidationError

	if graphqlConfig.Kind != api.GraphQLAPIKindGraphQLApi {
		errors = append(errors, config.ValidationError{
			Field:   "kind",
			Message: "Unsupported kind (must be 'GraphQLApi')",
		})
	}

	errors = append(errors, config.ValidateMetadata(&graphqlConfig.Metadata)...)

	spec := graphqlConfig.Spec
	if strings.TrimSpace(spec.DisplayName) == "" {
		errors = append(errors, config.ValidationError{Field: "spec.displayName", Message: "displayName is required"})
	}
	if strings.TrimSpace(spec.Version) == "" {
		errors = append(errors, config.ValidationError{Field: "spec.version", Message: "version is required"})
	}
	if strings.TrimSpace(spec.Context) == "" {
		errors = append(errors, config.ValidationError{Field: "spec.context", Message: "context is required"})
	} else if !strings.HasPrefix(spec.Context, "/") {
		errors = append(errors, config.ValidationError{Field: "spec.context", Message: "context must start with '/'"})
	} else if strings.HasSuffix(spec.Context, "/") && spec.Context != "/" {
		errors = append(errors, config.ValidationError{Field: "spec.context", Message: "Context cannot end with / (except for root context)"})
	}

	errors = append(errors, validateGraphQLUpstream("main", &spec.Upstream.Main)...)
	if spec.Upstream.Sandbox != nil {
		errors = append(errors, validateGraphQLUpstream("sandbox", spec.Upstream.Sandbox)...)
	}

	return spec.DisplayName, spec.Version, errors
}

// validateGraphQLUpstream validates a single upstream slot (main or sandbox). A
// GraphQLApi's upstream shape is identical to RestAPI's (reused unmodified from the
// same generated api.Upstream type), so this intentionally mirrors
// config.APIValidator's private validateUpstreamUrl in miniature: GraphQLApi does not
// support upstreamDefinitions references in this pass, so only a direct url is valid.
func validateGraphQLUpstream(label string, up *api.Upstream) []config.ValidationError {
	var errors []config.ValidationError
	if up == nil {
		return errors
	}

	if up.Ref != nil && strings.TrimSpace(*up.Ref) != "" {
		errors = append(errors, config.ValidationError{
			Field:   "spec.upstream." + label + ".ref",
			Message: "Upstream ref is not supported for GraphQLApi (no upstreamDefinitions list); use a direct url",
		})
	}

	if up.Url == nil || strings.TrimSpace(*up.Url) == "" {
		errors = append(errors, config.ValidationError{
			Field:   "spec.upstream." + label + ".url",
			Message: "Upstream URL is required",
		})
		return errors
	}

	parsedURL, err := url.Parse(*up.Url)
	if err != nil {
		errors = append(errors, config.ValidationError{
			Field:   "spec.upstream." + label + ".url",
			Message: fmt.Sprintf("Invalid URL format: %v", err),
		})
		return errors
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		errors = append(errors, config.ValidationError{
			Field:   "spec.upstream." + label + ".url",
			Message: "Upstream URL must use http or https scheme",
		})
	}
	if parsedURL.Host == "" {
		errors = append(errors, config.ValidationError{
			Field:   "spec.upstream." + label + ".url",
			Message: "Upstream URL must include a host",
		})
	}

	return errors
}
