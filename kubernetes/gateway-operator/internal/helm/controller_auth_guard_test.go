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
	"testing"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

// legacyGatewayChart mimics a gateway chart from before the generated controller admin
// credential: its defaults list the admin/admin user and have no "generate" key.
func legacyGatewayChart() *chart.Chart {
	return &chart.Chart{
		Metadata: &chart.Metadata{Name: "gateway", Version: "1.2.0"},
		Values: map[string]interface{}{"gateway": map[string]interface{}{"config": map[string]interface{}{
			"controller": map[string]interface{}{"auth": map[string]interface{}{
				"basic": map[string]interface{}{"enabled": true, "users": []interface{}{
					map[string]interface{}{"username": "admin", "password": "admin", "password_hashed": false, "roles": []interface{}{"admin"}},
				}},
				"idp": map[string]interface{}{"enabled": false},
			}},
		}}},
	}
}

func authValues(auth map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"gateway": map[string]interface{}{"config": map[string]interface{}{
		"controller": map[string]interface{}{"auth": auth},
	}}}
}

func TestCheckControllerAuthSupported(t *testing.T) {
	noUsers := authValues(map[string]interface{}{"basic": map[string]interface{}{"enabled": true, "generate": true, "users": []interface{}{}}})

	t.Run("legacy chart with no users is refused", func(t *testing.T) {
		err := checkControllerAuthSupported(legacyGatewayChart(), noUsers)
		require.Error(t, err)
		require.Contains(t, err.Error(), `"1.2.0"`)
		require.Contains(t, err.Error(), "gateway.helm.chartVersion")
	})

	t.Run("legacy chart with basic auth disabled and IDP off is refused", func(t *testing.T) {
		err := checkControllerAuthSupported(legacyGatewayChart(), authValues(map[string]interface{}{"basic": map[string]interface{}{"enabled": false}}))
		require.Error(t, err)
	})

	t.Run("legacy chart with explicit users is allowed", func(t *testing.T) {
		values := authValues(map[string]interface{}{"basic": map[string]interface{}{"users": []interface{}{
			map[string]interface{}{"username": "ops", "password": "$2y$12$x", "password_hashed": true},
		}}})
		require.NoError(t, checkControllerAuthSupported(legacyGatewayChart(), values))
	})

	t.Run("legacy chart with its own defaults is allowed", func(t *testing.T) {
		require.NoError(t, checkControllerAuthSupported(legacyGatewayChart(), map[string]interface{}{}))
	})

	t.Run("legacy chart with IDP auth is allowed", func(t *testing.T) {
		values := authValues(map[string]interface{}{"basic": map[string]interface{}{"users": []interface{}{}}, "idp": map[string]interface{}{"enabled": true}})
		require.NoError(t, checkControllerAuthSupported(legacyGatewayChart(), values))
	})

	t.Run("chart without controller auth settings is not checked", func(t *testing.T) {
		require.NoError(t, checkControllerAuthSupported(&chart.Chart{Metadata: &chart.Metadata{Name: "other"}, Values: map[string]interface{}{}}, noUsers))
	})

	t.Run("repository gateway chart generates the credential", func(t *testing.T) {
		ch, err := loader.Load(gatewayHelmChartPath)
		require.NoError(t, err)
		require.NoError(t, checkControllerAuthSupported(ch, noUsers))
	})
}
