/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/database"
)

// Check is one verification assertion. v1/v2/expected are optional counts.
type Check struct {
	Layer    string `json:"layer"`
	Name     string `json:"name"`
	Status   string `json:"status"` // PASS | FAIL | WARN
	V1       *int64 `json:"v1,omitempty"`
	V2       *int64 `json:"v2,omitempty"`
	Expected *int64 `json:"expected,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// VerifyReport is the structured output of the verify subcommand.
type VerifyReport struct {
	RunID   string         `json:"run_id"`
	Checks  []Check        `json:"checks"`
	Overall string         `json:"overall"`
	Summary map[string]int `json:"summary"`
}

// verifier holds both read-only connections plus loaded reconciliation state.
type verifier struct {
	v1, v2 *database.DB
	opts   *Options
	loc    *time.Location
	epoch  time.Time
	encKey []byte

	quarByTable map[string]int64
	quarKeys    map[string]bool // "table|key"
	signoff     map[string]bool // "table|key"

	checks []Check
}

const (
	statusPass = "PASS"
	statusFail = "FAIL"
	statusWarn = "WARN"
)

func runVerify(argv []string) error {
	o := &Options{}
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	registerCommonFlags(fs, o)
	fs.StringVar(&o.SourceTZ, "source-tz", "UTC", "Time zone of naive v1 TIMESTAMP values (must match the migrate run)")
	epochStr := fs.String("migration-epoch", defaultMigrationEpoch, "Migration epoch used at migrate time (for audit-identity checks)")
	fs.IntVar(&o.DecryptSampleSize, "decrypt-sample-size", 25, "Subscription tokens to sample for the decrypt round-trip")
	fs.StringVar(&o.EncKeyFile, "encryption-key-file", "", "File with the subscription-token key (else APIP_MIGRATION_ENCRYPTION_KEY); enables the token round-trip check")
	if err := fs.Parse(argv); err != nil {
		return err
	}
	if err := validateCommon(o); err != nil {
		return err
	}
	loc, err := time.LoadLocation(o.SourceTZ)
	if err != nil {
		return fmt.Errorf("invalid -source-tz: %w", err)
	}
	epoch, err := time.Parse(time.RFC3339, *epochStr)
	if err != nil {
		return fmt.Errorf("invalid -migration-epoch: %w", err)
	}
	logger := newLogger(o)
	// The key is optional for verify (it only enables the token decrypt round-trip);
	// report a load failure rather than silently swallowing it.
	if err := loadEncryptionKey(o); err != nil {
		// Do not log err verbatim — it may echo key material or the DSN. A generic
		// warning is enough; the token round-trip check simply won't run.
		logger.Warn("could not load the subscription-token key; the decrypt round-trip check will be skipped")
	}
	v1, err := openDB(o.V1DSN, logger)
	if err != nil {
		return fmt.Errorf("open v1: %w", err)
	}
	defer v1.Close()
	v2, err := openDB(o.V2DSN, logger)
	if err != nil {
		return fmt.Errorf("open v2: %w", err)
	}
	defer v2.Close()

	vr := &verifier{
		v1: v1, v2: v2, opts: o, loc: loc, epoch: epoch, encKey: o.EncryptionKey,
		quarByTable: map[string]int64{}, quarKeys: map[string]bool{}, signoff: map[string]bool{},
	}
	vr.loadReconciliationState()

	// Run all layers. Individual check errors are recorded as FAIL, not returned,
	// so one broken check never masks the rest.
	vr.layerA_counts()
	vr.layerB_scalars()
	vr.layerC_transforms()
	vr.layerD_integrity()
	vr.layerE_generated()
	vr.layerF_dropReconcile()

	report := vr.finish(o.RunID)
	if err := writeVerifyReport(o, report); err != nil {
		return err
	}
	printVerify(report)
	if report.Summary["FAIL"] > 0 {
		return fmt.Errorf("verify FAILED: %d check(s) failed", report.Summary["FAIL"])
	}
	return nil
}

// loadReconciliationState reads the latest quarantine JSONL and the sign-off file.
func (vr *verifier) loadReconciliationState() {
	qPath := filepath.Join(vr.opts.OutDir, suffixed("quarantine", vr.opts.RunID, false, "jsonl"))
	forEachJSONL(qPath, func(m map[string]any) {
		t, _ := m["source_table"].(string)
		k, _ := m["source_key"].(string)
		vr.quarByTable[t]++
		vr.quarKeys[t+"|"+k] = true
	})
	sPath := filepath.Join(vr.opts.OutDir, "quarantine-signoff.jsonl")
	forEachJSONL(sPath, func(m map[string]any) {
		t, _ := m["source_table"].(string)
		k, _ := m["source_key"].(string)
		vr.signoff[t+"|"+k] = true
	})
}

func forEachJSONL(path string, fn func(map[string]any)) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var m map[string]any
		if json.Unmarshal(line, &m) == nil {
			fn(m)
		}
	}
}

// ---- check recording helpers ----

func i64(v int64) *int64 { return &v }

func (vr *verifier) add(c Check) { vr.checks = append(vr.checks, c) }

func (vr *verifier) pass(layer, name, detail string) {
	vr.add(Check{Layer: layer, Name: name, Status: statusPass, Detail: detail})
}
func (vr *verifier) fail(layer, name, detail string) {
	vr.add(Check{Layer: layer, Name: name, Status: statusFail, Detail: detail})
}
func (vr *verifier) warn(layer, name, detail string) {
	vr.add(Check{Layer: layer, Name: name, Status: statusWarn, Detail: detail})
}

// eq records a PASS/FAIL count comparison.
func (vr *verifier) eq(layer, name string, got, want int64, detail string) {
	st := statusPass
	if got != want {
		st = statusFail
	}
	vr.add(Check{Layer: layer, Name: name, Status: st, V2: i64(got), Expected: i64(want), Detail: detail})
}

func (vr *verifier) finish(runID string) *VerifyReport {
	summary := map[string]int{"PASS": 0, "FAIL": 0, "WARN": 0, "total": len(vr.checks)}
	for _, c := range vr.checks {
		summary[c.Status]++
	}
	overall := statusPass
	if summary["FAIL"] > 0 {
		overall = statusFail
	}
	return &VerifyReport{RunID: runID, Checks: vr.checks, Overall: overall, Summary: summary}
}

func writeVerifyReport(o *Options, r *VerifyReport) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	p := filepath.Join(o.OutDir, suffixed("verify-report", o.RunID, false, "json"))
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return err
	}
	return os.Chmod(p, 0o600) // enforce 0600 on a pre-existing report file
}

func printVerify(r *VerifyReport) {
	// Sort for stable output: FAILs first, then WARNs, then PASS.
	order := map[string]int{statusFail: 0, statusWarn: 1, statusPass: 2}
	sorted := make([]Check, len(r.Checks))
	copy(sorted, r.Checks)
	sort.SliceStable(sorted, func(i, j int) bool { return order[sorted[i].Status] < order[sorted[j].Status] })
	fmt.Printf("\n=== verify report (run %s) — overall %s ===\n", r.RunID, r.Overall)
	for _, c := range sorted {
		if c.Status == statusPass {
			continue // keep the console focused on problems; full detail is in JSON
		}
		fmt.Printf("  [%s] %s/%s %s\n", c.Status, c.Layer, c.Name, c.Detail)
	}
	fmt.Printf("summary: PASS=%d WARN=%d FAIL=%d total=%d\n",
		r.Summary["PASS"], r.Summary["WARN"], r.Summary["FAIL"], r.Summary["total"])
}

// count returns the row count of a table (0 on error, recorded as a FAIL check).
func (vr *verifier) count(db *database.DB, table string) int64 {
	var n int64
	if err := db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil {
		vr.fail("A", "count "+table, err.Error())
		return 0
	}
	return n
}

func (vr *verifier) countWhere(db *database.DB, table, where string) int64 {
	var n int64
	if err := db.QueryRow("SELECT count(*) FROM " + table + " WHERE " + where).Scan(&n); err != nil {
		vr.fail("A", "count "+table+" where "+where, err.Error())
		return 0
	}
	return n
}
