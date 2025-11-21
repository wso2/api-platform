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

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/logger"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	log, err := logger.NewLogger(logger.Config{
		Level:  "info",
		Format: "console",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("Starting Policy xDS Server Example")

	// Create in-memory policy store
	policyStore := storage.NewPolicyStore()

	// Create snapshot manager
	snapshotManager := policyxds.NewSnapshotManager(policyStore, log)

	// Create policy manager
	policyManager := policyxds.NewPolicyManager(policyStore, snapshotManager, log)

	// Add some sample policies
	if err := addSamplePolicies(policyManager, log); err != nil {
		log.Error("Failed to add sample policies", zap.Error(err))
		os.Exit(1)
	}

	// Create and start xDS server
	xdsPort := 18000
	server := policyxds.NewServer(snapshotManager, xdsPort, log)

	// Start server in a goroutine
	go func() {
		log.Info("Policy xDS server starting", zap.Int("port", xdsPort))
		if err := server.Start(); err != nil {
			log.Error("Failed to start xDS server", zap.Error(err))
			os.Exit(1)
		}
	}()

	// Demonstrate policy operations: add, delete, update
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		step := 1

		for range ticker.C {
			log.Info("========================================")
			log.Info("Policy operation step", zap.Int("step", step))

			switch step {
			case 1:
				// Step 1: Add one dynamic policy
				policyID := "dynamic-policy-1"
				policy := createDynamicPolicy(policyID, 1)
				log.Info("➕ Step 1: Adding dynamic policy", zap.String("id", policyID))
				if err := policyManager.AddPolicy(policy); err != nil {
					log.Error("Failed to add dynamic policy", zap.Error(err))
				} else {
					log.Info("✅ Dynamic policy added",
						zap.Int("total_policies", len(policyManager.ListPolicies())))
				}

			case 2:
				// Step 2: Delete the user-api-v1
				policyID := "user-api-v1"
				log.Info("🗑️  Step 2: Deleting user API policy", zap.String("id", policyID))
				if err := policyManager.RemovePolicy(policyID); err != nil {
					log.Error("Failed to delete policy", zap.Error(err))
				} else {
					log.Info("✅ User API policy deleted",
						zap.Int("total_policies", len(policyManager.ListPolicies())))
				}

			case 3:
				// Step 3: Update petstore-api-v1 execution condition
				policyID := "petstore-api-v1"
				log.Info("🔄 Step 3: Updating Petstore API execution condition", zap.String("id", policyID))

				// Get the existing policy
				existingPolicy, err := policyManager.GetPolicy(policyID)
				if err != nil {
					log.Error("Failed to get policy for update", zap.Error(err))
				} else {
					// Update execution conditions on all policies
					newCondition := "request.method == 'GET' && user.authenticated == true"
					for i := range existingPolicy.Configuration.Routes {
						for j := range existingPolicy.Configuration.Routes[i].Policies {
							existingPolicy.Configuration.Routes[i].Policies[j].ExecutionCondition = &newCondition
						}
					}
					existingPolicy.Configuration.Metadata.UpdatedAt = time.Now().Unix()

					if err := policyManager.AddPolicy(existingPolicy); err != nil {
						log.Error("Failed to update policy", zap.Error(err))
					} else {
						log.Info("✅ Petstore API policy updated with new execution condition",
							zap.String("condition", newCondition),
							zap.Int("total_policies", len(policyManager.ListPolicies())))
					}
				}

			default:
				// Step 4+: Do nothing
				log.Info("⏸️  Step 4+: No operations (idle)", zap.Int("total_policies", len(policyManager.ListPolicies())))
			}

			step++
			log.Info("========================================")
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down Policy xDS server...")
	server.Stop()
	log.Info("Server stopped")
}

func addSamplePolicies(pm *policyxds.PolicyManager, log *zap.Logger) error {
	// Sample Policy 1: Petstore API
	policy1 := &models.StoredPolicyConfig{
		ID: "petstore-api-v1",
		Configuration: models.PolicyConfiguration{
			Routes: []models.RoutePolicy{
				{
					RouteKey: "/pets",
					Policies: []models.Policy{
						{
							Name:    "RateLimitPolicy",
							Version: "v1",
							Params: map[string]interface{}{
								"requestsPerMinute": 100,
								"burstSize":         10,
							},
						},
						{
							Name:    "AuthenticationPolicy",
							Version: "v1",
							Params: map[string]interface{}{
								"type":   "oauth2",
								"scopes": []string{"read:pets"},
							},
						},
						{
							Name:    "CachePolicy",
							Version: "v1",
							Params: map[string]interface{}{
								"ttl":      300,
								"cacheKey": "default",
							},
						},
					},
				},
				{
					RouteKey: "/pets/{petId}",
					Policies: []models.Policy{
						{
							Name:    "ValidationPolicy",
							Version: "v1",
							Params: map[string]interface{}{
								"validatePathParams": true,
								"requiredParams":     []string{"petId"},
							},
						},
					},
				},
			},
			Metadata: models.Metadata{
				CreatedAt:       time.Now().Unix(),
				UpdatedAt:       time.Now().Unix(),
				ResourceVersion: 1,
				APIName:         "Petstore",
				Version:         "v1",
				Context:         "/petstore/v1",
			},
		},
		Version: 1,
	}

	if err := pm.AddPolicy(policy1); err != nil {
		return fmt.Errorf("failed to add policy1: %w", err)
	}
	log.Info("Added sample policy", zap.String("id", policy1.ID))

	// Sample Policy 2: User API
	policy2 := &models.StoredPolicyConfig{
		ID: "user-api-v1",
		Configuration: models.PolicyConfiguration{
			Routes: []models.RoutePolicy{
				{
					RouteKey: "/users",
					Policies: []models.Policy{
						{
							Name:    "AuthenticationPolicy",
							Version: "v1",
							Params: map[string]interface{}{
								"type":   "jwt",
								"issuer": "example.com",
							},
						},
						{
							Name:    "LoggingPolicy",
							Version: "v1",
							Params: map[string]interface{}{
								"level":      "INFO",
								"logHeaders": true,
							},
						},
						{
							Name:    "TransformPolicy",
							Version: "v1",
							Params: map[string]interface{}{
								"removeFields": []string{"internal_id"},
							},
						},
					},
				},
			},
			Metadata: models.Metadata{
				CreatedAt:       time.Now().Unix(),
				UpdatedAt:       time.Now().Unix(),
				ResourceVersion: 1,
				APIName:         "UserAPI",
				Version:         "v1",
				Context:         "/users/v1",
			},
		},
		Version: 1,
	}

	if err := pm.AddPolicy(policy2); err != nil {
		return fmt.Errorf("failed to add policy2: %w", err)
	}
	log.Info("Added sample policy", zap.String("id", policy2.ID))

	return nil
}

func createDynamicPolicy(id string, counter int) *models.StoredPolicyConfig {
	condition := "something == true"
	return &models.StoredPolicyConfig{
		ID: id,
		Configuration: models.PolicyConfiguration{
			Routes: []models.RoutePolicy{
				{
					RouteKey: fmt.Sprintf("/dynamic/%d", counter),
					Policies: []models.Policy{
						{
							Name:               "DynamicPolicy",
							Version:            "v1",
							ExecutionCondition: &condition,
							Params: map[string]interface{}{
								"counter":   counter,
								"timestamp": time.Now().Unix(),
							},
						},
					},
				},
			},
			Metadata: models.Metadata{
				CreatedAt:       time.Now().Unix(),
				UpdatedAt:       time.Now().Unix(),
				ResourceVersion: int64(counter),
				APIName:         fmt.Sprintf("DynamicAPI-%d", counter),
				Version:         "v1",
				Context:         fmt.Sprintf("/dynamic/v1/%d", counter),
			},
		},
		Version: int64(counter),
	}
}
