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
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator"
)

func TestLogTranslationWarnings_OneLinePerWarningWithContext(t *testing.T) {
	// A real translation that is known to be lossy: an LLM proxy with two
	// additional providers going to a 1.1.0 gateway.
	artifact := &dto.LLMProxyDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.LLMProxy}
	artifact.Spec.AdditionalProviders = []dto.LLMProxyDeploymentAdditionalProvider{{ID: "anthropic", As: "claude"}, {ID: "mistral", As: "m"}}
	report, err := gatewaytranslator.Translate(constants.LLMProxy, "1.1", "1.1.0", artifact)
	require.NoError(t, err)
	require.Len(t, report.Warnings(), 2)

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	LogTranslationWarnings(logger, report, constants.LLMProxy, "dep-1", "gw-1", "1.1.0")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 2)
	for _, line := range lines {
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry))
		assert.Equal(t, "WARN", entry["level"])
		assert.Equal(t, "Deployment artifact adapted for older gateway", entry["msg"])
		assert.Equal(t, constants.LLMProxy, entry["kind"])
		assert.Equal(t, "spec.additionalProviders", entry["field"])
		assert.Equal(t, "dep-1", entry["deploymentID"])
		assert.Equal(t, "gw-1", entry["gatewayID"])
		assert.Equal(t, "1.1.0", entry["gatewayVersion"])
		assert.NotEmpty(t, entry["detail"])
	}
	assert.Contains(t, lines[0], "anthropic")
	assert.Contains(t, lines[1], "mistral")
}

func TestLogTranslationWarnings_SilentWhenLossless(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	LogTranslationWarnings(logger, gatewaytranslator.Report{}, constants.RestApi, "dep-1", "gw-1", "1.2.0")
	assert.Empty(t, buf.String())

	// A nil logger is tolerated so a test-constructed service without one cannot panic.
	LogTranslationWarnings(nil, gatewaytranslator.Report{}, constants.RestApi, "dep-1", "gw-1", "1.2.0")
}
