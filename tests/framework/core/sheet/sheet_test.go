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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package sheet

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/cucumber/godog"
)

// runFeature runs one in-memory feature through a real godog suite bound to a fresh
// recorder, so the hook wiring is exercised exactly as the runtime uses it.
func runFeature(t *testing.T, feature string) (*Recorder, map[string]string) {
	t.Helper()
	rec := NewRecorder()
	failedIDs := map[string]string{}
	var mu sync.Mutex
	suite := godog.TestSuite{
		Name: "sheet-unit",
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			rec.Bind(sc)
			sc.Step(`^a step passes$`, func() error { return nil })
			sc.Step(`^a step fails$`, func() error { return errors.New("boom") })
			sc.After(func(ctx context.Context, s *godog.Scenario, err error) (context.Context, error) {
				mu.Lock()
				failedIDs[s.Name] = FailedID(ctx)
				mu.Unlock()
				return ctx, nil
			})
		},
		Options: &godog.Options{
			Format:          "pretty",
			Output:          io.Discard,
			Strict:          true,
			Concurrency:     1,
			NoColors:        true,
			FeatureContents: []godog.Feature{{Name: "unit.feature", Contents: []byte(feature)}},
		},
	}
	suite.Run()
	return rec, failedIDs
}

func byID(results []Result) map[string]Result {
	out := map[string]Result{}
	for _, r := range results {
		out[r.Scenario+"|"+r.ID] = r
	}
	return out
}

func TestIsIDAndIDsFromTags(t *testing.T) {
	valid := []string{"AIW-PRV-001", "AIW-PRX-034", "A1-B2-999"}
	invalid := []string{"", "AIW-PRV-1", "aiw-prv-001", "AIW-LLM", "@AIW-PRV-001", "AIW-PRV-0001", "cp14-01", "AIW--001"}
	for _, v := range valid {
		if !IsID(v) {
			t.Errorf("IsID(%q) = false, want true", v)
		}
	}
	for _, v := range invalid {
		if IsID(v) {
			t.Errorf("IsID(%q) = true, want false", v)
		}
	}
	got := IDsFromTags([]string{"@AIW-LLM", "@AIW-PRV-002", "@known-issue", "@AIW-PRV-001", "@type:smoke"})
	if strings.Join(got, ",") != "AIW-PRV-002,AIW-PRV-001" {
		t.Fatalf("IDsFromTags kept order/filter wrong: %v", got)
	}
	if IDsFromTags(nil) != nil {
		t.Fatal("IDsFromTags(nil) should be nil")
	}
}

func TestScenarioWithoutCheckpointsSharesOneOutcome(t *testing.T) {
	rec, failed := runFeature(t, `Feature: f
  @AIW-PRV-001 @AIW-PRV-002
  Scenario: passes
    Given a step passes

  @AIW-PRV-003
  Scenario: fails
    Given a step passes
    And a step fails

  Scenario: untagged
    Given a step passes
`)
	got := byID(rec.Results())
	if len(got) != 3 {
		t.Fatalf("want 3 results (untagged scenario records nothing), got %v", rec.Results())
	}
	for _, key := range []string{"passes|AIW-PRV-001", "passes|AIW-PRV-002"} {
		if got[key].Status != Passed {
			t.Errorf("%s = %s, want passed", key, got[key].Status)
		}
	}
	f := got["fails|AIW-PRV-003"]
	if f.Status != Failed || f.Step != "a step fails" || f.Error != "boom" || f.Feature != "unit.feature" {
		t.Errorf("fails|AIW-PRV-003 = %+v, want failed at 'a step fails' with boom", f)
	}
	if failed["fails"] != "AIW-PRV-003" || failed["passes"] != "" || failed["untagged"] != "" {
		t.Errorf("FailedID = %v, want only the failing scenario's tagged ID", failed)
	}
}

func TestJourneyAttributesFailureAndMarksLaterIDsNotReached(t *testing.T) {
	rec, failed := runFeature(t, `Feature: f
  @AIW-PRV-001 @AIW-PRV-002 @AIW-PRV-003
  Scenario: journey
    Given a step passes
    * sheet check "AIW-PRV-001"
    And a step passes
    * sheet check "AIW-PRV-002"
    And a step fails
    * sheet check "AIW-PRV-003"
    And a step passes
`)
	got := byID(rec.Results())
	if got["journey|AIW-PRV-001"].Status != Passed {
		t.Errorf("AIW-PRV-001 = %+v, want passed", got["journey|AIW-PRV-001"])
	}
	if r := got["journey|AIW-PRV-002"]; r.Status != Failed || r.Step != "a step fails" || r.Error != "boom" {
		t.Errorf("AIW-PRV-002 = %+v, want failed at 'a step fails'", r)
	}
	if r := got["journey|AIW-PRV-003"]; r.Status != NotReached || !strings.Contains(r.Error, "AIW-PRV-002") {
		t.Errorf("AIW-PRV-003 = %+v, want not-reached naming AIW-PRV-002", r)
	}
	if failed["journey"] != "AIW-PRV-002" {
		t.Errorf("FailedID = %q, want AIW-PRV-002", failed["journey"])
	}
}

func TestHooksDoNotRepeatTheStepError(t *testing.T) {
	var out bytes.Buffer
	suite := godog.TestSuite{
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			NewRecorder().Bind(sc)
			sc.Step(`^a step fails$`, func() error { return errors.New("unique-boom") })
		},
		Options: &godog.Options{
			Format: "pretty", Output: &out, Strict: true, NoColors: true,
			FeatureContents: []godog.Feature{{Name: "r.feature", Contents: []byte("Feature: f\n  @AIW-PRV-001\n  Scenario: s\n    Given a step fails\n")}},
		},
	}
	suite.Run()
	if n := strings.Count(out.String(), "unique-boom"); n != 2 { // once inline, once under "Failed steps"
		t.Fatalf("the step error appears %d times, want 2:\n%s", n, out.String())
	}
	if strings.Contains(out.String(), "after scenario hook failed") {
		t.Fatalf("the recorder's hooks must not report a hook failure:\n%s", out.String())
	}
}

func TestJourneyPassingRecordsEveryCheckedID(t *testing.T) {
	rec, failed := runFeature(t, `Feature: f
  @AIW-PRV-002 @AIW-PRV-001
  Scenario: journey
    * sheet check "AIW-PRV-001"
    Given a step passes
    * sheet check "AIW-PRV-002"
    Given a step passes
`)
	for _, r := range rec.Results() {
		if r.Status != Passed {
			t.Errorf("%s = %+v, want passed", r.ID, r)
		}
	}
	if failed["journey"] != "" {
		t.Errorf("FailedID on a passing journey = %q, want empty", failed["journey"])
	}
}

func TestSetupFailureMarksEveryIDNotReached(t *testing.T) {
	rec, failed := runFeature(t, `Feature: f
  @AIW-PRV-001 @AIW-PRV-002
  Scenario: journey
    Given a step fails
    * sheet check "AIW-PRV-001"
    * sheet check "AIW-PRV-002"
`)
	for _, r := range rec.Results() {
		if r.Status != NotReached || r.Step != "a step fails" || !strings.Contains(r.Error, "before its first sheet check") {
			t.Errorf("%s = %+v, want not-reached from setup failure", r.ID, r)
		}
	}
	if failed["journey"] != "" {
		t.Errorf("FailedID for a setup failure = %q, want empty", failed["journey"])
	}
}

func TestTaggedButUncheckedIDFails(t *testing.T) {
	rec, _ := runFeature(t, `Feature: f
  @AIW-PRV-001 @AIW-PRV-002
  Scenario: journey
    * sheet check "AIW-PRV-001"
    Given a step passes
`)
	got := byID(rec.Results())
	if got["journey|AIW-PRV-001"].Status != Passed {
		t.Errorf("AIW-PRV-001 = %+v, want passed", got["journey|AIW-PRV-001"])
	}
	if r := got["journey|AIW-PRV-002"]; r.Status != Failed || !strings.Contains(r.Error, "no `sheet check`") {
		t.Errorf("AIW-PRV-002 = %+v, want failed for missing check", r)
	}
}

func TestMisusedChecksFailTheScenario(t *testing.T) {
	cases := map[string]string{
		"untagged scenario": `Feature: f
  Scenario: s
    * sheet check "AIW-PRV-001"
`,
		"id not tagged": `Feature: f
  @AIW-PRV-001
  Scenario: s
    * sheet check "AIW-PRV-002"
`,
		"duplicate check": `Feature: f
  @AIW-PRV-001
  Scenario: s
    * sheet check "AIW-PRV-001"
    * sheet check "AIW-PRV-001"
`,
		"malformed id": `Feature: f
  @AIW-PRV-001
  Scenario: s
    * sheet check "not-an-id"
`,
		"duplicate tag": `Feature: f
  @AIW-PRV-001 @AIW-PRV-001
  Scenario: s
    Given a step passes
`,
	}
	for name, feature := range cases {
		t.Run(name, func(t *testing.T) {
			rec := NewRecorder()
			suite := godog.TestSuite{
				ScenarioInitializer: func(sc *godog.ScenarioContext) {
					rec.Bind(sc)
					sc.Step(`^a step passes$`, func() error { return nil })
				},
				Options: &godog.Options{
					Format: "pretty", Output: io.Discard, Strict: true, NoColors: true,
					FeatureContents: []godog.Feature{{Name: "m.feature", Contents: []byte(feature)}},
				},
			}
			if status := suite.Run(); status == 0 {
				t.Fatal("misused sheet check should fail the run")
			}
		})
	}
}

func TestWriteSummary(t *testing.T) {
	empty := NewRecorder()
	var buf bytes.Buffer
	empty.WriteSummary(&buf)
	if buf.Len() != 0 {
		t.Fatalf("empty recorder should print nothing, got %q", buf.String())
	}
	rec := NewRecorder()
	rec.results = []Result{
		{ID: "AIW-PRV-001", Status: Passed, Scenario: "j"},
		{ID: "AIW-PRV-002", Status: Failed, Scenario: "j", Step: "x", Error: "boom"},
		{ID: "AIW-PRV-003", Status: NotReached, Scenario: "j", Error: "stopped"},
		{ID: "AIW-PRV-004", Status: Failed, Scenario: "k", Error: "hook"},
	}
	rec.WriteSummary(&buf)
	out := buf.String()
	for _, want := range []string{
		`SHEET AIW-PRV-002 FAILED in "j" at step "x": boom`,
		`SHEET AIW-PRV-003 NOT REACHED in "j": stopped`,
		`SHEET AIW-PRV-004 FAILED in "k": hook`,
		"SHEET results: 1 passed, 2 failed, 1 not reached",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "AIW-PRV-001") {
		t.Errorf("passed IDs should not get their own line:\n%s", out)
	}
}

func TestFailedIDWithoutState(t *testing.T) {
	if FailedID(context.Background()) != "" {
		t.Fatal("FailedID with no state should be empty")
	}
}

func TestSinkWriteAndRead(t *testing.T) {
	if _, err := NewSink(" "); err == nil {
		t.Fatal("NewSink with a blank root should fail")
	}
	root := filepath.Join(t.TempDir(), "sheet-results")
	if err := os.MkdirAll(filepath.Join(root, "stale"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "stale", "old.json"), []byte(`{"block":"old"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	sink, err := NewSink(root)
	if err != nil {
		t.Fatal(err)
	}
	if sink.Root() == "" || (*Sink)(nil).Root() != "" {
		t.Fatal("Root() should be set for a sink and empty for nil")
	}
	if err := sink.Write(Report{Block: "b", Runner: "r"}); err != nil {
		t.Fatalf("empty report should be skipped, got %v", err)
	}
	if err := sink.Write(Report{Runner: "r", Results: []Result{{ID: "AIW-PRV-001"}}}); err == nil {
		t.Fatal("a report without a block should fail")
	}
	if err := (*Sink)(nil).Write(Report{Block: "b", Runner: "r", Results: []Result{{ID: "AIW-PRV-001"}}}); err != nil {
		t.Fatalf("nil sink should be a no-op, got %v", err)
	}
	report := Report{Block: "ai-core/sqlite", Runner: "provider create", Results: []Result{{ID: "AIW-PRV-001", Status: Passed}}}
	if err := sink.Write(report); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "ai-core_sqlite", "provider_create.json")); err != nil {
		t.Fatalf("report not written to a sanitized path: %v", err)
	}
	reports, err := ReadReports(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || reports[0].Runner != "provider create" || reports[0].Results[0].Status != Passed {
		t.Fatalf("ReadReports = %+v, want only the fresh report (stale one cleared)", reports)
	}
	if err := os.WriteFile(filepath.Join(root, "bad.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadReports(root); err == nil {
		t.Fatal("a malformed report should fail ReadReports")
	}
	if _, err := ReadReports(filepath.Join(root, "missing")); err == nil {
		t.Fatal("a missing root should fail ReadReports")
	}
}

func TestRecorderConcurrentResults(t *testing.T) {
	rec := NewRecorder()
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			rec.mu.Lock()
			rec.results = append(rec.results, Result{ID: "AIW-PRV-001", Status: Passed})
			rec.mu.Unlock()
			_ = rec.Results()
		})
	}
	wg.Wait()
	if len(rec.Results()) != 20 {
		t.Fatalf("want 20 results, got %d", len(rec.Results()))
	}
}

const validManifest = `sheet: AIW-LLM
title: LLM Providers and Proxies
scenarios:
  - id: AIW-PRV-001
    title: Create a provider
    priority: P1
    ui:
      status: automated
      refs:
        - feature: suites/ui/features/a.feature
          scenario: "Journey: create"
    it:
      status: known-issue
      issue: https://github.com/wso2/api-platform/issues/1
      refs:
        - feature: suites/it/features/a.feature
          scenario: create
  - id: AIW-PRV-002
    title: Validation
    ui:
      status: planned
    it:
      status: exempt
      reason: UI only
`

func writeManifest(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadManifestValid(t *testing.T) {
	dir := t.TempDir()
	m, err := LoadManifest(writeManifest(t, dir, "AIW-LLM.yaml", validManifest))
	if err != nil {
		t.Fatal(err)
	}
	if m.Sheet != "AIW-LLM" || len(m.Scenarios) != 2 || m.Path == "" {
		t.Fatalf("unexpected manifest %+v", m)
	}
	e := m.Scenarios[0]
	if e.For(LayerUI).Status != Automated || e.For(LayerIT).Status != KnownIssue {
		t.Fatalf("For() returned the wrong layer: %+v", e)
	}
	all, err := LoadManifests(dir)
	if err != nil || len(all) != 1 {
		t.Fatalf("LoadManifests = %v, %v", all, err)
	}
}

func TestLoadManifestRejectsInvalid(t *testing.T) {
	replace := func(old, new string) string { return strings.Replace(validManifest, old, new, 1) }
	cases := map[string]string{
		"missing sheet":        replace("sheet: AIW-LLM", "sheet: \"\""),
		"bad id":               replace("AIW-PRV-001", "AIW-PRV-1"),
		"foreign id":           replace("AIW-PRV-001", "APIM-PRV-001"),
		"duplicate id":         replace("AIW-PRV-002", "AIW-PRV-001"),
		"missing title":        replace("title: Validation", "title: \"\""),
		"automated no refs":    replace("    ui:\n      status: planned", "    ui:\n      status: automated"),
		"known-issue no issue": replace("      issue: https://github.com/wso2/api-platform/issues/1\n", ""),
		"exempt no reason":     replace("      reason: UI only\n", ""),
		"planned with refs":    replace("    ui:\n      status: automated", "    ui:\n      status: planned"),
		"unknown status":       replace("status: planned", "status: someday"),
		"empty status":         replace("status: planned", "status: \"\""),
		"ref missing scenario": replace("          scenario: create\n", ""),
		"unknown field":        replace("priority: P1", "priorty: P1"),
		"no scenarios":         "sheet: AIW-LLM\nscenarios: []\n",
		"malformed yaml":       "sheet: [",
		"exempt with refs":     replace("      status: known-issue\n      issue: https://github.com/wso2/api-platform/issues/1", "      status: exempt\n      reason: x"),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadManifest(writeManifest(t, t.TempDir(), "m.yaml", body)); err == nil {
				t.Fatalf("manifest should be rejected:\n%s", body)
			}
		})
	}
	if _, err := LoadManifest(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("a missing manifest should fail")
	}
}

func TestLoadManifestsRejectsEmptyDirAndCrossFileDuplicates(t *testing.T) {
	if _, err := LoadManifests(t.TempDir()); err == nil {
		t.Fatal("a directory with no manifests should fail")
	}
	dir := t.TempDir()
	writeManifest(t, dir, "a.yaml", validManifest)
	writeManifest(t, dir, "b.yaml", strings.Replace(validManifest, "title: LLM Providers and Proxies", "title: copy", 1))
	if _, err := LoadManifests(dir); err == nil {
		t.Fatal("an ID listed in two manifests should fail")
	}
}
