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

package controller

import "github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"

// operatorValuesFilePath is the operator's default gateway values file: the layer between the
// chart defaults and a gateway's own values ConfigMap. The controller credential lookup reads
// it so the operator authenticates with the user the controller is actually rendered with.
func operatorValuesFilePath(cfg *config.OperatorConfig) string {
	if cfg == nil {
		return ""
	}
	return cfg.Gateway.HelmValuesFilePath
}
