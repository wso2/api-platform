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

package helm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func controllerUsers(t *testing.T, values map[string]interface{}) []interface{} {
	t.Helper()
	cur := values
	for _, k := range []string{"gateway", "config", "controller", "auth", "basic"} {
		next, ok := cur[k].(map[string]interface{})
		require.True(t, ok, "missing %s", k)
		cur = next
	}
	users, ok := cur["users"].([]interface{})
	require.True(t, ok)
	return users
}

func TestParseValues_StripsPasswordSecretRef(t *testing.T) {
	file := filepath.Join(t.TempDir(), "values.yaml")
	require.NoError(t, os.WriteFile(file, []byte(`
gateway:
  config:
    controller:
      auth:
        basic:
          enabled: true
          users:
            - username: operator
              password: "$2a$10$abcdefghijklmnopqrstuuJr2kSrCm3T4Bx4nQ0E7mY3l3q3Yy8y6"
              password_hashed: true
              roles: [admin]
              password_secret_ref: {name: controller-creds, key: password}
            - username: other
              password: plain
              password_hashed: false
`), 0o600))

	c := &Client{}
	values, err := c.parseValues(InstallOrUpgradeOptions{ValuesFilePath: file})
	require.NoError(t, err)

	users := controllerUsers(t, values)
	require.Len(t, users, 2)
	op := users[0].(map[string]interface{})
	require.NotContains(t, op, "password_secret_ref", "operator-only key must not reach config.toml")
	require.Equal(t, "operator", op["username"])
	require.Equal(t, true, op["password_hashed"])
	require.Equal(t, "plain", users[1].(map[string]interface{})["password"])
}

func TestParseValues_PasswordSecretRefNeedsHash(t *testing.T) {
	c := &Client{}
	_, err := c.parseValues(InstallOrUpgradeOptions{ValuesYAML: `
gateway:
  config:
    controller:
      auth:
        basic:
          enabled: true
          users:
            - username: operator
              password_secret_ref: {name: controller-creds, key: password}
`})
	require.ErrorContains(t, err, "password_secret_ref needs password set to the bcrypt hash")
}

func TestParseValues_NoControllerAuthIsUntouched(t *testing.T) {
	c := &Client{}
	values, err := c.parseValues(InstallOrUpgradeOptions{ValuesYAML: "gateway:\n  replicas: 2\n"})
	require.NoError(t, err)
	require.Equal(t, 2, values["gateway"].(map[string]interface{})["replicas"])
}
