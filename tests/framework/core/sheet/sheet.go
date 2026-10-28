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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/cucumber/godog"
)

// EnvOut overrides the default sheet results output directory.
const EnvOut = "IT_SHEET_OUT"

// CheckStepPattern is the Gherkin step that marks where one sheet ID's checks begin.
const CheckStepPattern = `^sheet check "([^"]*)"$`

var checkStep = regexp.MustCompile(CheckStepPattern)

// idPattern is the shape of a sheet scenario ID: sheet code, area, three-digit number.
var idPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]*-[A-Z][A-Z0-9]*-[0-9]{3}$`)

// IsID reports whether value is a sheet scenario ID such as AIW-PRV-001.
func IsID(value string) bool {
	return idPattern.MatchString(value)
}

// IDsFromTags returns the sheet IDs named by tags, in order, without the leading "@".
func IDsFromTags(tags []string) []string {
	var ids []string
	for _, tag := range tags {
		if id := strings.TrimPrefix(tag, "@"); IsID(id) {
			ids = append(ids, id)
		}
	}
	return ids
}

// Status is the outcome recorded for one sheet ID in one scenario.
type Status string

const (
	// Passed means every step attributed to the ID passed.
	Passed Status = "passed"
	// Failed means a step attributed to the ID failed, or the ID was tagged but never checked.
	Failed Status = "failed"
	// NotReached means the scenario stopped before the ID's checks began.
	NotReached Status = "not-reached"
)

// Result is one sheet ID's outcome in one scenario.
type Result struct {
	ID       string `json:"id"`
	Status   Status `json:"status"`
	Scenario string `json:"scenario"`
	Feature  string `json:"feature"`
	Step     string `json:"step,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Report is everything one runner recorded.
type Report struct {
	Block   string   `json:"block"`
	Runner  string   `json:"runner"`
	Results []Result `json:"results"`
}

type stateKey struct{}

type failure struct {
	id   string
	step string
	err  string
}

// scenarioState tracks the sheet checkpoints of one running scenario.
type scenarioState struct {
	scenario string
	feature  string
	tagged   []string
	journey  bool // the scenario declares sheet check steps
	checked  []string
	current  string
	failure  *failure
}

func (s *scenarioState) checkedIndex(id string) int {
	return slices.Index(s.checked, id)
}

func (s *scenarioState) check(id string) error {
	if !IsID(id) {
		return fmt.Errorf("sheet: %q is not a sheet scenario ID (expected the shape AIW-PRV-001)", id)
	}
	if !slices.Contains(s.tagged, id) {
		return fmt.Errorf("sheet: scenario %q checks %s but is not tagged @%s", s.scenario, id, id)
	}
	if s.checkedIndex(id) >= 0 {
		return fmt.Errorf("sheet: scenario %q checks %s more than once", s.scenario, id)
	}
	s.checked = append(s.checked, id)
	s.current = id
	return nil
}

func (s *scenarioState) fail(step, message string) {
	if s.failure == nil {
		s.failure = &failure{id: s.current, step: step, err: message}
	}
}

// results resolves every tagged ID's outcome once the scenario has finished.
func (s *scenarioState) results() []Result {
	out := make([]Result, 0, len(s.tagged))
	for _, id := range s.tagged {
		r := Result{ID: id, Scenario: s.scenario, Feature: s.feature}
		switch {
		case !s.journey:
			// A scenario without checkpoints covers all its IDs as one unit.
			r.Status = Passed
			if s.failure != nil {
				r.Status, r.Step, r.Error = Failed, s.failure.step, s.failure.err
			}
		case s.failure == nil:
			r.Status = Passed
			if s.checkedIndex(id) < 0 {
				r.Status, r.Error = Failed, "tagged @"+id+" but no `sheet check` step for it ran"
			}
		case s.failure.id == "":
			r.Status, r.Step = NotReached, s.failure.step
			r.Error = "the scenario failed before its first sheet check: " + s.failure.err
		case id == s.failure.id:
			r.Status, r.Step, r.Error = Failed, s.failure.step, s.failure.err
		case s.checkedIndex(id) >= 0 && s.checkedIndex(id) < s.checkedIndex(s.failure.id):
			r.Status = Passed
		default:
			r.Status, r.Error = NotReached, "the scenario stopped at "+s.failure.id+" before this check began"
		}
		out = append(out, r)
	}
	return out
}

// Recorder collects the sheet results of one runner's scenarios.
type Recorder struct {
	mu      sync.Mutex
	results []Result
}

// NewRecorder returns an empty recorder.
func NewRecorder() *Recorder {
	return &Recorder{}
}

// Bind registers the checkpoint step and the hooks that attribute step outcomes to
// sheet IDs. Bind before any other After hook that wants FailedID to be accurate.
func (r *Recorder) Bind(sc *godog.ScenarioContext) {
	sc.Before(func(ctx context.Context, pickle *godog.Scenario) (context.Context, error) {
		tags := make([]string, 0, len(pickle.Tags))
		for _, tag := range pickle.Tags {
			tags = append(tags, tag.Name)
		}
		ids := IDsFromTags(tags)
		if len(ids) == 0 {
			return ctx, nil
		}
		seen := map[string]bool{}
		for _, id := range ids {
			if seen[id] {
				return ctx, fmt.Errorf("sheet: scenario %q is tagged @%s more than once", pickle.Name, id)
			}
			seen[id] = true
		}
		journey := false
		for _, step := range pickle.Steps {
			journey = journey || checkStep.MatchString(step.Text)
		}
		return context.WithValue(ctx, stateKey{}, &scenarioState{
			scenario: pickle.Name, feature: pickle.Uri, tagged: ids, journey: journey,
		}), nil
	})

	sc.Step(CheckStepPattern, func(ctx context.Context, id string) error {
		state, ok := ctx.Value(stateKey{}).(*scenarioState)
		if !ok {
			return fmt.Errorf("sheet: `sheet check %q` used in a scenario that has no sheet ID tags", id)
		}
		return state.check(id)
	})

	// The hooks only observe: godog merges any error a hook returns into the scenario's
	// own, so returning the step's error again would report it twice.
	sc.StepContext().After(func(ctx context.Context, st *godog.Step, status godog.StepResultStatus, err error) (context.Context, error) {
		state, ok := ctx.Value(stateKey{}).(*scenarioState)
		if !ok {
			return ctx, nil
		}
		switch status {
		case godog.StepFailed, godog.StepUndefined, godog.StepPending:
			message := status.String()
			if err != nil {
				message = err.Error()
			}
			state.fail(st.Text, message)
		}
		return ctx, nil
	})

	sc.After(func(ctx context.Context, _ *godog.Scenario, scenarioErr error) (context.Context, error) {
		state, ok := ctx.Value(stateKey{}).(*scenarioState)
		if !ok {
			return ctx, nil
		}
		if scenarioErr != nil {
			// A hook failure has no step; the step hook already caught step failures.
			state.fail("", scenarioErr.Error())
		}
		r.mu.Lock()
		r.results = append(r.results, state.results()...)
		r.mu.Unlock()
		return ctx, nil
	})
}

// Results returns a copy of everything recorded so far.
func (r *Recorder) Results() []Result {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Result(nil), r.results...)
}

// WriteSummary prints one line per ID that did not pass, then the totals. It writes
// nothing when the runner recorded no sheet IDs.
func (r *Recorder) WriteSummary(w io.Writer) {
	results := r.Results()
	if len(results) == 0 {
		return
	}
	counts := map[Status]int{}
	for _, res := range results {
		counts[res.Status]++
		switch res.Status {
		case Failed:
			fmt.Fprintf(w, "SHEET %s FAILED in %q", res.ID, res.Scenario)
			if res.Step != "" {
				fmt.Fprintf(w, " at step %q", res.Step)
			}
			fmt.Fprintf(w, ": %s\n", res.Error)
		case NotReached:
			fmt.Fprintf(w, "SHEET %s NOT REACHED in %q: %s\n", res.ID, res.Scenario, res.Error)
		}
	}
	fmt.Fprintf(w, "SHEET results: %d passed, %d failed, %d not reached\n",
		counts[Passed], counts[Failed], counts[NotReached])
}

// FailedID names the sheet IDs a failed scenario failed: the ID whose checks were running
// in a journey, or every tagged ID, joined with "+", in a scenario without checkpoints.
// It is empty when the scenario has not failed, carries no sheet IDs, or failed before a
// journey's first sheet check.
func FailedID(ctx context.Context) string {
	state, ok := ctx.Value(stateKey{}).(*scenarioState)
	if !ok || state.failure == nil {
		return ""
	}
	if !state.journey {
		return strings.Join(state.tagged, "+")
	}
	return state.failure.id
}

// Sink writes runner reports beneath one directory.
type Sink struct {
	root string
}

// NewSink empties root and returns a sink writing into it, so one run's reports are
// never merged with an earlier run's.
func NewSink(root string) (*Sink, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("sheet: the sink needs a root directory")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("sheet: resolving sink root %q: %w", root, err)
	}
	if err := os.RemoveAll(abs); err != nil {
		return nil, fmt.Errorf("sheet: clearing sink root %q: %w", abs, err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("sheet: creating sink root %q: %w", abs, err)
	}
	return &Sink{root: abs}, nil
}

// Root returns the sink's absolute root directory.
func (s *Sink) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

// Write stores one runner's report as <root>/<block>/<runner>.json. A report with no
// results is not written.
func (s *Sink) Write(report Report) error {
	if s == nil || len(report.Results) == 0 {
		return nil
	}
	if strings.TrimSpace(report.Block) == "" || strings.TrimSpace(report.Runner) == "" {
		return fmt.Errorf("sheet: a report needs a block and a runner name")
	}
	dir := filepath.Join(s.root, safeName(report.Block))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("sheet: creating report directory %q: %w", dir, err)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("sheet: encoding report for %s/%s: %w", report.Block, report.Runner, err)
	}
	path := filepath.Join(dir, safeName(report.Runner)+".json")
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("sheet: writing report %q: %w", path, err)
	}
	return nil
}

// ReadReports loads every runner report beneath root.
func ReadReports(root string) ([]Report, error) {
	var reports []Report
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var report Report
		if err := json.Unmarshal(data, &report); err != nil {
			return fmt.Errorf("sheet: parsing report %q: %w", path, err)
		}
		reports = append(reports, report)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("sheet: reading reports beneath %q: %w", root, err)
	}
	return reports, nil
}

var unsafeNameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func safeName(name string) string {
	return unsafeNameChars.ReplaceAllString(name, "_")
}
