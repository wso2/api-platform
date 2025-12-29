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
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
)

// ValidateGoInterface checks if the policy implements required Policy interfaces
func ValidateGoInterface(policy *types.DiscoveredPolicy) []types.ValidationError {
	slog.Debug("Validating Go interface implementation",
		"policy", policy.Name,
		"version", policy.Version,
		"sourceFiles", len(policy.SourceFiles),
		"phase", "validation")

	var errors []types.ValidationError

	// Parse all Go source files
	fset := token.NewFileSet()
	var files []*ast.File

	for _, sourceFile := range policy.SourceFiles {
		slog.Debug("Parsing Go source file",
			"file", filepath.Base(sourceFile),
			"path", sourceFile,
			"phase", "validation")

		file, err := parser.ParseFile(fset, sourceFile, nil, parser.ParseComments)
		if err != nil {
			errors = append(errors, types.ValidationError{
				PolicyName:    policy.Name,
				PolicyVersion: policy.Version,
				FilePath:      sourceFile,
				Message:       fmt.Sprintf("failed to parse Go file: %v", err),
			})
			continue
		}
		files = append(files, file)
	}

	if len(files) == 0 {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.Path,
			Message:       "no valid Go source files found",
		})
		return errors
	}

	// Check for required methods
	hasMode := false
	hasOnRequest := false
	hasOnResponse := false
	hasNewPolicy := false

	for _, file := range files {
		for _, decl := range file.Decls {
			// Check for function declarations
			if funcDecl, ok := decl.(*ast.FuncDecl); ok {
				methodName := funcDecl.Name.Name
				hasReceiver := funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0

				slog.Debug("Found method/function",
					"name", methodName,
					"hasReceiver", hasReceiver,
					"phase", "validation")

				// Check for GetPolicy factory function
				if methodName == "GetPolicy" {
					hasNewPolicy = true
					slog.Debug("Found GetPolicy factory function", "phase", "validation")
				}

				// Check for interface methods
				if hasReceiver {
					switch methodName {
					case "Mode":
						hasMode = true
						slog.Debug("Found Mode method", "phase", "validation")
					case "OnRequest":
						hasOnRequest = true
						slog.Debug("Found OnRequest method", "phase", "validation")
					case "OnResponse":
						hasOnResponse = true
						slog.Debug("Found OnResponse method", "phase", "validation")
					}
				}
			}
		}
	}

	// Log validation summary
	slog.Debug("Interface validation summary",
		"policy", policy.Name,
		"hasMode", hasMode,
		"hasOnRequest", hasOnRequest,
		"hasOnResponse", hasOnResponse,
		"hasNewPolicy", hasNewPolicy,
		"phase", "validation")

	// Report missing required methods
	if !hasMode {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.Path,
			Message:       "missing required Mode() method implementation",
		})
	}

	if !hasOnRequest {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.Path,
			Message:       "missing required OnRequest() method implementation",
		})
	}

	if !hasOnResponse {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.Path,
			Message:       "missing required OnResponse() method implementation",
		})
	}

	if !hasNewPolicy {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.Path,
			Message:       "missing required GetPolicy() factory function",
		})
	}

	return errors
}

// ValidateGoMod checks if go.mod file exists and is valid
func ValidateGoMod(policy *types.DiscoveredPolicy) []types.ValidationError {
	slog.Debug("Validating go.mod",
		"policy", policy.Name,
		"goModPath", policy.GoModPath,
		"phase", "validation")

	var errors []types.ValidationError

	// Check if go.mod exists in the expected location
	goModPath := filepath.Join(policy.Path, "go.mod")
	if goModPath != policy.GoModPath {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      goModPath,
			Message:       "go.mod path mismatch",
		})
	}

	return errors
}

// sanitizeForGoIdent converts a string to a valid Go identifier
func sanitizeForGoIdent(s string) string {
	// Replace invalid characters with underscores
	result := strings.Builder{}
	for i, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r == '_') || (i > 0 && r >= '0' && r <= '9') {
			result.WriteRune(r)
		} else {
			result.WriteRune('_')
		}
	}
	return result.String()
}
