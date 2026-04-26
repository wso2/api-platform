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

package main

import (
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/config"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors/brokerdriver/kafka"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors/receiver/websocket"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors/receiver/websub"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/pkg/engine"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	apikeyauth "github.com/wso2/gateway-controllers/policies/api-key-auth"
	basicauth "github.com/wso2/gateway-controllers/policies/basic-auth"
	setheaders "github.com/wso2/gateway-controllers/policies/set-headers"
)

// registerConnectors registers all built-in receiver and broker-driver factories.
// To add a new receiver or broker-driver type:
//  1. Create the package under connectors/receiver/ or connectors/brokerdriver/
//  2. Register its factory here with the type name
//  3. Add bindings in channels.yaml — no changes to main.go or runtime needed
func registerConnectors(registry *connectors.Registry, cfg *config.Config) {
	registry.RegisterBrokerDriver("kafka", func(brokerDriverCfg map[string]interface{}) (connectors.BrokerDriver, error) {
		brokers := cfg.Kafka.Brokers // fallback to global config
		if brokerDriverCfg != nil {
			if b, ok := brokerDriverCfg["brokers"]; ok {
				switch v := b.(type) {
				case []interface{}:
					parsed := make([]string, 0, len(v))
					for _, item := range v {
						if s, ok := item.(string); ok {
							parsed = append(parsed, s)
						}
					}
					if len(parsed) > 0 {
						brokers = parsed
					}
				case []string:
					if len(v) > 0 {
						brokers = v
					}
				}
			}
		}
		return kafka.NewBrokerDriver(brokers)
	})

	registry.RegisterReceiver("websub", func(ecfg connectors.ReceiverConfig) (connectors.Receiver, error) {
		return websub.NewReceiver(ecfg, websub.Options{
			Port:                       cfg.Server.WebSubHTTPPort,
			VerificationTimeoutSeconds: cfg.WebSub.VerificationTimeoutSeconds,
			DefaultLeaseSeconds:        cfg.WebSub.DefaultLeaseSeconds,
			DeliveryMaxRetries:         cfg.WebSub.DeliveryMaxRetries,
			DeliveryInitialDelayMs:     cfg.WebSub.DeliveryInitialDelayMs,
			DeliveryMaxDelayMs:         cfg.WebSub.DeliveryMaxDelayMs,
			DeliveryConcurrency:        cfg.WebSub.DeliveryConcurrency,
			RuntimeID:                  cfg.RuntimeID,
			ConsumerGroupPrefix:        cfg.Kafka.ConsumerGroupPrefix,
			Brokers:                    cfg.Kafka.Brokers,
		})
	})

	registry.RegisterReceiver("websocket", func(ecfg connectors.ReceiverConfig) (connectors.Receiver, error) {
		return websocket.NewReceiver(ecfg, websocket.Options{
			Port:                cfg.Server.WebSocketPort,
			ConsumerGroupPrefix: cfg.Kafka.ConsumerGroupPrefix,
		})
	})
}

// registerPolicies registers compiled-in policies with the engine.
// Policies are sourced from build.yaml — add entries there to include new policies.
func registerPolicies(eng *engine.Engine) {
	_ = eng.RegisterPolicy(&policy.PolicyDefinition{
		Name:    "basic-auth",
		Version: "v1.0.1",
	}, basicauth.GetPolicy)
	_ = eng.RegisterPolicy(&policy.PolicyDefinition{
		Name:    "set-headers",
		Version: "v1.0.1",
	}, setheaders.GetPolicy)
	_ = eng.RegisterPolicy(&policy.PolicyDefinition{
		Name:    "api-key-auth",
		Version: "v1.0.1",
	}, apikeyauth.GetPolicy)
}
