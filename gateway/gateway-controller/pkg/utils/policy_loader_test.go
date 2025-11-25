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

package utils

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestPolicyLoader_LoadPoliciesFromDirectory(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	loader := NewPolicyLoader(logger)

	// Create a temporary directory for test policies
	tempDir := t.TempDir()

	// Test case 1: Valid JSON policy
	jsonPolicy := `{
  "name": "TestPolicy1",
  "version": "v1.0.0",
  "description": "Test policy 1",
  "flows": {
    "request": {
      "requireHeader": true,
      "requireBody": false
    }
  }
}`
	err := os.WriteFile(filepath.Join(tempDir, "policy1.json"), []byte(jsonPolicy), 0644)
	if err != nil {
		t.Fatalf("Failed to write test policy file: %v", err)
	}

	// Test case 2: Valid YAML policy
	yamlPolicy := `name: TestPolicy2
version: v1.0.1
description: Test policy 2
flows:
  response:
    requireHeader: false
    requireBody: true
`
	err = os.WriteFile(filepath.Join(tempDir, "policy2.yaml"), []byte(yamlPolicy), 0644)
	if err != nil {
		t.Fatalf("Failed to write test policy file: %v", err)
	}

	// Test case 3: Non-policy file (should be skipped)
	err = os.WriteFile(filepath.Join(tempDir, "README.md"), []byte("# Test"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Load policies
	policies, err := loader.LoadPoliciesFromDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to load policies: %v", err)
	}

	// Verify we loaded exactly 2 policies
	if len(policies) != 2 {
		t.Errorf("Expected 2 policies, got %d", len(policies))
	}

	// Verify policy 1
	key1 := "TestPolicy1|v1.0.0"
	policy1, exists := policies[key1]
	if !exists {
		t.Errorf("Policy1 not found")
	}
	if policy1.Name != "TestPolicy1" {
		t.Errorf("Expected policy name 'TestPolicy1', got '%s'", policy1.Name)
	}
	if policy1.Version != "v1.0.0" {
		t.Errorf("Expected version 'v1.0.0', got '%s'", policy1.Version)
	}

	// Verify policy 2
	key2 := "TestPolicy2|v1.0.1"
	policy2, exists := policies[key2]
	if !exists {
		t.Errorf("Policy2 not found")
	}
	if policy2.Name != "TestPolicy2" {
		t.Errorf("Expected policy name 'TestPolicy2', got '%s'", policy2.Name)
	}
	if policy2.Version != "v1.0.1" {
		t.Errorf("Expected version 'v1.0.1', got '%s'", policy2.Version)
	}
}

func TestPolicyLoader_DuplicatePolicy(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	loader := NewPolicyLoader(logger)

	tempDir := t.TempDir()

	// Create two policies with the same name and version
	policy1 := `{
  "name": "DuplicatePolicy",
  "version": "v1.0.0",
  "flows": {
    "request": {
      "requireHeader": true
    }
  }
}`

	err := os.WriteFile(filepath.Join(tempDir, "duplicate1.json"), []byte(policy1), 0644)
	if err != nil {
		t.Fatalf("Failed to write test policy file: %v", err)
	}

	err = os.WriteFile(filepath.Join(tempDir, "duplicate2.json"), []byte(policy1), 0644)
	if err != nil {
		t.Fatalf("Failed to write test policy file: %v", err)
	}

	// Load policies - should fail due to duplicate
	_, err = loader.LoadPoliciesFromDirectory(tempDir)
	if err == nil {
		t.Error("Expected error for duplicate policies, got nil")
	}
}

func TestPolicyLoader_InvalidPolicy(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	loader := NewPolicyLoader(logger)

	tempDir := t.TempDir()

	// Test case 1: Missing name
	invalidPolicy1 := `{
  "version": "v1.0.0",
  "flows": {
    "request": {
      "requireHeader": true
    }
  }
}`
	err := os.WriteFile(filepath.Join(tempDir, "invalid1.json"), []byte(invalidPolicy1), 0644)
	if err != nil {
		t.Fatalf("Failed to write test policy file: %v", err)
	}

	_, err = loader.LoadPoliciesFromDirectory(tempDir)
	if err == nil {
		t.Error("Expected error for policy without name, got nil")
	}

	// Clean up
	os.Remove(filepath.Join(tempDir, "invalid1.json"))

	// Test case 2: Missing version
	invalidPolicy2 := `{
  "name": "TestPolicy",
  "flows": {
    "request": {
      "requireHeader": true
    }
  }
}`
	err = os.WriteFile(filepath.Join(tempDir, "invalid2.json"), []byte(invalidPolicy2), 0644)
	if err != nil {
		t.Fatalf("Failed to write test policy file: %v", err)
	}

	_, err = loader.LoadPoliciesFromDirectory(tempDir)
	if err == nil {
		t.Error("Expected error for policy without version, got nil")
	}

	// Clean up
	os.Remove(filepath.Join(tempDir, "invalid2.json"))

	// Test case 3: Missing flows
	invalidPolicy3 := `{
  "name": "TestPolicy",
  "version": "v1.0.0"
}`
	err = os.WriteFile(filepath.Join(tempDir, "invalid3.json"), []byte(invalidPolicy3), 0644)
	if err != nil {
		t.Fatalf("Failed to write test policy file: %v", err)
	}

	_, err = loader.LoadPoliciesFromDirectory(tempDir)
	if err == nil {
		t.Error("Expected error for policy without flows, got nil")
	}
}

func TestPolicyLoader_NonExistentDirectory(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	loader := NewPolicyLoader(logger)

	// Load from non-existent directory - should return empty map without error
	policies, err := loader.LoadPoliciesFromDirectory("/nonexistent/directory")
	if err != nil {
		t.Errorf("Expected no error for non-existent directory, got %v", err)
	}
	if len(policies) != 0 {
		t.Errorf("Expected 0 policies from non-existent directory, got %d", len(policies))
	}
}
