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

package models

import (
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// TakesSubscriptions reports whether artifacts of the kind can be subscribed to.
func TakesSubscriptions(kind string) bool {
	return kind == KindRestApi || kind == KindMcp
}

// SubscriptionPlanNames returns the plans the artifact offers; nil means any active plan.
func SubscriptionPlanNames(cfg *StoredConfig) []string {
	if cfg == nil {
		return nil
	}
	var plans *[]string
	switch cfg.Kind {
	case KindRestApi:
		switch c := cfg.Configuration.(type) {
		case api.RestAPI:
			plans = c.Spec.SubscriptionPlans
		case *api.RestAPI:
			if c != nil {
				plans = c.Spec.SubscriptionPlans
			}
		}
	case KindMcp:
		// Read from the database, an MCP proxy carries only its source configuration.
		switch c := cfg.SourceConfiguration.(type) {
		case api.MCPProxyConfiguration:
			plans = c.Spec.SubscriptionPlans
		case *api.MCPProxyConfiguration:
			if c != nil {
				plans = c.Spec.SubscriptionPlans
			}
		}
	}
	if plans == nil {
		return nil
	}
	return *plans
}
