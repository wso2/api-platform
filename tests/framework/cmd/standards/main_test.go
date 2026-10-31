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

// writeSheetFixture lays out a framework-shaped tree with one manifest and the given
// UI and IT feature files, returning the manifest dir and the feature roots flag.
func writeSheetFixture(t *testing.T, manifest, uiFeature, itFeature string) (string, string) {
	t.Helper()
	root := t.TempDir()
	ui := filepath.Join(root, "suites", "ui", "features")
	it := filepath.Join(root, "suites", "it", "features")
	cov := filepath.Join(root, "coverage")
	for _, dir := range []string{ui, it, cov} {
		require.NoError(t, os.MkdirAll(dir, 0o755))
	}
	manifest = strings.ReplaceAll(manifest, "ROOT", root)
	require.NoError(t, os.WriteFile(filepath.Join(cov, "AIW-LLM.yaml"), []byte(manifest), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(ui, "create.feature"), []byte(uiFeature), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(it, "create.feature"), []byte(itFeature), 0o600))
	return cov, ui + "," + it
}

const sheetManifest = `sheet: AIW-LLM
title: LLM
scenarios:
  - id: AIW-PRV-001
    title: Create
    ui:
      status: automated
      refs:
        - feature: ROOT/suites/ui/features/create.feature
          scenario: "Journey: create a provider"
    it:
      status: automated
      refs:
        - feature: ROOT/suites/it/features/create.feature
          scenario: Create a provider
  - id: AIW-PRV-002
    title: Validation
    ui:
      status: automated
      refs:
        - feature: ROOT/suites/ui/features/create.feature
          scenario: "Journey: create a provider"
    it:
      status: known-issue
      issue: https://github.com/wso2/api-platform/issues/1
      refs:
        - feature: ROOT/suites/it/features/create.feature
          scenario: Reject an empty form
  - id: AIW-PRV-003
    title: Later
    ui:
      status: planned
    it:
      status: exempt
      reason: UI only
`

const sheetUIFeature = `@AIW-LLM
Feature: Providers
  Background:
    Given the user is signed in

  @AIW-PRV-001 @AIW-PRV-002
  Scenario: Journey: create a provider
    * sheet check "AIW-PRV-002"
    When something
    * sheet check "AIW-PRV-001"
    Then something else

  Scenario: Unrelated
    Given nothing
`

const sheetITFeature = `@AIW-LLM
Feature: Providers API
  @AIW-PRV-001 @type:smoke
  Scenario: Create a provider
    Given something

  @AIW-PRV-002 @known-issue
  Scenario Outline: Reject an empty form
    Given <x>

    @AIW-NOT-TAGGEDHERE
    Examples:
      | x |
      | y |
`

func TestCheckSheetsAcceptsConsistentManifest(t *testing.T) {
	cov, roots := writeSheetFixture(t, sheetManifest, sheetUIFeature, sheetITFeature)
	require.Empty(t, checkSheets(cov, roots, ""))
	require.Empty(t, checkSheets("", roots, ""))
}

func TestCheckSheetsReportsDrift(t *testing.T) {
	cases := []struct {
		name, manifest, ui, it, want string
	}{
		{"missing manifest dir", "", sheetUIFeature, sheetITFeature, "no manifests found"},
		{"unknown tag", sheetManifest, strings.Replace(sheetUIFeature, "Scenario: Unrelated", "@AIW-PRV-099\n  Scenario: Unrelated", 1), sheetITFeature, "which no sheet manifest lists"},
		{"tag not referenced", strings.Replace(sheetManifest, "    it:\n      status: exempt\n      reason: UI only\n", "    it:\n      status: exempt\n      reason: UI only\n", 1), sheetUIFeature, strings.Replace(sheetITFeature, "@AIW-PRV-001 @type:smoke", "@AIW-PRV-001 @AIW-PRV-003", 1), "do not name it"},
		{"missing check", sheetManifest, strings.Replace(sheetUIFeature, "    * sheet check \"AIW-PRV-001\"\n", "", 1), sheetITFeature, `has no ` + "`sheet check" + ` "AIW-PRV-001"`},
		{"untagged check", sheetManifest, strings.Replace(sheetUIFeature, "    Given nothing", `    * sheet check "AIW-PRV-001"`, 1), sheetITFeature, "carries no sheet ID tags"},
		{"check not tagged", sheetManifest, strings.Replace(sheetUIFeature, `    Then something else`, `    * sheet check "AIW-PRV-003"`, 1), sheetITFeature, "is not tagged @AIW-PRV-003"},
		{"duplicate check", sheetManifest, strings.Replace(sheetUIFeature, `    Then something else`, `    * sheet check "AIW-PRV-001"`, 1), sheetITFeature, "more than once"},
		{"check in background", sheetManifest, strings.Replace(sheetUIFeature, "    Given the user is signed in", `    * sheet check "AIW-PRV-001"`, 1), sheetITFeature, "not in a Background"},
		{"feature-level id tag", sheetManifest, strings.Replace(sheetUIFeature, "@AIW-LLM", "@AIW-PRV-001", 1), sheetITFeature, "not on the feature or rule"},
		{"ref to missing scenario", strings.Replace(sheetManifest, "scenario: Create a provider", "scenario: Gone", 1), sheetUIFeature, sheetITFeature, "no scenario with that name"},
		{"ref to missing feature", strings.Replace(sheetManifest, "it/features/create.feature\n          scenario: Create", "it/features/gone.feature\n          scenario: Create", 1), sheetUIFeature, sheetITFeature, "feature file not found"},
		{"ref in the wrong layer", strings.Replace(sheetManifest, "ROOT/suites/it/features/create.feature\n          scenario: Create a provider", "ROOT/suites/ui/features/create.feature\n          scenario: Create a provider", 1), sheetUIFeature, sheetITFeature, "must point into suites/it"},
		{"ref scenario lacks tag", sheetManifest, sheetUIFeature, strings.Replace(sheetITFeature, "@AIW-PRV-001 @type:smoke", "@type:smoke", 1), "is not tagged @AIW-PRV-001"},
		{"known-issue without tag", sheetManifest, sheetUIFeature, strings.Replace(sheetITFeature, "@AIW-PRV-002 @known-issue", "@AIW-PRV-002", 1), "needs at least one referenced scenario tagged @known-issue"},
		{"automated but known-issue", sheetManifest, sheetUIFeature, strings.Replace(sheetITFeature, "@AIW-PRV-001 @type:smoke", "@AIW-PRV-001 @known-issue", 1), "its status must be known-issue"},
		{"journey without checks", sheetManifest, strings.NewReplacer("    * sheet check \"AIW-PRV-002\"\n", "", "    * sheet check \"AIW-PRV-001\"\n", "").Replace(sheetUIFeature), sheetITFeature, "covers 2 sheet IDs but has no `sheet check` steps"},
		{"feature without its sheet tag", sheetManifest, sheetUIFeature, strings.Replace(sheetITFeature, "@AIW-LLM\n", "", 1), "must be tagged @AIW-LLM"},
		{"check inside a doc string", sheetManifest, strings.Replace(sheetUIFeature, "    * sheet check \"AIW-PRV-001\"\n", "    Given a body:\n      \"\"\"\n      * sheet check \"AIW-PRV-001\"\n      \"\"\"\n", 1), sheetITFeature, `has no ` + "`sheet check" + ` "AIW-PRV-001"`},
		{"known-issue tag above Examples", sheetManifest, sheetUIFeature, strings.NewReplacer("@AIW-PRV-002 @known-issue", "@AIW-PRV-002", "@AIW-NOT-TAGGEDHERE", "@known-issue").Replace(sheetITFeature), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cov, roots := writeSheetFixture(t, tc.manifest, tc.ui, tc.it)
			if tc.manifest == "" {
				require.NoError(t, os.Remove(filepath.Join(cov, "AIW-LLM.yaml")))
			}
			issues := strings.Join(checkSheets(cov, roots, ""), "\n")
			if tc.want == "" {
				require.Empty(t, issues)
				return
			}
			require.Contains(t, issues, tc.want)
		})
	}
}

func TestCheckSheetsMatchesAbsoluteRootsToRelativeRefs(t *testing.T) {
	cov, roots := writeSheetFixture(t, sheetManifest, sheetUIFeature, sheetITFeature)
	// The fixture's refs are absolute; relative roots must name the same files.
	t.Chdir(filepath.Dir(cov))
	relative := strings.NewReplacer(filepath.Dir(cov)+string(filepath.Separator), "").Replace(roots)
	require.Empty(t, checkSheets(cov, relative, ""))
}

func TestCheckSheetsRequiresEnrolledFeatures(t *testing.T) {
	cov, roots := writeSheetFixture(t, sheetManifest, sheetUIFeature, sheetITFeature)
	root := filepath.Dir(cov)
	suite := func(layer, body string) string {
		path := filepath.Join(root, "suites", layer, layer+"-suite.yaml")
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
		return path
	}
	runs := "blocks:\n  - name: b\n    runners:\n      - name: r\n        features:\n          - features/create.feature\n"
	suites := suite("ui", runs) + "," + suite("it", runs)
	require.Empty(t, checkSheets(cov, roots, suites))

	suites = suite("ui", runs) + "," + suite("it", "blocks:\n  - name: b\n    runners: []\n")
	require.Contains(t, strings.Join(checkSheets(cov, roots, suites), "\n"), "no runner in the checked suites runs this feature")
	require.Contains(t, strings.Join(checkSheets(cov, roots, filepath.Join(root, "missing.yaml")), "\n"), "read suite YAML")
}

func TestCheckSheetsAcceptsKnownIssueAlongsidePassingRefs(t *testing.T) {
	// A known-issue layer may also reference passing scenarios, e.g. a journey that covers
	// the ID's working part next to a known-issue scenario for its defect.
	manifest := strings.Replace(sheetManifest, `          scenario: Reject an empty form
`, `          scenario: Reject an empty form
        - feature: ROOT/suites/it/features/create.feature
          scenario: Create a provider
`, 1)
	it := strings.Replace(sheetITFeature, "@AIW-PRV-001 @type:smoke", "@AIW-PRV-001 @AIW-PRV-002 @type:smoke", 1)
	cov, roots := writeSheetFixture(t, manifest, sheetUIFeature, it)
	require.Empty(t, checkSheets(cov, roots, ""))
}

func TestParseFeatureFileHandlesTagsRulesAndComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.feature")
	body := "@f1\nFeature: x\n  @r1\n  Rule: r\n    @s1 @s2 # trailing comment\n    Example: one\n      * sheet check \"AIW-PRV-001\"\n"
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	f, err := parseFeatureFile(path)
	require.NoError(t, err)
	require.Equal(t, []string{"f1", "r1"}, f.featureTags)
	require.Len(t, f.scenarios, 1)
	require.Equal(t, []string{"s1", "s2"}, f.scenarios[0].tags)
	require.Equal(t, []string{"AIW-PRV-001"}, f.scenarios[0].checks)

	docString := filepath.Join(t.TempDir(), "doc.feature")
	require.NoError(t, os.WriteFile(docString, []byte("Feature: x\n  @s\n  Scenario: one\n    Given a body:\n      ```\n      Scenario: not a scenario\n      @not-a-tag\n      ```\n    * sheet check \"AIW-PRV-001\"\n"), 0o600))
	f, err = parseFeatureFile(docString)
	require.NoError(t, err)
	require.Len(t, f.scenarios, 1, "a doc string's content is not Gherkin")
	require.Equal(t, []string{"s"}, f.scenarios[0].tags)
	require.Equal(t, []string{"AIW-PRV-001"}, f.scenarios[0].checks)
	_, err = parseFeatureFile(filepath.Join(t.TempDir(), "missing.feature"))
	require.Error(t, err)
	require.Equal(t, "", string(layerOf("tools/x.feature")))
}
