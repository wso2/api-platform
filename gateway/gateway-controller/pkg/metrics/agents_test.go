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

package metrics

import (
	"strings"
	"testing"
)

// agents_total is a resource count — how many Agents this controller has deployed —
// and never moves when one of them is called. Agent invocation volume is a business
// metric computed downstream from analytics events, where it can be broken down by
// operation and consumer.
func TestAgentsTotal_IsRegisteredAsAResourceGauge(t *testing.T) {
	once = resetOnce()
	registry = nil
	Enabled = true

	reg := Init()
	if reg == nil {
		t.Fatal("Init() returned nil")
	}

	AgentsTotal.WithLabelValues("deployed").Set(3)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() failed: %v", err)
	}

	for _, family := range families {
		if family.GetName() != "gateway_controller_agents_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			if metric.GetGauge() == nil {
				t.Error("agents_total must be a gauge, not a counter or histogram")
			}
			if got := metric.GetGauge().GetValue(); got != 3 {
				t.Errorf("agents_total = %v, want 3", got)
			}
		}
		return
	}
	t.Error("agents_total was not registered")
}

// The cardinality rule for A2A: only bounded dimensions may be metric labels. A
// caller-supplied contextId, taskId, messageId or consumer id belongs on an analytics
// event and on a span — per-request storage — never on a label, where each distinct
// value creates a permanent time series. This asserts it across every metric the
// controller registers, not just the Agent one, so a later metric cannot reintroduce
// the problem quietly.
func TestNoRegisteredMetricUsesAnUnboundedA2ALabel(t *testing.T) {
	once = resetOnce()
	registry = nil
	Enabled = true

	reg := Init()
	if reg == nil {
		t.Fatal("Init() returned nil")
	}

	// Exercise the Agent gauge so its family is present in the gather output; the
	// label-name scan below is what the assertion is about.
	AgentsTotal.WithLabelValues("deployed").Set(1)

	forbidden := []string{"contextid", "taskid", "messageid", "credentialid", "token", "consumerid"}

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() failed: %v", err)
	}
	for _, family := range families {
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				name := strings.ToLower(strings.ReplaceAll(label.GetName(), "_", ""))
				for _, bad := range forbidden {
					if name == bad {
						t.Errorf("metric %s uses unbounded label %q; caller-supplied identifiers "+
							"belong in analytics events and traces, not metric labels",
							family.GetName(), label.GetName())
					}
				}
			}
		}
	}
}
