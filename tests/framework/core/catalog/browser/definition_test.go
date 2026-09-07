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

package browser

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlaywrightVersionFromModule(t *testing.T) {
	for module, want := range map[string]string{
		"v0.6201.1": "1.62.1",
		"v0.6201.0": "1.62.1",
		"v0.6100.0": "1.61.0",
		"v0.5700.2": "1.57.0",
	} {
		got, err := playwrightVersionFromModule(module)
		require.NoError(t, err, module)
		require.Equal(t, want, got, module)
	}
	_, err := playwrightVersionFromModule("v1.2.3")
	require.Error(t, err, "a version that does not encode a playwright minor+patch must be rejected")
}

// The triad pin: the module in this build must encode exactly PlaywrightVersion, or the
// browser image and the connecting client drift apart — the binding's most-reported failure.
func TestPlaywrightVersionMatchesModule(t *testing.T) {
	require.NoError(t, PlaywrightVersionMatchesModule())
}

func TestPlaywrightImageCarriesThePin(t *testing.T) {
	require.Equal(t, "mcr.microsoft.com/playwright:v"+PlaywrightVersion+"-noble", imgPlaywright())
}
