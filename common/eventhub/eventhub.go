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

package eventhub

import (
	"database/sql"
	"log/slog"
)

// eventHub is the main EventHub implementation that delegates to a backend
type eventHub struct {
	backend EventhubImpl
	logger  *slog.Logger
	config  Config
}

// New creates a new EventHub backed by SQLite
func New(db *sql.DB, logger *slog.Logger, config Config) EventHub {
	backendConfig := DefaultSQLBackendConfig()
	backendConfig.PollInterval = config.PollInterval
	backendConfig.CleanupInterval = config.CleanupInterval
	backendConfig.RetentionPeriod = config.RetentionPeriod
	backend := NewSQLBackend(db, logger, backendConfig)
	return &eventHub{
		backend: backend,
		logger:  logger,
		config:  config,
	}
}

// NewWithBackend creates a new EventHub with a custom backend
func NewWithBackend(backend EventhubImpl, logger *slog.Logger) EventHub {
	return &eventHub{
		backend: backend,
		logger:  logger,
	}
}

func (h *eventHub) Initialize() error {
	return h.backend.Initialize()
}

func (h *eventHub) RegisterGateway(gatewayID string) error {
	return h.backend.RegisterGateway(gatewayID)
}

func (h *eventHub) PublishEvent(gatewayID string, event Event) error {
	return h.backend.Publish(gatewayID, event)
}

func (h *eventHub) Subscribe(gatewayID string) (<-chan Event, error) {
	return h.backend.Subscribe(gatewayID)
}

func (h *eventHub) Unsubscribe(gatewayID string, subscriber <-chan Event) error {
	return h.backend.Unsubscribe(gatewayID, subscriber)
}

func (h *eventHub) UnsubscribeAll(gatewayID string) error {
	return h.backend.UnsubscribeAll(gatewayID)
}

func (h *eventHub) CleanUpEvents() error {
	return h.backend.Cleanup(h.config.RetentionPeriod)
}

func (h *eventHub) Close() error {
	return h.backend.Close()
}
