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

package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/configcompat"
)

func loadPEWithDestinationGuard(t *testing.T, names ...string) (*Config, error) {
	t.Helper()
	body := ""
	for _, name := range names {
		body += fmt.Sprintf("\n[policy_engine.http_client.%s]\nenabled = true\npreset = \"public_only\"\n", name)
	}
	return Load(writePEInterpConfig(t, body))
}

func TestLoad_DestinationGuard(t *testing.T) {
	cfg, err := loadPEWithDestinationGuard(t, configcompat.DestinationGuardKey)
	require.NoError(t, err)
	assert.Equal(t, "public_only", cfg.PolicyEngine.HTTPClient.DestinationGuard.Preset)
}

// A config written before the rename keeps loading, under the new name.
func TestLoad_DestinationGuardUnderItsPreviousName(t *testing.T) {
	cfg, err := loadPEWithDestinationGuard(t, configcompat.LegacyDestinationGuardKey)
	require.NoError(t, err)
	assert.True(t, cfg.PolicyEngine.HTTPClient.DestinationGuard.Enabled)
	assert.Equal(t, "public_only", cfg.PolicyEngine.HTTPClient.DestinationGuard.Preset)
}

func TestLoad_DestinationGuardUnderBothNamesFails(t *testing.T) {
	_, err := loadPEWithDestinationGuard(t, configcompat.LegacyDestinationGuardKey, configcompat.DestinationGuardKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "both set")
}
