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

package unit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCmdNamingConvention validates that command files follow the naming convention:
// 1. Each file in cmd/ (except root.go) should implement a command matching its filename
// 2. The full command path should match the directory structure
//
// Examples:
//   - cli/src/cmd/gateway/api/delete.go → implements "delete" and parent chain is "gateway api"
//   - cli/src/cmd/gateway/mcp/generate.go → implements "generate" and parent chain is "gateway mcp"
//   - cli/src/cmd/version.go → implements "version" at root level
//
// Pattern: Full file path from cmd/ should match the cobra command hierarchy.
func TestCmdNamingConvention(t *testing.T) {
	// Navigate to cmd directory from test/unit
	cmdDir := filepath.Join("..", "..", "cmd")
	violations := []string{}

	err := filepath.Walk(cmdDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip non-Go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip test files
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Skip root.go - it's the exception to the rule
		filename := filepath.Base(path)
		if filename == "root.go" {
			return nil
		}

		// Get relative path from cmd directory
		relPath, err := filepath.Rel(cmdDir, path)
		if err != nil {
			t.Logf("Warning: Failed to get relative path for %s: %v", path, err)
			return nil
		}

		// Extract expected cmd name and path components from file path
		expectedCommand := strings.TrimSuffix(filename, ".go")
		pathParts := strings.Split(filepath.ToSlash(relPath), "/")

		// Build expected cmd chain: all directories + filename (without .go)
		expectedCommandChain := make([]string, len(pathParts))
		copy(expectedCommandChain, pathParts)
		expectedCommandChain[len(expectedCommandChain)-1] = expectedCommand

		// Parse the Go file to find cobra.Command definitions
		foundCommands, parseErr := extractCommandUseFields(path)
		if parseErr != nil {
			t.Logf("Warning: Failed to parse %s: %v", path, parseErr)
			return nil
		}

		// Determine per-file concise mapping
		fullCmdPath := "ap " + strings.Join(expectedCommandChain, " ")
		if len(foundCommands) == 0 {
			// No cobra.Command definitions found — this is a violation
			t.Logf("\"%s\" has no cmd defined", relPath)
			violations = append(violations, fmt.Sprintf(
				"❌ %s\n"+
					"   No cobra.Command definitions found\n"+
					"   → File should implement a command named '%s' (full: %s)",
				relPath,
				expectedCommand,
				fullCmdPath,
			))
			return nil
		}

		// Collect found command names (first word of Use or the literal)
		foundNames := []string{}
		matched := false
		for _, cmd := range foundCommands {
			fields := strings.Fields(cmd.Use)
			if len(fields) == 0 {
				continue
			}
			cmdName := fields[0]
			foundNames = append(foundNames, cmdName)
			if cmdName == expectedCommand {
				matched = true
			}
		}

		if matched {
			t.Logf("\"%s\" matches \"%s\" cmd", relPath, fullCmdPath)
		} else {
			t.Logf("\"%s\" does not match expected \"%s\" (found: %s)", relPath, fullCmdPath, strings.Join(foundNames, ", "))
			violations = append(violations, fmt.Sprintf(
				"❌ %s\n"+
					"   Expected cmd name: '%s'\n"+
					"   Expected full cmd: %s\n"+
					"   Found commands: %v\n"+
					"   → File path must match cmd hierarchy",
				relPath,
				expectedCommand,
				fullCmdPath,
				foundCommands,
			))
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk command directory: %v", err)
	}

	if len(violations) > 0 {
		t.Errorf("\n\n🚨 Cmd Naming Convention Violations Detected!\n\n"+
			"The following files do not follow the cmd naming convention:\n"+
			"Each file path in cmd/ should match the full cmd hierarchy.\n\n"+
			"%s\n\n"+
			"📋 Convention Rules:\n"+
			"  • File name (without .go) must match the cobra.Command Use field\n"+
			"  • Directory structure must match cmd hierarchy\n"+
			"  • Example: gateway/api/delete.go → cmd 'delete' in 'ap gateway api delete'\n"+
			"  • Example: gateway/mcp/generate.go → cmd 'generate' in 'ap gateway mcp generate'\n"+
			"  • Exception: root.go (root cmd)\n\n"+
			"Please ensure file path matches: cmd/<parent1>/<parent2>/<cmd>.go\n"+
			"where <cmd> matches the cobra.Command Use field.\n",
			strings.Join(violations, "\n\n"))
	}
}

// commandUse represents a cobra.Command Use value with its position
type commandUse struct {
	Use  string
	Line int
}

// extractCommandUseFields parses a Go file and extracts all cobra.Command Use field values with line numbers
func extractCommandUseFields(filePath string) ([]commandUse, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	// Build a map of const string values in this file for resolving identifiers used in Use fields
	constMap := map[string]string{}
	for _, decl := range node.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.CONST {
			for _, spec := range gen.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					for i, name := range vs.Names {
						if i < len(vs.Values) {
							if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
								constMap[name.Name] = strings.Trim(lit.Value, "\"")
							}
						}
					}
				}
			}
		}
	}

	var commandUses []commandUse

	processed := map[int]bool{}
	ast.Inspect(node, func(n ast.Node) bool {
		// Support both direct composite literal (&cobra.Command{...}) and plain composite literal (cobra.Command{...})
		var comp *ast.CompositeLit

		if ue, ok := n.(*ast.UnaryExpr); ok {
			if cl, ok := ue.X.(*ast.CompositeLit); ok {
				comp = cl
			}
		}

		if cl, ok := n.(*ast.CompositeLit); ok {
			comp = cl
		}

		if comp == nil {
			return true
		}

		// Avoid processing the same composite literal twice (visited via UnaryExpr and CompositeLit)
		key := int(comp.Pos())
		if processed[key] {
			return true
		}
		processed[key] = true

		// Check if it's a cobra.Command type
		if sel, ok := comp.Type.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "cobra" && sel.Sel.Name == "Command" {
				// Found a cobra.Command, now extract the Use field
				for _, elt := range comp.Elts {
					if kv, ok := elt.(*ast.KeyValueExpr); ok {
						if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Use" {
							// Handle string literal value
							if val, ok := kv.Value.(*ast.BasicLit); ok && val.Kind == token.STRING {
								useValue := strings.Trim(val.Value, "\"")
								pos := fset.Position(val.Pos())
								commandUses = append(commandUses, commandUse{Use: useValue, Line: pos.Line})
								continue
							}

							// Handle identifier value (e.g., Use: AddCmdLiteral)
							if identVal, ok := kv.Value.(*ast.Ident); ok {
								if v, found := constMap[identVal.Name]; found {
									// Use value resolved from const map
									// Use position of the identifier for line number
									pos := fset.Position(identVal.Pos())
									commandUses = append(commandUses, commandUse{Use: v, Line: pos.Line})
								} else {
									// Fallback: use the identifier name as-is
									pos := fset.Position(identVal.Pos())
									commandUses = append(commandUses, commandUse{Use: identVal.Name, Line: pos.Line})
								}
							}
						}
					}
				}
			}
		}

		return true
	})

	return commandUses, nil
}
