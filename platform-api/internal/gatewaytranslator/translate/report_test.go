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

package translate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReport_NilReceiverIsANoOp(t *testing.T) {
	var r *Report
	r.Warn("Mcp", "spec.x", "ignored %d", 1)
	assert.True(t, r.Empty())
	assert.Nil(t, r.Warnings())
}

func TestReport_RecordsInOrderAndFormats(t *testing.T) {
	var r Report
	assert.True(t, r.Empty())
	assert.Nil(t, r.Warnings(), "an empty report has no warnings, not an empty slice")

	r.Warn("Mcp", "spec.upstream.url", "no %s suffix", "/mcp")
	r.Warn("LlmProxy", "spec.additionalProviders", "dropped %q", "p2")

	ws := r.Warnings()
	require.Len(t, ws, 2)
	assert.Equal(t, Warning{Kind: "Mcp", Field: "spec.upstream.url", Msg: "no /mcp suffix"}, ws[0])
	assert.Equal(t, "LlmProxy spec.additionalProviders: dropped \"p2\"", ws[1].String())
	assert.False(t, r.Empty())
}

func TestReport_WarningsIsACopy(t *testing.T) {
	var r Report
	r.Warn("Mcp", "f", "m")
	ws := r.Warnings()
	ws[0].Msg = "mutated"
	assert.Equal(t, "m", r.Warnings()[0].Msg)
}
