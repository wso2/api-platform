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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TC-35: Missing PLATFORM_SECRET_ENCRYPTION_KEY with APIP_DEMO_MODE=true →
// server starts successfully with an auto-generated ephemeral key.
func TestLoadConfig_MissingSecretEncryptionKey_DemoMode_GeneratesEphemeralKey(t *testing.T) {
	t.Setenv("APIP_DEMO_MODE", "true")
	t.Setenv("PLATFORM_SECRET_ENCRYPTION_KEY", "")
	// Ensure the koanf env-var alias doesn't accidentally provide a value.
	t.Setenv("APIP_DATABASE_SECRET_ENCRYPTION_KEY", "")

	cfg, err := LoadConfig("")
	require.NoError(t, err, "LoadConfig must succeed in DEMO_MODE even without a secret encryption key")
	assert.NotEmpty(t, cfg.Database.SecretEncryptionKey,
		"an ephemeral key must be generated when PLATFORM_SECRET_ENCRYPTION_KEY is absent in DEMO_MODE")
}

// TC-35 (negative): Missing key WITHOUT demo mode → fatal error returned.
func TestLoadConfig_MissingSecretEncryptionKey_NonDemoMode_ReturnsError(t *testing.T) {
	t.Setenv("APIP_DEMO_MODE", "false")
	t.Setenv("PLATFORM_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("APIP_DATABASE_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_ENCRYPTION_KEY", "")
	t.Setenv("AUTH_JWT_SECRET_KEY", "")

	_, err := LoadConfig("")
	assert.Error(t, err, "LoadConfig must return an error when no encryption key is configured and DEMO_MODE is off")
	assert.Contains(t, err.Error(), "no encryption key configured for secrets management")
}

// TC-35: Ephemeral key must be unique each LoadConfig call (i.e. truly random, not a constant).
func TestLoadConfig_EphemeralKey_IsRandomPerCall(t *testing.T) {
	t.Setenv("APIP_DEMO_MODE", "true")
	t.Setenv("PLATFORM_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("APIP_DATABASE_SECRET_ENCRYPTION_KEY", "")

	cfg1, err := LoadConfig("")
	require.NoError(t, err)
	cfg2, err := LoadConfig("")
	require.NoError(t, err)

	assert.NotEqual(t, cfg1.Database.SecretEncryptionKey, cfg2.Database.SecretEncryptionKey,
		"ephemeral keys must differ between independent LoadConfig calls")
}

// TestLoadConfig_ExplicitSecretEncryptionKey verifies the normal path where the
// env var is set — no ephemeral generation occurs and the value is passed through.
func TestLoadConfig_ExplicitSecretEncryptionKey_UsedAsIs(t *testing.T) {
	const stableKey = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	t.Setenv("PLATFORM_SECRET_ENCRYPTION_KEY", stableKey)
	t.Setenv("APIP_DEMO_MODE", "false")

	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.Equal(t, stableKey, cfg.Database.SecretEncryptionKey)
}

// Ensure APIP_DEMO_MODE="1" is also accepted as a truthy value.
func TestLoadConfig_DemoModeOne_AcceptedAsTruthy(t *testing.T) {
	t.Setenv("APIP_DEMO_MODE", "1")
	t.Setenv("PLATFORM_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("APIP_DATABASE_SECRET_ENCRYPTION_KEY", "")

	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.Database.SecretEncryptionKey)
}

// Ensure APIP_DEMO_MODE with surrounding whitespace is handled gracefully.
func TestLoadConfig_DemoModeWhitespace_Trimmed(t *testing.T) {
	t.Setenv("APIP_DEMO_MODE", "  true  ")
	t.Setenv("PLATFORM_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("APIP_DATABASE_SECRET_ENCRYPTION_KEY", "")

	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.Database.SecretEncryptionKey)
}

// cleanEnvForTest clears all environment variables that LoadConfig reads from the
// environment so each test starts from a known baseline.
func init() {
	// Clear vars that would leak from the host environment and break assertions.
	for _, v := range []string{
		"PLATFORM_SECRET_ENCRYPTION_KEY",
		"APIP_DATABASE_SECRET_ENCRYPTION_KEY",
		"APIP_DEMO_MODE",
	} {
		os.Unsetenv(v)
	}
}
