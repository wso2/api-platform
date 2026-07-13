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

// Package kindsupport registers this module's "WebSubApi"/"WebBrokerApi" kind
// support into gateway-controller (core)'s registries — the various
// Register* functions in pkg/storage, pkg/utils, and pkg/policy that core
// exposes as its abstraction layer for kinds it doesn't know about natively.
// Call Register() once during binary startup, before any deployment traffic
// is served.
package kindsupport

import (
	"encoding/json"
	"fmt"
	"strings"

	commonconstants "github.com/wso2/api-platform/common/constants"
	coreconfig "github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policy"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"

	eventgateway "github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/api/eventgateway"
	eventgatewayconfig "github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/policyhooks"
)

// Register wires WebSubApi/WebBrokerApi kind support into every core registry
// that gateway-controller exposes for kinds it doesn't know about natively.
func Register() {
	storage.RegisterKindUnmarshaler("WebSubApi", unmarshalWebSubAPI)
	storage.RegisterKindUnmarshaler("WebBrokerApi", unmarshalWebBrokerApi)

	utils.RegisterKindDeployParser("WebSubApi", parseWebSubDeploy)
	utils.RegisterKindDeployParser("WebBrokerApi", parseWebBrokerDeploy)

	utils.RegisterKindConfigValidator("WebSubApi", eventgatewayconfig.ValidateWebSubAPI)
	utils.RegisterKindConfigValidator("WebBrokerApi", eventgatewayconfig.ValidateWebBrokerAPI)

	utils.RegisterKindTopicsForUpdate("WebSubApi", topicsForWebSubUpdate)

	utils.RegisterKindDisplayNameVersionExtractor("WebSubApi", displayNameVersion)
	utils.RegisterKindDisplayNameVersionExtractor("WebBrokerApi", displayNameVersion)

	utils.RegisterKindVhostSentinelResolver(fmt.Sprintf("%T", eventgateway.WebSubAPI{}), resolveWebSubVhostSentinels)
	utils.RegisterKindVhostSentinelResolver(fmt.Sprintf("%T", eventgateway.WebBrokerApi{}), resolveWebBrokerVhostSentinels)

	policy.RegisterEventGatewayPolicyChainBuilder(policyhooks.BuildPolicyChains)
}

func unmarshalWebSubAPI(cfg *models.StoredConfig, jsonData string) error {
	var config eventgateway.WebSubAPI
	if err := json.Unmarshal([]byte(jsonData), &config); err != nil {
		return fmt.Errorf("failed to unmarshal configuration: %w", err)
	}
	cfg.SourceConfiguration = config
	cfg.Configuration = config
	return nil
}

func unmarshalWebBrokerApi(cfg *models.StoredConfig, jsonData string) error {
	var config eventgateway.WebBrokerApi
	if err := json.Unmarshal([]byte(jsonData), &config); err != nil {
		return fmt.Errorf("failed to unmarshal configuration: %w", err)
	}
	cfg.SourceConfiguration = config
	cfg.Configuration = config
	return nil
}

func annotationValue(annotations *map[string]string, key string) string {
	if annotations == nil {
		return ""
	}
	return (*annotations)[key]
}

func parseWebSubDeploy(parser *coreconfig.Parser, data []byte, contentType string) (any, string, string, string, error) {
	var webSubConfig eventgateway.WebSubAPI
	if err := parser.Parse(data, contentType, &webSubConfig); err != nil {
		return nil, "", "", "", err
	}
	handle := webSubConfig.Metadata.Name
	kind := string(webSubConfig.Kind)
	annotationArtifactID := annotationValue(webSubConfig.Metadata.Annotations, commonconstants.AnnotationArtifactID)
	return webSubConfig, handle, kind, annotationArtifactID, nil
}

func parseWebBrokerDeploy(parser *coreconfig.Parser, data []byte, contentType string) (any, string, string, string, error) {
	var webBrokerConfig eventgateway.WebBrokerApi
	if err := parser.Parse(data, contentType, &webBrokerConfig); err != nil {
		return nil, "", "", "", err
	}
	handle := webBrokerConfig.Metadata.Name
	kind := string(webBrokerConfig.Kind)
	annotationArtifactID := annotationValue(webBrokerConfig.Metadata.Annotations, commonconstants.AnnotationArtifactID)
	return webBrokerConfig, handle, kind, annotationArtifactID, nil
}

func topicsForWebSubUpdate(tm *storage.TopicManager, apiConfig models.StoredConfig) ([]string, []string) {
	topics := tm.GetAllByConfig(apiConfig.UUID)
	topicsToRegister := []string{}
	topicsToUnregister := []string{}
	apiTopicsPerRevision := make(map[string]bool)

	webSubCfg, ok := apiConfig.Configuration.(eventgateway.WebSubAPI)
	if !ok {
		return topicsToRegister, topicsToUnregister
	}
	asyncData := webSubCfg.Spec

	var channels map[string]eventgateway.WebSubChannel
	if asyncData.Channels != nil {
		channels = *asyncData.Channels
	}
	for chName := range channels {
		contextWithVersion := strings.ReplaceAll(asyncData.Context, "$version", asyncData.Version)
		contextWithVersion = strings.TrimPrefix(contextWithVersion, "/")
		contextWithVersion = strings.ReplaceAll(contextWithVersion, "/", "_")
		name := strings.TrimPrefix(chName, "/")
		modifiedTopic := fmt.Sprintf("%s_%s", contextWithVersion, name)
		apiTopicsPerRevision[modifiedTopic] = true
	}

	for _, topic := range topics {
		if _, exists := apiTopicsPerRevision[topic]; !exists {
			topicsToUnregister = append(topicsToUnregister, topic)
		}
	}
	for topic := range apiTopicsPerRevision {
		if tm.IsTopicExist(apiConfig.UUID, topic) {
			continue
		}
		topicsToRegister = append(topicsToRegister, topic)
	}

	return topicsToRegister, topicsToUnregister
}

// UpdateTopicManager matches storage.ConfigStore.SetWebSubTopicUpdater's hook
// signature — call storage.SetWebSubTopicUpdater(kindsupport.UpdateTopicManager)
// once at startup.
func UpdateTopicManager(cfg *models.StoredConfig, tm *storage.TopicManager) error {
	webSubCfg, ok := cfg.Configuration.(eventgateway.WebSubAPI)
	if !ok {
		return fmt.Errorf("configuration is not a WebSubAPI")
	}
	asyncData := webSubCfg.Spec
	apiTopicsPerRevision := make(map[string]bool)
	var channels map[string]eventgateway.WebSubChannel
	if asyncData.Channels != nil {
		channels = *asyncData.Channels
	}
	for chName := range channels {
		contextWithVersion := strings.ReplaceAll(asyncData.Context, "$version", asyncData.Version)
		contextWithVersion = strings.TrimPrefix(contextWithVersion, "/")
		contextWithVersion = strings.ReplaceAll(contextWithVersion, "/", "_")
		name := strings.TrimPrefix(chName, "/")
		modifiedTopic := fmt.Sprintf("%s_%s", contextWithVersion, name)
		tm.Add(cfg.UUID, modifiedTopic)
		apiTopicsPerRevision[modifiedTopic] = true
	}
	for _, topic := range tm.GetAllByConfig(cfg.UUID) {
		if _, exists := apiTopicsPerRevision[topic]; !exists {
			tm.Remove(cfg.UUID, topic)
		}
	}
	return nil
}

func displayNameVersion(configuration any) (string, string, error) {
	switch c := configuration.(type) {
	case eventgateway.WebSubAPI:
		return c.Spec.DisplayName, c.Spec.Version, nil
	case eventgateway.WebBrokerApi:
		return c.Spec.DisplayName, c.Spec.Version, nil
	default:
		return "", "", fmt.Errorf("configuration is not a WebSubAPI or WebBrokerApi (kind: %T)", configuration)
	}
}

func resolveWebSubVhostSentinels(cfg any, routerCfg *coreconfig.RouterConfig) (any, error) {
	c, ok := cfg.(eventgateway.WebSubAPI)
	if !ok {
		return cfg, nil
	}
	if c.Spec.Vhosts == nil {
		main := routerCfg.VHosts.Main.Default
		c.Spec.Vhosts = &struct {
			Main    string  `json:"main" yaml:"main"`
			Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: main,
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

func resolveWebBrokerVhostSentinels(cfg any, routerCfg *coreconfig.RouterConfig) (any, error) {
	c, ok := cfg.(eventgateway.WebBrokerApi)
	if !ok {
		return cfg, nil
	}
	if c.Spec.Vhosts == nil {
		main := routerCfg.VHosts.Main.Default
		c.Spec.Vhosts = &struct {
			Main    string  `json:"main" yaml:"main"`
			Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: main,
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
