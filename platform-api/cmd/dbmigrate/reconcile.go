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

// Targeted reconciliation (§8.1). `dbmigrate migrate -only-keys <file>` re-runs a
// specific set of rows through the SAME per-row migrationcore.UpsertX/DeleteX the batch
// uses — no new transform logic. It is the live dual-write path's reconcile step: an
// operator dumps the unreconciled `v2_dual_write_failures` rows to a keys file and
// replays them here. `-since` (no key list) triggers a full idempotent re-sync, which
// heals the enable-vs-backfill gap.
//
// UPSERT replays reuse the existing per-table iterators, filtered in-loop to the key
// set (see mc.want). DELETE replays go through migrationcore.DeleteX directly, because
// an upsert-reconcile can NEVER heal a delete: the v1 row is already gone, so the
// iterator would never see it and the orphaned v2 row would survive.

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/migrationcore"
)

// keyFilter is the reconciliation work list parsed from -only-keys.
type keyFilter struct {
	upserts map[string]map[string]bool // v2 table -> key -> present
	deletes []deleteKey                // ordered delete replays
}

type deleteKey struct{ table, key string }

// loadKeyFilter parses a -only-keys file. Each non-blank, non-comment ('#') line is
// "<op> <table> <key>" where op is "upsert" or "delete". key is the row's natural key;
// composite keys are "|"-joined in the DeleteX argument order (documented per table in
// dispatchDelete) — e.g. deployment_status = "<org>|<artifact>|<gateway>".
func loadKeyFilter(path string) (*keyFilter, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open -only-keys file: %w", err)
	}
	defer f.Close()

	kf := &keyFilter{upserts: map[string]map[string]bool{}}
	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.Fields(text)
		if len(fields) != 3 {
			return nil, fmt.Errorf("-only-keys line %d: want '<op> <table> <key>', got %q", lineNo, text)
		}
		op, table, key := fields[0], fields[1], fields[2]
		switch op {
		case "upsert":
			if kf.upserts[table] == nil {
				kf.upserts[table] = map[string]bool{}
			}
			kf.upserts[table][key] = true
		case "delete":
			kf.deletes = append(kf.deletes, deleteKey{table: table, key: key})
		default:
			return nil, fmt.Errorf("-only-keys line %d: unknown op %q (want upsert|delete)", lineNo, op)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return kf, nil
}

// want reports whether (table, key) is in the upsert work list. In batch mode
// (mc.only == nil) it always returns true, so the per-iterator guard is a no-op and
// batch output stays byte-identical. In a -since full re-sync mc.only is also nil, so
// every row is re-run.
func (mc *migCtx) want(table, key string) bool {
	if mc.only == nil {
		return true
	}
	return mc.only.upserts[table][key]
}

// reconcileDeletes replays each delete from the work list through migrationcore.DeleteX.
func (mc *migCtx) reconcileDeletes() error {
	if mc.only == nil {
		return nil
	}
	n := 0
	for _, d := range mc.only.deletes {
		if err := dispatchDelete(mc.v2, d.table, d.key); err != nil {
			return fmt.Errorf("reconcile delete %s %q: %w", d.table, d.key, err)
		}
		n++
	}
	if n > 0 {
		mc.run.logger.Info("reconciled deletes", "count", n)
	}
	return nil
}

// dispatchDelete routes a (table, key) to the matching migrationcore.DeleteX. The
// artifact type tables (rest_apis/llm_*/mcp_proxies/websub_apis/webbroker_apis) delete
// via DeleteArtifact so the v2 FK cascade removes the type row + children, mirroring the
// live path. Composite keys are "|"-joined in DeleteX argument order.
func dispatchDelete(v2 *database.DB, table, key string) error {
	p := strings.Split(key, "|")
	switch table {
	case "artifacts", "rest_apis", "llm_providers", "llm_proxies", "mcp_proxies", "websub_apis", "webbroker_apis":
		return migrationcore.DeleteArtifact(v2, p[0])
	case "organizations":
		return migrationcore.DeleteOrganization(v2, p[0])
	case "projects":
		return migrationcore.DeleteProject(v2, p[0])
	case "applications":
		return migrationcore.DeleteApplication(v2, p[0])
	case "subscription_plans":
		return migrationcore.DeleteSubscriptionPlan(v2, p[0])
	case "subscriptions":
		return migrationcore.DeleteSubscription(v2, p[0])
	case "gateways":
		return migrationcore.DeleteGateway(v2, p[0])
	case "gateway_tokens":
		return migrationcore.DeleteGatewayToken(v2, p[0])
	case "gateway_custom_policies":
		return migrationcore.DeleteGatewayCustomPolicy(v2, p[0])
	case "api_keys":
		return migrationcore.DeleteAPIKey(v2, p[0])
	case "deployments":
		return migrationcore.DeleteDeployment(v2, p[0])
	case "llm_provider_templates":
		return migrationcore.DeleteLLMProviderTemplate(v2, p[0])
	case "artifact_gateway_mappings": // <org>|<artifact>|<gateway>
		if len(p) != 3 {
			return fmt.Errorf("artifact_gateway_mappings key %q: want <org>|<artifact>|<gateway>", key)
		}
		return migrationcore.DeleteArtifactGatewayMapping(v2, p[0], p[1], p[2])
	case "application_api_key_mappings": // <application>|<apiKey>
		if len(p) != 2 {
			return fmt.Errorf("application_api_key_mappings key %q: want <application>|<apiKey>", key)
		}
		return migrationcore.DeleteApplicationAPIKeyMapping(v2, p[0], p[1])
	case "application_artifact_mappings": // <application>|<artifact>
		if len(p) != 2 {
			return fmt.Errorf("application_artifact_mappings key %q: want <application>|<artifact>", key)
		}
		return migrationcore.DeleteApplicationArtifactMapping(v2, p[0], p[1])
	case "gateway_custom_policy_usages": // <policy>|<artifact>
		if len(p) != 2 {
			return fmt.Errorf("gateway_custom_policy_usages key %q: want <policy>|<artifact>", key)
		}
		return migrationcore.DeletePolicyUsage(v2, p[0], p[1])
	case "deployment_status": // <org>|<artifact>|<gateway>
		if len(p) != 3 {
			return fmt.Errorf("deployment_status key %q: want <org>|<artifact>|<gateway>", key)
		}
		return migrationcore.DeleteDeploymentStatus(v2, p[0], p[1], p[2])
	default:
		return fmt.Errorf("unknown delete table %q", table)
	}
}
