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

package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/configcompat"
)

func destinationGuardTable(name string) string {
	return fmt.Sprintf(`
[platform_api.http_client.%s]
enabled = true
preset = "public_only"
max_redirects = 2
`, name)
}

func TestLoadConfig_DestinationGuard(t *testing.T) {
	cfg, err := loadWithKeys(t, destinationGuardTable(configcompat.DestinationGuardKey))
	require.NoError(t, err)
	assert.Equal(t, "public_only", cfg.HTTPClient.DestinationGuard.Preset)
	assert.Equal(t, 2, cfg.HTTPClient.DestinationGuard.MaxRedirects)
}

// A config written before the rename keeps loading, under the new name.
func TestLoadConfig_DestinationGuardUnderItsPreviousName(t *testing.T) {
	cfg, err := loadWithKeys(t, destinationGuardTable(configcompat.LegacyDestinationGuardKey))
	require.NoError(t, err)
	assert.True(t, cfg.HTTPClient.DestinationGuard.Enabled)
	assert.Equal(t, "public_only", cfg.HTTPClient.DestinationGuard.Preset)
}

func TestLoadConfig_DestinationGuardUnderBothNamesFails(t *testing.T) {
	_, err := loadWithKeys(t, destinationGuardTable(configcompat.LegacyDestinationGuardKey)+destinationGuardTable(configcompat.DestinationGuardKey))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "both set")
}
