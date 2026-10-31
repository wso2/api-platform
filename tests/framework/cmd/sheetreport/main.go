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

// Command sheetreport merges a run's per-runner sheet results with the scenario-sheet
// manifests into one row per sheet ID and layer, as CSV for pasting into the sheet or
// as a Markdown table for a CI step summary.
package main

import (
	"bufio"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/sheet"
)

// row is one sheet ID in one layer.
type row struct {
	Sheet, ID, Title, Priority string
	Layer                      sheet.Layer
	Coverage                   sheet.CoverageStatus
	Result                     string
	Scenarios, Step, Error     string
	Refs, Issue                string
	SheetUpdate                string
}

const notRun = "not-run"

func main() {
	manifests := flag.String("manifests", "coverage", "scenario-sheet manifest directory")
	results := flag.String("results", "", "comma-separated layer=dir pairs, e.g. ui=suites/ui/sheet-results,it=suites/it/sheet-results")
	leg := flag.String("leg", "", "optional label for this run, e.g. the gateway version under test")
	format := flag.String("format", "csv", "csv or markdown")
	out := flag.String("out", "", "output file (default stdout)")
	flag.Parse()

	if err := run(*manifests, *results, *leg, *format, *out); err != nil {
		fmt.Fprintln(os.Stderr, "sheetreport:", err)
		os.Exit(1)
	}
}

func run(manifestDir, results, leg, format, out string) error {
	loaded, err := sheet.LoadManifests(manifestDir)
	if err != nil {
		return err
	}
	byLayer, err := loadResults(results)
	if err != nil {
		return err
	}
	rows := buildRows(loaded, byLayer)

	w := io.Writer(os.Stdout)
	if out != "" {
		f, err := os.Create(out)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	switch format {
	case "csv":
		return writeCSV(w, rows, leg)
	case "markdown":
		return writeMarkdown(w, rows, leg)
	}
	return fmt.Errorf("unknown format %q (want csv or markdown)", format)
}

// loadResults reads every layer's reports and indexes their results by sheet ID.
// A layer that was not part of this run is absent from the map.
func loadResults(spec string) (map[sheet.Layer]map[string][]sheet.Result, error) {
	byLayer := map[sheet.Layer]map[string][]sheet.Result{}
	for pair := range strings.SplitSeq(spec, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		name, dir, ok := strings.Cut(pair, "=")
		layer := sheet.Layer(name)
		if !ok || (layer != sheet.LayerUI && layer != sheet.LayerIT) || dir == "" {
			return nil, fmt.Errorf("-results entry %q must be ui=<dir> or it=<dir>", pair)
		}
		index := map[string][]sheet.Result{}
		_, statErr := os.Stat(dir)
		if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
			return nil, fmt.Errorf("reading %s results: %w", layer, statErr)
		}
		if statErr == nil {
			reports, err := sheet.ReadReports(dir)
			if err != nil {
				return nil, err
			}
			for _, report := range reports {
				for _, r := range report.Results {
					index[r.ID] = append(index[r.ID], r)
				}
			}
		}
		byLayer[layer] = index
	}
	return byLayer, nil
}

// worst folds several scenario results for one ID into the result the sheet shows:
// any failure wins, then not-reached, then passed.
func worst(results []sheet.Result) (sheet.Result, bool) {
	rank := map[sheet.Status]int{sheet.Failed: 3, sheet.NotReached: 2, sheet.Passed: 1}
	var picked sheet.Result
	found := false
	for _, r := range results {
		if !found || rank[r.Status] > rank[picked.Status] {
			picked, found = r, true
		}
	}
	return picked, found
}

func buildRows(manifests []sheet.Manifest, byLayer map[sheet.Layer]map[string][]sheet.Result) []row {
	var rows []row
	for _, m := range manifests {
		for _, e := range m.Scenarios {
			for _, layer := range sheet.Layers {
				cov := e.For(layer)
				r := row{
					Sheet: m.Sheet, ID: e.ID, Title: e.Title, Priority: e.Priority,
					Layer: layer, Coverage: cov.Status, SheetUpdate: e.SheetUpdate,
				}
				var refs []string
				for _, ref := range cov.Refs {
					refs = append(refs, ref.Feature+" :: "+ref.Scenario)
				}
				r.Refs = strings.Join(refs, "; ")
				r.Issue = cov.Issue
				if index, ran := byLayer[layer]; ran {
					r.Result = notRun
					if picked, ok := worst(index[e.ID]); ok {
						r.Result, r.Step, r.Error = string(picked.Status), picked.Step, picked.Error
						var names []string
						for _, res := range index[e.ID] {
							names = append(names, res.Scenario)
						}
						sort.Strings(names)
						r.Scenarios = strings.Join(names, "; ")
					}
				}
				rows = append(rows, r)
			}
		}
	}
	return rows
}

func writeCSV(w io.Writer, rows []row, leg string) error {
	out := csv.NewWriter(w)
	_ = out.Write([]string{"Sheet", "ID", "Title", "Priority", "Layer", "Leg", "Coverage", "Result", "Scenarios", "Failed step", "Error", "Automation ref", "Issue", "Sheet update"})
	for _, r := range rows {
		_ = out.Write([]string{r.Sheet, r.ID, r.Title, r.Priority, string(r.Layer), leg, string(r.Coverage), r.Result, r.Scenarios, r.Step, r.Error, r.Refs, r.Issue, r.SheetUpdate})
	}
	out.Flush()
	return out.Error()
}

func cell(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "|", `\|`)
	if len(s) > 160 {
		s = s[:157] + "..."
	}
	return s
}

// untested reports whether a layer has no test by design or not yet: a planned or exempt
// layer that did not run is not a missing result.
func untested(r row) bool {
	return r.Result == notRun && (r.Coverage == sheet.Planned || r.Coverage == sheet.Exempt)
}

func writeMarkdown(w io.Writer, rows []row, leg string) error {
	bw := bufio.NewWriter(w)
	counts := map[string]int{}
	for _, r := range rows {
		if r.Result != "" && !untested(r) {
			counts[r.Result]++
		}
		counts["coverage:"+string(r.Coverage)]++
	}
	title := "Scenario sheet results"
	if leg != "" {
		title += " (" + leg + ")"
	}
	fmt.Fprintf(bw, "### %s\n\n", title)
	fmt.Fprintf(bw, "Run: %d passed, %d failed, %d not reached, %d not run. Coverage: %d automated, %d known-issue, %d exempt, %d planned.\n\n",
		counts[string(sheet.Passed)], counts[string(sheet.Failed)], counts[string(sheet.NotReached)], counts[notRun],
		counts["coverage:automated"], counts["coverage:known-issue"], counts["coverage:exempt"], counts["coverage:planned"])
	fmt.Fprintln(bw, "| ID | Layer | Coverage | Result | Failed step | Error |")
	fmt.Fprintln(bw, "|---|---|---|---|---|---|")
	for _, r := range rows {
		// Only layers that ran, and nothing for IDs that have no test.
		if r.Result == "" || untested(r) {
			continue
		}
		fmt.Fprintf(bw, "| %s | %s | %s | %s | %s | %s |\n",
			r.ID, r.Layer, r.Coverage, cell(r.Result), cell(r.Step), cell(r.Error))
	}
	// A buffered writer keeps the first write error and returns it here.
	return bw.Flush()
}
