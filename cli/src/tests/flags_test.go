/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package tests

import (
	"testing"

	"github.com/wso2/api-platform/cli/utils"
)

// TestFlagShortNamesUnique ensures all short flag names are unique
func TestFlagShortNamesUnique(t *testing.T) {
	shortFlags := map[string]string{
		utils.FlagNameShort:     utils.FlagName,
		utils.FlagServerShort:   utils.FlagServer,
		utils.FlagTokenShort:    utils.FlagToken,
		utils.FlagEnvTokenShort: utils.FlagEnvToken,
		utils.FlagInsecureShort: utils.FlagInsecure,
		utils.FlagOutputShort:   utils.FlagOutput,
	}

	// Check for duplicates
	seen := make(map[string]string)
	for short, long := range shortFlags {
		if existingLong, exists := seen[short]; exists {
			t.Errorf("Duplicate short flag '%s' used for both '%s' and '%s'", short, existingLong, long)
		}
		seen[short] = long
	}
}

// TestFlagLongNamesUnique ensures all long flag names are unique
func TestFlagLongNamesUnique(t *testing.T) {
	longFlags := []string{
		utils.FlagName,
		utils.FlagServer,
		utils.FlagToken,
		utils.FlagEnvToken,
		utils.FlagInsecure,
		utils.FlagOutput,
	}

	seen := make(map[string]bool)
	for _, flag := range longFlags {
		if seen[flag] {
			t.Errorf("Duplicate long flag '%s' found", flag)
		}
		seen[flag] = true
	}
}
