/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package publishers

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/moesif/moesifapi-go"
	"github.com/moesif/moesifapi-go/models"
	"github.com/wso2/api-platform/gateway/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/policy-engine/internal/config"
)

const (
	anonymous = "anonymous"
)

// Moesif represents a Moesif publisher.
type Moesif struct {
	cfg    *config.PublisherConfig
	api    moesifapi.API
	events []*models.EventModel
	mu     sync.Mutex
}

// MoesifConfig holds the configs specific for the Moesif publisher.
type MoesifConfig struct {
	ApplicationID      string `mapstructure:"application_id" default:""`
	PublishInterval    int    `mapstructure:"publish_interval" default:"5"`
	EventQueueSize     int    `mapstructure:"event_queue_size" default:"10000"`
	BatchSize          int    `mapstructure:"batch_size" default:"50"`
	TimerWakeupSeconds int    `mapstructure:"timer_wakeup_seconds" default:"3"`
}

// NewMoesif creates a new Moesif publisher.
func NewMoesif(pubCfg *config.PublisherConfig) *Moesif {
	moesifCfg := &MoesifConfig{}

	err := mapstructure.Decode(pubCfg.Settings, moesifCfg)
	if err != nil {
		slog.Error("Error decoding Moesif config", "error", err)
		return nil
	}

	// Read moesifApplicationId from environment variable first, fallback to config
	moesifApplicationId := os.Getenv("MOESIF_KEY")
	if moesifApplicationId == "" {
		moesifApplicationId = moesifCfg.ApplicationID
	}

	// Moesif Client Configs
	eventQueueSize, batchSize, timerWakeupSeconds :=
		moesifCfg.EventQueueSize,
		moesifCfg.BatchSize,
		moesifCfg.TimerWakeupSeconds

	apiClient := moesifapi.NewAPI(moesifApplicationId, nil, eventQueueSize, batchSize, timerWakeupSeconds)
	moesif := &Moesif{
		cfg:    pubCfg,
		events: []*models.EventModel{},
		api:    apiClient,
		mu:     sync.Mutex{},
	}
	go func() {
		for {
			time.Sleep(time.Duration(moesifCfg.PublishInterval) * time.Second)
			moesif.mu.Lock()
			if len(moesif.events) > 0 {
				slog.Info(fmt.Sprintf("Publishing %d events to Moesif", len(moesif.events)))
				err := moesif.api.QueueEvents(moesif.events)
				if err != nil {
					slog.Error("Error publishing events to Moesif", "error", err)
				}
				moesif.events = []*models.EventModel{}
			}
			moesif.mu.Unlock()
		}
	}()
	return moesif
}

// Publish publishes an event to Moesif.
func (m *Moesif) Publish(event *dto.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	slog.Info("Preparing event to be published to Moesif")
	uri := event.API.APIContext + event.Operation.APIResourceTemplate
	if event.Operation.APIResourceTemplate != "" {
		uri = event.Operation.APIResourceTemplate
	}

	req := models.EventRequestModel{
		Time:       &event.RequestTimestamp,
		Uri:        uri,
		Verb:       event.Operation.APIMethod,
		ApiVersion: &event.API.APIVersion,
		IpAddress:  &event.UserIP,
		Headers: map[string]interface{}{ // TODO (osura): Need to populate them dynamically
			"User-Agent":   event.UserAgentHeader,
			"Content-Type": "application/json",
		},
		Body: nil,
	}
	respTime := event.RequestTimestamp
	if event.Latencies != nil {
		respTime = event.RequestTimestamp.Add(time.Duration(event.Latencies.ResponseLatency) * time.Millisecond)
	}

	rspHeaders := map[string]string{ //TODO (osura): Need to populate them dynamically
		"Vary":          "Accept-Encoding",
		"Pragma":        "no-cache",
		"Expires":       "-1",
		"Content-Type":  "application/json; charset=utf-8",
		"Cache-Control": "no-cache",
	}

	rsp := models.EventResponseModel{
		Time:    &respTime,
		Status:  event.ProxyResponseCode,
		Headers: rspHeaders,
	}

	// Medatadata Map for the event
	metadataMap := make(map[string]interface{})
	// API Metadata
	metadataMap["correlationId"] = event.MetaInfo.CorrelationID
	metadataMap["apiContext"] = event.API.APIContext
	metadataMap["apiName"] = event.API.APIName
	metadataMap["apiVersion"] = event.API.APIVersion
	metadataMap["apiType"] = event.API.APIType

	// AI Metadata
	if event.API.APIType == "LlmProvider" {
		// Safely extract aiMetadata with nil check
		if aiMetadataVal, exists := event.Properties["aiMetadata"]; exists && aiMetadataVal != nil {
			if aiMetadata, ok := aiMetadataVal.(dto.AIMetadata); ok {
				slog.Debug("aiMetadata from publisher", "aiMetadata", aiMetadata)
				//[Required Format] key:aiMetadata -> dto.AIMetadata object
				metadataMap["aiMetadata"] = aiMetadata
			} else {
				slog.Warn("AI Metadata property cannot be converted to the required format")
			}
		} else {
			slog.Warn("AI Metadata property cannot be found in the event properties")
		}

		// Safely extract aiTokenUsage with nil check
		if aiTokenUsageVal, exists := event.Properties["aiTokenUsage"]; exists && aiTokenUsageVal != nil {
			if aiTokenUsage, ok := aiTokenUsageVal.(dto.AITokenUsage); ok {
				slog.Debug("tokenUsage from publisher", "tokenUsage", aiTokenUsage)
				//[Required Format] key:aiTokenUsage ->  dto.AITokenUsage object
				metadataMap["aiTokenUsage"] = aiTokenUsage
			} else {
				slog.Warn("Token usage property cannot be converted to the required format")
			}
		} else {
			slog.Warn("AI Token Usage data cannot be found in the event properties")
		}
	}

	userID := anonymous
	eventModel := &models.EventModel{
		Request:  req,
		Response: rsp,
		UserId:   &userID,
		Metadata: metadataMap,
	}
	m.events = append(m.events, eventModel)
	slog.Debug(fmt.Sprintf("Event added to the queue. Queue size: %d", len(m.events)))
	slog.Debug("Events", "events", m.events)
}
