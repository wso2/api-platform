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

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckSteps(t *testing.T) {
	root := t.TempDir()
	good := `package steps
import "github.com/cucumber/godog"
func bind(sc *godog.ScenarioContext) { sc.Step("^one$", func() {}) }
`
	require.NoError(t, os.WriteFile(filepath.Join(root, "good.go"), []byte(good), 0o600))
	require.Empty(t, checkSteps(root))

	bad := `package steps
import ("net/http"; "time")
func bind(sc interface{}) { _, _ = http.NewRequest("", "", nil); _, _ = http.Get(""); time.Sleep(0) }
`
	require.NoError(t, os.WriteFile(filepath.Join(root, "bad.go"), []byte(bad), 0o600))
	issues := checkSteps(root)
	require.Len(t, issues, 3)
	joined := strings.Join(issues, "\n")
	require.Contains(t, joined, "direct HTTP construction")
	require.Contains(t, joined, "fixed sleeps")

	aliased := `package steps
import (clock "time"; web "net/http")
func bind(sc interface{}) { _, _ = web.NewRequest("", "", nil); _, _ = web.Get(""); clock.Sleep(0) }
`
	require.NoError(t, os.WriteFile(filepath.Join(root, "aliased.go"), []byte(aliased), 0o600))
	issues = checkSteps(root)
	require.Len(t, issues, 6)

	dotImported := `package steps
import (. "net/http"; . "time")
func bind(sc interface{}) { _, _ = NewRequest("", "", nil); _, _ = Get(""); Sleep(0) }
`
	require.NoError(t, os.WriteFile(filepath.Join(root, "dot_imported.go"), []byte(dotImported), 0o600))
	issues = checkSteps(root)
	require.Len(t, issues, 9)
}

func TestChecksSkipEmptyRoots(t *testing.T) {
	require.Empty(t, checkSteps(""))
	require.Empty(t, checkFeatures(""))
	require.Empty(t, checkScripts(""))
	require.Empty(t, checkArchitecture(""))
	require.Empty(t, checkUnitTestLayout(""))
	require.Empty(t, checkDependencies("", nil))
	require.Empty(t, checkDocumentation(""))
}

func TestCheckStepsFindsDuplicateBindings(t *testing.T) {
	root := t.TempDir()
	source := `package steps
func bind(sc interface{}) { sc.Step("^same$", nil); sc.Step(` + "`^same$`" + `, nil) }
`
	require.NoError(t, os.WriteFile(filepath.Join(root, "duplicate.go"), []byte(source), 0o600))
	require.Len(t, checkSteps(root), 1)
	require.Contains(t, checkSteps(root)[0], "duplicate step pattern")
}

func TestSplitRootsDropsBlankEntries(t *testing.T) {
	require.Empty(t, splitRoots(""))
	require.Equal(t, []string{"a", "b"}, splitRoots(" a, ,b,"))
}

func TestCheckFeaturesFindsLiteralNames(t *testing.T) {
	root := t.TempDir()
	feature := `Feature: names
  Scenario: unsafe
    Given body:
      "metadata": {
        "name": "shared-api"
        "context": "/shared-api"
      }
  Scenario: safe
    Given body:
      "metadata": {
        "name": "${UNIQUE:api}"
        "context": "${VALUE:apiContext}"
      }
`
	require.NoError(t, os.WriteFile(filepath.Join(root, "names.feature"), []byte(feature), 0o600))
	issues := checkFeatures(root)
	require.Len(t, issues, 2)
	joined := strings.Join(issues, "\n")
	require.Contains(t, joined, "metadata.name")
	require.Contains(t, joined, "metadata.context")
}

func TestIsGeneratedValue(t *testing.T) {
	for _, value := range []string{"${UNIQUE:name}", "${VALUE:name}", "${CTX:name}", "{{name}}"} {
		require.True(t, isGeneratedValue(value), value)
	}
	require.False(t, isGeneratedValue("shared-name"))
}

func TestCheckFeaturesRejectsInlineYAMLButAllowsJSON(t *testing.T) {
	root := t.TempDir()
	feature := `Feature: payloads
  Scenario: yaml configuration
    Given an API definition:
      """
      kind: RestApi
      apiVersion: v1
      metadata:
        name: shared-api
      """
  Scenario: JSON request body
    When I send a request with body:
      """
      {"name":"request"}
      """
`
	path := filepath.Join(root, "payloads.feature")
	require.NoError(t, os.WriteFile(path, []byte(feature), 0o600))

	issues := checkFeatures(root)
	require.Len(t, issues, 1)
	require.Contains(t, issues[0], "inline YAML configuration")
	require.Contains(t, issues[0], ":4:")
}

func TestIsInlineYAMLHandlesEmptyAndUnterminatedDocuments(t *testing.T) {
	require.False(t, isInlineYAML(nil))
	require.False(t, isInlineYAML([]string{"plain text"}))
	require.False(t, isInlineYAML([]string{`{"name":"value"}`}))
	require.True(t, isInlineYAML([]string{"kind: RestApi"}))
	require.True(t, isInlineYAML([]string{"- kind: RestApi"}))
}

func TestCheckScriptsRejectsShellSleep(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "bad.sh"), []byte("#!/bin/sh\nsleep 2\n"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "comment.sh"), []byte("#!/bin/sh\n# sleep 2\n"), 0o700))

	issues := checkScripts(root)
	require.Len(t, issues, 1)
	require.Contains(t, issues[0], "shell sleeps are prohibited")
}

func TestCheckScriptsAllowsOperationalVMWatcher(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "vmwatch.sh"), []byte("#!/bin/sh\nsleep 2\n"), 0o700))
	require.Empty(t, checkScripts(root))
}

func TestCheckArchitectureRejectsForbiddenImports(t *testing.T) {
	root := t.TempDir()
	core := filepath.Join(root, "core", "runtime")
	common := filepath.Join(root, "suites", "it", "steps", "common")
	require.NoError(t, os.MkdirAll(core, 0o700))
	require.NoError(t, os.MkdirAll(common, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(core, "bad.go"), []byte(`package runtime
import _ "example/suites/it/steps"
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(common, "bad.go"), []byte(`package common
import _ "example/suites/it/steps/platformgateway"
`), 0o600))

	issues := checkArchitecture(root)
	require.Len(t, issues, 2)
	joined := strings.Join(issues, "\n")
	require.Contains(t, joined, "core package must not import suite package")
	require.Contains(t, joined, "common step package must not import product step package")
}

func TestCheckUnitTestLayoutAllowsIntegrationTestsAndRejectsMultipleUnitFiles(t *testing.T) {
	root := t.TempDir()
	module := filepath.Join(root, "core", "example")
	require.NoError(t, os.MkdirAll(module, 0o700))
	for _, name := range []string{"alpha_test.go", "beta_test.go", "docker_integration_test.go"} {
		require.NoError(t, os.WriteFile(filepath.Join(module, name), []byte("package example\n"), 0o600))
	}

	issues := checkUnitTestLayout(root)
	require.Len(t, issues, 1)
	require.Contains(t, issues[0], "alpha_test.go, beta_test.go")
}

func TestCheckSuiteYAMLsRejectsFrameworkOnlyRunner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "suite.yaml")
	content := "blocks:\n  - runners:\n      - name: framework-cleanup\n        tags: '@framework'\n        features: [features/framework.feature]\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	issues := checkSuiteYAMLs(path)
	require.Len(t, issues, 1)
	require.Contains(t, issues[0], "framework-only runner")
}

func TestCheckSuiteYAMLsFollowsBlockFiles(t *testing.T) {
	const frameworkRunner = "blocks:\n  - runners:\n      - name: framework-cleanup\n        tags: '@framework'\n        features: [features/framework.feature]\n"
	const productRunner = "blocks:\n  - runners:\n      - name: api-deploy\n        features: [features/api_deploy.feature]\n"

	// writeSuite writes a suite naming the given block files, and those files, into a new directory.
	writeSuite := func(t *testing.T, blockFiles []string, files map[string]string) string {
		t.Helper()
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "blocks"), 0o700))
		for name, content := range files {
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
		}
		suite := "blockFiles:\n"
		for _, name := range blockFiles {
			suite += "  - " + name + "\n"
		}
		path := filepath.Join(dir, "suite.yaml")
		require.NoError(t, os.WriteFile(path, []byte(suite), 0o600))
		return path
	}

	t.Run("a framework-only runner in a block file is reported with that file", func(t *testing.T) {
		path := writeSuite(t, []string{"blocks/core.yaml"}, map[string]string{"blocks/core.yaml": frameworkRunner})
		issues := checkSuiteYAMLs(path)
		require.Len(t, issues, 1)
		require.Contains(t, issues[0], filepath.Join("blocks", "core.yaml"))
		require.Contains(t, issues[0], "framework-only runner")
	})

	t.Run("clean block files report nothing", func(t *testing.T) {
		path := writeSuite(t, []string{"blocks/a.yaml", "blocks/b.yaml"},
			map[string]string{"blocks/a.yaml": productRunner, "blocks/b.yaml": productRunner})
		require.Empty(t, checkSuiteYAMLs(path))
	})

	t.Run("every block file is checked, not only the first", func(t *testing.T) {
		path := writeSuite(t, []string{"blocks/a.yaml", "blocks/b.yaml"},
			map[string]string{"blocks/a.yaml": productRunner, "blocks/b.yaml": frameworkRunner})
		issues := checkSuiteYAMLs(path)
		require.Len(t, issues, 1)
		require.Contains(t, issues[0], filepath.Join("blocks", "b.yaml"))
	})

	t.Run("a missing block file is reported", func(t *testing.T) {
		path := writeSuite(t, []string{"blocks/missing.yaml"}, nil)
		issues := checkSuiteYAMLs(path)
		require.Len(t, issues, 1)
		require.Contains(t, issues[0], "read suite YAML")
		require.Contains(t, issues[0], filepath.Join("blocks", "missing.yaml"))
	})

	t.Run("a malformed block file is reported", func(t *testing.T) {
		path := writeSuite(t, []string{"blocks/bad.yaml"}, map[string]string{"blocks/bad.yaml": "blocks: [\n"})
		issues := checkSuiteYAMLs(path)
		require.Len(t, issues, 1)
		require.Contains(t, issues[0], "parse suite YAML")
		require.Contains(t, issues[0], filepath.Join("blocks", "bad.yaml"))
	})

	for _, entry := range []string{"/etc/blocks.yaml", "../blocks.yaml", "\"\""} {
		t.Run("a block file path outside the suite directory is reported: "+entry, func(t *testing.T) {
			path := writeSuite(t, []string{entry}, nil)
			issues := checkSuiteYAMLs(path)
			require.Len(t, issues, 1)
			require.Contains(t, issues[0], "must be a path inside the suite file's directory")
		})
	}
}

func TestCheckCleanupRegistrationRequiresRegistrationOrDelegation(t *testing.T) {
	root := t.TempDir()
	source := `package steps
func createAPI() error { return nil }
func createResource() error { return createAPI() }
func createJSONAPI() error { return createAPI() }
func createAPIFromTemplate() error { return createAPI() }
`
	path := filepath.Join(root, "steps.go")
	require.NoError(t, os.WriteFile(path, []byte(source), 0o600))
	issues := checkCleanupRegistration(root)
	require.Len(t, issues, 1)
	require.Contains(t, issues[0], `"createAPI"`)
}

func TestCheckDependenciesRejectsUnapprovedThirdPartyImport(t *testing.T) {
	root := t.TempDir()
	source := `package steps
import _ "example.com/unapproved/parser"
`
	require.NoError(t, os.WriteFile(filepath.Join(root, "steps.go"), []byte(source), 0o600))

	issues := checkDependencies(root, nil)
	require.Len(t, issues, 1)
	require.Contains(t, issues[0], "not approved")
}

func TestCheckDependenciesAcceptsAnImportingProjectsOwnPackages(t *testing.T) {
	root := t.TempDir()
	source := `package steps
import (
	_ "example.com/project/tests/it/steps/common"
	_ "example.com/project/other"
)
`
	require.NoError(t, os.WriteFile(filepath.Join(root, "steps.go"), []byte(source), 0o600))

	issues := checkDependencies(root, []string{"example.com/project/tests/it/"})

	require.Len(t, issues, 1)
	require.Contains(t, issues[0], "example.com/project/other")
}

func TestCheckDocumentationRequiresDocGo(t *testing.T) {
	root := t.TempDir()
	packageDir := filepath.Join(root, "example")
	require.NoError(t, os.MkdirAll(packageDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, "example.go"), []byte(`package example
func Exported() {}
`), 0o600))

	issues := checkDocumentation(root)
	require.Len(t, issues, 2)
	require.Contains(t, issues[0], "must contain doc.go")
	require.Contains(t, strings.Join(issues, "\n"), "documentation comment")
}
