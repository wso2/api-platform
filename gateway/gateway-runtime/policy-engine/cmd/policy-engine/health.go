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
	"context"
	"log/slog"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/admin"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/health"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/xdsclient"
)

// errPolicyConfigNotLoaded is the policy_engine check's reason while the policy engine
// has not loaded its policy configuration.
const errPolicyConfigNotLoaded = health.Reason("policy configuration not loaded")

// buildHealthCheckers builds the checkers behind the gateway's health routes. Both report
// whether the policy engine has loaded its policy configuration, as its admin health
// endpoint does (provider). Readiness adds the dependencies a runtime needs to serve
// traffic correctly: the configuration link to the controller (when configuration comes
// over xDS; xdsClient is nil otherwise), tolerated down for the configured grace window,
// and the Python executor (when one is configured; pythonExecutor is nil otherwise).
func buildHealthCheckers(cfg *config.Config, logger *slog.Logger, provider admin.HealthProvider,
	xdsClient *xdsclient.Client, pythonExecutor health.CheckFunc) (liveness, readiness *health.Checker) {
	hc := cfg.PolicyEngine.Health
	options := func(healthyStatus string) health.Options {
		return health.Options{
			HealthyStatus: healthyStatus,
			CacheTTL:      hc.CacheTTL,
			Logger:        logger,
			DebugLogger:   logger,
		}
	}
	policyEngine := policyEngineCheck(provider)
	liveness = health.NewChecker(options("healthy"))
	liveness.Register("policy_engine", policyEngine)
	readiness = health.NewChecker(options("ready"))
	// Liveness already logs the policy engine's state changes; readiness reports the same
	// check without logging it a second time.
	readiness.RegisterUnlogged("policy_engine", policyEngine)
	if xdsClient != nil {
		readiness.Register("xds_connection", xdsClient.ConnectionCheck(hc.XDSConnection.Grace))
	}
	if pythonExecutor != nil {
		readiness.Register("python_executor", pythonExecutor,
			health.WithTimeout(hc.PythonExecutor.CheckTimeout))
	}
	return liveness, readiness
}

// policyEngineCheck reports the policy engine down until it has loaded its policy
// configuration.
func policyEngineCheck(provider admin.HealthProvider) health.CheckFunc {
	return func(context.Context) error {
		if provider.IsHealthy() {
			return nil
		}
		return errPolicyConfigNotLoaded
	}
}
