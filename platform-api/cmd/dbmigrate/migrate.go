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
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/utils"
	"github.com/wso2/api-platform/platform-api/migrationcore"
)

// migCtx bundles the shared state threaded through every table migration.
type migCtx struct {
	v1, v2 *database.DB
	run    *Run
	im     *IdentityMap
	sets   *Sets
	h      *handleGen
	opts   *Options
	loc    *time.Location
	encKey []byte
	epoch  time.Time

	// artifactPlans is collected only when -populate-artifact-subscription-plans is
	// set: (artifact, org, plan handles from the config blob) for later resolution.
	artifactPlans []artifactPlanRef

	// Subscription dedup sets for v2's stricter uniqueness (§G).
	subAppSeen  map[string]bool // org|artifact|application_id
	subHashSeen map[string]bool // artifact|token_hash

	// Targeted-reconciliation state (§8.1). reconcile flips coreOpts to the live
	// semantics (DO UPDATE + on-demand identity upsert); only, when non-nil, filters
	// the iterators to the -only-keys work list. Both are zero/nil for the batch, so
	// batch behaviour is byte-identical.
	reconcile bool
	only      *keyFilter
}

// deterministicUUID is a thin alias over the product's deterministic UUIDv7 helper
// so synthesized PKs are idempotent/resumable without persisted state.
func deterministicUUID(entity string, ts time.Time) string {
	return utils.GenerateDeterministicUUIDv7(entity, ts)
}

// coreOpts builds the shared-core Options. For the BATCH caller: insert-only
// (ON CONFLICT DO NOTHING) and skip per-row identity upserts (user_idp_references is
// bulk pre-seeded). For a RECONCILE run (§8.1) it uses the live dual-write semantics —
// InsertOnly:false (DO UPDATE, so changed rows are healed, not skipped) and
// SkipIdentityUpsert:false (seed the actor on demand, no bulk pre-seed).
func (mc *migCtx) coreOpts() migrationcore.Options {
	return migrationcore.Options{
		EncryptionKey:      mc.encKey,
		SourceTZ:           mc.loc,
		Epoch:              mc.epoch,
		InsertOnly:         !mc.reconcile,
		SkipIdentityUpsert: !mc.reconcile,
		DryRun:             mc.opts.DryRun,
	}
}

// nsp / ntp convert scanned nullable columns into the pointer form V1Row uses.
func nsp(ns sql.NullString) *string {
	if ns.Valid {
		s := ns.String
		return &s
	}
	return nil
}
func ntp(nt sql.NullTime) *time.Time {
	if nt.Valid {
		t := nt.Time
		return &t
	}
	return nil
}

// insert is a dry-run-guarded idempotent INSERT used only by the batch-only
// optional tables (audit markers, artifact_subscription_plans) that migrationcore
// does not own. The migrated per-type entities go through migrationcore.UpsertX.
func (mc *migCtx) insert(table string, cols []string, args []any, conflict string) error {
	if mc.opts.DryRun {
		return nil
	}
	return insertRow(mc.v2, table, cols, args, conflict)
}

type artifactPlanRef struct {
	artifactUUID string
	org          string
	planHandles  []string
}

func runMigrate(argv []string) error {
	o := &Options{}
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	registerCommonFlags(fs, o)
	fs.BoolVar(&o.DryRun, "dry-run", false, "Transform and validate everything but perform no inserts")
	fs.BoolVar(&o.InitSchema, "init-schema", true, "Apply the v2 core + plugin DDL before migrating (idempotent)")
	fs.StringVar(&o.CoreSchema, "core-schema",
		"internal/database/schema.postgres.sql", "Path to the v2 core PostgreSQL DDL")
	fs.StringVar(&o.PluginSchema, "plugin-schema",
		"plugins/eventgateway/schema/schema.postgres.sql", "Path to the EventGateway plugin PostgreSQL DDL")
	fs.StringVar(&o.SourceTZ, "source-tz", "UTC", "Time zone of naive v1 TIMESTAMP values")
	fs.IntVar(&o.BatchSize, "batch-size", 1000, "Rows per progress checkpoint")
	fs.StringVar(&o.IDPRefStrategy, "idp-ref-strategy", "org-uuid", "Strategy for organizations.idp_organization_ref_uuid (org-uuid)")
	fs.StringVar(&o.GroupIDStrategy, "group-id-strategy", "handle", "Strategy for llm_provider_templates.group_id (handle)")
	epochStr := fs.String("migration-epoch", defaultMigrationEpoch, "Fixed epoch (RFC3339) for deterministic synthesized UUIDs")
	fs.BoolVar(&o.PopulateArtifactSubPlans, "populate-artifact-subscription-plans", false, "Derive artifact_subscription_plans rows from config blobs (redundant; off by default)")
	fs.BoolVar(&o.AuditMarker, "audit-marker", false, "Emit one 'migrated' audit row per organization")
	fs.BoolVar(&o.SkipDecryptCheck, "skip-decrypt-check", false, "Skip the mandatory subscription-token decrypt guard (dry-runs without the key)")
	fs.IntVar(&o.DecryptSampleSize, "decrypt-sample-size", 25, "Number of subscription tokens to sample for the decrypt guard")
	fs.StringVar(&o.EncKeyFile, "encryption-key-file", "", "File containing the subscription-token key (alternative to APIP_MIGRATION_ENCRYPTION_KEY)")
	fs.StringVar(&o.OnlyKeys, "only-keys", "", "Reconcile (§8.1): path to a file of '<op> <table> <key>' lines (op=upsert|delete) to replay through the shared UpsertX/DeleteX; enables live upsert semantics (DO UPDATE)")
	fs.StringVar(&o.Since, "since", "", "Reconcile (§8.1): full idempotent re-sync of all rows (heals the enable-vs-backfill gap); RFC3339 marker, logged for the operator")
	if err := fs.Parse(argv); err != nil {
		return err
	}
	if err := validateCommon(o); err != nil {
		return err
	}

	epoch, err := time.Parse(time.RFC3339, *epochStr)
	if err != nil {
		return fmt.Errorf("invalid -migration-epoch: %w", err)
	}
	loc, err := time.LoadLocation(o.SourceTZ)
	if err != nil {
		return fmt.Errorf("invalid -source-tz %q: %w", o.SourceTZ, err)
	}
	// The subscription-token decrypt guard is mandatory for a live run; only a
	// --dry-run may skip it (it validates tokens against the real key before writes).
	if o.SkipDecryptCheck && !o.DryRun {
		return fmt.Errorf("-skip-decrypt-check is only allowed with -dry-run; a live run must validate subscription tokens against the real key")
	}
	if err := loadEncryptionKey(o); err != nil {
		return err
	}
	if o.EncryptionKey == nil && !o.SkipDecryptCheck {
		return fmt.Errorf("no encryption key: set APIP_MIGRATION_ENCRYPTION_KEY or -encryption-key-file for the mandatory token decrypt guard, or pass -skip-decrypt-check (dry-runs only)")
	}

	logger := newLogger(o)

	// Reconcile mode (§8.1): -only-keys replays a targeted work list through the shared
	// UpsertX/DeleteX; -since (no key list) re-runs every row (a full idempotent re-sync).
	// Either flag flips coreOpts to the live semantics via migCtx.reconcile.
	reconcile := o.OnlyKeys != "" || o.Since != ""
	var only *keyFilter
	if o.OnlyKeys != "" {
		only, err = loadKeyFilter(o.OnlyKeys)
		if err != nil {
			return err
		}
	}
	if reconcile {
		logger.Info("reconcile mode: live upsert semantics (DO UPDATE + on-demand identity)",
			"only_keys", o.OnlyKeys, "since", o.Since)
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

	if o.InitSchema && !o.DryRun {
		if err := applySchemas(v2, o, logger); err != nil {
			return err
		}
	}

	// Preflight: fail-fast on any artifact kind outside the six migrated types.
	if err := preflightKinds(v1); err != nil {
		return err
	}

	run, err := newRun(o, logger)
	if err != nil {
		return err
	}
	defer run.close()

	// Preflight: mandatory subscription-token decrypt guard (fail the whole run if
	// any sampled token does not round-trip under the configured key).
	if !o.SkipDecryptCheck {
		if err := checkTokenDecryption(v1, o.EncryptionKey, o.DecryptSampleSize, logger); err != nil {
			return fmt.Errorf("subscription-token decrypt guard FAILED (wrong key ⇒ all tokens garbage): %w", err)
		}
		logger.Info("subscription-token decrypt guard passed")
	} else {
		logger.Warn("skipping subscription-token decrypt guard (-skip-decrypt-check)")
	}

	var im *IdentityMap
	if reconcile {
		// Reconcile seeds each actor on demand (SkipIdentityUpsert:false in coreOpts), so
		// the bulk pre-seed is skipped. A minimal map carries only the migration actor for
		// the optional tables, which reconcile does not run.
		im = &IdentityMap{epoch: epoch, m: map[string]string{}}
		im.actorUUID = utils.GenerateDeterministicUUIDv7(migrationActorIDPID, epoch)
		im.m[migrationActorIDPID] = im.actorUUID
	} else {
		im, err = collectAndSeedIdentities(v1, v2, run, epoch)
		if err != nil {
			return err
		}
	}

	mc := &migCtx{
		v1: v1, v2: v2, run: run, im: im,
		sets: newSets(), h: newHandleGen(run),
		opts: o, loc: loc, encKey: o.EncryptionKey, epoch: epoch,
		reconcile: reconcile, only: only,
	}

	// Migrate in FK order. Each step checkpoints on completion.
	steps := []struct {
		name string
		fn   func(*migCtx) error
	}{
		{"organizations", migrateOrganizations},
		{"projects", migrateProjects},
		{"applications", migrateApplications},
		{"rest_apis", migrateRestAPIs},
		{"llm_provider_templates", migrateLLMProviderTemplates},
		{"llm_providers", migrateLLMProviders},
		{"llm_proxies", migrateLLMProxies},
		{"mcp_proxies", migrateMCPProxies},
		{"websub_apis", migrateWebSubAPIs},
		{"webbroker_apis", migrateWebBrokerAPIs},
		{"subscription_plans", migrateSubscriptionPlans},
		{"subscriptions", migrateSubscriptions},
		{"gateways", migrateGateways},
		{"artifact_gateway_mappings", migrateArtifactGatewayMappings},
		{"gateway_custom_policies", migrateGatewayCustomPolicies},
		{"gateway_custom_policy_usages", migrateGatewayCustomPolicyUsages},
		{"gateway_tokens", migrateGatewayTokens},
		{"deployments", migrateDeployments},
		{"deployment_status", migrateDeploymentStatus},
		{"api_keys", migrateAPIKeys},
		{"application_api_key_mappings", migrateApplicationAPIKeyMappings},
		{"application_artifact_mappings", migrateApplicationArtifactMappings},
		{"artifact_subscription_plans", migrateArtifactSubscriptionPlans},
		{"audit", migrateAuditMarkers},
		{"devportals", dropDevportals},
		{"publication_mappings", dropPublicationMappings},
	}
	if reconcile {
		// A reconcile replay only re-runs the core per-entity upserts. The enumerate-only
		// §H drop steps and the optional batch-only tables (audit markers,
		// artifact_subscription_plans) are not part of a live mirror.
		skip := map[string]bool{
			"artifact_subscription_plans": true,
			"audit":                       true,
			"devportals":                  true,
			"publication_mappings":        true,
		}
		kept := steps[:0:0]
		for _, s := range steps {
			if !skip[s.name] {
				kept = append(kept, s)
			}
		}
		steps = kept
	}

	for _, s := range steps {
		logger.Info("migrating", "table", s.name, "mode", run.mode)
		if err := s.fn(mc); err != nil {
			_ = run.saveCheckpoint()
			_ = run.writeReport()
			return fmt.Errorf("migrate %s: %w", s.name, err)
		}
		run.markTableComplete(s.name)
		if err := run.saveCheckpoint(); err != nil {
			return fmt.Errorf("checkpoint after %s: %w", s.name, err)
		}
	}

	// Replay tombstoned deletes last: an upsert-reconcile can never heal a delete (the v1
	// row is gone), so DeleteX is applied directly against v2 (§8.1, delete-reconcile gap).
	if reconcile {
		if err := mc.reconcileDeletes(); err != nil {
			_ = run.writeReport()
			return err
		}
	}

	if err := run.writeReport(); err != nil {
		return err
	}
	logger.Info("migration complete", "mode", run.mode, "run_id", o.RunID, "out_dir", o.OutDir)
	printReportSummary(run.report)
	return nil
}

// loadEncryptionKey resolves the 32-byte key from env or file (never a flag).
func loadEncryptionKey(o *Options) error {
	raw := os.Getenv("APIP_MIGRATION_ENCRYPTION_KEY")
	if raw == "" && o.EncKeyFile != "" {
		b, err := os.ReadFile(o.EncKeyFile)
		if err != nil {
			return fmt.Errorf("read encryption-key-file: %w", err)
		}
		raw = strings.TrimSpace(string(b))
	}
	if raw == "" {
		return nil
	}
	key, err := utils.DeriveEncryptionKey(raw)
	if err != nil {
		return fmt.Errorf("encryption key: %w", err)
	}
	o.EncryptionKey = key
	return nil
}

// applySchemas applies the v2 core DDL then the EventGateway plugin DDL. Both are
// idempotent (CREATE TABLE IF NOT EXISTS). The plugin DDL is NOT auto-applied for
// Postgres by the product, so the tool applies it every run (§K.2).
func applySchemas(v2 *database.DB, o *Options, logger *slog.Logger) error {
	core, err := os.ReadFile(o.CoreSchema)
	if err != nil {
		return fmt.Errorf("read core schema %s: %w", o.CoreSchema, err)
	}
	if err := v2.InitSchemaSQL(string(core), logger); err != nil {
		return fmt.Errorf("apply core schema: %w", err)
	}
	plugin, err := os.ReadFile(o.PluginSchema)
	if err != nil {
		return fmt.Errorf("read plugin schema %s: %w", o.PluginSchema, err)
	}
	if err := v2.InitSchemaSQL(string(plugin), logger); err != nil {
		return fmt.Errorf("apply plugin schema: %w", err)
	}
	logger.Info("applied v2 core + plugin DDL")
	return nil
}

// preflightKinds fails fast if v1 holds any artifact kind outside the six migrated
// types (unknown destination ⇒ mapping incomplete).
func preflightKinds(v1 *database.DB) error {
	set, err := loadStringSet(v1, "SELECT DISTINCT kind FROM artifacts")
	if err != nil {
		return fmt.Errorf("preflight kinds: %w", err)
	}
	valid := map[string]bool{
		constants.RestApi: true, constants.LLMProvider: true, constants.LLMProxy: true,
		constants.MCPProxy: true, constants.WebSubApi: true, constants.WebBrokerApi: true,
	}
	for k := range set {
		if !valid[k] {
			return fmt.Errorf("fail-fast: v1 artifacts contains unmapped kind %q (expected one of RestApi/LlmProvider/LlmProxy/Mcp/WebSubApi/WebBrokerApi)", k)
		}
	}
	return nil
}

// checkTokenDecryption samples subscription tokens and fails if any does not
// round-trip under the configured key.
func checkTokenDecryption(v1 *database.DB, key []byte, sample int, logger *slog.Logger) error {
	if sample <= 0 {
		sample = 25
	}
	rows, err := v1.Query(v1.Rebind(
		fmt.Sprintf("SELECT uuid, subscription_token FROM subscriptions WHERE subscription_token <> '' ORDER BY uuid %s", v1.FetchFirstClause(sample))))
	if err != nil {
		return err
	}
	defer rows.Close()
	checked := 0
	for rows.Next() {
		var uuid, token string
		if err := rows.Scan(&uuid, &token); err != nil {
			return err
		}
		if _, err := utils.DecryptSubscriptionToken(key, token); err != nil {
			return fmt.Errorf("token for subscription %s did not decrypt: %w", uuid, err)
		}
		checked++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	logger.Info("decrypt guard sampled tokens", "checked", checked)
	return nil
}

// carriedHandle generates+persists the handle for a table that CARRIES the v1
// handle (source = v1 handle), flagging TRUNCATED when the 255→40 narrowing bites.
func (mc *migCtx) carriedHandle(table, org, v1uuid, v1handle string) (string, error) {
	h, err := mc.h.generate(table, org, v1uuid, v1handle)
	if err != nil {
		return "", err
	}
	if _, cut := truncateStr(v1handle, 40); cut || slug(v1handle) != h {
		mc.run.flag(table, v1uuid, FlagTruncated,
			map[string]any{"handle": v1handle, "len": len(v1handle)},
			map[string]any{"handle": h})
	}
	return h, nil
}

