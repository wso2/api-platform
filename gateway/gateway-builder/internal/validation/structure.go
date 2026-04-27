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

package validation

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/fsutil"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
)

// ValidateDirectoryStructure validates the Go policy directory structure
func ValidateDirectoryStructure(policy *types.DiscoveredPolicy) []types.ValidationError {
	var errors []types.ValidationError

	// Check policy definition file exists
	if err := fsutil.ValidatePathExists(policy.YAMLPath, types.PolicyDefinitionFile); err != nil {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.YAMLPath,
			Message:       err.Error(),
		})
	}

	// Check go.mod exists
	if err := fsutil.ValidatePathExists(policy.GoModPath, "go.mod"); err != nil {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.GoModPath,
			Message:       err.Error(),
		})
	}

	// Check at least one .go file exists
	if len(policy.SourceFiles) == 0 {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.Path,
			Message:       "no .go source files found",
		})
	}

	// Verify all source files exist
	for _, sourceFile := range policy.SourceFiles {
		if err := fsutil.ValidatePathExists(sourceFile, "source file"); err != nil {
			errors = append(errors, types.ValidationError{
				PolicyName:    policy.Name,
				PolicyVersion: policy.Version,
				FilePath:      sourceFile,
				Message:       fmt.Sprintf("%s: %s", filepath.Base(sourceFile), err.Error()),
			})
		}
	}

	return errors
}

// ValidatePythonDirectoryStructure validates the Python policy directory structure
func ValidatePythonDirectoryStructure(policy *types.DiscoveredPolicy) []types.ValidationError {
	var errors []types.ValidationError

	// Check policy definition file exists
	if err := fsutil.ValidatePathExists(policy.YAMLPath, types.PolicyDefinitionFile); err != nil {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.YAMLPath,
			Message:       err.Error(),
		})
	}

	// Check policy.py exists (recommended but optional)
	policyPyPath := filepath.Join(policy.Path, "policy.py")
	if err := fsutil.ValidatePathExists(policyPyPath, "policy.py"); err != nil {
		// This is a warning, not an error - policy.py is recommended but optional
		// as long as there are other .py files
		slog.Debug("policy.py not found, checking for other Python files",
			"policy", policy.Name,
			"path", policyPyPath)
	}

	// Check at least one .py file exists
	if len(policy.SourceFiles) == 0 {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.Path,
			Message:       "no .py source files found",
		})
	}

	// Verify all source files exist
	for _, sourceFile := range policy.SourceFiles {
		if err := fsutil.ValidatePathExists(sourceFile, "source file"); err != nil {
			errors = append(errors, types.ValidationError{
				PolicyName:    policy.Name,
				PolicyVersion: policy.Version,
				FilePath:      sourceFile,
				Message:       fmt.Sprintf("%s: %s", filepath.Base(sourceFile), err.Error()),
			})
		}
	}

	return errors
}
