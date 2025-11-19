package validation

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/envoy-policy-engine/builder/pkg/types"
)

// ValidateGoInterface checks if the policy implements required Policy interfaces
func ValidateGoInterface(policy *types.DiscoveredPolicy) []types.ValidationError {
	var errors []types.ValidationError

	// Parse all Go source files
	fset := token.NewFileSet()
	var files []*ast.File

	for _, sourceFile := range policy.SourceFiles {
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
	hasName := false
	hasValidate := false
	hasExecuteRequest := false
	hasExecuteResponse := false
	hasNewPolicy := false

	for _, file := range files {
		for _, decl := range file.Decls {
			// Check for function declarations
			if funcDecl, ok := decl.(*ast.FuncDecl); ok {
				methodName := funcDecl.Name.Name

				// Check for NewPolicy factory function
				if methodName == "NewPolicy" {
					hasNewPolicy = true
				}

				// Check for interface methods
				if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
					switch methodName {
					case "Name":
						hasName = true
					case "Validate":
						hasValidate = true
					case "ExecuteRequest":
						hasExecuteRequest = true
					case "ExecuteResponse":
						hasExecuteResponse = true
					}
				}
			}
		}
	}

	// Report missing required methods
	if !hasName {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.Path,
			Message:       "missing required Name() method implementation",
		})
	}

	if !hasValidate {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.Path,
			Message:       "missing required Validate() method implementation",
		})
	}

	if !hasExecuteRequest && !hasExecuteResponse {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.Path,
			Message:       "must implement at least ExecuteRequest() or ExecuteResponse() method",
		})
	}

	if !hasNewPolicy {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.Path,
			Message:       "missing required NewPolicy() factory function",
		})
	}

	return errors
}

// ValidateGoMod checks if go.mod file exists and is valid
func ValidateGoMod(policy *types.DiscoveredPolicy) []types.ValidationError {
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
