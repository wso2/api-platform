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
	"math"
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

// webSocketSpecRules supplies the spec.displayName and spec.version rules
// WebSocketApi shares with RestApi and GraphQLApi, so every kind accepts the same
// names and version formats.
var webSocketSpecRules = config.NewAPIValidator()

// webSocketValidation holds the gateway-level inputs validateWebSocketAPIConfig
// needs but a KindConfigValidator is not handed. Zero values (an unconfigured
// binary) skip the bound and policy checks rather than failing every deploy.
var webSocketValidation struct {
	upstream             config.WebSocketConnectionLimits
	policyValidator      *config.PolicyValidator
	upstreamTLSValidator *config.UpstreamTLSValidator
}

// ConfigureWebSocketAPIValidation supplies the gateway's upstream connection
// bounds, policy definitions and upstream TLS validator to deploy-time
// validation, so an out-of-bounds setting, an unknown policy or an unusable tls
// block is rejected with a 400 instead of being accepted and then failing
// silently when the route is built. Call once at startup.
func ConfigureWebSocketAPIValidation(limits config.WebSocketConfig, policyValidator *config.PolicyValidator, upstreamTLSValidator *config.UpstreamTLSValidator) {
	webSocketValidation.upstream = limits.Upstream
	webSocketValidation.policyValidator = policyValidator
	webSocketValidation.upstreamTLSValidator = upstreamTLSValidator
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
// mirrors validateGraphQLAPIConfig's structural checks and adds what a
// WebSocketApi has that a GraphQLApi does not: a channel list, and upstream
// definitions carrying connection-pool bounds for the backend connections.
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
	errors = append(errors, webSocketSpecRules.ValidateDisplayName(strings.TrimSpace(spec.DisplayName))...)
	errors = append(errors, webSocketSpecRules.ValidateVersion(strings.TrimSpace(spec.Version))...)
	if strings.TrimSpace(spec.Context) == "" {
		errors = append(errors, config.ValidationError{Field: "spec.context", Message: "context is required"})
	} else if !strings.HasPrefix(spec.Context, "/") {
		errors = append(errors, config.ValidationError{Field: "spec.context", Message: "context must start with '/'"})
	} else if strings.HasSuffix(spec.Context, "/") && spec.Context != "/" {
		errors = append(errors, config.ValidationError{Field: "spec.context", Message: "Context cannot end with / (except for root context)"})
	}

	defNames, defErrors := validateWebSocketUpstreamDefinitions(spec.UpstreamDefinitions)
	errors = append(errors, defErrors...)
	errors = append(errors, validateWebSocketUpstream("main", &spec.Upstream.Main, defNames)...)
	if spec.Upstream.Sandbox != nil {
		errors = append(errors, validateWebSocketUpstream("sandbox", spec.Upstream.Sandbox, defNames)...)
	}

	errors = append(errors, validateWebSocketChannels(spec.Channels)...)

	if webSocketValidation.policyValidator != nil {
		errors = append(errors, webSocketValidation.policyValidator.ValidateWebSocketAPIPolicies(&websocketConfig)...)
	}
	if webSocketValidation.upstreamTLSValidator != nil {
		errors = append(errors, webSocketValidation.upstreamTLSValidator.ValidateWebSocketAPI(&websocketConfig)...)
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

// validateBoundedConnections requires a connection count (maxConnections or
// maxPendingHandshakes) of at least 1 and no more than the gateway's maximum; a
// zero maximum means the gateway sets none.
func validateBoundedConnections(field string, value int, maxValue uint32) []config.ValidationError {
	if value < 1 {
		return []config.ValidationError{{Field: field, Message: field[strings.LastIndex(field, ".")+1:] + " must be at least 1"}}
	}
	// Envoy's circuit-breaker threshold is a uint32; a larger value would wrap
	// when narrowed, so it is rejected even when the gateway sets no maximum.
	if uint64(value) > math.MaxUint32 {
		return []config.ValidationError{{
			Field:   field,
			Message: fmt.Sprintf("%d exceeds the largest supported value of %d", value, uint32(math.MaxUint32)),
		}}
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

// webSocketUpstreamDefinitionNamePattern is the name rule a RestApi upstream
// definition follows, so a definition name means the same thing on every kind.
var webSocketUpstreamDefinitionNamePattern = regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`)

// validateWebSocketUpstreamDefinitions validates spec.upstreamDefinitions and
// returns the set of valid definition names, which upstream slots may reference.
// Every target in a definition shares one cluster, and so one scheme: all ws://
// or all wss://. A tls block is only meaningful on wss:// targets.
func validateWebSocketUpstreamDefinitions(defs *[]api.WebSocketUpstreamDefinition) (map[string]bool, []config.ValidationError) {
	names := map[string]bool{}
	var errors []config.ValidationError
	if defs == nil {
		return names, errors
	}

	for i, def := range *defs {
		fieldBase := fmt.Sprintf("spec.upstreamDefinitions[%d]", i)
		name := strings.TrimSpace(def.Name)
		switch {
		case name == "":
			errors = append(errors, config.ValidationError{Field: fieldBase + ".name", Message: "Upstream definition name is required"})
		case len(name) > 100 || !webSocketUpstreamDefinitionNamePattern.MatchString(name):
			errors = append(errors, config.ValidationError{
				Field:   fieldBase + ".name",
				Message: "Upstream definition name must be 1-100 characters of letters, numbers, hyphens and underscores",
			})
		case names[name]:
			errors = append(errors, config.ValidationError{Field: fieldBase + ".name", Message: fmt.Sprintf("Duplicate upstream definition name %q", name)})
		default:
			names[name] = true
		}

		if def.BasePath != nil {
			if bp := *def.BasePath; !strings.HasPrefix(bp, "/") || (len(bp) > 1 && strings.HasSuffix(bp, "/")) {
				errors = append(errors, config.ValidationError{
					Field:   fieldBase + ".basePath",
					Message: "basePath must start with '/' and must not end with '/'",
				})
			}
		}

		if len(def.Upstreams) == 0 {
			errors = append(errors, config.ValidationError{Field: fieldBase + ".upstreams", Message: "At least one upstream target is required"})
		}
		secure, plain := false, false
		for t, target := range def.Upstreams {
			field := fmt.Sprintf("%s.upstreams[%d].url", fieldBase, t)
			parsedURL, err := url.Parse(strings.TrimSpace(target.Url))
			switch {
			case strings.TrimSpace(target.Url) == "":
				errors = append(errors, config.ValidationError{Field: field, Message: "Upstream URL is required"})
				continue
			case err != nil:
				errors = append(errors, config.ValidationError{Field: field, Message: fmt.Sprintf("Invalid URL format: %v", err)})
				continue
			case parsedURL.Scheme != "ws" && parsedURL.Scheme != "wss":
				errors = append(errors, config.ValidationError{Field: field, Message: "Upstream URL must use ws or wss scheme"})
				continue
			case parsedURL.Host == "":
				errors = append(errors, config.ValidationError{Field: field, Message: "Upstream URL must include a host"})
				continue
			case parsedURL.Path != "" && parsedURL.Path != "/":
				errors = append(errors, config.ValidationError{Field: field, Message: "Upstream URL must be host and port only; set the path with basePath"})
				continue
			}
			if parsedURL.Scheme == "wss" {
				secure = true
			} else {
				plain = true
			}
		}
		if secure && plain {
			errors = append(errors, config.ValidationError{
				Field:   fieldBase + ".upstreams",
				Message: "All targets in an upstream definition must use the same scheme (all ws:// or all wss://)",
			})
		}
		if def.Tls != nil && plain {
			errors = append(errors, config.ValidationError{Field: fieldBase + ".tls", Message: "tls requires every target in the definition to use wss://"})
		}

		if def.Timeout != nil && def.Timeout.Connect != nil {
			errors = append(errors, validatePositiveDuration(fieldBase+".timeout.connect", *def.Timeout.Connect)...)
		}
		errors = append(errors, validateWebSocketUpstreamConnection(fieldBase+".connection", def.Connection)...)
	}

	return names, errors
}

// validateWebSocketUpstreamConnection checks a definition's connection-count
// bounds are positive and stay within the gateway's configured upstream
// maximums — the same bounds the transformer enforces when it builds the
// cluster, checked here so an out-of-bounds value fails the deploy instead of
// leaving a stored API with no route.
func validateWebSocketUpstreamConnection(fieldBase string, conn *api.WebSocketUpstreamConnection) []config.ValidationError {
	var errors []config.ValidationError
	if conn == nil {
		return errors
	}
	limits := webSocketValidation.upstream

	if conn.MaxConnections != nil {
		errors = append(errors, validateBoundedConnections(fieldBase+".maxConnections", *conn.MaxConnections, limits.MaxConnectionsMax)...)
	}
	if conn.MaxPendingHandshakes != nil {
		errors = append(errors, validateBoundedConnections(fieldBase+".maxPendingHandshakes", *conn.MaxPendingHandshakes, 0)...)
	}

	return errors
}

// validateWebSocketUpstream validates one upstream slot: either a direct ws/wss
// url, which uses the gateway-wide connection defaults, or a ref to one of the
// API's upstream definitions, which carries its own connection settings and tls.
func validateWebSocketUpstream(label string, up *api.Upstream, defNames map[string]bool) []config.ValidationError {
	var errors []config.ValidationError
	if up == nil {
		return errors
	}

	fieldBase := "spec.upstream." + label
	hasURL := up.Url != nil && strings.TrimSpace(*up.Url) != ""
	hasRef := up.Ref != nil && strings.TrimSpace(*up.Ref) != ""

	// As on a RestApi, connection settings and tls belong to an upstream
	// definition; on an inline url a tls block would otherwise be silently ignored.
	if up.Tls != nil {
		errors = append(errors, config.ValidationError{
			Field:   fieldBase + ".tls",
			Message: "tls is not supported on an inline upstream; put it on an upstreamDefinitions entry and reference it with ref",
		})
	}

	switch {
	case hasURL && hasRef:
		return append(errors, config.ValidationError{Field: fieldBase, Message: "Set either url or ref, not both"})
	case !hasURL && !hasRef:
		return append(errors, config.ValidationError{Field: fieldBase + ".url", Message: "Upstream url or ref is required"})
	case hasRef:
		if ref := strings.TrimSpace(*up.Ref); !defNames[ref] {
			errors = append(errors, config.ValidationError{
				Field:   fieldBase + ".ref",
				Message: fmt.Sprintf("Upstream ref %q does not match any upstreamDefinitions entry", ref),
			})
		}
		return errors
	}

	parsedURL, err := url.Parse(strings.TrimSpace(*up.Url))
	if err != nil {
		return append(errors, config.ValidationError{Field: fieldBase + ".url", Message: fmt.Sprintf("Invalid URL format: %v", err)})
	}
	if parsedURL.Scheme != "ws" && parsedURL.Scheme != "wss" {
		errors = append(errors, config.ValidationError{Field: fieldBase + ".url", Message: "Upstream URL must use ws or wss scheme"})
	}
	if parsedURL.Host == "" {
		errors = append(errors, config.ValidationError{Field: fieldBase + ".url", Message: "Upstream URL must include a host"})
	}
	return errors
}
