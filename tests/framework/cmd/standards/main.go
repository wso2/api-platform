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

// Command standards checks integration-suite source for rules that prevent
// timing, response-state, and resource-isolation defects.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type uniqueFieldRule struct {
	name    string
	pattern *regexp.Regexp
}

var uniqueFieldRegistry = []uniqueFieldRule{
	{name: "metadata.name", pattern: regexp.MustCompile(`(?i)(?:["']name["']|^\s*name)\s*:\s*["']?([^"'\s]+)`)},
	{name: "metadata.context", pattern: regexp.MustCompile(`(?i)(?:["']context["']|^\s*context)\s*:\s*["']?([^"'\s]+)`)},
}
var yamlField = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*\s*:`)
var shellSleep = regexp.MustCompile(`(^|[;&|{])\s*sleep\s+`)
var cleanupCreationMethods = map[string]struct{}{
	"createResource":             {},
	"createAPI":                  {},
	"createJSONAPI":              {},
	"createResourceFromTemplate": {},
}

func main() {
	steps := flag.String("steps", "suites/it/steps", "step-definition directory")
	features := flag.String("features", "", "optional feature directory")
	scripts := flag.String("scripts", "tools", "optional shell-script directory")
	root := flag.String("root", ".", "framework root for architecture checks")
	unitRoot := flag.String("unit-root", ".", "framework root for unit-test layout checks")
	suites := flag.String("suites", "", "comma-separated product suite YAML files")
	docs := flag.String("docs", "core", "framework source directory for documentation checks")
	flag.Parse()

	issues := append(checkSteps(*steps), checkFeatures(*features)...)
	issues = append(issues, checkScripts(*scripts)...)
	issues = append(issues, checkArchitecture(*root)...)
	issues = append(issues, checkUnitTestLayout(*unitRoot)...)
	issues = append(issues, checkSuiteYAMLs(*suites)...)
	issues = append(issues, checkCleanupRegistration(*steps)...)
	issues = append(issues, checkDependencies(*steps)...)
	issues = append(issues, checkDocumentation(*docs)...)
	sort.Strings(issues)
	for _, issue := range issues {
		fmt.Fprintln(os.Stderr, issue)
	}
	if len(issues) > 0 {
		os.Exit(1)
	}
}

func checkSteps(root string) []string {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	var issues []string
	patterns := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fileSet := token.NewFileSet()
		file, parseErr := parser.ParseFile(fileSet, path, nil, 0)
		if parseErr != nil {
			issues = append(issues, fmt.Sprintf("%s: parse error: %v", path, parseErr))
			return nil
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.CallExpr:
				if selector, ok := n.Fun.(*ast.SelectorExpr); ok {
					if selector.Sel.Name == "Sleep" && isPackageSelector(selector.X, "time") {
						issues = append(issues, fmt.Sprintf("%s:%d: fixed sleeps are prohibited; use bounded polling", path, fileSet.Position(n.Pos()).Line))
					}
					if selector.Sel.Name == "NewRequest" || selector.Sel.Name == "NewRequestWithContext" ||
						selector.Sel.Name == "Get" || selector.Sel.Name == "Post" || selector.Sel.Name == "Head" {
						if isPackageSelector(selector.X, "http") {
							issues = append(issues, fmt.Sprintf("%s:%d: direct HTTP construction bypasses the suite funnel", path, fileSet.Position(n.Pos()).Line))
						}
					}
					if selector.Sel.Name == "Step" && len(n.Args) > 0 {
						if value, ok := n.Args[0].(*ast.BasicLit); ok && value.Kind == token.STRING {
							pattern := strings.Trim(value.Value, "`")
							if previous, exists := patterns[pattern]; exists {
								issues = append(issues, fmt.Sprintf("%s:%d: duplicate step pattern %q (first at %s)", path, fileSet.Position(n.Pos()).Line, pattern, previous))
							} else {
								patterns[pattern] = fmt.Sprintf("%s:%d", path, fileSet.Position(n.Pos()).Line)
							}
						}
					}
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		issues = append(issues, fmt.Sprintf("%s: %v", root, err))
	}
	return issues
}

func checkArchitecture(root string) []string {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	var issues []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		normalized := filepath.ToSlash(path)
		fileSet := token.NewFileSet()
		file, parseErr := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			issues = append(issues, fmt.Sprintf("%s: parse error: %v", path, parseErr))
			return nil
		}
		for _, spec := range file.Imports {
			importPath := strings.Trim(spec.Path.Value, `"`)
			switch {
			case hasPathSegment(normalized, "core") && strings.Contains(importPath, "/suites/"):
				issues = append(issues, fmt.Sprintf("%s:%d: core package must not import suite package %q", path, fileSet.Position(spec.Pos()).Line, importPath))
			case strings.Contains(normalized, "/steps/common/") || strings.HasPrefix(normalized, "steps/common/"):
				if strings.Contains(importPath, "/steps/platformgateway") {
					issues = append(issues, fmt.Sprintf("%s:%d: common step package must not import product step package %q", path, fileSet.Position(spec.Pos()).Line, importPath))
				}
			}
		}
		return nil
	})
	if err != nil {
		issues = append(issues, fmt.Sprintf("%s: %v", root, err))
	}
	return issues
}

func hasPathSegment(path, segment string) bool {
	return path == segment || strings.HasPrefix(path, segment+"/") || strings.Contains(path, "/"+segment+"/")
}

func checkUnitTestLayout(root string) []string {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	var issues []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			return readErr
		}
		var unitTests []string
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, "_test.go") ||
				strings.HasSuffix(name, "_integration_test.go") ||
				strings.HasSuffix(name, "_docker_test.go") {
				continue
			}
			unitTests = append(unitTests, name)
		}
		if len(unitTests) > 1 {
			sort.Strings(unitTests)
			issues = append(issues, fmt.Sprintf("%s: framework modules must consolidate unit tests into one file; found %s", path, strings.Join(unitTests, ", ")))
		}
		return nil
	})
	if err != nil {
		issues = append(issues, fmt.Sprintf("%s: %v", root, err))
	}
	return issues
}

func checkSuiteYAMLs(paths string) []string {
	if strings.TrimSpace(paths) == "" {
		return nil
	}
	var issues []string
	for _, rawPath := range strings.Split(paths, ",") {
		path := strings.TrimSpace(rawPath)
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			issues = append(issues, fmt.Sprintf("%s: read suite YAML: %v", path, err))
			continue
		}
		var suite struct {
			Blocks []struct {
				Runners []struct {
					Name     string   `yaml:"name"`
					Tags     string   `yaml:"tags"`
					Features []string `yaml:"features"`
				} `yaml:"runners"`
			} `yaml:"blocks"`
		}
		if err := yaml.Unmarshal(data, &suite); err != nil {
			issues = append(issues, fmt.Sprintf("%s: parse suite YAML: %v", path, err))
			continue
		}
		for _, block := range suite.Blocks {
			for _, runner := range block.Runners {
				if !hasFrameworkMarker(runner.Tags) && !hasFrameworkMarker(runner.Name) {
					continue
				}
				issues = append(issues, fmt.Sprintf("%s: framework-only runner %q must not be registered in a product suite", path, runner.Name))
			}
		}
	}
	return issues
}

func hasFrameworkMarker(value string) bool {
	value = strings.ToLower(value)
	for _, marker := range []string{"@framework", "@infra", "@migration", "framework-", "framework_"} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func checkCleanupRegistration(root string) []string {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	var issues []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fileSet := token.NewFileSet()
		file, parseErr := parser.ParseFile(fileSet, path, nil, 0)
		if parseErr != nil {
			issues = append(issues, fmt.Sprintf("%s: parse error: %v", path, parseErr))
			return nil
		}
		ast.Inspect(file, func(node ast.Node) bool {
			fn, ok := node.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				return true
			}
			if _, recognized := cleanupCreationMethods[fn.Name.Name]; !recognized {
				return true
			}
			registered := false
			delegates := false
			ast.Inspect(fn.Body, func(inner ast.Node) bool {
				call, ok := inner.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch callee := call.Fun.(type) {
				case *ast.SelectorExpr:
					if callee.Sel.Name == "Register" || callee.Sel.Name == "RegisterScoped" || callee.Sel.Name == "RegisterRunner" {
						registered = true
					}
					if _, ok := cleanupCreationMethods[callee.Sel.Name]; ok {
						delegates = true
					}
				case *ast.Ident:
					if _, ok := cleanupCreationMethods[callee.Name]; ok {
						delegates = true
					}
				}
				return true
			})
			if !registered && !delegates {
				issues = append(issues, fmt.Sprintf("%s:%d: resource-creation method %q must register cleanup or delegate to a registered creation method", path, fileSet.Position(fn.Pos()).Line, fn.Name.Name))
			}
			return false
		})
		return nil
	})
	if err != nil {
		issues = append(issues, fmt.Sprintf("%s: %v", root, err))
	}
	return issues
}

var approvedStepImports = map[string]struct{}{
	"github.com/cucumber/godog":           {},
	"github.com/stretchr/testify/assert":  {},
	"github.com/stretchr/testify/require": {},
	"gopkg.in/yaml.v3":                    {},
}

func checkDependencies(root string) []string {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	var issues []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fileSet := token.NewFileSet()
		file, parseErr := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			issues = append(issues, fmt.Sprintf("%s: parse error: %v", path, parseErr))
			return nil
		}
		for _, spec := range file.Imports {
			importPath := strings.Trim(spec.Path.Value, `"`)
			if !isApprovedStepImport(importPath) {
				issues = append(issues, fmt.Sprintf("%s:%d: import %q is not approved for step definitions", path, fileSet.Position(spec.Pos()).Line, importPath))
			}
		}
		return nil
	})
	if err != nil {
		issues = append(issues, fmt.Sprintf("%s: %v", root, err))
	}
	return issues
}

func isApprovedStepImport(importPath string) bool {
	if _, ok := approvedStepImports[importPath]; ok {
		return true
	}
	if isStandardLibraryImport(importPath) {
		return true
	}
	return strings.HasPrefix(importPath, "github.com/wso2/api-platform/tests/framework/")
}

func isStandardLibraryImport(importPath string) bool {
	return !strings.Contains(importPath, ".")
}

func checkDocumentation(root string) []string {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	var issues []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			return readErr
		}
		hasGo := false
		hasDoc := false
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if name == "doc.go" {
				hasDoc = true
			}
			if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
				hasGo = true
			}
		}
		if !hasGo {
			return nil
		}
		if !hasDoc {
			issues = append(issues, fmt.Sprintf("%s: framework package must contain doc.go", path))
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			filePath := filepath.Join(path, entry.Name())
			fileSet := token.NewFileSet()
			file, parseErr := parser.ParseFile(fileSet, filePath, nil, 0)
			if parseErr != nil {
				issues = append(issues, fmt.Sprintf("%s: parse error: %v", filePath, parseErr))
				continue
			}
			for _, declaration := range file.Decls {
				switch declaration := declaration.(type) {
				case *ast.FuncDecl:
					if declaration.Name.IsExported() && declaration.Doc == nil {
						issues = append(issues, fmt.Sprintf("%s:%d: exported identifier %q must have a documentation comment", filePath, fileSet.Position(declaration.Pos()).Line, declaration.Name.Name))
					}
				case *ast.GenDecl:
					for _, specification := range declaration.Specs {
						typeSpec, ok := specification.(*ast.TypeSpec)
						if ok && typeSpec.Name.IsExported() && typeSpec.Doc == nil && declaration.Doc == nil {
							issues = append(issues, fmt.Sprintf("%s:%d: exported identifier %q must have a documentation comment", filePath, fileSet.Position(typeSpec.Pos()).Line, typeSpec.Name.Name))
						}
						valueSpec, ok := specification.(*ast.ValueSpec)
						if !ok || valueSpec.Doc != nil || declaration.Doc != nil {
							continue
						}
						for _, name := range valueSpec.Names {
							if name.IsExported() {
								issues = append(issues, fmt.Sprintf("%s:%d: exported identifier %q must have a documentation comment", filePath, fileSet.Position(name.Pos()).Line, name.Name))
							}
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		issues = append(issues, fmt.Sprintf("%s: %v", root, err))
	}
	return issues
}

func isPackageSelector(expr ast.Expr, packageName string) bool {
	selector, ok := expr.(*ast.Ident)
	return ok && selector.Name == packageName
}

func checkFeatures(root string) []string {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	var issues []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".feature") {
			return nil
		}
		file, openErr := os.Open(path)
		if openErr != nil {
			return openErr
		}
		defer file.Close()
		line := 0
		metadataLine := false
		inDocString := false
		docStartLine := 0
		var docLines []string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line++
			text := scanner.Text()
			trimmed := strings.TrimSpace(text)
			if trimmed == `"""` {
				if inDocString {
					if isInlineYAML(docLines) {
						issues = append(issues, fmt.Sprintf("%s:%d: inline YAML configuration is prohibited; use a reusable YAML resource", path, docStartLine))
					}
					inDocString = false
					docLines = nil
				} else {
					inDocString = true
					docStartLine = line
					docLines = nil
				}
				metadataLine = false
				continue
			}
			if inDocString {
				docLines = append(docLines, text)
				continue
			}
			if strings.Contains(strings.ToLower(text), "metadata") {
				metadataLine = true
			}
			if metadataLine {
				for _, rule := range uniqueFieldRegistry {
					match := rule.pattern.FindStringSubmatch(text)
					if len(match) != 2 || isGeneratedValue(match[1]) {
						continue
					}
					issues = append(issues, fmt.Sprintf("%s:%d: literal %s value %q; use a generated or stored context value", path, line, rule.name, match[1]))
				}
			}
			if metadataLine && strings.Contains(text, "}") {
				metadataLine = false
			}
		}
		if inDocString && isInlineYAML(docLines) {
			issues = append(issues, fmt.Sprintf("%s:%d: unterminated doc string contains inline YAML configuration", path, docStartLine))
		}
		if scanErr := scanner.Err(); scanErr != nil {
			return scanErr
		}
		return nil
	})
	if err != nil {
		issues = append(issues, fmt.Sprintf("%s: %v", root, err))
	}
	return issues
}

func checkScripts(root string) []string {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	var issues []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".sh") || filepath.Base(path) == "vmwatch.sh" {
			return nil
		}
		file, openErr := os.Open(path)
		if openErr != nil {
			return openErr
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		line := 0
		for scanner.Scan() {
			line++
			text := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(text, "#") {
				continue
			}
			if shellSleep.MatchString(text) {
				issues = append(issues, fmt.Sprintf("%s:%d: shell sleeps are prohibited; use a bounded polling command", path, line))
			}
		}
		if scanErr := scanner.Err(); scanErr != nil {
			return scanErr
		}
		return nil
	})
	if err != nil {
		issues = append(issues, fmt.Sprintf("%s: %v", root, err))
	}
	return issues
}

func isGeneratedValue(value string) bool {
	return strings.Contains(value, "${UNIQUE:") ||
		strings.Contains(value, "${VALUE:") ||
		strings.Contains(value, "${CTX:") ||
		strings.Contains(value, "{{")
}

func isInlineYAML(lines []string) bool {
	var content strings.Builder
	for _, line := range lines {
		content.WriteString(line)
		content.WriteByte('\n')
	}
	value := strings.TrimSpace(content.String())
	if value == "" || json.Valid([]byte(value)) {
		return false
	}
	for _, line := range strings.Split(value, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "---") {
			continue
		}
		return yamlField.MatchString(trimmed) || strings.HasPrefix(trimmed, "- ")
	}
	return false
}
