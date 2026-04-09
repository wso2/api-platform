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
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors/endpoint/kafka"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors/entrypoint/websocket"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors/entrypoint/websub"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/pkg/engine"
)

// registerConnectors registers all built-in entrypoint and endpoint factories.
// To add a new entrypoint or endpoint type:
//  1. Create the package under connectors/entrypoint/ or connectors/endpoint/
//  2. Register its factory here with the type name
//  3. Add bindings in channels.yaml — no changes to main.go or runtime needed
func registerConnectors(registry *connectors.Registry, cfg *config.Config) {
	registry.RegisterEndpoint("kafka", func() (connectors.Endpoint, error) {
		return kafka.NewEndpoint(cfg.Kafka.Brokers)
	})

	registry.RegisterEntrypoint("websub", func(ecfg connectors.EntrypointConfig) (connectors.Entrypoint, error) {
		return websub.NewEntrypoint(ecfg, websub.Options{
			Port:                       cfg.Server.WebSubPort,
			VerificationTimeoutSeconds: cfg.WebSub.VerificationTimeoutSeconds,
			DefaultLeaseSeconds:        cfg.WebSub.DefaultLeaseSeconds,
			DeliveryMaxRetries:         cfg.WebSub.DeliveryMaxRetries,
			DeliveryInitialDelayMs:     cfg.WebSub.DeliveryInitialDelayMs,
			DeliveryMaxDelayMs:         cfg.WebSub.DeliveryMaxDelayMs,
			DeliveryConcurrency:        cfg.WebSub.DeliveryConcurrency,
			RuntimeID:                  cfg.RuntimeID,
			ConsumerGroupPrefix:        cfg.Kafka.ConsumerGroupPrefix,
		})
	})

	registry.RegisterEntrypoint("websocket", func(ecfg connectors.EntrypointConfig) (connectors.Entrypoint, error) {
		return websocket.NewEntrypoint(ecfg, websocket.Options{
			Port:                cfg.Server.WebSocketPort,
			ConsumerGroupPrefix: cfg.Kafka.ConsumerGroupPrefix,
		})
	})
}

// registerPolicies registers compiled-in policies with the engine.
// Add policy registrations here as they become available.
func registerPolicies(eng *engine.Engine) {
	_ = eng
}
