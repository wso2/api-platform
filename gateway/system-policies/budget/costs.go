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

package budget

import (
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/wso2/api-platform/sdk/core/counters"
)

// AI cost stats (Interfaces 1.3, 4.2): what every LLM request cost, with or without a
// budget, per month and per project, agent, API key, user, provider and model. Counted
// on the route the request enters, like budget spend, when the route asks for it
// (paramCostStats; [ai_cost_stats] enabled on the gateway).

// Counter namespaces of the cost stats: USD, and requests charged $0 by reason.
const (
	namespaceCost                 = "ai-cost"
	namespaceCostMissingPrice     = "ai-cost-missing-price"
	namespaceCostUsageUnavailable = "ai-cost-usage-unavailable"
	metadataCostCaller            = "x-wso2-cost-caller"
	metadataUsage                 = "llm:usage:v1"
	costLabelProject              = "project"
	costLabelAgent                = "agent"
	costLabelAPIKey               = "apiKey"
	costLabelUser                 = "user"
	costLabelLLMProvider          = "llmProvider"
	costLabelModel                = "model"
	usageModelField               = "Model"
)

// costNamespace keys the cost counters: a UUIDv5 of the values, so every gateway counts
// the same combination in the same counter.
var costNamespace = uuid.MustParse("4b8a2f61-9c3e-5d7a-b1f4-0e6c8a2d9f53")

// costCaller is who sent a request and to which provider, taken when it is admitted.
type costCaller struct {
	project, agent, apiKey, user, provider string
}

// costLabels are a request's values, labelling its cost counters for the controller's
// report; empty values are left out.
func (c costCaller) costLabels(model string) map[string]string {
	labels := map[string]string{costLabelLLMProvider: c.provider}
	for k, v := range map[string]string{
		costLabelProject: c.project, costLabelAgent: c.agent, costLabelAPIKey: c.apiKey,
		costLabelUser: c.user, costLabelModel: model,
	} {
		if v != "" {
			labels[k] = v
		}
	}
	return labels
}

// costID names the counter of one combination of values.
func costID(labels map[string]string) string {
	parts := make([]string, 0, len(labels))
	for _, k := range []string{costLabelAgent, costLabelAPIKey, costLabelLLMProvider, costLabelModel, costLabelProject, costLabelUser} {
		parts = append(parts, k+"="+url.QueryEscape(labels[k]))
	}
	return uuid.NewSHA1(costNamespace, []byte(strings.Join(parts, "|"))).String()
}

// monthStart is the first of the UTC month that holds now.
func monthStart(now time.Time) time.Time {
	now = now.UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// usageModel returns the model the response's token usage names (sdk/ai/llmusage's
// Usage, published by llm-cost's usage extraction), or "" when there is none. Read by
// field name so the policy does not depend on a particular sdk/ai version.
func usageModel(usage any) string {
	v := reflect.ValueOf(usage)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return ""
	}
	f := v.FieldByName(usageModelField)
	if !f.IsValid() || f.Kind() != reflect.String {
		return ""
	}
	return f.String()
}

// recordCost counts one request's cost for its month: its USD when priced, else one
// request under the reason it was not (status is costStatusMissingPrice or
// costStatusUsageUnavailable).
func (t *tracker) recordCost(caller costCaller, model string, month time.Time, cost float64, status string) {
	labels := caller.costLabels(model)
	id := costID(labels)
	namespace, delta := namespaceCost, cost
	switch status {
	case costStatusMissingPrice:
		namespace, delta = namespaceCostMissingPrice, 1
	case costStatusUsageUnavailable:
		namespace, delta = namespaceCostUsageUnavailable, 1
	}
	key := counters.Key{Namespace: namespace, ID: id, Window: month}
	t.counters.Describe(key, labels)
	t.counters.Add(key, delta)
}
