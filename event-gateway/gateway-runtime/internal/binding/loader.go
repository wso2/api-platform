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

package binding

import (
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/hub"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/pkg/engine"
	"gopkg.in/yaml.v3"
)

// LoadChannels reads channels.yaml and registers all channel bindings in the hub.
func LoadChannels(filePath string, eng *engine.Engine, h *hub.Hub) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read channels file %s: %w", filePath, err)
	}

	var cfg ChannelsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse channels file %s: %w", filePath, err)
	}

	for _, binding := range cfg.Channels {
		if err := registerBinding(binding, eng, h); err != nil {
			return fmt.Errorf("failed to register binding %s: %w", binding.Name, err)
		}
	}

	return nil
}

// registerBinding creates route keys from the binding and builds policy chains.
func registerBinding(b Binding, eng *engine.Engine, h *hub.Hub) error {
	vhost := b.Vhost
	if vhost == "" {
		vhost = "*"
	}

	var inboundKey, outboundKey string

	switch b.Mode {
	case "websub":
		// WebSub bindings produce multiple route keys
		channelPath := path.Join(b.Context, b.Name)
		inboundKey = generateRouteKey("SUB", channelPath, vhost)
		outboundKey = generateRouteKey("DELIVER", channelPath, vhost)

		hubPath := path.Join(b.Context, "_hub")
		registerKey := generateRouteKey("REGISTER", hubPath, vhost)
		hubSubscribeKey := generateRouteKey("HUB_SUBSCRIBE", hubPath, vhost)

		// Build policy chains for hub operations if policies are defined
		if len(b.Policies.Inbound) > 0 {
			if err := buildAndRegisterChain(eng, registerKey, b.Policies.Inbound); err != nil {
				return err
			}
			if err := buildAndRegisterChain(eng, hubSubscribeKey, b.Policies.Inbound); err != nil {
				return err
			}
		}

	case "protocol-mediation":
		channelPath := path.Join(b.Context, b.Entrypoint.Path)
		inboundKey = generateRouteKey("PUBLISH", channelPath, vhost)
		outboundKey = generateRouteKey("DELIVER", channelPath, vhost)

	default:
		return fmt.Errorf("unknown binding mode: %s", b.Mode)
	}

	// Build inbound chain
	if len(b.Policies.Inbound) > 0 {
		if err := buildAndRegisterChain(eng, inboundKey, b.Policies.Inbound); err != nil {
			return err
		}
	}

	// Build outbound chain
	if len(b.Policies.Outbound) > 0 {
		if err := buildAndRegisterChain(eng, outboundKey, b.Policies.Outbound); err != nil {
			return err
		}
	}

	// Register binding in hub
	h.RegisterBinding(hub.ChannelBinding{
		Name:             b.Name,
		Mode:             b.Mode,
		Context:          b.Context,
		Version:          b.Version,
		Vhost:            vhost,
		InboundChainKey:  inboundKey,
		OutboundChainKey: outboundKey,
		EndpointTopic:    b.Endpoint.Topic,
		Ordering:         b.Endpoint.Ordering,
	})

	slog.Info("Registered channel binding",
		"name", b.Name,
		"mode", b.Mode,
		"inbound_key", inboundKey,
		"outbound_key", outboundKey,
	)

	return nil
}

func buildAndRegisterChain(eng *engine.Engine, routeKey string, policies []PolicyRef) error {
	specs := make([]engine.PolicySpec, len(policies))
	for i, p := range policies {
		specs[i] = engine.PolicySpec{
			Name:       p.Name,
			Version:    p.Version,
			Enabled:    true,
			Parameters: p.Params,
		}
	}

	chain, err := eng.BuildChain(routeKey, specs)
	if err != nil {
		return fmt.Errorf("failed to build chain %s: %w", routeKey, err)
	}
	eng.RegisterChain(routeKey, chain)
	return nil
}

// generateRouteKey creates a route key in the Method|Path|Vhost format.
func generateRouteKey(method, fullPath, vhost string) string {
	return fmt.Sprintf("%s|%s|%s", method, fullPath, vhost)
}
