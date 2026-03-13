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

// methodSig holds the canonical parameter and result type strings for a function
// extracted from AST. Type strings are normalized: package qualifiers are stripped
// (e.g. *policy.RequestContext → *RequestContext) so qualified and local names
// compare equal. The keyword 'any' is normalized to 'interface{}'.
type methodSig struct {
	paramTypes  []string
	resultTypes []string
}

// expectedMethodSig defines the required parameter and result types for every
// known policy interface method. Validation fails if a method is present but has
// wrong types — this catches signature drift early, before Docker build.
//
// Type strings follow the SDK interfaces in sdk/gateway/policy/v1alpha/:
//   - Package qualifiers are stripped (both policy.RequestContext and RequestContext match).
//   - Pointer annotations are preserved (*RequestContext ≠ RequestContext).
//
// SDK interface signatures:
//
//	OnRequestHeaders(ctx *RequestHeaderContext) RequestHeaderAction
//	OnResponseHeaders(ctx *ResponseHeaderContext) ResponseHeaderAction
//	OnRequestBody(ctx *RequestContext) RequestAction
//	OnResponseBody(ctx *ResponseContext) ResponseAction
//	OnRequestBodyChunk(ctx *RequestStreamContext, chunk *StreamBody) RequestChunkAction
//	OnResponseBodyChunk(ctx *ResponseStreamContext, chunk *StreamBody) ResponseChunkAction
//	NeedsMoreRequestData(accumulated []byte) bool
//	NeedsMoreResponseData(accumulated []byte) bool
var expectedMethodSig = map[string]methodSig{
	"OnRequestHeaders":      {paramTypes: []string{"*RequestHeaderContext"}, resultTypes: []string{"RequestHeaderAction"}},
	"OnResponseHeaders":     {paramTypes: []string{"*ResponseHeaderContext"}, resultTypes: []string{"ResponseHeaderAction"}},
	"OnRequestBody":         {paramTypes: []string{"*RequestContext"}, resultTypes: []string{"RequestAction"}},
	"OnResponseBody":        {paramTypes: []string{"*ResponseContext"}, resultTypes: []string{"ResponseAction"}},
	"OnRequestBodyChunk":    {paramTypes: []string{"*RequestStreamContext", "*StreamBody"}, resultTypes: []string{"RequestChunkAction"}},
	"OnResponseBodyChunk":   {paramTypes: []string{"*ResponseStreamContext", "*StreamBody"}, resultTypes: []string{"ResponseChunkAction"}},
	"NeedsMoreRequestData":  {paramTypes: []string{"[]byte"}, resultTypes: []string{"bool"}},
	"NeedsMoreResponseData": {paramTypes: []string{"[]byte"}, resultTypes: []string{"bool"}},
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
//  2. At least one sub-interface method is implemented on the concrete type returned
//     by GetPolicy. Checks are bound to that type; falls back to all types when the
//     declared return type is an external interface (not defined in this package).
//  3. Streaming interface coherence — all three methods of a streaming interface
//     must be implemented on the same receiver type:
//     - StreamingResponsePolicy: OnResponseBodyChunk + OnResponseBody + NeedsMoreResponseData
//     - StreamingRequestPolicy:  OnRequestBodyChunk  + OnRequestBody  + NeedsMoreRequestData
//  4. Method signature types match the SDK interface for all known methods found
//     on the policy type (both parameter and result types are checked).
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
	//   - top-level functions → look for GetPolicy; record full param/result type strings
	//   - methods (with receiver) → group by receiver type name with full type info
	var getPolicySig *methodSig
	// getPolicyReturnType is the base type name from GetPolicy's first result.
	// Used to identify the concrete policy type in typeMethods.
	var getPolicyReturnType string
	// typeMethods: receiver type name → method name → signature (with type strings)
	typeMethods := make(map[string]map[string]methodSig)

	for _, file := range files {
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			sig := methodSig{
				paramTypes:  extractFieldTypes(funcDecl.Type.Params),
				resultTypes: extractFieldTypes(funcDecl.Type.Results),
			}

			if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
				// Top-level function.
				if funcDecl.Name.Name == "GetPolicy" {
					getPolicySig = &sig
					// Capture the base type name of the first result as the concrete policy type
					// so that sub-interface and type checks can be bound to it.
					if funcDecl.Type.Results != nil && len(funcDecl.Type.Results.List) > 0 {
						getPolicyReturnType = extractTypeName(funcDecl.Type.Results.List[0].Type)
					}
					slog.Debug("Found GetPolicy factory function",
						"params", len(sig.paramTypes),
						"results", len(sig.resultTypes),
						"returnType", getPolicyReturnType,
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
				"params", len(sig.paramTypes),
				"results", len(sig.resultTypes),
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
		if len(getPolicySig.paramTypes) != 2 {
			errors = append(errors, types.ValidationError{
				PolicyName:    policy.Name,
				PolicyVersion: policy.Version,
				FilePath:      policy.Path,
				Message: fmt.Sprintf(
					"GetPolicy has wrong parameter count: expected 2 (metadata PolicyMetadata, params map[string]interface{}), got %d",
					len(getPolicySig.paramTypes)),
			})
		}
		if len(getPolicySig.resultTypes) != 2 {
			errors = append(errors, types.ValidationError{
				PolicyName:    policy.Name,
				PolicyVersion: policy.Version,
				FilePath:      policy.Path,
				Message: fmt.Sprintf(
					"GetPolicy has wrong return count: expected 2 (Policy, error), got %d",
					len(getPolicySig.resultTypes)),
			})
		}
	}

	// Determine which receiver types to run policy checks on.
	// Prefer the concrete type returned by GetPolicy if it has receiver methods in
	// this package; fall back to all types when the return type is an external
	// interface (e.g. policy.Policy from the SDK, not defined in user code).
	checkTypes := typeMethods
	if getPolicyReturnType != "" {
		if policyMethods, found := typeMethods[getPolicyReturnType]; found && len(policyMethods) > 0 {
			checkTypes = map[string]map[string]methodSig{
				getPolicyReturnType: policyMethods,
			}
		}
	}

	// ── 2, 3, 4. Per-type checks ─────────────────────────────────────────────
	hasSubInterfaceMethod := false

	for typeName, methods := range checkTypes {
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

		// 4. Method signature type checks for all known methods found on this type.
		// Count mismatches are reported as count errors; when counts match, individual
		// type strings are compared (package qualifiers stripped, pointers preserved).
		for methodName, expected := range expectedMethodSig {
			sig, found := methods[methodName]
			if !found {
				continue
			}
			if len(sig.paramTypes) != len(expected.paramTypes) {
				errors = append(errors, types.ValidationError{
					PolicyName:    policy.Name,
					PolicyVersion: policy.Version,
					FilePath:      policy.Path,
					Message: fmt.Sprintf(
						"method %s on type %s has wrong parameter count: expected %d, got %d",
						methodName, typeName, len(expected.paramTypes), len(sig.paramTypes)),
				})
			} else {
				for i, wantType := range expected.paramTypes {
					if sig.paramTypes[i] != wantType {
						errors = append(errors, types.ValidationError{
							PolicyName:    policy.Name,
							PolicyVersion: policy.Version,
							FilePath:      policy.Path,
							Message: fmt.Sprintf(
								"method %s on type %s has wrong parameter types: expected (%s), got (%s)",
								methodName, typeName,
								strings.Join(expected.paramTypes, ", "),
								strings.Join(sig.paramTypes, ", ")),
						})
						break // one error per method is enough
					}
				}
			}
			if len(sig.resultTypes) != len(expected.resultTypes) {
				errors = append(errors, types.ValidationError{
					PolicyName:    policy.Name,
					PolicyVersion: policy.Version,
					FilePath:      policy.Path,
					Message: fmt.Sprintf(
						"method %s on type %s has wrong return count: expected %d, got %d",
						methodName, typeName, len(expected.resultTypes), len(sig.resultTypes)),
				})
			} else {
				for i, wantType := range expected.resultTypes {
					if sig.resultTypes[i] != wantType {
						errors = append(errors, types.ValidationError{
							PolicyName:    policy.Name,
							PolicyVersion: policy.Version,
							FilePath:      policy.Path,
							Message: fmt.Sprintf(
								"method %s on type %s has wrong return types: expected (%s), got (%s)",
								methodName, typeName,
								strings.Join(expected.resultTypes, ", "),
								strings.Join(sig.resultTypes, ", ")),
						})
						break
					}
				}
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
		"policyReturnType", getPolicyReturnType,
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

// canonicalTypeString converts an AST type expression to a normalized string for
// comparison. Package qualifiers are stripped from selector expressions so both
// qualified (policy.RequestContext) and local (RequestContext) names compare equal.
// The keyword 'any' is normalized to 'interface{}' since they are aliases in Go 1.18+.
func canonicalTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return "*" + canonicalTypeString(t.X)
	case *ast.Ident:
		if t.Name == "any" {
			return "interface{}"
		}
		return t.Name
	case *ast.SelectorExpr:
		// Strip package qualifier: pkg.T → T
		return t.Sel.Name
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + canonicalTypeString(t.Elt)
		}
		return "[...]" + canonicalTypeString(t.Elt)
	case *ast.MapType:
		return "map[" + canonicalTypeString(t.Key) + "]" + canonicalTypeString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.Ellipsis:
		return "..." + canonicalTypeString(t.Elt)
	default:
		return "unknown"
	}
}

// extractFieldTypes returns the canonical type string for each parameter or result
// in a field list, expanding named fields that share a single type declaration.
// For example, "a, b int" expands to ["int", "int"].
func extractFieldTypes(fieldList *ast.FieldList) []string {
	if fieldList == nil {
		return nil
	}
	var result []string
	for _, field := range fieldList.List {
		typeStr := canonicalTypeString(field.Type)
		if len(field.Names) == 0 {
			// Unnamed / embedded — single entry.
			result = append(result, typeStr)
		} else {
			// Named fields — one entry per name.
			for range field.Names {
				result = append(result, typeStr)
			}
		}
	}
	return result
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
