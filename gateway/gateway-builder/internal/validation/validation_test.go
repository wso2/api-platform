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

package validation

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-builder/internal/testutils"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// ==== ValidatePolicies tests ====

func TestValidatePolicies_EmptyList(t *testing.T) {
	result, err := ValidatePolicies([]*types.DiscoveredPolicy{})

	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestValidatePolicies_DuplicatePolicy(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := testutils.CreateValidPolicyDir(t, tmpDir, "testpolicy", "v1.0.0")

	policies := []*types.DiscoveredPolicy{
		{Name: "testpolicy", Version: "v1.0.0", Path: policyDir, YAMLPath: filepath.Join(policyDir, "policy.yaml"), GoModPath: filepath.Join(policyDir, "go.mod"), SourceFiles: []string{filepath.Join(policyDir, "policy.go")}, Definition: &policy.PolicyDefinition{Name: "testpolicy", Version: "v1.0.0"}},
		{Name: "testpolicy", Version: "v1.0.0", Path: policyDir, YAMLPath: filepath.Join(policyDir, "policy.yaml"), GoModPath: filepath.Join(policyDir, "go.mod"), SourceFiles: []string{filepath.Join(policyDir, "policy.go")}, Definition: &policy.PolicyDefinition{Name: "testpolicy", Version: "v1.0.0"}},
	}

	result, err := ValidatePolicies(policies)

	assert.Error(t, err)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0].Message, "duplicate policy")
}

func TestValidatePolicies_ValidPolicy(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := testutils.CreateValidPolicyDir(t, tmpDir, "testpolicy", "v1.0.0")

	policies := []*types.DiscoveredPolicy{
		{
			Name:        "testpolicy",
			Version:     "v1.0.0",
			Path:        policyDir,
			YAMLPath:    filepath.Join(policyDir, "policy.yaml"),
			GoModPath:   filepath.Join(policyDir, "go.mod"),
			SourceFiles: []string{filepath.Join(policyDir, "policy.go")},
			Definition:  &policy.PolicyDefinition{Name: "testpolicy", Version: "v1.0.0"},
		},
	}

	result, err := ValidatePolicies(policies)

	if err != nil {
		t.Logf("Validation errors: %+v", result.Errors)
	}
	require.NoError(t, err)
	assert.True(t, result.Valid)
}

// ==== FormatValidationErrors tests ====

func TestFormatValidationErrors_Valid(t *testing.T) {
	result := &types.ValidationResult{Valid: true}

	output := FormatValidationErrors(result)

	assert.Equal(t, "All validations passed", output)
}

func TestFormatValidationErrors_WithErrors(t *testing.T) {
	result := &types.ValidationResult{
		Valid: false,
		Errors: []types.ValidationError{
			{PolicyName: "test", PolicyVersion: "v1.0.0", Message: "error message", FilePath: "/path/to/file", LineNumber: 42},
		},
	}

	output := FormatValidationErrors(result)

	assert.Contains(t, output, "Validation failed with 1 error(s)")
	assert.Contains(t, output, "[test v")
	assert.Contains(t, output, "error message")
	assert.Contains(t, output, "/path/to/file:42")
}

func TestFormatValidationErrors_WithWarnings(t *testing.T) {
	result := &types.ValidationResult{
		Valid: false,
		Errors: []types.ValidationError{
			{PolicyName: "test", PolicyVersion: "v1.0.0", Message: "error"},
		},
		Warnings: []types.ValidationWarning{
			{PolicyName: "test", PolicyVersion: "v1.0.0", Message: "warning message", FilePath: "/path/warning"},
		},
	}

	output := FormatValidationErrors(result)

	assert.Contains(t, output, "Warnings (1)")
	assert.Contains(t, output, "warning message")
	assert.Contains(t, output, "/path/warning")
}

func TestFormatValidationErrors_NoFilePath(t *testing.T) {
	result := &types.ValidationResult{
		Valid: false,
		Errors: []types.ValidationError{
			{PolicyName: "test", PolicyVersion: "v1.0.0", Message: "error without file"},
		},
	}

	output := FormatValidationErrors(result)

	assert.Contains(t, output, "error without file")
	assert.NotContains(t, output, "File:")
}

// ==== ValidateDirectoryStructure tests ====

func TestValidateDirectoryStructure_AllFilesPresent(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := testutils.CreateValidPolicyDir(t, tmpDir, "test-policy", "v1.0.0")

	policy := &types.DiscoveredPolicy{
		Name:        "test-policy",
		Version:     "v1.0.0",
		Path:        policyDir,
		YAMLPath:    filepath.Join(policyDir, "policy.yaml"),
		GoModPath:   filepath.Join(policyDir, "go.mod"),
		SourceFiles: []string{filepath.Join(policyDir, "policy.go")},
	}

	errors := ValidateDirectoryStructure(policy)

	assert.Empty(t, errors)
}

func TestValidateDirectoryStructure_MissingYAML(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "test-policy")
	testutils.CreateDir(t, policyDir)

	// Create go.mod and source file but not policy.yaml
	testutils.WriteFile(t, filepath.Join(policyDir, "go.mod"), "module test")
	testutils.WriteFile(t, filepath.Join(policyDir, "policy.go"), "package test")

	policy := &types.DiscoveredPolicy{
		Name:        "test-policy",
		Version:     "v1.0.0",
		Path:        policyDir,
		YAMLPath:    filepath.Join(policyDir, "policy.yaml"),
		GoModPath:   filepath.Join(policyDir, "go.mod"),
		SourceFiles: []string{filepath.Join(policyDir, "policy.go")},
	}

	errors := ValidateDirectoryStructure(policy)

	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0].Message, "does not exist")
}

func TestValidateDirectoryStructure_MissingGoMod(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "test-policy")
	testutils.CreateDir(t, policyDir)

	// Create policy.yaml and source file but not go.mod
	testutils.WriteFile(t, filepath.Join(policyDir, "policy.yaml"), "name: test")
	testutils.WriteFile(t, filepath.Join(policyDir, "policy.go"), "package test")

	policy := &types.DiscoveredPolicy{
		Name:        "test-policy",
		Version:     "v1.0.0",
		Path:        policyDir,
		YAMLPath:    filepath.Join(policyDir, "policy.yaml"),
		GoModPath:   filepath.Join(policyDir, "go.mod"),
		SourceFiles: []string{filepath.Join(policyDir, "policy.go")},
	}

	errors := ValidateDirectoryStructure(policy)

	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0].Message, "does not exist")
}

func TestValidateDirectoryStructure_NoSourceFiles(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "test-policy")
	testutils.CreateDir(t, policyDir)

	testutils.WriteFile(t, filepath.Join(policyDir, "policy.yaml"), "name: test")
	testutils.WriteFile(t, filepath.Join(policyDir, "go.mod"), "module test")

	policy := &types.DiscoveredPolicy{
		Name:        "test-policy",
		Version:     "v1.0.0",
		Path:        policyDir,
		YAMLPath:    filepath.Join(policyDir, "policy.yaml"),
		GoModPath:   filepath.Join(policyDir, "go.mod"),
		SourceFiles: []string{}, // No source files
	}

	errors := ValidateDirectoryStructure(policy)

	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0].Message, "no .go source files found")
}

func TestValidateDirectoryStructure_MissingSourceFile(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "test-policy")
	testutils.CreateDir(t, policyDir)

	testutils.WriteFile(t, filepath.Join(policyDir, "policy.yaml"), "name: test")
	testutils.WriteFile(t, filepath.Join(policyDir, "go.mod"), "module test")
	// Don't create policy.go

	policy := &types.DiscoveredPolicy{
		Name:        "test-policy",
		Version:     "v1.0.0",
		Path:        policyDir,
		YAMLPath:    filepath.Join(policyDir, "policy.yaml"),
		GoModPath:   filepath.Join(policyDir, "go.mod"),
		SourceFiles: []string{filepath.Join(policyDir, "policy.go")}, // File doesn't exist
	}

	errors := ValidateDirectoryStructure(policy)

	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0].Message, "policy.go")
}

// ==== ValidateYAMLSchema tests ====

func TestValidateYAMLSchema_ValidDefinition(t *testing.T) {
	policy := &types.DiscoveredPolicy{
		Name:       "test-policy",
		Version:    "v1.0.0",
		Definition: &policy.PolicyDefinition{Name: "test-policy", Version: "v1.0.0"},
	}

	errors := ValidateYAMLSchema(policy)

	assert.Empty(t, errors)
}

func TestValidateYAMLSchema_MissingName(t *testing.T) {
	policy := &types.DiscoveredPolicy{
		Name:       "test-policy",
		Version:    "v1.0.0",
		Definition: &policy.PolicyDefinition{Name: "", Version: "v1.0.0"},
	}

	errors := ValidateYAMLSchema(policy)

	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0].Message, "policy name is required")
}

func TestValidateYAMLSchema_MissingVersion(t *testing.T) {
	policy := &types.DiscoveredPolicy{
		Name:       "test-policy",
		Version:    "v1.0.0",
		Definition: &policy.PolicyDefinition{Name: "test-policy", Version: ""},
	}

	errors := ValidateYAMLSchema(policy)

	// Expect 2 errors: "version is required" + "invalid version format"
	assert.Len(t, errors, 2)
	assert.Contains(t, errors[0].Message, "policy version is required")
}

func TestValidateYAMLSchema_InvalidVersionFormat(t *testing.T) {
	testCases := []struct {
		name    string
		version string
	}{
		{"no v prefix", "1.0.0"},
		{"too short", "v1"},
		{"no dot", "v1000"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			policy := &types.DiscoveredPolicy{
				Name:       "test-policy",
				Version:    tc.version,
				Definition: &policy.PolicyDefinition{Name: "test-policy", Version: tc.version},
			}

			errors := ValidateYAMLSchema(policy)

			assert.NotEmpty(t, errors)
			assert.Contains(t, errors[0].Message, "invalid version format")
		})
	}
}

// ==== isValidVersion tests ====

func TestIsValidVersion(t *testing.T) {
	testCases := []struct {
		version  string
		expected bool
	}{
		{"v1.0.0", true},
		{"v1.2.3", true},
		{"v0.0.1", true},
		{"v10.20.30", true},
		{"1.0.0", false},   // missing v prefix
		{"v1", false},      // too short
		{"v123", false},    // no dot
		{"", false},        // empty
		{"v", false},       // just v
		{"v1.", false},     // too short (length < 5)
		{"version", false}, // not starting with v followed by numbers
	}

	for _, tc := range testCases {
		t.Run(tc.version, func(t *testing.T) {
			result := isValidVersion(tc.version)
			assert.Equal(t, tc.expected, result, "version: %s", tc.version)
		})
	}
}

// ==== ValidateGoInterface tests ====

func TestValidateGoInterface_ValidPolicy(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := testutils.CreateValidPolicyDir(t, tmpDir, "testpolicy", "v1.0.0")

	policy := &types.DiscoveredPolicy{
		Name:        "testpolicy",
		Version:     "v1.0.0",
		Path:        policyDir,
		SourceFiles: []string{filepath.Join(policyDir, "policy.go")},
	}

	errors := ValidateGoInterface(policy)

	assert.Empty(t, errors)
}

func TestValidateGoInterface_InvalidGoSyntax(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "test-policy")
	testutils.CreateDir(t, policyDir)

	// Create invalid Go file
	testutils.WriteFile(t, filepath.Join(policyDir, "policy.go"), "this is not valid go code")

	policy := &types.DiscoveredPolicy{
		Name:        "test-policy",
		Version:     "v1.0.0",
		Path:        policyDir,
		SourceFiles: []string{filepath.Join(policyDir, "policy.go")},
	}

	errors := ValidateGoInterface(policy)

	assert.NotEmpty(t, errors)
	assert.Contains(t, errors[0].Message, "failed to parse Go file")
}

func TestValidateGoInterface_NoValidFiles(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "test-policy")
	testutils.CreateDir(t, policyDir)

	// Create invalid Go file - all files will fail to parse
	testutils.WriteFile(t, filepath.Join(policyDir, "policy.go"), "not valid")

	policy := &types.DiscoveredPolicy{
		Name:        "test-policy",
		Version:     "v1.0.0",
		Path:        policyDir,
		SourceFiles: []string{filepath.Join(policyDir, "policy.go")},
	}

	errors := ValidateGoInterface(policy)

	// Should have parsing error + "no valid Go source files found"
	hasNoValidFilesError := false
	for _, err := range errors {
		if err.Message == "no valid Go source files found" {
			hasNoValidFilesError = true
		}
	}
	assert.True(t, hasNoValidFilesError)
}

func TestValidateGoInterface_MissingMethods(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "test-policy")
	testutils.CreateDir(t, policyDir)

	// Valid Go but missing required methods
	goCode := `package test

type TestPolicy struct{}
`
	testutils.WriteFile(t, filepath.Join(policyDir, "policy.go"), goCode)

	policy := &types.DiscoveredPolicy{
		Name:        "test-policy",
		Version:     "v1.0.0",
		Path:        policyDir,
		SourceFiles: []string{filepath.Join(policyDir, "policy.go")},
	}

	errors := ValidateGoInterface(policy)

	assert.Len(t, errors, 4) // Missing Mode, OnRequest, OnResponse, GetPolicy
}

func TestValidateGoInterface_MissingGetPolicy(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "test-policy")
	testutils.CreateDir(t, policyDir)

	// Has methods but no GetPolicy factory
	goCode := `package test

type TestPolicy struct{}

func (p *TestPolicy) Mode() int { return 0 }
func (p *TestPolicy) OnRequest() {}
func (p *TestPolicy) OnResponse() {}
`
	testutils.WriteFile(t, filepath.Join(policyDir, "policy.go"), goCode)

	policy := &types.DiscoveredPolicy{
		Name:        "test-policy",
		Version:     "v1.0.0",
		Path:        policyDir,
		SourceFiles: []string{filepath.Join(policyDir, "policy.go")},
	}

	errors := ValidateGoInterface(policy)

	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0].Message, "missing required GetPolicy() factory function")
}

// ==== ValidateGoMod tests ====

func TestValidateGoMod_PathMatch(t *testing.T) {
	policy := &types.DiscoveredPolicy{
		Name:      "test-policy",
		Version:   "v1.0.0",
		Path:      "/path/to/policy",
		GoModPath: "/path/to/policy/go.mod",
	}

	errors := ValidateGoMod(policy)

	assert.Empty(t, errors)
}

func TestValidateGoMod_PathMismatch(t *testing.T) {
	policy := &types.DiscoveredPolicy{
		Name:      "test-policy",
		Version:   "v1.0.0",
		Path:      "/path/to/policy",
		GoModPath: "/different/path/go.mod",
	}

	errors := ValidateGoMod(policy)

	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0].Message, "go.mod path mismatch")
}

// ==== sanitizeForGoIdent tests ====

func TestSanitizeForGoIdent(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with-dash", "with_dash"},
		{"with.dot", "with_dot"},
		{"with space", "with_space"},
		{"123start", "_23start"},       // digits at start become _
		{"test123", "test123"},         // digits after first char ok
		{"UPPER", "UPPER"},             // uppercase ok
		{"_underscore", "_underscore"}, // underscore at start ok
		{"", ""},                        // empty
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := sanitizeForGoIdent(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
