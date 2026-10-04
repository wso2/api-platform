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

package migration

import (
	"context"
	"database/sql"
	"fmt"
)

// baseMigrator provides Name/DependsOn boilerplate for a resource. Concrete
// migrators embed it and implement Migrate/Verify.
type baseMigrator struct {
	name      string
	dependsOn []string
}

func (b baseMigrator) Name() string        { return b.name }
func (b baseMigrator) DependsOn() []string { return b.dependsOn }

// migrateReverse is the default reverse (v2 -> v1) behavior: unsupported. A
// migrator whose forward transform is cleanly invertible overrides this method
// (§13). Forward Migrate dispatches here when rc.Direction == DirectionReverse.
func (b baseMigrator) migrateReverse(_ context.Context, _ *RunContext, rep *ResourceReport) (*ResourceReport, error) {
	rep.Status = StatusSkip
	return rep, ErrReverseUnsupported
}

// runResource applies perRow to every item, handling the dry-run / batched-tx
// split so migrators don't repeat it:
//   - dry-run: perRow is called with a nil queryer (q == nil) — the kernels and
//     the perRow body skip all target writes but still run every transform,
//     length assert, and token decrypt for validation.
//   - real run: items are written in BatchSize-sized transactions; a row error
//     rolls back its batch (the batch boundary of §12 / §8).
//
// perRow must guard its own INSERTs with `if q != nil { ... }`; kernel helpers
// (resolveActor, externalizeSecret, …) are already nil-q-safe.
func runResource[T any](ctx context.Context, rc *RunContext, items []T,
	perRow func(ctx context.Context, q queryer, it T) error) error {

	if !rc.writes() || rc.Tgt == nil {
		for _, it := range items {
			if err := perRow(ctx, nil, it); err != nil {
				return err
			}
		}
		return nil
	}

	bs := rc.BatchSize
	if bs < 1 {
		bs = 1
	}
	for i := 0; i < len(items); i += bs {
		end := min(i+bs, len(items))
		batch := items[i:end]
		if err := withTx(ctx, rc.Tgt, func(tx *sql.Tx) error {
			for _, it := range batch {
				if err := perRow(ctx, tx, it); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return fmt.Errorf("batch [%d:%d]: %w", i, end, err)
		}
	}
	return nil
}

// handleExistsChecker returns an existsCheck for GenerateHandle that probes the
// target table's handle column (org-scoped). Returns nil when there is no target
// (dry-run) — GenerateHandle then returns the bare slug.
func (rc *RunContext) handleExistsChecker(table, orgCol, orgVal string) func(string) bool {
	if rc.Tgt == nil || !rc.writes() {
		return nil
	}
	q := fmt.Sprintf("SELECT 1 FROM %s WHERE handle = $1 AND %s = $2 LIMIT 1", table, orgCol)
	return func(h string) bool {
		var one int
		err := rc.Tgt.QueryRowContext(context.Background(), q, h, orgVal).Scan(&one)
		return err == nil // a row found (err==nil) means the handle is taken
	}
}

// verifyRowCounts is the standard §9 check: compare a source count to a target
// count and set PASS/WARN on the report. mismatchIsWarn controls whether a
// difference is a WARN (expected for split/derived tables) or a FAIL.
func verifyRowCounts(ctx context.Context, rc *RunContext, rep *ResourceReport,
	srcCountQuery, tgtCountQuery string, mismatchIsWarn bool) {

	if rc.Src != nil {
		if n, err := scanCount(ctx, rc.Src, srcCountQuery); err == nil {
			rep.SrcCount = n
		} else {
			rep.warn("source count failed: %v", err)
		}
	}
	// Dry-run performs zero writes, so target row-count parity is not applicable —
	// the transform + gates having run without error is the dry-run's success
	// signal. Report the source count and leave the status PASS instead of
	// failing every table against an empty target (which made --dry-run exit 1
	// and read as a total failure).
	if rc.DryRun {
		rep.addf("dry-run: transform validated, no writes — target parity not applicable")
		return
	}
	if rc.Tgt == nil {
		rep.warn("no target — count parity skipped")
		return
	}
	n, err := scanCount(ctx, rc.Tgt, tgtCountQuery)
	if err != nil {
		rep.fail("target count failed: %v", err)
		return
	}
	rep.TgtCount = n
	if rep.SrcCount != rep.TgtCount {
		if mismatchIsWarn {
			rep.warn("row count differs (src=%d tgt=%d) — expected for split/derived/filtered table", rep.SrcCount, rep.TgtCount)
		} else {
			rep.fail("row count parity FAILED (src=%d tgt=%d)", rep.SrcCount, rep.TgtCount)
		}
	}
}

// newReport builds a fresh PASS report for a resource.
func newReport(name string) *ResourceReport {
	return &ResourceReport{Name: name, Status: StatusPass}
}

// nullString scans a possibly-NULL text column.
func nullToString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// emptyResourceMigrator is a no-op reporter for tables the migration
// intentionally leaves empty (user_organization_mappings, audit,
// artifact_subscription_plans — §B.4 / no v2 writer). It records why.
type emptyResourceMigrator struct {
	baseMigrator
	reason string
}

func (m *emptyResourceMigrator) Migrate(_ context.Context, _ *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	rep.Status = StatusSkip
	rep.addf("intentionally empty: %s", m.reason)
	return rep, nil
}

func (m *emptyResourceMigrator) Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error) {
	rep := newReport(m.name)
	rep.Status = StatusSkip
	rep.addf("intentionally empty: %s", m.reason)
	if rc.Tgt != nil {
		if n, err := scanCount(ctx, rc.Tgt, "SELECT COUNT(*) FROM "+m.name); err == nil {
			rep.TgtCount = n
			if n != 0 {
				rep.warn("expected 0 rows in %s but found %d (pre-existing data?)", m.name, n)
			}
		}
	}
	return rep, nil
}

func init() {
	register(&emptyResourceMigrator{baseMigrator: baseMigrator{name: "user_organization_mappings"},
		reason: "not read by audit resolution — left empty (§B.4)"})
	register(&emptyResourceMigrator{baseMigrator: baseMigrator{name: "artifact_subscription_plans"},
		reason: "no v2 writer exists — migrated empty"})
}
