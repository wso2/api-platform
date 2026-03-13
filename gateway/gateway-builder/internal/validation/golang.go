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

// methodSig holds the parameter and result counts for a function extracted from AST.
type methodSig struct {
	params  int
	results int
}

// expectedMethodArity defines the required parameter and result counts for every
// known policy interface method. Validation fails if a method is present but has
// the wrong arity — this catches signature drift early, before Docker build.
//
// Counts follow the SDK interfaces in sdk/gateway/policy/v1alpha/interface.go:
//
//	GetPolicy(metadata PolicyMetadata, params map[string]interface{}) (Policy, error) → 2 params, 2 results
//	OnRequestHeaders(ctx *RequestHeaderContext) RequestHeaderAction                   → 1 param,  1 result
//	OnResponseHeaders(ctx *ResponseHeaderContext) ResponseHeaderAction                → 1 param,  1 result
//	OnRequestBody(ctx *RequestContext) RequestAction                                  → 1 param,  1 result
//	OnResponseBody(ctx *ResponseContext) ResponseAction                               → 1 param,  1 result
//	OnRequestBodyChunk(ctx *RequestStreamContext, chunk *StreamBody) RequestChunkAction  → 2 params, 1 result
//	OnResponseBodyChunk(ctx *ResponseStreamContext, chunk *StreamBody) ResponseChunkAction → 2 params, 1 result
//	NeedsMoreRequestData(accumulated []byte) bool                                     → 1 param,  1 result
//	NeedsMoreResponseData(accumulated []byte) bool                                    → 1 param,  1 result
var expectedMethodArity = map[string]methodSig{
	"OnRequestHeaders":      {params: 1, results: 1},
	"OnResponseHeaders":     {params: 1, results: 1},
	"OnRequestBody":         {params: 1, results: 1},
	"OnResponseBody":        {params: 1, results: 1},
	"OnRequestBodyChunk":    {params: 2, results: 1},
	"OnResponseBodyChunk":   {params: 2, results: 1},
	"NeedsMoreRequestData":  {params: 1, results: 1},
	"NeedsMoreResponseData": {params: 1, results: 1},
}

// subInterfaceMethods is the set of method names that count as a sub-interface
// implementation (a policy must have at least one of these).
var subInterfaceMethods = map[string]bool{
	"OnRequestHeaders":    true,
	"OnResponseHeaders":   true,
	"OnRequestBody":       true,
	"OnResponseBody":      true,
	"OnRequestBodyChunk":  true,
	"OnResponseBodyChunk": true,
}

// ValidateGoInterface checks that the policy source files satisfy the required
// policy interface contracts. It uses Go AST parsing (not full type checking) and
// validates:
//
//  1. GetPolicy factory function exists with the correct arity (2 params, 2 results).
//  2. At least one sub-interface method is implemented on a receiver type.
//  3. Streaming interface coherence — all three methods of a streaming interface
//     must be implemented on the same receiver type:
//     - StreamingResponsePolicy: OnResponseBodyChunk + OnResponseBody + NeedsMoreResponseData
//     - StreamingRequestPolicy:  OnRequestBodyChunk  + OnRequestBody  + NeedsMoreRequestData
//  4. Method signature arity matches the SDK interface for all known methods.
func ValidateGoInterface(policy *types.DiscoveredPolicy) []types.ValidationError {
	slog.Debug("Validating Go interface implementation",
		"policy", policy.Name,
		"version", policy.Version,
		"sourceFiles", len(policy.SourceFiles),
		"phase", "validation")

	var errors []types.ValidationError

	// Parse all Go source files.
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

	// Scan all declarations:
	//   - top-level functions → look for GetPolicy
	//   - methods (with receiver) → group by receiver type name
	var getPolicySig *methodSig
	// typeMethods: receiver type name → method name → signature
	typeMethods := make(map[string]map[string]methodSig)

	for _, file := range files {
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			sig := methodSig{
				params:  countFields(funcDecl.Type.Params),
				results: countFields(funcDecl.Type.Results),
			}

			if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
				// Top-level function.
				if funcDecl.Name.Name == "GetPolicy" {
					getPolicySig = &sig
					slog.Debug("Found GetPolicy factory function",
						"params", sig.params,
						"results", sig.results,
						"phase", "validation")
				}
				continue
			}

			// Method — extract receiver type name (dereference pointer if needed).
			recvTypeName := extractTypeName(funcDecl.Recv.List[0].Type)
			if _, ok := typeMethods[recvTypeName]; !ok {
				typeMethods[recvTypeName] = make(map[string]methodSig)
			}
			typeMethods[recvTypeName][funcDecl.Name.Name] = sig

			slog.Debug("Found method",
				"type", recvTypeName,
				"method", funcDecl.Name.Name,
				"params", sig.params,
				"results", sig.results,
				"phase", "validation")
		}
	}

	// ── 1. Validate GetPolicy ────────────────────────────────────────────────
	if getPolicySig == nil {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.Path,
			Message:       "missing required GetPolicy() factory function: must have signature GetPolicy(metadata PolicyMetadata, params map[string]interface{}) (Policy, error)",
		})
	} else {
		if getPolicySig.params != 2 {
			errors = append(errors, types.ValidationError{
				PolicyName:    policy.Name,
				PolicyVersion: policy.Version,
				FilePath:      policy.Path,
				Message: fmt.Sprintf(
					"GetPolicy has wrong parameter count: expected 2 (metadata PolicyMetadata, params map[string]interface{}), got %d",
					getPolicySig.params),
			})
		}
		if getPolicySig.results != 2 {
			errors = append(errors, types.ValidationError{
				PolicyName:    policy.Name,
				PolicyVersion: policy.Version,
				FilePath:      policy.Path,
				Message: fmt.Sprintf(
					"GetPolicy has wrong return count: expected 2 (Policy, error), got %d",
					getPolicySig.results),
			})
		}
	}

	// ── 2 & 3 & 4. Per-type checks ───────────────────────────────────────────
	hasSubInterfaceMethod := false

	for typeName, methods := range typeMethods {
		// 2. Sub-interface method presence.
		for name := range methods {
			if subInterfaceMethods[name] {
				hasSubInterfaceMethod = true
				break
			}
		}

		// 3a. StreamingResponsePolicy coherence.
		// OnResponseBodyChunk requires OnResponseBody + NeedsMoreResponseData on the same type.
		if _, hasChunk := methods["OnResponseBodyChunk"]; hasChunk {
			if _, hasBody := methods["OnResponseBody"]; !hasBody {
				errors = append(errors, types.ValidationError{
					PolicyName:    policy.Name,
					PolicyVersion: policy.Version,
					FilePath:      policy.Path,
					Message: fmt.Sprintf(
						"type %s implements OnResponseBodyChunk but is missing OnResponseBody — "+
							"StreamingResponsePolicy requires all three on the same type: "+
							"OnResponseBody, OnResponseBodyChunk, NeedsMoreResponseData",
						typeName),
				})
			}
			if _, hasNeeds := methods["NeedsMoreResponseData"]; !hasNeeds {
				errors = append(errors, types.ValidationError{
					PolicyName:    policy.Name,
					PolicyVersion: policy.Version,
					FilePath:      policy.Path,
					Message: fmt.Sprintf(
						"type %s implements OnResponseBodyChunk but is missing NeedsMoreResponseData — "+
							"StreamingResponsePolicy requires all three on the same type: "+
							"OnResponseBody, OnResponseBodyChunk, NeedsMoreResponseData",
						typeName),
				})
			}
		}

		// 3b. StreamingRequestPolicy coherence.
		// OnRequestBodyChunk requires OnRequestBody + NeedsMoreRequestData on the same type.
		if _, hasChunk := methods["OnRequestBodyChunk"]; hasChunk {
			if _, hasBody := methods["OnRequestBody"]; !hasBody {
				errors = append(errors, types.ValidationError{
					PolicyName:    policy.Name,
					PolicyVersion: policy.Version,
					FilePath:      policy.Path,
					Message: fmt.Sprintf(
						"type %s implements OnRequestBodyChunk but is missing OnRequestBody — "+
							"StreamingRequestPolicy requires all three on the same type: "+
							"OnRequestBody, OnRequestBodyChunk, NeedsMoreRequestData",
						typeName),
				})
			}
			if _, hasNeeds := methods["NeedsMoreRequestData"]; !hasNeeds {
				errors = append(errors, types.ValidationError{
					PolicyName:    policy.Name,
					PolicyVersion: policy.Version,
					FilePath:      policy.Path,
					Message: fmt.Sprintf(
						"type %s implements OnRequestBodyChunk but is missing NeedsMoreRequestData — "+
							"StreamingRequestPolicy requires all three on the same type: "+
							"OnRequestBody, OnRequestBodyChunk, NeedsMoreRequestData",
						typeName),
				})
			}
		}

		// 4. Method arity checks for all known methods found on this type.
		for methodName, expected := range expectedMethodArity {
			sig, found := methods[methodName]
			if !found {
				continue
			}
			if sig.params != expected.params {
				errors = append(errors, types.ValidationError{
					PolicyName:    policy.Name,
					PolicyVersion: policy.Version,
					FilePath:      policy.Path,
					Message: fmt.Sprintf(
						"method %s on type %s has wrong parameter count: expected %d, got %d",
						methodName, typeName, expected.params, sig.params),
				})
			}
			if sig.results != expected.results {
				errors = append(errors, types.ValidationError{
					PolicyName:    policy.Name,
					PolicyVersion: policy.Version,
					FilePath:      policy.Path,
					Message: fmt.Sprintf(
						"method %s on type %s has wrong return count: expected %d, got %d",
						methodName, typeName, expected.results, sig.results),
				})
			}
		}
	}

	if !hasSubInterfaceMethod {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.Path,
			Message: "missing sub-interface method: must implement at least one of " +
				"OnRequestHeaders, OnResponseHeaders, OnRequestBody, OnResponseBody, " +
				"OnRequestBodyChunk, or OnResponseBodyChunk",
		})
	}

	slog.Debug("Interface validation summary",
		"policy", policy.Name,
		"hasSubInterfaceMethod", hasSubInterfaceMethod,
		"hasGetPolicy", getPolicySig != nil,
		"typeCount", len(typeMethods),
		"errorCount", len(errors),
		"phase", "validation")

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

// countFields returns the total number of parameters or results in a field list.
// Handles both named fields (e.g. "a, b int" → 2) and unnamed fields (e.g. "int" → 1).
func countFields(fieldList *ast.FieldList) int {
	if fieldList == nil {
		return 0
	}
	count := 0
	for _, field := range fieldList.List {
		if len(field.Names) == 0 {
			count++ // unnamed / embedded
		} else {
			count += len(field.Names)
		}
	}
	return count
}

// extractTypeName returns the base type name from a receiver type expression,
// dereferencing pointer types (*T → T) and selector expressions (pkg.T → T).
func extractTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return extractTypeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return t.Sel.Name
	default:
		return "unknown"
	}
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
