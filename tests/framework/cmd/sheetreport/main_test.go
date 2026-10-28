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
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/sheet"
)

const manifest = `sheet: AIW-LLM
title: LLM
scenarios:
  - id: AIW-PRV-001
    title: Create
    priority: P1
    ui:
      status: automated
      refs:
        - feature: suites/ui/features/a.feature
          scenario: "Journey: create"
    it:
      status: automated
      refs:
        - feature: suites/it/features/a.feature
          scenario: create
  - id: AIW-PRV-002
    title: Validate
    sheetUpdate: Add is disabled until required fields are filled.
    ui:
      status: automated
      refs:
        - feature: suites/ui/features/a.feature
          scenario: "Journey: create"
    it:
      status: exempt
      reason: UI only
  - id: AIW-PRV-003
    title: Later
    ui:
      status: planned
    it:
      status: planned
`

func fixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	cov := filepath.Join(root, "coverage")
	require.NoError(t, os.MkdirAll(cov, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cov, "AIW-LLM.yaml"), []byte(manifest), 0o600))
	uiSink, err := sheet.NewSink(filepath.Join(root, "ui"))
	require.NoError(t, err)
	require.NoError(t, uiSink.Write(sheet.Report{Block: "ai-core", Runner: "r1", Results: []sheet.Result{
		{ID: "AIW-PRV-002", Status: sheet.Passed, Scenario: "Journey: create"},
		{ID: "AIW-PRV-001", Status: sheet.Failed, Scenario: "Journey: create", Step: "the user sees | it", Error: "boom"},
	}}))
	require.NoError(t, uiSink.Write(sheet.Report{Block: "ai-core", Runner: "r2", Results: []sheet.Result{
		{ID: "AIW-PRV-001", Status: sheet.Passed, Scenario: "Other"},
	}}))
	return cov, uiSink.Root(), filepath.Join(root, "it-missing")
}

func TestWorstPicksFailureThenNotReached(t *testing.T) {
	_, ok := worst(nil)
	require.False(t, ok)
	got, ok := worst([]sheet.Result{{Status: sheet.Passed}, {Status: sheet.NotReached, Scenario: "n"}, {Status: sheet.Passed}})
	require.True(t, ok)
	require.Equal(t, sheet.NotReached, got.Status)
	got, _ = worst([]sheet.Result{{Status: sheet.NotReached}, {Status: sheet.Failed, Scenario: "f"}})
	require.Equal(t, "f", got.Scenario)
}

func TestRunWritesCSVRows(t *testing.T) {
	cov, ui, itDir := fixture(t)
	out := filepath.Join(t.TempDir(), "report.csv")
	require.NoError(t, run(cov, "ui="+ui+",it="+itDir, "latest", "csv", out))
	data, err := os.ReadFile(out)
	require.NoError(t, err)
	records, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 1+6) // header + 3 IDs x 2 layers
	byKey := map[string][]string{}
	for _, rec := range records[1:] {
		byKey[rec[1]+"/"+rec[4]] = rec
	}
	ui1 := byKey["AIW-PRV-001/ui"]
	require.Equal(t, []string{"failed", "Journey: create; Other", "the user sees | it", "boom"}, ui1[7:11])
	require.Equal(t, "latest", ui1[5])
	require.Equal(t, "suites/ui/features/a.feature :: Journey: create", ui1[11])
	require.Equal(t, "not-run", byKey["AIW-PRV-001/it"][7], "an IT layer with no reports is not-run")
	require.Equal(t, "exempt", byKey["AIW-PRV-002/it"][6])
	require.Equal(t, "passed", byKey["AIW-PRV-002/ui"][7])
	require.Equal(t, "Add is disabled until required fields are filled.", byKey["AIW-PRV-002/ui"][13])
	require.Len(t, records[0], 14)
}

func TestRunWritesMarkdownAndOmitsUnrunLayers(t *testing.T) {
	cov, ui, _ := fixture(t)
	out := filepath.Join(t.TempDir(), "summary.md")
	require.NoError(t, run(cov, "ui="+ui, "", "markdown", out))
	data, err := os.ReadFile(out)
	require.NoError(t, err)
	md := string(data)
	require.Contains(t, md, "### Scenario sheet results\n")
	require.Contains(t, md, "Run: 1 passed, 1 failed, 0 not reached, 0 not run.", "planned IDs are not counted as not run")
	require.Contains(t, md, `| AIW-PRV-001 | ui | automated | failed | the user sees \| it | boom |`)
	require.NotContains(t, md, "AIW-PRV-003", "planned IDs with nothing run are left out")
	require.NotContains(t, md, "| AIW-PRV-001 | it |", "a layer outside this run has no rows")
}

func TestRunRejectsBadInput(t *testing.T) {
	cov, ui, _ := fixture(t)
	require.Error(t, run(t.TempDir(), "", "", "csv", ""), "no manifests")
	require.Error(t, run(cov, "web="+ui, "", "csv", ""), "unknown layer")
	require.Error(t, run(cov, "ui", "", "csv", ""), "missing dir")
	require.Error(t, run(cov, "ui="+ui, "", "xml", ""), "unknown format")
	require.Error(t, run(cov, "ui="+ui, "", "csv", filepath.Join(t.TempDir(), "no", "such", "file.csv")))
	require.NoError(t, os.WriteFile(filepath.Join(ui, "bad.json"), []byte("{"), 0o600))
	require.Error(t, run(cov, "ui="+ui, "", "csv", ""), "malformed report")
}

func TestCellEscapesAndTruncates(t *testing.T) {
	require.Equal(t, `a \| b c`, cell("a | b\nc"))
	long := strings.Repeat("x", 200)
	require.Len(t, cell(long), 160)
}
