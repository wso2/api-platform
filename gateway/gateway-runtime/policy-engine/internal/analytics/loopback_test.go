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

package analytics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
)

func loopbackEvent(apiType string, props map[string]interface{}) *dto.Event {
	return &dto.Event{API: &dto.ExtendedAPI{API: dto.API{APIType: apiType}}, Properties: props}
}

func TestLoopbackCarryover(t *testing.T) {
	budget := map[string]interface{}{"names": []string{"research-monthly"}, "costStatus": "priced"}
	providerHop := func() *dto.Event {
		return loopbackEvent("LlmProvider", map[string]interface{}{
			constants.BudgetPropertyKey: budget,
			"aiMetadata":                dto.AIMetadata{Model: "gpt-4o", LLMCost: 0.00065},
		})
	}

	t.Run("the proxy's event takes the provider hop's budget and cost", func(t *testing.T) {
		l := newLoopbackCarryover()
		l.keep("req-1", providerHop())
		proxy := loopbackEvent("LlmProxy", map[string]interface{}{"aiMetadata": dto.AIMetadata{Model: "gpt-4o"}})

		l.apply("req-1", proxy)

		assert.Equal(t, budget, proxy.Properties[constants.BudgetPropertyKey])
		assert.Equal(t, 0.00065, proxy.Properties["aiMetadata"].(dto.AIMetadata).LLMCost)
	})

	t.Run("the proxy's own values win", func(t *testing.T) {
		l := newLoopbackCarryover()
		l.keep("req-1", providerHop())
		own := map[string]interface{}{"names": []string{"proxy-budget"}}
		proxy := loopbackEvent("LlmProxy", map[string]interface{}{
			constants.BudgetPropertyKey: own,
			"aiMetadata":                dto.AIMetadata{LLMCost: 0.5},
		})

		l.apply("req-1", proxy)

		assert.Equal(t, own, proxy.Properties[constants.BudgetPropertyKey])
		assert.Equal(t, 0.5, proxy.Properties["aiMetadata"].(dto.AIMetadata).LLMCost)
	})

	t.Run("only an LLM proxy event of the same request, once", func(t *testing.T) {
		l := newLoopbackCarryover()
		l.keep("req-1", providerHop())

		other := loopbackEvent("LlmProxy", map[string]interface{}{})
		l.apply("req-2", other)
		assert.NotContains(t, other.Properties, constants.BudgetPropertyKey)

		rest := loopbackEvent("RestApi", map[string]interface{}{})
		l.apply("req-1", rest)
		assert.NotContains(t, rest.Properties, constants.BudgetPropertyKey)

		first := loopbackEvent("LlmProxy", map[string]interface{}{})
		l.apply("req-1", first)
		assert.Contains(t, first.Properties, constants.BudgetPropertyKey)
		second := loopbackEvent("LlmProxy", map[string]interface{}{})
		l.apply("req-1", second)
		assert.NotContains(t, second.Properties, constants.BudgetPropertyKey)
	})

	t.Run("entries expire", func(t *testing.T) {
		l := newLoopbackCarryover()
		now := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
		l.now = func() time.Time { return now }
		l.keep("req-1", providerHop())
		now = now.Add(loopbackCarryTTL + time.Second)

		proxy := loopbackEvent("LlmProxy", map[string]interface{}{})
		l.apply("req-1", proxy)

		assert.NotContains(t, proxy.Properties, constants.BudgetPropertyKey)
	})

	t.Run("a hop with nothing to carry is not stored", func(t *testing.T) {
		l := newLoopbackCarryover()
		l.keep("req-1", loopbackEvent("LlmProvider", map[string]interface{}{}))
		assert.Empty(t, l.entries)
	})
}
