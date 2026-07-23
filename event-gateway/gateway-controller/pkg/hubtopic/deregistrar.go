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

// Package hubtopic implements gateway-controller (core)'s
// restapi.SetWebSubTopicDeregistrar hook, moved out of core's
// pkg/service/restapi/service.go (deregisterWebSubTopics).
package hubtopic

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"

	eventgatewayconfig "github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/config"
)

// Deregistrar deregisters WebSub hub topics for a deleted WebSubApi config.
type Deregistrar struct {
	deploymentService  *utils.APIDeploymentService
	httpClient          *http.Client
	eventGatewayConfig  eventgatewayconfig.EventGatewayConfig
}

// New creates a new Deregistrar.
func New(deploymentService *utils.APIDeploymentService, httpClient *http.Client, cfg eventgatewayconfig.EventGatewayConfig) *Deregistrar {
	return &Deregistrar{
		deploymentService:  deploymentService,
		httpClient:         httpClient,
		eventGatewayConfig: cfg,
	}
}

// Deregister matches the signature required by restapi.SetWebSubTopicDeregistrar.
func (d *Deregistrar) Deregister(cfg *models.StoredConfig, log *slog.Logger) error {
	topicsToUnregister := d.deploymentService.GetTopicsForDelete(*cfg)

	var deregErrs int32
	var wg sync.WaitGroup

	if len(topicsToUnregister) > 0 {
		wg.Add(1)
		go func(list []string) {
			defer wg.Done()
			log.Info("Starting topic deregistration", slog.Int("total_topics", len(list)), slog.String("api_id", cfg.UUID))
			var childWg sync.WaitGroup
			for _, topic := range list {
				childWg.Add(1)
				go func(topic string) {
					defer childWg.Done()
					ctx, cancel := context.WithTimeout(context.Background(), time.Duration(d.eventGatewayConfig.TimeoutSeconds)*time.Second)
					defer cancel()
					if err := d.deploymentService.UnregisterTopicWithHub(ctx, d.httpClient, topic, d.eventGatewayConfig.RouterHost, d.eventGatewayConfig.WebSubHubListenerPort, log); err != nil {
						log.Error("Failed to deregister topic from WebSubHub",
							slog.Any("error", err),
							slog.String("topic", topic),
							slog.String("api_id", cfg.UUID))
						atomic.AddInt32(&deregErrs, 1)
					} else {
						log.Info("Successfully deregistered topic from WebSubHub",
							slog.String("topic", topic),
							slog.String("api_id", cfg.UUID))
					}
				}(topic)
			}
			childWg.Wait()
		}(topicsToUnregister)
	}

	wg.Wait()

	log.Info("Topic lifecycle operations completed",
		slog.String("api_id", cfg.UUID),
		slog.Int("deregistered", len(topicsToUnregister)),
		slog.Int("deregister_errors", int(deregErrs)))

	if deregErrs > 0 {
		return fmt.Errorf("topic lifecycle operations failed")
	}
	return nil
}
