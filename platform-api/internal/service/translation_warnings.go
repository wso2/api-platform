/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package service

import (
	"log/slog"

	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator"
)

// LogTranslationWarnings writes one Warn line per lossy decision the gateway
// translator took while adapting a deployment artifact for an older gateway.
// The translator itself does no logging; the deploy service that owns the
// deployment id and the gateway calls this right after Translate. It is
// exported so the event-gateway plugin's deploy services can share it.
//
// Messages name fields and versions only, never credentials or upstream URLs.
func LogTranslationWarnings(logger *slog.Logger, report gatewaytranslator.Report, kind, deploymentID, gatewayID, gatewayVersion string) {
	if logger == nil || report.Empty() {
		return
	}
	for _, w := range report.Warnings() {
		logger.Warn("Deployment artifact adapted for older gateway",
			"kind", kind,
			"field", w.Field,
			"detail", w.Msg,
			"deploymentID", deploymentID,
			"gatewayID", gatewayID,
			"gatewayVersion", gatewayVersion,
		)
	}
}
