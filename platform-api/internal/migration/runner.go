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
	"errors"
	"fmt"
)

// Run executes a full migration (or reverse / dry-run / verify-only) per cfg.
// It opens both endpoints, enforces the preflight gates (forward), then walks the
// selected migrators in FK order, producing a per-resource PASS/WARN/FAIL report.
func Run(ctx context.Context, cfg *Config) error {
	log := NewLogger(cfg.LogLevel, cfg.LogFormat)
	log.Info("migration starting",
		"build", BuildInfo(),
		"direction", cfg.Direction,
		"dryRun", cfg.DryRun,
		"verifyOnly", cfg.VerifyOnly,
		"resume", cfg.Resume,
		"v1", cfg.V1.Redacted(),
		"v2", cfg.V2.Redacted(),
		"encryptionKey", redactKey(cfg.EncryptionKey),
		"v1Timezone", cfg.V1TimezoneName,
		"batchSize", cfg.EffectiveBatchSize(),
	)

	cp, err := LoadCheckpoint(cfg.CheckpointFile)
	if err != nil {
		return err
	}
	// Make leftover checkpoint state visible: minted handles in the file are
	// REUSED (resume-safe by design), so a file from another run or an older
	// export silently steers this one (the planner hard-fails only if a reused
	// handle collides). A dry-run legitimately pre-fills the file the real run
	// then resumes from, so only "completed" state without --resume is a WARN.
	minted, completed := cp.Stats()
	log.Info("checkpoint loaded", "path", cfg.CheckpointFile, "mintedHandles", minted, "completedResources", completed)
	if completed > 0 && !cfg.Resume && !cfg.VerifyOnly {
		log.Warn("checkpoint already records completed resources from a previous run but --resume is not set — "+
			"its minted handles will be reused anyway; for a NEW migration pass a fresh --checkpoint-file",
			"path", cfg.CheckpointFile, "completedResources", completed)
	}
	k, err := NewKernels(cfg, log, cp)
	if err != nil {
		return err
	}

	// Resolve endpoints by direction: forward Src=v1/Tgt=v2; reverse Src=v2/Tgt=v1.
	srcConn, tgtConn := cfg.V1, cfg.V2
	sourceIsV1 := true
	if cfg.Direction == DirectionReverse {
		srcConn, tgtConn = cfg.V2, cfg.V1
		sourceIsV1 = false
	}

	src, err := openDB(ctx, srcConn)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	var tgt *sql.DB
	if t, terr := openDB(ctx, tgtConn); terr != nil {
		if cfg.DryRun {
			log.Warn("target not reachable — dry-run continues without target existsCheck/verify", "error", terr.Error())
		} else {
			return fmt.Errorf("open target: %w", terr)
		}
	} else {
		tgt = t
		defer tgt.Close()
	}

	rc := &RunContext{
		Cfg:       cfg,
		Log:       log,
		Kernels:   k,
		CP:        cp,
		Src:       src,
		Tgt:       tgt,
		Direction: cfg.Direction,
		DryRun:    cfg.DryRun,
		BatchSize: cfg.EffectiveBatchSize(),
		Handles:   &HandleReport{},
	}
	_ = sourceIsV1

	migrators, err := selectedMigrators(cfg)
	if err != nil {
		return err
	}
	// Reverse keeps the SAME parents-first order: it still INSERTS rows (into v1),
	// so FK parents must exist before children — the dependency direction does not
	// flip with the data direction. Walking children-first (an undo order) broke a
	// v2-created org/project/application triple on `applications_organization_uuid_fkey`.

	// Preflight gates run only for a forward, writing run against the v1 source.
	if cfg.Direction == DirectionForward && !cfg.VerifyOnly {
		if err := runPreflight(ctx, cfg, k, src, tgt); err != nil {
			return err
		}
	}

	reports := make([]*ResourceReport, 0, len(migrators))
	for _, m := range migrators {
		name := m.Name()

		if cfg.Resume && !cfg.VerifyOnly && cp.IsComplete(name) {
			log.Info("resume: skipping already-complete resource", "resource", name)
			reports = append(reports, &ResourceReport{Name: name, Status: StatusSkip, Messages: []string{"already complete (resume)"}})
			continue
		}

		if !cfg.VerifyOnly {
			log.Info("migrating resource", "resource", name, "direction", cfg.Direction)
			if _, err := m.Migrate(ctx, rc); err != nil {
				if errors.Is(err, ErrReverseUnsupported) {
					log.Warn("reverse not supported for resource — skipping", "resource", name)
					reports = append(reports, &ResourceReport{Name: name, Status: StatusSkip, Messages: []string{"reverse unsupported (§13)"}})
					continue
				}
				log.Error("resource migrate failed", "resource", name, "error", err.Error())
				if !cfg.ContinueOnError {
					return fmt.Errorf("%s: %w", name, err)
				}
				reports = append(reports, &ResourceReport{Name: name, Status: StatusFail, Messages: []string{err.Error()}})
				continue
			}
			if !cfg.DryRun {
				if err := cp.MarkComplete(name); err != nil {
					log.Warn("failed to checkpoint completion", "resource", name, "error", err.Error())
				}
			}
		}

		// Verify (read-only) whenever a target is available.
		if tgt != nil {
			rep, verr := m.Verify(ctx, rc)
			if verr != nil {
				rep = &ResourceReport{Name: name, Status: StatusFail, Messages: []string{"verify error: " + verr.Error()}}
			}
			reports = append(reports, rep)
		} else {
			reports = append(reports, &ResourceReport{Name: name, Status: StatusSkip, Messages: []string{"no target — verify skipped (dry-run)"}})
		}
	}

	printReport(log, reports, cfg)

	// §B.14 handles report: every org/project uuid → v2 handle (+ source), so
	// the rows WITHOUT a Choreo handle can be resolved against Choreo by uuid.
	// Written for dry-runs too (the plan is deterministic), so the lookups can
	// happen before the real run. A write failure is logged, not fatal: the
	// migration itself is already complete at this point.
	if cfg.Direction == DirectionForward && !cfg.VerifyOnly && cfg.HandlesReport != "" && rc.Handles.Len() > 0 {
		if sum, err := rc.Handles.Write(cfg.HandlesReport); err != nil {
			log.Error("handles report NOT written", "path", cfg.HandlesReport, "error", err.Error())
		} else {
			log.Info("handles report written (share privately — handles are PII)", "path", cfg.HandlesReport,
				"organizations", sum.Orgs, "organizationsWithoutChoreoHandle", sum.OrgsNonChoreo,
				"projects", sum.Projects, "projectsWithoutChoreoHandle", sum.ProjectsNonChoreo, "projectsSuffixed", sum.ProjectsSuffixed)
		}
	}

	for _, r := range reports {
		if r.Status == StatusFail {
			return errors.New("migration completed with FAILURES — see report above")
		}
	}
	return nil
}
