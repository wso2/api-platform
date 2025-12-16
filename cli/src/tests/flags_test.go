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
)

// parseFlagsFromFile reads the flags.go file and extracts all flag constant values
func parseFlagsFromFile(t *testing.T) map[string]string {
	t.Helper()
	// Read the flags.go file
	flagsFilePath := filepath.Join("..", "utils", "flags.go")
	content, err := os.ReadFile(flagsFilePath)
	if err != nil {
		t.Fatalf("Failed to read flags.go: %v", err)
	}

	fileContent := string(content)
	allFlags := make(map[string]string)

	// Regular expression to match flag constant declarations
	// Matches patterns like: FlagName = "name" or FlagNameShort = "n"
	flagPattern := regexp.MustCompile(`(?m)^\s*(Flag\w+)\s*=\s*"([^"]+)"`)

	matches := flagPattern.FindAllStringSubmatch(fileContent, -1)
	for _, match := range matches {
		if len(match) == 3 {
			constantName := match[1]
			flagValue := match[2]
			allFlags[constantName] = flagValue
		}
	}

	if len(allFlags) == 0 {
		t.Fatal("No flag constants found in flags.go - check the file format")
	}

	return allFlags
}

// TestFlagValuesUnique ensures all flag values are unique across all flags
func TestFlagValuesUnique(t *testing.T) {
	allFlags := parseFlagsFromFile(t)

	// Check for duplicate values
	valueToConst := make(map[string][]string)
	for constName, value := range allFlags {
		valueToConst[value] = append(valueToConst[value], constName)
	}

	// Report any duplicates
	foundDuplicates := false
	for value, constants := range valueToConst {
		if len(constants) > 1 {
			foundDuplicates = true
			t.Errorf("Duplicate flag value '%s' used in: %v", value, constants)
		}
	}

	if !foundDuplicates {
		t.Logf("✓ All %d flag values are unique", len(allFlags))
	}
}
