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
	"log/slog"
)

// ErrReverseUnsupported is returned by a migrator whose forward transform is not
// invertible (a v2-NEW resource with no v1 home, §13). The runner logs it and
// continues rather than failing the whole reverse run.
var ErrReverseUnsupported = errors.New("reverse migration not supported for this resource")

// RunContext is the per-run environment handed to every migrator. Src/Tgt are
// already role-resolved for the direction: forward → Src=v1, Tgt=v2; reverse →
// Src=v2, Tgt=v1.
type RunContext struct {
	Cfg     *Config
	Log     *slog.Logger
	Kernels *Kernels
	CP      *Checkpoint

	Src *sql.DB
	Tgt *sql.DB

	Direction Direction
	DryRun    bool
	BatchSize int

	// Handles collects every organization/project handle decision of a forward
	// run; the runner writes it to Cfg.HandlesReport at the end (§B.14).
	Handles *HandleReport
}

// writes reports whether this run should perform target writes.
func (rc *RunContext) writes() bool { return !rc.DryRun }

// ResourceMigrator is the common interface every per-resource migrator satisfies.
type ResourceMigrator interface {
	// Name is the stable resource identifier used for ordering, selection and
	// reporting (matches the DB_MAPPING resource / v2 table name).
	Name() string
	// DependsOn lists resource names whose §7 predecessors must precede this one,
	// used to FK-dependency-check a partial --resources selection.
	DependsOn() []string
	// Migrate moves data for rc.Direction (no target writes when rc.DryRun).
	Migrate(ctx context.Context, rc *RunContext) (*ResourceReport, error)
	// Verify runs the §9 read-only checks and returns a report.
	Verify(ctx context.Context, rc *RunContext) (*ResourceReport, error)
}

// Status is the per-resource verification verdict.
type Status string

const (
	StatusPass Status = "PASS"
	StatusWarn Status = "WARN"
	StatusFail Status = "FAIL"
	StatusSkip Status = "SKIP"
)

// ResourceReport is the per-resource outcome, aggregated into the final report.
type ResourceReport struct {
	Name     string
	Status   Status
	SrcCount int64
	TgtCount int64
	Messages []string
}

func (r *ResourceReport) addf(format string, a ...any) {
	r.Messages = append(r.Messages, fmt.Sprintf(format, a...))
}

// fail sets FAIL and records a message.
func (r *ResourceReport) fail(format string, a ...any) {
	r.Status = StatusFail
	r.addf(format, a...)
}

// warn downgrades to WARN (never overrides a FAIL) and records a message.
func (r *ResourceReport) warn(format string, a ...any) {
	if r.Status != StatusFail {
		r.Status = StatusWarn
	}
	r.addf(format, a...)
}

// orderedResourceNames is the FK-safe migration order (DB_MAPPING §a / prompt §7).
// The runner walks this order in BOTH directions (reverse still inserts, so
// parents must exist first — see runner.go).
var orderedResourceNames = []string{
	"user_idp_references",
	"organizations",
	"user_organization_mappings", // intentionally empty (§B.4) — no-op reporter
	"projects",
	"applications",
	"subscription_plans", // + subscription_plan_limits child
	"artifacts",          // base split; per-type rows follow
	"rest_apis",
	"llm_provider_templates",
	"llm_providers",
	"llm_proxies",
	"mcp_proxies",
	"websub_apis",    // eventgateway plugin
	"webbroker_apis", // eventgateway plugin
	"gateways",       // + gateway_endpoints child
	"gateway_tokens",
	"gateway_custom_policies", // + usages child
	"api_keys",
	"artifact_gateway_mappings",
	"deployments",
	"deployment_status",
	"subscriptions",
	"application_artifact_mappings", // late: FK -> artifacts
	"application_api_key_mappings",  // late: FK -> api_keys
	"artifact_subscription_plans",   // no v2 writer — migrate empty (no-op reporter)
	"secrets",                       // emitted inline during artifact migration; verify-only pass
}

// registry maps resource name -> migrator. Resource files register themselves in
// their init(); AllMigrators() returns them in orderedResourceNames order.
var registry = map[string]ResourceMigrator{}

func register(m ResourceMigrator) {
	if _, dup := registry[m.Name()]; dup {
		panic("duplicate migrator registered: " + m.Name())
	}
	registry[m.Name()] = m
}

// AllMigrators returns the registered migrators in FK order. Names in
// orderedResourceNames with no registered migrator yet are skipped (lets the
// client be built up incrementally without breaking the runner).
func AllMigrators() []ResourceMigrator {
	out := make([]ResourceMigrator, 0, len(orderedResourceNames))
	for _, name := range orderedResourceNames {
		if m, ok := registry[name]; ok {
			out = append(out, m)
		}
	}
	return out
}

// selectedMigrators applies --resources/--skip and FK-dependency-checks the
// selection: a chosen resource whose predecessors are neither selected nor known
// to be present is a hard error (would break FK integrity, §12).
func selectedMigrators(cfg *Config) ([]ResourceMigrator, error) {
	all := AllMigrators()
	if len(cfg.Resources) == 0 && len(cfg.Skip) == 0 {
		return all, nil
	}
	inScope := map[string]bool{}
	var chosen []ResourceMigrator
	for _, m := range all {
		if cfg.selected(m.Name()) {
			inScope[m.Name()] = true
			chosen = append(chosen, m)
		}
	}
	// FK-dependency check: every dependency of a chosen resource must also be in
	// scope (else it must already be present in the target — we cannot know that
	// statically, so we warn loudly rather than silently proceed).
	var missing []string
	for _, m := range chosen {
		for _, dep := range m.DependsOn() {
			if !inScope[dep] {
				missing = append(missing, fmt.Sprintf("%s needs %s", m.Name(), dep))
			}
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("selection breaks FK order — the following predecessors are not selected "+
			"(include them, or ensure they are already migrated in the target): %v", missing)
	}
	return chosen, nil
}
