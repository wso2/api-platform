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
	"regexp"
	"strings"
	"time"

	commonconstants "github.com/wso2/api-platform/common/constants"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// WebSocketApi is wired in through the same generic extension points GraphQLApi
// uses — RegisterKindDeployParser / RegisterKindConfigValidator — rather than a
// case in api_deployment.go's core switch. It is compiled into this binary
// directly, so it self-registers here via init() rather than from a separate
// module's Init().
func init() {
	RegisterKindDeployParser(webSocketApiKind, parseWebSocketAPIDeployment)
	RegisterKindConfigValidator(webSocketApiKind, validateWebSocketAPIConfig)
	RegisterKindDesiredStateResolver(webSocketApiKind, webSocketAPIDesiredState)
	RegisterKindRouteContextResolver(webSocketApiKind, webSocketAPIRouteContext)
}

const webSocketApiKind = string(api.WebSocketAPIKindWebSocketApi)

// webSocketVersionPattern is the spec.version format the WebSocketApi schema
// declares: major-only (v1) or major.minor (v1.0).
var webSocketVersionPattern = regexp.MustCompile(`^v\d+(\.\d+)?$`)

// webSocketValidation holds the gateway-level inputs validateWebSocketAPIConfig
// needs but a KindConfigValidator is not handed. Zero values (an unconfigured
// binary) skip the bound and policy checks rather than failing every deploy.
var webSocketValidation struct {
	downstream      config.WebSocketConnectionLimits
	upstream        config.WebSocketConnectionLimits
	policyValidator *config.PolicyValidator
}

// ConfigureWebSocketAPIValidation supplies the gateway's WebSocket connection
// bounds and policy definitions to deploy-time validation, so an out-of-bounds
// setting or an unknown policy is rejected with a 400 instead of being accepted
// and then failing silently when the route is built. Call once at startup.
func ConfigureWebSocketAPIValidation(limits config.WebSocketConfig, policyValidator *config.PolicyValidator) {
	webSocketValidation.downstream = limits.Downstream
	webSocketValidation.upstream = limits.Upstream
	webSocketValidation.policyValidator = policyValidator
}

// webSocketAPIRouteContext is the KindRouteContextResolver for WebSocketApi.
// WebSocket APIs carry no vhosts, so every one is served on the gateway's
// default virtual hosts and its context alone identifies its routes.
func webSocketAPIRouteContext(cfg any) string {
	websocketConfig, ok := cfg.(api.WebSocketAPI)
	if !ok {
		return ""
	}
	return strings.ReplaceAll(websocketConfig.Spec.Context, "$version", websocketConfig.Spec.Version)
}

// webSocketAPIDesiredState is the KindDesiredStateResolver for WebSocketApi.
func webSocketAPIDesiredState(cfg any) models.DesiredState {
	websocketConfig, ok := cfg.(api.WebSocketAPI)
	if ok && websocketConfig.Spec.DeploymentState != nil &&
		*websocketConfig.Spec.DeploymentState == api.WebSocketAPIConfigDataDeploymentStateUndeployed {
		return models.StateUndeployed
	}
	return models.StateDeployed
}

// parseWebSocketAPIDeployment is the KindDeployParser for WebSocketApi. The whole
// request body parses directly into api.WebSocketAPI (the deployable shape), and
// the identifiers living outside the spec block are extracted for the caller.
func parseWebSocketAPIDeployment(parser *config.Parser, data []byte, contentType string) (any, string, string, string, error) {
	var websocketConfig api.WebSocketAPI
	if err := parser.Parse(data, contentType, &websocketConfig); err != nil {
		return nil, "", "", "", fmt.Errorf("failed to unmarshal WebSocket API configuration: %w", err)
	}
	handle := websocketConfig.Metadata.Name
	kind := string(websocketConfig.Kind)
	annotationArtifactID := annotationValue(websocketConfig.Metadata.Annotations, commonconstants.AnnotationArtifactID)
	return websocketConfig, handle, kind, annotationArtifactID, nil
}

// validateWebSocketAPIConfig is the KindConfigValidator for WebSocketApi. It
// mirrors validateGraphQLAPIConfig's structural checks and adds the two things a
// WebSocketApi has that a GraphQLApi does not: a channel list, and connection
// settings on both the downstream connection and each upstream slot.
//
// Policy references are checked through the PolicyValidator supplied by
// ConfigureWebSocketAPIValidation. Rejecting a policy that needs the message
// body is not done here: policy definitions do not record a policy's processing
// mode (tasks.md T026), so the policy engine forces body processing off for
// WebSocketApi routes instead.
func validateWebSocketAPIConfig(cfg any) (apiName, apiVersion string, validationErrors []config.ValidationError) {
	websocketConfig, ok := cfg.(api.WebSocketAPI)
	if !ok {
		return "", "", []config.ValidationError{{
			Field:   "config",
			Message: fmt.Sprintf("unexpected configuration type %T for WebSocketApi", cfg),
		}}
	}

	var errors []config.ValidationError

	if websocketConfig.Kind != api.WebSocketAPIKindWebSocketApi {
		errors = append(errors, config.ValidationError{
			Field:   "kind",
			Message: "Unsupported kind (must be 'WebSocketApi')",
		})
	}

	errors = append(errors, config.ValidateMetadata(&websocketConfig.Metadata)...)

	spec := websocketConfig.Spec
	if strings.TrimSpace(spec.DisplayName) == "" {
		errors = append(errors, config.ValidationError{Field: "spec.displayName", Message: "displayName is required"})
	}
	if strings.TrimSpace(spec.Version) == "" {
		errors = append(errors, config.ValidationError{Field: "spec.version", Message: "version is required"})
	} else if !webSocketVersionPattern.MatchString(spec.Version) {
		errors = append(errors, config.ValidationError{
			Field:   "spec.version",
			Message: "version must look like 'v1' or 'v1.0'",
		})
	}
	if strings.TrimSpace(spec.Context) == "" {
		errors = append(errors, config.ValidationError{Field: "spec.context", Message: "context is required"})
	} else if !strings.HasPrefix(spec.Context, "/") {
		errors = append(errors, config.ValidationError{Field: "spec.context", Message: "context must start with '/'"})
	} else if strings.HasSuffix(spec.Context, "/") && spec.Context != "/" {
		errors = append(errors, config.ValidationError{Field: "spec.context", Message: "Context cannot end with / (except for root context)"})
	}

	errors = append(errors, validateWebSocketUpstream("main", &spec.Upstream.Main)...)
	if spec.Upstream.Sandbox != nil {
		errors = append(errors, validateWebSocketUpstream("sandbox", spec.Upstream.Sandbox)...)
	}

	errors = append(errors, validateWebSocketChannels(spec.Channels)...)
	errors = append(errors, validateWebSocketConnectionSettings(spec.Connection)...)

	if webSocketValidation.policyValidator != nil {
		errors = append(errors, webSocketValidation.policyValidator.ValidateWebSocketAPIPolicies(&websocketConfig)...)
	}

	return spec.DisplayName, spec.Version, errors
}

// validateWebSocketChannels enforces FR-003's channel rules: at least one
// channel, each with a path, and no two channels sharing a path.
//
// Duplicate paths are rejected rather than last-one-wins because the routes they
// generate are keyed by path: a duplicate would silently drop one channel's
// policies while still appearing in the artifact.
func validateWebSocketChannels(channels []api.WebSocketChannel) []config.ValidationError {
	var errors []config.ValidationError

	if len(channels) == 0 {
		return append(errors, config.ValidationError{
			Field:   "spec.channels",
			Message: "At least one channel is required",
		})
	}

	seen := make(map[string]int, len(channels))
	for i, channel := range channels {
		field := fmt.Sprintf("spec.channels[%d].path", i)
		path := strings.TrimSpace(channel.Path)

		if path == "" {
			errors = append(errors, config.ValidationError{Field: field, Message: "Channel path is required"})
			continue
		}
		if !strings.HasPrefix(path, "/") {
			errors = append(errors, config.ValidationError{Field: field, Message: "Channel path must start with '/'"})
			continue
		}
		if first, dup := seen[path]; dup {
			errors = append(errors, config.ValidationError{
				Field:   field,
				Message: fmt.Sprintf("Duplicate channel path %q (already defined at spec.channels[%d])", path, first),
			})
			continue
		}
		seen[path] = i
	}

	return errors
}

// validateWebSocketConnectionSettings checks the downstream connection block's
// durations parse, are positive, and stay within the gateway's configured
// maximums — the same bounds the transformer enforces when it builds the route,
// checked here so an out-of-bounds value fails the deploy instead of leaving a
// stored API with no route.
func validateWebSocketConnectionSettings(conn *api.WebSocketConnectionSettings) []config.ValidationError {
	var errors []config.ValidationError
	if conn == nil {
		return errors
	}
	limits := webSocketValidation.downstream

	if conn.IdleTimeout != nil {
		errors = append(errors, validateBoundedDuration("spec.connection.idleTimeout", *conn.IdleTimeout, limits.IdleTimeoutMax)...)
	}
	if conn.MaxConnectionDuration != nil {
		errors = append(errors, validateBoundedDuration("spec.connection.maxConnectionDuration", *conn.MaxConnectionDuration, limits.MaxConnectionDurationMax)...)
	}
	if conn.MaxConnections != nil {
		errors = append(errors, validateBoundedConnections("spec.connection.maxConnections", *conn.MaxConnections, limits.MaxConnectionsMax)...)
	}

	return errors
}

// validateBoundedDuration is validatePositiveDuration plus the gateway ceiling;
// a zero ceiling means the gateway sets none.
func validateBoundedDuration(field, raw string, maxValue time.Duration) []config.ValidationError {
	if errs := validatePositiveDuration(field, raw); len(errs) > 0 {
		return errs
	}
	parsed, _ := time.ParseDuration(strings.TrimSpace(raw))
	if maxValue > 0 && parsed > maxValue {
		return []config.ValidationError{{
			Field:   field,
			Message: fmt.Sprintf("%s exceeds the gateway's configured maximum of %s", raw, maxValue),
		}}
	}
	return nil
}

// validateBoundedConnections requires a connection ceiling of at least 1 and no
// more than the gateway's maximum; a zero maximum means the gateway sets none.
func validateBoundedConnections(field string, value int, maxValue uint32) []config.ValidationError {
	if value < 1 {
		return []config.ValidationError{{Field: field, Message: "maxConnections must be at least 1"}}
	}
	if maxValue > 0 && uint64(value) > uint64(maxValue) {
		return []config.ValidationError{{
			Field:   field,
			Message: fmt.Sprintf("%d exceeds the gateway's configured maximum of %d", value, maxValue),
		}}
	}
	return nil
}

// validatePositiveDuration parses a duration string and requires it to be > 0.
// A zero or negative value is rejected rather than treated as "unlimited": for a
// WebSocketApi that would be exactly the unbounded connection these settings
// exist to prevent.
func validatePositiveDuration(field, raw string) []config.ValidationError {
	parsed, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return []config.ValidationError{{
			Field:   field,
			Message: fmt.Sprintf("Invalid duration %q (expected a value such as \"300s\", \"30m\" or \"1h\")", raw),
		}}
	}
	if parsed <= 0 {
		return []config.ValidationError{{
			Field:   field,
			Message: "Duration must be greater than zero",
		}}
	}
	return nil
}

// validateWebSocketUpstream validates one upstream slot. A WebSocketApi has no
// upstreamDefinitions list, so a ref cannot be resolved; and the URL must be a
// WebSocket scheme, since the gateway opens the backend leg as a WebSocket.
func validateWebSocketUpstream(label string, up *api.Upstream) []config.ValidationError {
	var errors []config.ValidationError
	if up == nil {
		return errors
	}

	fieldBase := "spec.upstream." + label

	if up.Ref != nil && strings.TrimSpace(*up.Ref) != "" {
		errors = append(errors, config.ValidationError{
			Field:   fieldBase + ".ref",
			Message: "Upstream ref is not supported for WebSocketApi (no upstreamDefinitions list); use a direct url",
		})
	}

	if up.Url == nil || strings.TrimSpace(*up.Url) == "" {
		errors = append(errors, config.ValidationError{
			Field:   fieldBase + ".url",
			Message: "Upstream URL is required",
		})
		return append(errors, validateWebSocketUpstreamTimeout(fieldBase, up.Timeout)...)
	}

	parsedURL, err := url.Parse(*up.Url)
	if err != nil {
		errors = append(errors, config.ValidationError{
			Field:   fieldBase + ".url",
			Message: fmt.Sprintf("Invalid URL format: %v", err),
		})
		return append(errors, validateWebSocketUpstreamTimeout(fieldBase, up.Timeout)...)
	}

	if parsedURL.Scheme != "ws" && parsedURL.Scheme != "wss" {
		errors = append(errors, config.ValidationError{
			Field:   fieldBase + ".url",
			Message: "Upstream URL must use ws or wss scheme",
		})
	}
	if parsedURL.Host == "" {
		errors = append(errors, config.ValidationError{
			Field:   fieldBase + ".url",
			Message: "Upstream URL must include a host",
		})
	}

	return append(errors, validateWebSocketUpstreamTimeout(fieldBase, up.Timeout)...)
}

// validateWebSocketUpstreamTimeout checks one slot's connection-pool settings.
// These bound the gateway-to-backend connection and are independent of the
// API-level (downstream) connection block.
func validateWebSocketUpstreamTimeout(fieldBase string, timeout *api.UpstreamTimeout) []config.ValidationError {
	var errors []config.ValidationError
	if timeout == nil {
		return errors
	}

	if timeout.Connect != nil {
		errors = append(errors, validatePositiveDuration(fieldBase+".timeout.connect", *timeout.Connect)...)
	}
	limits := webSocketValidation.upstream
	if timeout.IdleTimeout != nil {
		errors = append(errors, validateBoundedDuration(fieldBase+".timeout.idleTimeout", *timeout.IdleTimeout, limits.IdleTimeoutMax)...)
	}
	if timeout.MaxConnectionDuration != nil {
		errors = append(errors, validateBoundedDuration(fieldBase+".timeout.maxConnectionDuration", *timeout.MaxConnectionDuration, limits.MaxConnectionDurationMax)...)
	}
	if timeout.MaxConnections != nil {
		errors = append(errors, validateBoundedConnections(fieldBase+".timeout.maxConnections", *timeout.MaxConnections, limits.MaxConnectionsMax)...)
	}

	return errors
}
