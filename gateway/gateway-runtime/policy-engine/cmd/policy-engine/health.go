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
	"io"
	"log/slog"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/health"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/xdsclient"
)

// runtimeComponent names the gateway runtime in every health answer.
const runtimeComponent = "gateway-runtime"

// buildHealthCheckers builds the checkers behind the gateway's health routes, and a
// closer for the health log file, if one is configured. Liveness has no checks of its
// own: the policy engine answering at all is what it proves. Readiness adds the
// dependencies a runtime needs to serve traffic correctly: the configuration link to
// the controller (when configuration comes over xDS; xdsClient is nil otherwise),
// tolerated down for the configured grace window.
func buildHealthCheckers(cfg *config.Config, logger *slog.Logger, xdsClient *xdsclient.Client) (liveness, readiness *health.Checker, closer io.Closer) {
	hc := cfg.PolicyEngine.Health
	healthLogger, closer := health.NewLogger(logger,
		func(w io.Writer) slog.Handler { return newLogHandler(cfg, w) },
		health.FileLogConfig{Path: hc.LogFile, MaxSizeMB: hc.LogMaxSizeMB, MaxFiles: hc.LogMaxFiles})
	options := func(healthyStatus string) health.Options {
		return health.Options{
			Component:      runtimeComponent,
			HealthyStatus:  healthyStatus,
			CheckTimeout:   hc.CheckTimeout,
			CacheTTL:       hc.CacheTTL,
			Logger:         healthLogger,
			DebugLogger:    logger,
			BackoffInitial: hc.BackoffInitial,
			BackoffMax:     hc.BackoffMax,
		}
	}
	liveness = health.NewChecker(options("healthy"))
	readiness = health.NewChecker(options("ready"))
	if xdsClient != nil {
		readiness.Register("xds_connection", xdsClient.ConnectionCheck(hc.XDSGrace))
	}
	return liveness, readiness, closer
}
