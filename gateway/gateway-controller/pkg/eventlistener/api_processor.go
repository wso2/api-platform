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

package eventlistener

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	policybuilder "github.com/wso2/api-platform/gateway/gateway-controller/pkg/policy"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// processAPIEvent dispatches API events by action
func (l *EventListener) processAPIEvent(event eventhub.Event) {
	switch event.Action {
	case "CREATE", "UPDATE":
		l.handleAPICreateOrUpdate(event)
	case "DELETE":
		l.handleAPIDelete(event)
	default:
		l.logger.Warn("Unknown API event action",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID))
	}
}

func (l *EventListener) updateSnapshotAsync(entityID, correlationID, failureMessage string) {
	if l.snapshotManager == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := l.snapshotManager.UpdateSnapshot(ctx, correlationID); err != nil {
			l.logger.Error(failureMessage,
				slog.String("entity_id", entityID),
				slog.Any("error", err))
		}
	}()
}

// handleAPICreateOrUpdate handles API create or update events
func (l *EventListener) handleAPICreateOrUpdate(event eventhub.Event) {
	entityID := event.EntityID

	l.logger.Info("Processing API create/update event",
		slog.String("api_id", entityID),
		slog.String("action", event.Action),
		slog.String("event_id", event.EventID))

	// Fetch the latest config from the database (it was already persisted by the publishing replica)
	if l.db == nil {
		l.logger.Warn("Database not available, cannot process API event",
			slog.String("api_id", entityID))
		return
	}

	storedConfig, err := l.db.GetConfig(entityID)
	if err != nil {
		l.logger.Error("Failed to fetch API configuration from database",
			slog.String("api_id", entityID),
			slog.Any("error", err))
		return
	}

	// Resolve policy configuration (handles secret resolution)
	resolvedCfg, err := l.resolvePolicyConfiguration(storedConfig)
	if err != nil {
		l.logger.Error("Failed to resolve policy configuration for API",
			slog.String("api_id", entityID),
			slog.String("event_id", event.EventID),
			slog.Any("error", err))
		return
	}

	// Update in-memory store
	existing, _ := l.store.Get(entityID)
	if existing != nil {
		// Update existing config
		if err := l.store.Update(storedConfig); err != nil {
			l.logger.Error("Failed to update API configuration in memory store",
				slog.String("api_id", entityID),
				slog.Any("error", err))
			return
		}
	} else {
		// Add new config
		if err := l.store.Add(storedConfig); err != nil {
			l.logger.Error("Failed to add API configuration to memory store",
				slog.String("api_id", entityID),
				slog.Any("error", err))
			return
		}
	}

	// Update xDS snapshot
	l.updateSnapshotAsync(entityID, event.EventID, "Failed to update xDS snapshot after replica sync")

	// Update policies
	l.updatePoliciesForAPI(resolvedCfg, event.EventID)

	l.logger.Info("Successfully processed API create/update event",
		slog.String("api_id", entityID),
		slog.String("event_id", event.EventID))
}

// handleAPIDelete handles API delete events
func (l *EventListener) handleAPIDelete(event eventhub.Event) {
	entityID := event.EntityID

	l.logger.Info("Processing API delete event",
		slog.String("api_id", entityID),
		slog.String("event_id", event.EventID))

	existingConfig, err := l.store.Get(entityID)
	if err != nil && !storage.IsNotFoundError(err) {
		l.logger.Error("Failed to load API from memory store before deletion",
			slog.String("api_id", entityID),
			slog.Any("error", err))
		return
	}

	// Remove from in-memory store
	if err := l.store.Delete(entityID); err != nil {
		if !storage.IsNotFoundError(err) {
			l.logger.Error("Failed to delete API from memory store",
				slog.String("api_id", entityID),
				slog.Any("error", err))
		}
	}

	if err := l.store.RemoveAPIKeysByAPI(entityID); err != nil && !storage.IsNotFoundError(err) {
		l.logger.Warn("Failed to remove API keys from memory store after API deletion",
			slog.String("api_id", entityID),
			slog.Any("error", err))
	}

	if existingConfig != nil && l.apiKeyXDSManager != nil {
		apiName, apiVersion := extractAPINameVersion(existingConfig)
		if apiName != "" {
			if err := l.apiKeyXDSManager.RemoveAPIKeysByAPI(entityID, apiName, apiVersion, event.EventID); err != nil {
				l.logger.Warn("Failed to remove API keys from policy engine after API deletion",
					slog.String("api_id", entityID),
					slog.String("api_name", apiName),
					slog.String("api_version", apiVersion),
					slog.String("event_id", event.EventID),
					slog.Any("error", err))
			}
		}
	}

	// Update xDS snapshot
	l.updateSnapshotAsync(entityID, event.EventID, "Failed to update xDS snapshot after API deletion")

	// Remove policies
	if l.policyManager != nil {
		policyID := entityID + "-policies"
		if err := l.policyManager.RemovePolicy(policyID); err != nil {
			if !storage.IsPolicyNotFoundError(err) {
				l.logger.Warn("Failed to remove policy after API deletion",
					slog.String("api_id", entityID),
					slog.Any("error", err))
			}
		}
	}

	l.logger.Info("Successfully processed API delete event",
		slog.String("api_id", entityID),
		slog.String("event_id", event.EventID))
}

// updatePoliciesForAPI derives and updates policy configuration for an API
func (l *EventListener) updatePoliciesForAPI(cfg *models.StoredConfig, correlationID string) {
	if l.policyManager == nil || l.systemConfig == nil {
		return
	}

	// Policies are derived only for artifact kinds that can expose route-level policies.
	if cfg.Kind != string(api.RestApi) && cfg.Kind != string(api.WebSubApi) &&
		cfg.Kind != string(api.LlmProvider) && cfg.Kind != string(api.LlmProxy) &&
		cfg.Kind != string(api.Mcp) {
		return
	}

	storedPolicy := policybuilder.DerivePolicyFromAPIConfig(cfg, l.routerConfig, l.systemConfig, l.policyDefinitions)
	if storedPolicy != nil {
		if err := l.policyManager.AddPolicy(storedPolicy); err != nil {
			l.logger.Error("Failed to update policy from replica sync",
				slog.String("api_id", cfg.UUID),
				slog.String("correlation_id", correlationID),
				slog.Any("error", err))
		}
	} else {
		policyID := cfg.UUID + "-policies"
		if existingPolicy, err := l.policyManager.GetPolicy(policyID); err == nil && existingPolicy != nil {
			if err := l.policyManager.RemovePolicy(policyID); err != nil && !storage.IsPolicyNotFoundError(err) {
				l.logger.Error("Failed to remove policy from replica sync",
					slog.String("api_id", cfg.UUID),
					slog.String("correlation_id", correlationID),
					slog.Any("error", err))
			}
		}
	}
}

// resolvePolicyConfiguration resolves policy templates and secret references in the configuration.
// Returns the resolved configuration or an error if policy resolution fails.
func (l *EventListener) resolvePolicyConfiguration(storedCfg *models.StoredConfig) (*models.StoredConfig, error) {
	resolvedCfg, validationErrors := l.policyResolver.ResolvePolicies(storedCfg)
	if len(validationErrors) > 0 {
		errMsgs := make([]string, 0, len(validationErrors))
		for _, ve := range validationErrors {
			errMsgs = append(errMsgs, ve.Message)
		}
		errMsg := strings.Join(errMsgs, "; ")

		slog.Error("Policy resolution failed",
			slog.String("config_handle", storedCfg.Handle),
			slog.String("errors", errMsg),
		)

		return nil, fmt.Errorf("policy resolution failed with %d errors: %s", len(validationErrors), errMsg)
	}
	return resolvedCfg, nil
}

// extractAPINameVersion extracts the display name and version from a StoredConfig.
// Works for RestApi and WebSubApi kinds by checking the Configuration type.
func extractAPINameVersion(cfg *models.StoredConfig) (string, string) {
	if cfg == nil {
		return "", ""
	}
	// Use denormalized fields on StoredConfig
	return cfg.DisplayName, cfg.Version
}
