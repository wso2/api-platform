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

package helmgateway

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/helm"
)

// A gateway with no legacy release name is looked up under its current name only: an
// empty name would make Helm fail the lookup ("Release name is invalid").
func TestReleaseNamesFor(t *testing.T) {
	short := releaseNamesFor("e2e-env")
	assert.Equal(t, []string{helm.GetReleaseName("e2e-env")}, short)

	for _, name := range []string{"gw", "e2e-env", strings.Repeat("g", 40), strings.Repeat("g", 60), strings.Repeat("g", 200)} {
		names := releaseNamesFor(name)
		assert.NotEmpty(t, names)
		for _, n := range names {
			assert.NotEmpty(t, n, "gateway %q", name)
		}
		if legacy := helm.LegacyReleaseName(name); legacy != "" {
			assert.Contains(t, names, legacy, "gateway %q", name)
		}
	}
}
