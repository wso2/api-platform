/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"strings"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
	"go.uber.org/zap"
)

// processAPIEvents handles API events based on action type
func (el *EventListener) processAPIEvents(event eventhub.Event) {
	log := el.logger.With(
		zap.String("api_id", event.EntityID),
		zap.String("action", event.Action),
	)

	apiID := event.EntityID

	switch event.Action {
	case "CREATE", "UPDATE":
		el.handleAPICreateOrUpdate(apiID, log)
	case "DELETE":
		el.handleAPIDelete(apiID, log)
	default:
		log.Warn("Unknown action type")
	}
}

// handleAPICreateOrUpdate fetches the API from DB and updates XDS
func (el *EventListener) handleAPICreateOrUpdate(apiID string, log *zap.Logger) {
	// 1. Fetch API configuration from database
	config, err := el.db.GetConfig(apiID)
	if err != nil {
		log.Error("Failed to fetch API config from database", zap.Error(err))
		return
	}

	// 2. Update in-memory store (add or update)
	_, err = el.store.Get(apiID)
	if err != nil {
		// Config doesn't exist in memory - add it
		if err := el.store.Add(config); err != nil {
			log.Error("Failed to add config to store", zap.Error(err))
			return
		}
		log.Info("Added API config to in-memory store")
	} else {
		// Config exists - update it
		if err := el.store.Update(config); err != nil {
			log.Error("Failed to update config in store", zap.Error(err))
			return
		}
		log.Info("Updated API config in in-memory store")
	}

	var storedPolicy *models.StoredPolicyConfig

	if el.policyManager != nil {
		storedPolicy = el.buildStoredPolicyFromAPI(config)
	}

	// 3. Trigger async XDS snapshot update
	correlationID := fmt.Sprintf("event-%s-%d", apiID, time.Now().UnixNano())
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := el.snapshotManager.UpdateSnapshot(ctx, correlationID); err != nil {
			log.Error("Failed to update xDS snapshot", zap.Error(err))
		} else {
			log.Info("xDS snapshot updated successfully")
		}
	}()

	// 4. Update PolicyManager (if configured)
	if storedPolicy != nil {
		if err := el.policyManager.AddPolicy(storedPolicy); err != nil {
			log.Warn("Failed to add policy to PolicyManager", zap.Error(err))
		} else {
			log.Info("Added policy to PolicyManager")
		}
	}
}

// handleAPIDelete removes the API from in-memory store and updates XDS
func (el *EventListener) handleAPIDelete(apiID string, log *zap.Logger) {
	// 1. Check if config exists in store (for logging/policy removal)
	config, err := el.store.Get(apiID)
	if err != nil {
		log.Warn("Config not found in store, may already be deleted")
		// Continue anyway to ensure cleanup
	}

	// 2. Remove from in-memory store
	if err := el.store.Delete(apiID); err != nil {
		log.Warn("Failed to delete config from store", zap.Error(err))
		// Continue - config may not exist
	} else {
		log.Info("Removed API config from in-memory store")
	}

	// 3. Trigger async XDS snapshot update
	correlationID := fmt.Sprintf("event-delete-%s-%d", apiID, time.Now().UnixNano())
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := el.snapshotManager.UpdateSnapshot(ctx, correlationID); err != nil {
			log.Error("Failed to update xDS snapshot after delete", zap.Error(err))
		} else {
			log.Info("xDS snapshot updated after delete")
		}
	}()

	// 4. Remove from PolicyManager (if configured)
	if el.policyManager != nil && config != nil {
		policyID := apiID + "-policies"
		if err := el.policyManager.RemovePolicy(policyID); err != nil {
			log.Warn("Failed to remove policy from PolicyManager", zap.Error(err))
		} else {
			log.Info("Removed policy from PolicyManager")
		}
	}
}

// buildStoredPolicyFromAPI constructs a StoredPolicyConfig from an API config
// Merging rules: When operation has policies, they define the order (can reorder, override, or extend API policies).
// Remaining API-level policies not mentioned in operation policies are appended at the end.
// When operation has no policies, API-level policies are used in their declared order.
// RouteKey uses the fully qualified route path (context + operation path) and must match the route name format
// used by the xDS translator for consistency.
func (el *EventListener) buildStoredPolicyFromAPI(cfg *models.StoredConfig) *models.StoredPolicyConfig {
	apiCfg := &cfg.Configuration

	// Collect API-level policies
	apiPolicies := make(map[string]policyenginev1.PolicyInstance) // name -> policy
	if cfg.GetPolicies() != nil {
		for _, p := range *cfg.GetPolicies() {
			apiPolicies[p.Name] = convertAPIPolicy(p)
		}
	}

	routes := make([]policyenginev1.PolicyChain, 0)
	switch apiCfg.Kind {
	case api.Asyncwebsub:
		// Build routes with merged policies for WebSub
		apiData, err := apiCfg.Spec.AsWebhookAPIData()
		if err != nil {
			el.logger.Error("Failed to parse WebSub API data", zap.Error(err))
			return nil
		}
		for _, ch := range apiData.Channels {
			var finalPolicies []policyenginev1.PolicyInstance

			if ch.Policies != nil && len(*ch.Policies) > 0 {
				// Operation has policies: use operation policy order as authoritative
				finalPolicies = make([]policyenginev1.PolicyInstance, 0, len(*ch.Policies))
				addedNames := make(map[string]struct{})

				for _, opPolicy := range *ch.Policies {
					finalPolicies = append(finalPolicies, convertAPIPolicy(opPolicy))
					addedNames[opPolicy.Name] = struct{}{}
				}

				// Add any API-level policies not mentioned in operation policies (append at end)
				if apiData.Policies != nil {
					for _, apiPolicy := range *apiData.Policies {
						if _, exists := addedNames[apiPolicy.Name]; !exists {
							finalPolicies = append(finalPolicies, apiPolicies[apiPolicy.Name])
						}
					}
				}
			} else {
				// No operation policies: use API-level policies in their declared order
				if apiData.Policies != nil {
					finalPolicies = make([]policyenginev1.PolicyInstance, 0, len(*apiData.Policies))
					for _, p := range *apiData.Policies {
						finalPolicies = append(finalPolicies, apiPolicies[p.Name])
					}
				}
			}

			routeKey := xds.GenerateRouteName("POST", apiData.Context, apiData.Version, ch.Path, el.routerConfig.GatewayHost)
			routes = append(routes, policyenginev1.PolicyChain{
				RouteKey: routeKey,
				Policies: finalPolicies,
			})
		}
	case api.RestApi:
		// Build routes with merged policies for REST API
		apiData, err := apiCfg.Spec.AsAPIConfigData()
		if err != nil {
			el.logger.Error("Failed to parse REST API data", zap.Error(err))
			return nil
		}
		for _, op := range apiData.Operations {
			var finalPolicies []policyenginev1.PolicyInstance

			if op.Policies != nil && len(*op.Policies) > 0 {
				// Operation has policies: use operation policy order as authoritative
				finalPolicies = make([]policyenginev1.PolicyInstance, 0, len(*op.Policies))
				addedNames := make(map[string]struct{})

				for _, opPolicy := range *op.Policies {
					finalPolicies = append(finalPolicies, convertAPIPolicy(opPolicy))
					addedNames[opPolicy.Name] = struct{}{}
				}

				// Add any API-level policies not mentioned in operation policies (append at end)
				if apiData.Policies != nil {
					for _, apiPolicy := range *apiData.Policies {
						if _, exists := addedNames[apiPolicy.Name]; !exists {
							finalPolicies = append(finalPolicies, apiPolicies[apiPolicy.Name])
						}
					}
				}
			} else {
				// No operation policies: use API-level policies in their declared order
				if apiData.Policies != nil {
					finalPolicies = make([]policyenginev1.PolicyInstance, 0, len(*apiData.Policies))
					for _, p := range *apiData.Policies {
						finalPolicies = append(finalPolicies, apiPolicies[p.Name])
					}
				}
			}

			// Determine effective vhosts (fallback to global router defaults when not provided)
			effectiveMainVHost := el.routerConfig.VHosts.Main.Default
			effectiveSandboxVHost := el.routerConfig.VHosts.Sandbox.Default
			if apiData.Vhosts != nil {
				if strings.TrimSpace(apiData.Vhosts.Main) != "" {
					effectiveMainVHost = apiData.Vhosts.Main
				}
				if apiData.Vhosts.Sandbox != nil && strings.TrimSpace(*apiData.Vhosts.Sandbox) != "" {
					effectiveSandboxVHost = *apiData.Vhosts.Sandbox
				}
			}

			vhosts := []string{effectiveMainVHost}
			if apiData.Upstream.Sandbox != nil && apiData.Upstream.Sandbox.Url != nil &&
				strings.TrimSpace(*apiData.Upstream.Sandbox.Url) != "" {
				vhosts = append(vhosts, effectiveSandboxVHost)
			}

			for _, vhost := range vhosts {
				routes = append(routes, policyenginev1.PolicyChain{
					RouteKey: xds.GenerateRouteName(string(op.Method), apiData.Context, apiData.Version, op.Path, vhost),
					Policies: finalPolicies,
				})
			}
		}
	}

	// If there are no policies at all, return nil (skip creation)
	policyCount := 0
	for _, r := range routes {
		policyCount += len(r.Policies)
	}
	if policyCount == 0 {
		return nil
	}

	now := time.Now().Unix()
	stored := &models.StoredPolicyConfig{
		ID: cfg.ID + "-policies",
		Configuration: policyenginev1.Configuration{
			Routes: routes,
			Metadata: policyenginev1.Metadata{
				CreatedAt:       now,
				UpdatedAt:       now,
				ResourceVersion: 0,
				APIName:         cfg.GetDisplayName(),
				Version:         cfg.GetVersion(),
				Context:         cfg.GetContext(),
			},
		},
		Version: 0,
	}
	return stored
}

// convertAPIPolicy converts generated api.Policy to policyenginev1.PolicyInstance
func convertAPIPolicy(p api.Policy) policyenginev1.PolicyInstance {
	paramsMap := make(map[string]interface{})
	if p.Params != nil {
		for k, v := range *p.Params {
			paramsMap[k] = v
		}
	}
	return policyenginev1.PolicyInstance{
		Name:               p.Name,
		Version:            p.Version,
		Enabled:            true, // Default to enabled
		ExecutionCondition: p.ExecutionCondition,
		Parameters:         paramsMap,
	}
}
