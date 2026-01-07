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
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/wso2/api-platform/cli/utils"
)

// TestFlagValuesUnique ensures all flag values are unique across all flags
func TestFlagValuesUnique(t *testing.T) {
	flagsFilePath := filepath.Join("..", "..", "..", "src", "utils", "flags.go")
	content, err := os.ReadFile(flagsFilePath)
	if err != nil {
		t.Fatalf("Failed to read flags.go: %v", err)
	}

	constPattern := regexp.MustCompile(`(?m)^\s*Flag\w+\s*=\s*"([^"]+)"`)
	matches := constPattern.FindAllStringSubmatch(string(content), -1)

	allFlagValues := []string{}
	for _, match := range matches {
		if len(match) == 2 {
			allFlagValues = append(allFlagValues, match[1])
		}
	}

	for _, short := range utils.GetShortFlags() {
		allFlagValues = append(allFlagValues, short)
	}

	seen := make(map[string]bool)
	duplicates := make(map[string]int)

	for _, value := range allFlagValues {
		if seen[value] {
			duplicates[value]++
		}
		seen[value] = true
	}

	if len(duplicates) > 0 {
		for value, count := range duplicates {
			t.Errorf("Duplicate flag value '%s' found %d times", value, count+1)
		}
	} else {
		t.Logf("✓ All %d flag values are unique", len(allFlagValues))
	}
}
