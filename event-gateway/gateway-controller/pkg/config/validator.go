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
	coreconfig "github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"

	eventgateway "github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/api/eventgateway"
)

// urlFriendlyNameRegex/versionRegex mirror gateway-controller (core)'s
// config.APIValidator patterns (see pkg/config/api_validator.go). Duplicated
// here since those fields are unexported on APIValidator; the patterns
// themselves are stable, generic string-format rules, not business logic.
var (
	urlFriendlyNameRegex = regexp.MustCompile(`^[a-zA-Z0-9\-_\. ]+$`)
	versionRegex         = regexp.MustCompile(`^v?\d+(\.\d+)?(\.\d+)?$`)
)

// ValidateWebSubAPI validates a WebSubAPI configuration. Matches the
// utils.KindConfigValidator signature for registration via
// utils.RegisterKindConfigValidator.
func ValidateWebSubAPI(cfg any) (apiName, apiVersion string, validationErrors []coreconfig.ValidationError) {
	config, ok := cfg.(eventgateway.WebSubAPI)
	if !ok {
		return "", "", []coreconfig.ValidationError{{Field: "config", Message: "configuration is not a WebSubAPI"}}
	}

	var errors []coreconfig.ValidationError

	if config.Kind != eventgateway.WebSubAPIKindWebSubApi {
		errors = append(errors, coreconfig.ValidationError{
			Field:   "kind",
			Message: "Unsupported kind (must be 'WebSubApi')",
		})
	}

	if config.ApiVersion != eventgateway.WebSubAPIApiVersionGatewayApiPlatformWso2Comv1 {
		errors = append(errors, coreconfig.ValidationError{
			Field:   "version",
			Message: "Unsupported API version (must be 'gateway.api-platform.wso2.com/v1')",
		})
	}

	errors = append(errors, validateAsyncData(&config.Spec)...)
	mgmtMetadata := api.Metadata(config.Metadata)
	errors = append(errors, coreconfig.ValidateMetadata(&mgmtMetadata)...)

	return config.Spec.DisplayName, config.Spec.Version, errors
}

// ValidateWebBrokerAPI validates a WebBrokerApi configuration. Matches the
// utils.KindConfigValidator signature for registration via
// utils.RegisterKindConfigValidator.
func ValidateWebBrokerAPI(cfg any) (apiName, apiVersion string, validationErrors []coreconfig.ValidationError) {
	config, ok := cfg.(eventgateway.WebBrokerApi)
	if !ok {
		return "", "", []coreconfig.ValidationError{{Field: "config", Message: "configuration is not a WebBrokerApi"}}
	}

	var errors []coreconfig.ValidationError

	if config.Kind != eventgateway.WebBrokerApiKindWebBrokerApi {
		errors = append(errors, coreconfig.ValidationError{
			Field:   "kind",
			Message: "Unsupported kind (must be 'WebBrokerApi')",
		})
	}

	if config.ApiVersion != eventgateway.WebBrokerApiApiVersionGatewayApiPlatformWso2Comv1 {
		errors = append(errors, coreconfig.ValidationError{
			Field:   "version",
			Message: "Unsupported API version (must be 'gateway.api-platform.wso2.com/v1')",
		})
	}

	errors = append(errors, validateWebBrokerData(&config.Spec)...)
	mgmtMetadata := api.Metadata(config.Metadata)
	errors = append(errors, coreconfig.ValidateMetadata(&mgmtMetadata)...)

	return config.Spec.DisplayName, config.Spec.Version, errors
}

// validateWebBrokerData validates the data section of a WebBrokerApi configuration.
func validateWebBrokerData(spec *eventgateway.WebBrokerApiData) []coreconfig.ValidationError {
	var errors []coreconfig.ValidationError

	if spec.DisplayName == "" {
		errors = append(errors, coreconfig.ValidationError{
			Field:   "spec.displayName",
			Message: "API name is required",
		})
	} else if len(spec.DisplayName) > 100 {
		errors = append(errors, coreconfig.ValidationError{
			Field:   "spec.displayName",
			Message: "API name must be 1-100 characters",
		})
	} else if !urlFriendlyNameRegex.MatchString(spec.DisplayName) {
		errors = append(errors, coreconfig.ValidationError{
			Field:   "spec.displayName",
			Message: "API name must be URL-friendly (only letters, numbers, spaces, hyphens, underscores, and dots allowed)",
		})
	}

	if spec.Version == "" {
		errors = append(errors, coreconfig.ValidationError{
			Field:   "spec.version",
			Message: "API version is required",
		})
	} else if !versionRegex.MatchString(spec.Version) {
		errors = append(errors, coreconfig.ValidationError{
			Field:   "spec.version",
			Message: "API version must follow semantic versioning pattern (e.g., v1.0, v2.1.3)",
		})
	}

	v := coreconfig.NewAPIValidator()
	errors = append(errors, v.ValidateContext(spec.Context)...)

	errors = append(errors, validateWebBrokerChannels(spec.Channels)...)

	return errors
}

// validateWebBrokerChannels validates the channels map configuration.
func validateWebBrokerChannels(channels map[string]eventgateway.WebBrokerApiChannel) []coreconfig.ValidationError {
	var errors []coreconfig.ValidationError

	if len(channels) == 0 {
		errors = append(errors, coreconfig.ValidationError{
			Field:   "spec.channels",
			Message: "At least one channel is required",
		})
		return errors
	}

	for chName := range channels {
		if strings.TrimSpace(chName) == "" {
			errors = append(errors, coreconfig.ValidationError{
				Field:   "spec.channels",
				Message: "Channel name (key) must not be empty",
			})
			continue
		}

		if !validatePathParametersForAsyncAPIs(chName) {
			errors = append(errors, coreconfig.ValidationError{
				Field:   fmt.Sprintf("spec.channels.%s", chName),
				Message: "Channel name has {} in parameters",
			})
		}
	}

	return errors
}

// validateAsyncData validates the data section of a WebSubAPI configuration.
func validateAsyncData(spec *eventgateway.WebhookAPIData) []coreconfig.ValidationError {
	var errors []coreconfig.ValidationError

	if spec.DisplayName == "" {
		errors = append(errors, coreconfig.ValidationError{
			Field:   "spec.displayName",
			Message: "API name is required",
		})
	} else if len(spec.DisplayName) > 100 {
		errors = append(errors, coreconfig.ValidationError{
			Field:   "spec.displayName",
			Message: "API name must be 1-100 characters",
		})
	} else if !urlFriendlyNameRegex.MatchString(spec.DisplayName) {
		errors = append(errors, coreconfig.ValidationError{
			Field:   "spec.displayName",
			Message: "API name must be URL-friendly (only letters, numbers, spaces, hyphens, underscores, and dots allowed)",
		})
	}

	if spec.Version == "" {
		errors = append(errors, coreconfig.ValidationError{
			Field:   "spec.version",
			Message: "API version is required",
		})
	} else if !versionRegex.MatchString(spec.Version) {
		errors = append(errors, coreconfig.ValidationError{
			Field:   "spec.version",
			Message: "API version must follow semantic versioning pattern (e.g., v1.0, v2.1.3)",
		})
	}

	v := coreconfig.NewAPIValidator()
	errors = append(errors, v.ValidateContext(spec.Context)...)

	var channels map[string]eventgateway.WebSubChannel
	if spec.Channels != nil {
		channels = *spec.Channels
	}
	errors = append(errors, validateChannelPolicies(channels)...)

	return errors
}

// validateChannelPolicies validates the channels map configuration.
func validateChannelPolicies(channelPolicies map[string]eventgateway.WebSubChannel) []coreconfig.ValidationError {
	var errors []coreconfig.ValidationError

	if len(channelPolicies) == 0 {
		errors = append(errors, coreconfig.ValidationError{
			Field:   "spec.channels",
			Message: "At least one channel is required",
		})
		return errors
	}

	for chName := range channelPolicies {
		if strings.TrimSpace(chName) == "" {
			errors = append(errors, coreconfig.ValidationError{
				Field:   "spec.channels",
				Message: "Channel name (key) must not be empty",
			})
			continue
		}

		if !validatePathParametersForAsyncAPIs(chName) {
			errors = append(errors, coreconfig.ValidationError{
				Field:   fmt.Sprintf("spec.channels.%s", chName),
				Message: "Channel name has {} in parameters",
			})
		}
	}

	return errors
}

// validatePathParametersForAsyncAPIs returns true when the path does not
// contain '{' or '}'. Async/WebSub channel paths do not currently support
// templated path parameters.
func validatePathParametersForAsyncAPIs(path string) bool {
	return !strings.Contains(path, "{") && !strings.Contains(path, "}")
}
