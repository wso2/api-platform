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
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Quarantine reason codes (closed enum).
const (
	ReasonOrphanFK           = "ORPHAN_FK"
	ReasonBlobUnparseable    = "BLOB_UNPARSEABLE"
	ReasonNullRequired       = "NULL_REQUIRED"
	ReasonHandleUnresolvable = "HANDLE_UNRESOLVABLE"
	ReasonDupKey             = "DUP_KEY"
)

// Migrate-with-flag codes.
const (
	FlagDefaultedNull       = "DEFAULTED_NULL"
	FlagTruncated           = "TRUNCATED"
	FlagPlaceholderIDP      = "PLACEHOLDER_IDP"
	FlagSynthesized         = "SYNTHESIZED"
	FlagPlaintextCredential = "PLAINTEXT_CREDENTIAL"
)

// Drop feature labels (intentional §H drops).
const (
	DropDevportals          = "devportals"
	DropDevPortalAssoc      = "dev_portal_association"
	DropBillingPlan         = "billing_plan"
	DropLLMProviderStatus   = "llm_provider_status"
	DropLLMProxyStatus      = "llm_proxy_status"
	DropMCPStatus           = "mcp_status"
	DropPublicationMappings = "publication_mappings"
)

// Report is the machine-readable summary written at the end of a run.
type Report struct {
	RunID              string              `json:"run_id"`
	Mode               string              `json:"mode"`
	SourceTZ           string              `json:"source_tz"`
	IDPRefStrategy     string              `json:"idp_ref_strategy"`
	GroupIDStrategy    string              `json:"group_id_strategy"`
	PopulateArtSubplan bool                `json:"populate_art_subplan"`
	ProcessedPerTable  map[string]int      `json:"processed_per_table"`
	QuarantineByCode   map[string]int      `json:"quarantine_by_code"`
	QuarantineByTable  map[string]int      `json:"quarantine_by_table"`
	DropsByFeature     map[string]int      `json:"drops_by_feature"`
	DropsByTable       map[string]int      `json:"drops_by_table"`
	FlagsByCode        map[string]int      `json:"flags_by_code"`
	DroppedConfigFields map[string][]string `json:"dropped_config_fields,omitempty"`
	StartedAt          string              `json:"started_at"`
	FinishedAt         string              `json:"finished_at"`
}

// Checkpoint persists resume state: the generated-handle map (persist-and-replay,
// §B) and which tables have completed. It lives in a file, never the v2 DB.
type Checkpoint struct {
	RunID           string            `json:"run_id"`
	CompletedTables map[string]bool   `json:"completed_tables"`
	Handles         map[string]string `json:"handles"` // "table|v1uuid" -> final handle
}

// Run bundles all file-based migration state and the append-only output writers.
type Run struct {
	opts   *Options
	logger *slog.Logger
	mode   string // "live" | "dry-run"

	mu     sync.Mutex
	qFile  *os.File
	fFile  *os.File
	dFile  *os.File
	report *Report
	ckpt   *Checkpoint

	ckptPath string
}

func suffixed(base, runID string, dryRun bool, ext string) string {
	if dryRun {
		return fmt.Sprintf("%s-%s-dryrun.%s", base, runID, ext)
	}
	return fmt.Sprintf("%s-%s.%s", base, runID, ext)
}

// newRun opens the output writers, loads any existing checkpoint (for the handle
// map), and initializes the report. The JSONL outputs are truncated per invocation:
// a resume re-processes all tables idempotently (ON CONFLICT DO NOTHING), so the
// outputs are always a clean, non-duplicated picture of the latest attempt.
func newRun(opts *Options, logger *slog.Logger) (*Run, error) {
	dir := opts.OutDir
	open := func(base string) (*os.File, error) {
		p := filepath.Join(dir, suffixed(base, opts.RunID, opts.DryRun, "jsonl"))
		f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return nil, err
		}
		// O_CREATE sets the mode only on creation; enforce 0600 on a pre-existing
		// (possibly world-readable) file too — these artifacts hold full v1 rows.
		if err := f.Chmod(0o600); err != nil {
			_ = f.Close()
			return nil, err
		}
		return f, nil
	}
	qf, err := open("quarantine")
	if err != nil {
		return nil, err
	}
	ff, err := open("flags")
	if err != nil {
		return nil, err
	}
	df, err := open("drops")
	if err != nil {
		return nil, err
	}

	mode := "live"
	if opts.DryRun {
		mode = "dry-run"
	}

	r := &Run{
		opts:   opts,
		logger: logger,
		mode:   mode,
		qFile:  qf,
		fFile:  ff,
		dFile:  df,
		report: &Report{
			RunID:              opts.RunID,
			Mode:               mode,
			SourceTZ:           opts.SourceTZ,
			IDPRefStrategy:     opts.IDPRefStrategy,
			GroupIDStrategy:    opts.GroupIDStrategy,
			PopulateArtSubplan: opts.PopulateArtifactSubPlans,
			ProcessedPerTable:  map[string]int{},
			QuarantineByCode:   map[string]int{},
			QuarantineByTable:  map[string]int{},
			DropsByFeature:     map[string]int{},
			DropsByTable:       map[string]int{},
			FlagsByCode:        map[string]int{},
			StartedAt:          nowRFC3339(),
		},
	}

	// The checkpoint (handle map) is only meaningful for live runs; dry-runs never
	// insert, so they neither read nor write it.
	if !opts.DryRun {
		r.ckptPath = filepath.Join(dir, suffixed("migration-state", opts.RunID, false, "json"))
		r.ckpt = loadCheckpoint(r.ckptPath, opts.RunID, logger)
	} else {
		r.ckpt = &Checkpoint{RunID: opts.RunID, CompletedTables: map[string]bool{}, Handles: map[string]string{}}
	}
	return r, nil
}

func loadCheckpoint(path, runID string, logger *slog.Logger) *Checkpoint {
	c := &Checkpoint{RunID: runID, CompletedTables: map[string]bool{}, Handles: map[string]string{}}
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	var loaded Checkpoint
	if err := json.Unmarshal(data, &loaded); err != nil {
		logger.Warn("checkpoint unreadable, starting fresh", "path", path, "error", err)
		return c
	}
	if loaded.CompletedTables == nil {
		loaded.CompletedTables = map[string]bool{}
	}
	if loaded.Handles == nil {
		loaded.Handles = map[string]string{}
	}
	logger.Info("resuming from checkpoint", "handles", len(loaded.Handles), "completed_tables", len(loaded.CompletedTables))
	return &loaded
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func (r *Run) writeJSONL(f *os.File, obj any) {
	b, err := json.Marshal(obj)
	if err != nil {
		r.logger.Error("marshal jsonl record", "error", err)
		return
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		r.logger.Error("write jsonl record", "error", err)
	}
}

// quarantine records a non-migrated row (full v1 row preserved for re-injection).
func (r *Run) quarantine(sourceTable, sourceKey, reasonCode, detail string, fullRow any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.writeJSONL(r.qFile, map[string]any{
		"run_id":         r.opts.RunID,
		"source_table":   sourceTable,
		"source_key":     sourceKey,
		"reason_code":    reasonCode,
		"detail":         detail,
		"full_row":       fullRow,
		"quarantined_at": nowRFC3339(),
	})
	r.report.QuarantineByCode[reasonCode]++
	r.report.QuarantineByTable[sourceTable]++
}

// flag records a migrate-with-flag event (the row IS in v2; this is the audit trail).
func (r *Run) flag(sourceTable, sourceKey, flagCode string, oldV, newV any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.writeJSONL(r.fFile, map[string]any{
		"run_id":       r.opts.RunID,
		"source_table": sourceTable,
		"source_key":   sourceKey,
		"flag_code":    flagCode,
		"old":          oldV,
		"new":          newV,
	})
	r.report.FlagsByCode[flagCode]++
}

// drop records an intentional §H drop (id + human label) so loss is auditable.
// scope is "row" (the whole v1 row is discarded — devportals, dev_portal assoc,
// publication_mappings) or "field" (the row IS migrated but one column is dropped —
// billing_plan, llm/mcp status). verify reconciles row-drops only.
func (r *Run) drop(scope, sourceTable, sourceKey, feature, label string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.writeJSONL(r.dFile, map[string]any{
		"run_id":       r.opts.RunID,
		"source_table": sourceTable,
		"source_key":   sourceKey,
		"scope":        scope,
		"feature":      feature,
		"label":        label,
	})
	r.report.DropsByFeature[feature]++
	r.report.DropsByTable[sourceTable]++
}

// dropRow / dropField are thin, self-documenting wrappers over drop.
func (r *Run) dropRow(sourceTable, sourceKey, feature, label string) {
	r.drop("row", sourceTable, sourceKey, feature, label)
}
func (r *Run) dropField(sourceTable, sourceKey, feature, label string) {
	r.drop("field", sourceTable, sourceKey, feature, label)
}

// --- migrationcore.Reporter implementation (exported method set) ---
// The batch writes the existing JSONL files; the live dual-write path implements
// the same interface with logs + the v2_dual_write_failures table.

func (r *Run) Flag(table, key, code string, oldV, newV any)      { r.flag(table, key, code, oldV, newV) }
func (r *Run) Quarantine(table, key, code, detail string, row any) { r.quarantine(table, key, code, detail, row) }
func (r *Run) Dropped(scope, table, key, feature, label string)  { r.drop(scope, table, key, feature, label) }
func (r *Run) DroppedFields(structName string, fields []string)  { r.recordDroppedFields(structName, fields) }

// addProcessed increments the migrated-row count for a v2 table.
func (r *Run) addProcessed(table string, n int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.report.ProcessedPerTable[table] += n
}

// recordDroppedFields stores the §E DisallowUnknownFields discovery for a config struct.
func (r *Run) recordDroppedFields(structName string, fields []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(fields) == 0 {
		return
	}
	if r.report.DroppedConfigFields == nil {
		r.report.DroppedConfigFields = map[string][]string{}
	}
	r.report.DroppedConfigFields[structName] = fields
}

// getHandle returns a previously generated handle for (table, v1uuid), if any.
func (r *Run) getHandle(table, v1uuid string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	h, ok := r.ckpt.Handles[table+"|"+v1uuid]
	return h, ok
}

// putHandle records a generated handle for (table, v1uuid).
func (r *Run) putHandle(table, v1uuid, handle string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ckpt.Handles[table+"|"+v1uuid] = handle
}

// saveCheckpoint atomically flushes the handle map + completion state to disk.
func (r *Run) saveCheckpoint() error {
	if r.opts.DryRun {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	b, err := json.MarshalIndent(r.ckpt, "", "  ")
	if err != nil {
		return err
	}
	tmp := r.ckptPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil { // enforce on a pre-existing stale tmp
		return err
	}
	return os.Rename(tmp, r.ckptPath)
}

func (r *Run) markTableComplete(table string) {
	r.mu.Lock()
	r.ckpt.CompletedTables[table] = true
	r.mu.Unlock()
}

// writeReport writes the migration-report JSON and syncs the JSONL files.
func (r *Run) writeReport() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.report.FinishedAt = nowRFC3339()
	b, err := json.MarshalIndent(r.report, "", "  ")
	if err != nil {
		return err
	}
	p := filepath.Join(r.opts.OutDir, suffixed("migration-report", r.opts.RunID, r.opts.DryRun, "json"))
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return err
	}
	return os.Chmod(p, 0o600) // enforce 0600 on a pre-existing report file
}

func (r *Run) close() {
	for _, f := range []*os.File{r.qFile, r.fFile, r.dFile} {
		if f != nil {
			_ = f.Sync()
			_ = f.Close()
		}
	}
}
