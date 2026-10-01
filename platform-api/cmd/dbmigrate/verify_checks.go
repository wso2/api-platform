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
	"encoding/json"
	"fmt"
	"sort"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// perTypeKinds maps each v2 per-type table to its v1 artifacts.kind.
var perTypeKinds = []struct{ table, kind string }{
	{"rest_apis", constants.RestApi},
	{"llm_providers", constants.LLMProvider},
	{"llm_proxies", constants.LLMProxy},
	{"mcp_proxies", constants.MCPProxy},
	{"websub_apis", constants.WebSubApi},
	{"webbroker_apis", constants.WebBrokerApi},
}

// ---- Layer A: coverage counts ----

func (vr *verifier) layerA_counts() {
	// Split: each per-type count reconciles with v1 artifacts of that kind.
	var sumPerType int64
	for _, pt := range perTypeKinds {
		v1kind := vr.countWhere(vr.v1, "artifacts", "kind = '"+pt.kind+"'")
		v2t := vr.count(vr.v2, pt.table)
		sumPerType += v2t
		quar := vr.quarByTable[pt.table]
		vr.add(Check{Layer: "A", Name: "split artifacts[" + pt.kind + "] -> " + pt.table,
			Status: passIf(v2t+quar == v1kind), V1: i64(v1kind), V2: i64(v2t), Expected: i64(v1kind),
			Detail: fmt.Sprintf("v2(%d)+quarantine(%d)==v1(%d)", v2t, quar, v1kind)})
	}
	// Σ per-type == v2.artifacts.
	v2art := vr.count(vr.v2, "artifacts")
	vr.eq("A", "v2 artifacts == Σ per-type", v2art, sumPerType, "")

	// Derived tables.
	v1throttled := vr.countWhere(vr.v1, "subscription_plans", "throttle_limit_count IS NOT NULL AND throttle_limit_unit IS NOT NULL")
	vr.eq("A", "subscription_plan_limits == v1 throttled plans", vr.count(vr.v2, "subscription_plan_limits"), v1throttled, "")

	v1vhost := vr.countWhere(vr.v1, "gateways", "vhost IS NOT NULL AND vhost <> ''")
	vr.eq("A", "gateway_endpoints == v1 gateways with vhost", vr.count(vr.v2, "gateway_endpoints"), v1vhost, "")

	v1gwAssoc := vr.countWhere(vr.v1, "association_mappings", "association_type = 'gateway'")
	agm := vr.count(vr.v2, "artifact_gateway_mappings")
	vr.add(Check{Layer: "A", Name: "artifact_gateway_mappings == v1 gateway associations",
		Status: passIf(agm+vr.quarByTable["artifact_gateway_mappings"] == v1gwAssoc),
		V1: i64(v1gwAssoc), V2: i64(agm), Expected: i64(v1gwAssoc),
		Detail: fmt.Sprintf("v2(%d)+quar(%d)==v1(%d)", agm, vr.quarByTable["artifact_gateway_mappings"], v1gwAssoc)})

	// New v2 feature tables that must stay empty.
	vr.eq("A", "websub_api_hmac_secrets == 0", vr.count(vr.v2, "websub_api_hmac_secrets"), 0, "new v2 feature, no v1 source")
	vr.eq("A", "secrets == 0", vr.count(vr.v2, "secrets"), 0, "")
}

// ---- Layer B: scalar equivalence (uuid join, FULL) ----

type v1Art struct {
	name, version, kind, org string
	createdAt, updatedAt     sql.NullTime
}

func (vr *verifier) loadV1Artifacts() map[string]v1Art {
	m := map[string]v1Art{}
	rows, err := vr.v1.Query(`SELECT uuid, name, version, kind, organization_uuid, created_at, updated_at FROM artifacts`)
	if err != nil {
		vr.fail("B", "load v1 artifacts", err.Error())
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var uuid string
		var a v1Art
		if err := rows.Scan(&uuid, &a.name, &a.version, &a.kind, &a.org, &a.createdAt, &a.updatedAt); err != nil {
			vr.fail("B", "scan v1 artifacts", err.Error())
			return m
		}
		m[uuid] = a
	}
	if err := rows.Err(); err != nil {
		vr.fail("B", "iterate v1 artifacts", err.Error())
	}
	return m
}

func (vr *verifier) layerB_scalars() {
	arts := vr.loadV1Artifacts()

	// display_name == v1 artifacts.name, created_at instant preserved, kind == type.
	nameMismatch, tsMismatch, typeMismatch, orphan := 0, 0, 0, 0
	for _, pt := range perTypeKinds {
		rows, err := vr.v2.Query("SELECT uuid, display_name, created_at FROM " + pt.table)
		if err != nil {
			vr.fail("B", "read "+pt.table, err.Error())
			continue
		}
		for rows.Next() {
			var uuid, displayName string
			var createdAt sql.NullTime
			if err := rows.Scan(&uuid, &displayName, &createdAt); err != nil {
				vr.fail("B", "scan "+pt.table, err.Error())
				break
			}
			a, ok := arts[uuid]
			if !ok {
				orphan++
				continue
			}
			if a.name != displayName {
				nameMismatch++
			}
			if a.createdAt.Valid && createdAt.Valid && !reinterpretTZ(a.createdAt.Time, vr.loc).Equal(createdAt.Time.UTC()) {
				tsMismatch++
			}
		}
		rows.Close()
	}
	// kind == type via v2 artifacts.
	rows, err := vr.v2.Query("SELECT uuid, type FROM artifacts")
	if err == nil {
		for rows.Next() {
			var uuid, typ string
			if err := rows.Scan(&uuid, &typ); err != nil {
				vr.fail("B", "scan artifacts", err.Error())
				break
			}
			if a, ok := arts[uuid]; ok && a.kind != typ {
				typeMismatch++
			}
		}
		if err := rows.Err(); err != nil {
			vr.fail("B", "iterate artifacts", err.Error())
		}
		rows.Close()
	}
	vr.add(Check{Layer: "B", Name: "display_name == v1 artifacts.name", Status: passIf(nameMismatch == 0), Detail: fmt.Sprintf("%d mismatch", nameMismatch)})
	vr.add(Check{Layer: "B", Name: "created_at instant preserved", Status: passIf(tsMismatch == 0), Detail: fmt.Sprintf("%d mismatch", tsMismatch)})
	vr.add(Check{Layer: "B", Name: "v1 kind == v2 artifacts.type", Status: passIf(typeMismatch == 0), Detail: fmt.Sprintf("%d mismatch", typeMismatch)})
	vr.add(Check{Layer: "B", Name: "no v2 per-type row missing its v1 artifact", Status: passIf(orphan == 0), Detail: fmt.Sprintf("%d orphan", orphan)})

	// subscriptions.artifact_uuid == v1.api_uuid.
	v1sub := loadKV(vr.v1, "SELECT uuid, api_uuid FROM subscriptions")
	subMismatch := 0
	rs, err := vr.v2.Query("SELECT uuid, artifact_uuid FROM subscriptions")
	if err == nil {
		for rs.Next() {
			var uuid, art string
			if err := rs.Scan(&uuid, &art); err != nil {
				vr.fail("B", "scan subscriptions", err.Error())
				break
			}
			if v, ok := v1sub[uuid]; ok && v != art {
				subMismatch++
			}
		}
		if err := rs.Err(); err != nil {
			vr.fail("B", "iterate subscriptions", err.Error())
		}
		rs.Close()
	}
	vr.add(Check{Layer: "B", Name: "subscriptions api_uuid -> artifact_uuid", Status: passIf(subMismatch == 0), Detail: fmt.Sprintf("%d mismatch", subMismatch)})

	// subscription_plans.display_name == v1.plan_name.
	v1plan := loadKV(vr.v1, "SELECT uuid, plan_name FROM subscription_plans")
	planMismatch := 0
	rp, err := vr.v2.Query("SELECT uuid, display_name FROM subscription_plans")
	if err == nil {
		for rp.Next() {
			var uuid, dn string
			if err := rp.Scan(&uuid, &dn); err != nil {
				vr.fail("B", "scan subscription_plans", err.Error())
				break
			}
			if v, ok := v1plan[uuid]; ok && v != dn {
				planMismatch++
			}
		}
		if err := rp.Err(); err != nil {
			vr.fail("B", "iterate subscription_plans", err.Error())
		}
		rp.Close()
	}
	vr.add(Check{Layer: "B", Name: "subscription_plans plan_name -> display_name", Status: passIf(planMismatch == 0), Detail: fmt.Sprintf("%d mismatch", planMismatch)})
}

// ---- Layer C: transform round-trip (decode-and-compare, FULL) ----

type cfgRow struct {
	config    []byte
	transport string
}

func (vr *verifier) layerC_transforms() {
	websubPrep := func(b []byte) []byte { out, _, _ := preprocessWebSubRaw(b); return out }

	// Transport-bearing configs (v1 column moved into the blob).
	compareConfigs(vr, "rest_apis",
		vr.loadV1Cfg("rest_apis", true), vr.loadV2Cfg("rest_apis"),
		func(c *model.RestAPIConfig, tr string) {
			if p := parseTransport(tr); len(p) > 0 {
				c.Transport = p
			}
		},
		func(c *model.RestAPIConfig) []string { return c.Transport }, nil)
	compareConfigs(vr, "websub_apis",
		vr.loadV1Cfg("websub_apis", true), vr.loadV2Cfg("websub_apis"),
		func(c *model.WebSubAPIConfiguration, tr string) {
			if p := parseTransport(tr); len(p) > 0 {
				c.Transport = p
			}
		},
		func(c *model.WebSubAPIConfiguration) []string { return c.Transport }, websubPrep)
	compareConfigs(vr, "webbroker_apis",
		vr.loadV1Cfg("webbroker_apis", true), vr.loadV2Cfg("webbroker_apis"),
		func(c *model.WebBrokerAPIConfiguration, tr string) {
			if p := parseTransport(tr); len(p) > 0 {
				c.Transport = p
			}
		},
		func(c *model.WebBrokerAPIConfiguration) []string { return c.Transport }, nil)

	// Non-transport configs (re-marshal equality only).
	compareConfigs[model.LLMProviderConfig](vr, "llm_providers",
		vr.loadV1Cfg("llm_providers", false), vr.loadV2Cfg("llm_providers"), nil, nil, nil)
	compareConfigs[model.LLMProxyConfig](vr, "llm_proxies",
		vr.loadV1Cfg("llm_proxies", false), vr.loadV2Cfg("llm_proxies"), nil, nil, nil)
	compareConfigs[model.MCPProxyConfiguration](vr, "mcp_proxies",
		vr.loadV1Cfg("mcp_proxies", false), vr.loadV2Cfg("mcp_proxies"), nil, nil, nil)

	vr.checkPlanLimits()
	vr.checkGatewayEndpoints()
	vr.checkTokens()
}

func (vr *verifier) loadV1Cfg(table string, withTransport bool) map[string]cfgRow {
	m := map[string]cfgRow{}
	q := "SELECT uuid, configuration"
	if withTransport {
		q += ", transport"
	}
	q += " FROM " + table
	rows, err := vr.v1.Query(q)
	if err != nil {
		vr.fail("C", "load v1 cfg "+table, err.Error())
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var uuid string
		var cfg []byte
		var tr sql.NullString
		var scanErr error
		if withTransport {
			scanErr = rows.Scan(&uuid, &cfg, &tr)
		} else {
			scanErr = rows.Scan(&uuid, &cfg)
		}
		if scanErr != nil {
			vr.fail("C", "scan v1 cfg "+table, scanErr.Error())
			return m
		}
		m[uuid] = cfgRow{config: cfg, transport: tr.String}
	}
	return m
}

func (vr *verifier) loadV2Cfg(table string) map[string][]byte {
	m := map[string][]byte{}
	rows, err := vr.v2.Query("SELECT uuid, configuration FROM " + table)
	if err != nil {
		vr.fail("C", "load v2 cfg "+table, err.Error())
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var uuid string
		var cfg []byte
		if err := rows.Scan(&uuid, &cfg); err != nil {
			vr.fail("C", "scan v2 cfg "+table, err.Error())
			return m
		}
		m[uuid] = cfg
	}
	return m
}

// compareConfigs decodes each v2 config blob back into the v2 model, applies the
// documented forward transform to the v1 source, and asserts equality — the
// round-trip correctness gate. When getTransport != nil it additionally asserts
// the moved transport column landed inside the blob.
func compareConfigs[T any](vr *verifier, table string, v1 map[string]cfgRow, v2 map[string][]byte,
	inject func(*T, string), getTransport func(*T) []string, prep func([]byte) []byte) {
	total, decodeErr, mismatch, transportBad, missing := 0, 0, 0, 0, 0
	for uuid, v2b := range v2 {
		total++
		var t2 T
		if err := json.Unmarshal(v2b, &t2); err != nil {
			decodeErr++
			continue
		}
		v1r, ok := v1[uuid]
		if !ok {
			missing++
			continue
		}
		src := v1r.config
		if prep != nil {
			src = prep(src)
		}
		var t1 T
		if err := json.Unmarshal(src, &t1); err != nil {
			mismatch++
			continue
		}
		if inject != nil {
			inject(&t1, v1r.transport)
		}
		b1, _ := json.Marshal(t1)
		b2, _ := json.Marshal(t2)
		if string(b1) != string(b2) {
			mismatch++
		}
		if getTransport != nil {
			if !stringsEqual(getTransport(&t2), parseTransport(v1r.transport)) {
				transportBad++
			}
		}
	}
	vr.add(Check{Layer: "C", Name: "blob decodes into v2 model " + table, Status: passIf(decodeErr == 0),
		V2: i64(int64(total)), Detail: fmt.Sprintf("%d decode error(s) of %d", decodeErr, total)})
	vr.add(Check{Layer: "C", Name: "config round-trip invariants " + table, Status: passIf(mismatch == 0 && missing == 0),
		Detail: fmt.Sprintf("%d mismatch, %d missing-in-v1 of %d", mismatch, missing, total)})
	if getTransport != nil {
		vr.add(Check{Layer: "C", Name: "transport column -> blob " + table, Status: passIf(transportBad == 0),
			Detail: fmt.Sprintf("%d transport mismatch of %d", transportBad, total)})
	}
}

func (vr *verifier) checkPlanLimits() {
	// v1 plan -> throttle; v2 limits by plan.
	type thr struct {
		count sql.NullInt64
		unit  sql.NullString
		stop  sql.NullBool
	}
	v1 := map[string]thr{}
	rows, err := vr.v1.Query(`SELECT uuid, throttle_limit_count, throttle_limit_unit, stop_on_quota_reach FROM subscription_plans`)
	if err != nil {
		vr.fail("C", "load v1 plans", err.Error())
		return
	}
	for rows.Next() {
		var uuid string
		var t thr
		_ = rows.Scan(&uuid, &t.count, &t.unit, &t.stop)
		v1[uuid] = t
	}
	rows.Close()

	type lim struct {
		count             int64
		unit, ltype       string
		amount, stop      int64
	}
	v2 := map[string]lim{}
	lr, err := vr.v2.Query(`SELECT subscription_plan_uuid, limit_count, time_unit, limit_type, time_amount, stop_on_quota_reach FROM subscription_plan_limits`)
	if err != nil {
		vr.fail("C", "load v2 limits", err.Error())
		return
	}
	for lr.Next() {
		var plan string
		var l lim
		_ = lr.Scan(&plan, &l.count, &l.unit, &l.ltype, &l.amount, &l.stop)
		v2[plan] = l
	}
	lr.Close()

	bad, missing, spurious := 0, 0, 0
	for uuid, t := range v1 {
		hasThrottle := t.count.Valid && t.unit.Valid && t.unit.String != ""
		l, present := v2[uuid]
		if hasThrottle {
			if !present {
				missing++
				continue
			}
			wantUnit, _ := caseConvertThrottleUnit(t.unit.String)
			wantStop := int64(1)
			if t.stop.Valid {
				wantStop = int64(boolToSmallint(t.stop.Bool))
			}
			if l.count != t.count.Int64 || l.unit != wantUnit || l.ltype != constants.LimitTypeRequestCount || l.amount != 1 || l.stop != wantStop {
				bad++
			}
		} else if present {
			spurious++
		}
	}
	vr.add(Check{Layer: "C", Name: "subscription_plan_limits values", Status: passIf(bad == 0 && missing == 0 && spurious == 0),
		Detail: fmt.Sprintf("%d wrong, %d missing, %d spurious", bad, missing, spurious)})
}

func (vr *verifier) checkGatewayEndpoints() {
	v1 := loadKV(vr.v1, "SELECT uuid, vhost FROM gateways WHERE vhost IS NOT NULL AND vhost <> ''")
	rows, err := vr.v2.Query("SELECT gateway_uuid, url FROM gateway_endpoints")
	if err != nil {
		vr.fail("C", "load gateway_endpoints", err.Error())
		return
	}
	defer rows.Close()
	bad := 0
	seen := map[string]bool{}
	for rows.Next() {
		var gw, url string
		_ = rows.Scan(&gw, &url)
		seen[gw] = true
		if v, ok := v1[gw]; !ok || v != url {
			bad++
		}
	}
	missing := 0
	for gw := range v1 {
		if !seen[gw] {
			missing++
		}
	}
	vr.add(Check{Layer: "C", Name: "gateway_endpoints url == v1 vhost", Status: passIf(bad == 0 && missing == 0),
		Detail: fmt.Sprintf("%d wrong url, %d gateways missing endpoint", bad, missing)})
}

func (vr *verifier) checkTokens() {
	v1 := loadKV(vr.v1, "SELECT uuid, subscription_token FROM subscriptions")
	v1hash := loadKV(vr.v1, "SELECT uuid, subscription_token_hash FROM subscriptions")
	rows, err := vr.v2.Query("SELECT uuid, subscription_token, subscription_token_hash FROM subscriptions")
	if err != nil {
		vr.fail("C", "load v2 subscriptions", err.Error())
		return
	}
	defer rows.Close()
	tokBad, hashBad := 0, 0
	var sampleTok []string
	for rows.Next() {
		var uuid, tok, hash string
		_ = rows.Scan(&uuid, &tok, &hash)
		if v, ok := v1[uuid]; ok && v != tok {
			tokBad++
		}
		if v, ok := v1hash[uuid]; ok && v != hash {
			hashBad++
		}
		if len(sampleTok) < vr.opts.DecryptSampleSize {
			sampleTok = append(sampleTok, tok)
		}
	}
	vr.add(Check{Layer: "C", Name: "subscription_token bytes == v1 (verbatim)", Status: passIf(tokBad == 0), Detail: fmt.Sprintf("%d mismatch", tokBad)})
	vr.add(Check{Layer: "C", Name: "subscription_token_hash == v1", Status: passIf(hashBad == 0), Detail: fmt.Sprintf("%d mismatch", hashBad)})

	if vr.encKey != nil {
		fail := 0
		for _, tok := range sampleTok {
			if _, err := utils.DecryptSubscriptionToken(vr.encKey, tok); err != nil {
				fail++
			}
		}
		vr.add(Check{Layer: "C", Name: "sampled token decrypt round-trip (v2 key)", Status: passIf(fail == 0),
			Detail: fmt.Sprintf("%d/%d failed", fail, len(sampleTok))})
	} else {
		vr.warn("C", "sampled token decrypt round-trip", "skipped (no key provided)")
	}
}

// ---- Layer D: v2 referential integrity + uniqueness (FULL) ----

func (vr *verifier) layerD_integrity() {
	fks := []struct{ child, col, parent, pcol string }{
		{"rest_apis", "uuid", "artifacts", "uuid"},
		{"llm_providers", "uuid", "artifacts", "uuid"},
		{"llm_providers", "template_uuid", "llm_provider_templates", "uuid"},
		{"llm_proxies", "provider_uuid", "llm_providers", "uuid"},
		{"mcp_proxies", "uuid", "artifacts", "uuid"},
		{"websub_apis", "uuid", "artifacts", "uuid"},
		{"webbroker_apis", "uuid", "artifacts", "uuid"},
		{"subscriptions", "artifact_uuid", "artifacts", "uuid"},
		{"subscription_plan_limits", "subscription_plan_uuid", "subscription_plans", "uuid"},
		{"gateway_endpoints", "gateway_uuid", "gateways", "uuid"},
		{"artifact_gateway_mappings", "artifact_uuid", "artifacts", "uuid"},
		{"artifact_gateway_mappings", "gateway_uuid", "gateways", "uuid"},
		{"deployments", "artifact_uuid", "artifacts", "uuid"},
		{"deployment_status", "deployment_uuid", "deployments", "uuid"},
		{"gateway_custom_policy_usages", "artifact_uuid", "artifacts", "uuid"},
		{"api_keys", "artifact_uuid", "artifacts", "uuid"},
		{"application_api_key_mappings", "api_key_id", "api_keys", "uuid"},
		{"application_artifact_mappings", "artifact_uuid", "artifacts", "uuid"},
	}
	for _, fk := range fks {
		q := fmt.Sprintf("SELECT count(*) FROM %s c LEFT JOIN %s p ON c.%s = p.%s WHERE c.%s IS NOT NULL AND p.%s IS NULL",
			fk.child, fk.parent, fk.col, fk.pcol, fk.col, fk.pcol)
		var orphans int64
		if err := vr.v2.QueryRow(q).Scan(&orphans); err != nil {
			vr.fail("D", "FK "+fk.child+"."+fk.col, err.Error())
			continue
		}
		vr.add(Check{Layer: "D", Name: "FK " + fk.child + "." + fk.col + " -> " + fk.parent, Status: passIf(orphans == 0),
			Detail: fmt.Sprintf("%d orphan(s)", orphans)})
	}

	// Uniqueness: (organization_uuid, handle) per type table + others.
	uniq := []struct{ table, cols string }{
		{"organizations", "handle"},
		{"projects", "organization_uuid, handle"},
		{"applications", "organization_uuid, handle"},
		{"rest_apis", "organization_uuid, handle"},
		{"llm_providers", "organization_uuid, handle"},
		{"llm_proxies", "organization_uuid, handle"},
		{"mcp_proxies", "organization_uuid, handle"},
		{"websub_apis", "organization_uuid, handle"},
		{"webbroker_apis", "organization_uuid, handle"},
		{"subscription_plans", "organization_uuid, handle"},
		{"gateways", "organization_uuid, handle"},
		{"llm_provider_templates", "organization_uuid, group_id, version"},
		{"llm_provider_templates", "organization_uuid, handle"},
		{"api_keys", "artifact_uuid, handle"},
		{"user_idp_references", "idp_id"},
	}
	for _, u := range uniq {
		q := fmt.Sprintf("SELECT count(*) FROM (SELECT %s FROM %s GROUP BY %s HAVING count(*) > 1) d", u.cols, u.table, u.cols)
		var dups int64
		if err := vr.v2.QueryRow(q).Scan(&dups); err != nil {
			vr.fail("D", "unique "+u.table+"("+u.cols+")", err.Error())
			continue
		}
		vr.add(Check{Layer: "D", Name: "unique " + u.table + "(" + u.cols + ")", Status: passIf(dups == 0),
			Detail: fmt.Sprintf("%d duplicate group(s)", dups)})
	}
}

// ---- Layer E: generated / placeholder / default correctness ----

func (vr *verifier) layerE_generated() {
	// Carried handles: v2.handle == slug(v1 handle) for the common (non-suffixed) case.
	vr.checkCarriedHandles("organizations", "organizations")
	vr.checkCarriedHandles("applications", "applications")
	vr.checkCarriedHandles("llm_provider_templates", "llm_provider_templates")

	// idp_organization_ref_uuid == uuid.
	vr.eq("E", "idp_organization_ref_uuid == org uuid",
		vr.countWhere(vr.v2, "organizations", "idp_organization_ref_uuid <> uuid"), 0, "must be 0")

	// Template defaults.
	vr.eq("E", "template group_id == handle", vr.countWhere(vr.v2, "llm_provider_templates", "group_id <> handle"), 0, "")
	vr.eq("E", "template version == v1.0", vr.countWhere(vr.v2, "llm_provider_templates", "version <> 'v1.0'"), 0, "")
	vr.eq("E", "template managed_by == organization", vr.countWhere(vr.v2, "llm_provider_templates", "managed_by <> 'organization'"), 0, "")
	vr.eq("E", "template is_latest == 1", vr.countWhere(vr.v2, "llm_provider_templates", "is_latest <> 1"), 0, "")
	vr.eq("E", "template enabled == 1", vr.countWhere(vr.v2, "llm_provider_templates", "enabled <> 1"), 0, "")

	// data_version / origin defaults where the columns exist.
	for _, t := range []string{"organizations", "projects", "applications", "rest_apis", "llm_providers", "llm_proxies", "mcp_proxies", "websub_apis", "webbroker_apis", "subscription_plans", "gateways", "llm_provider_templates"} {
		vr.eq("E", "data_version=1.0 in "+t, vr.countWhere(vr.v2, t, "data_version <> '1.0'"), 0, "")
	}
	for _, t := range []string{"rest_apis", "llm_providers", "llm_proxies", "mcp_proxies", "websub_apis", "webbroker_apis", "llm_provider_templates"} {
		vr.eq("E", "origin=control_plane in "+t, vr.countWhere(vr.v2, t, "origin <> 'control_plane'"), 0, "")
	}

	// Audit identity: every non-null created_by/updated_by resolves in user_idp_references.
	auditTables := []string{"organizations", "projects", "applications", "rest_apis", "llm_providers",
		"llm_proxies", "mcp_proxies", "websub_apis", "webbroker_apis", "subscription_plans",
		"gateways", "gateway_custom_policies", "gateway_tokens", "deployments", "api_keys"}
	unresolved := int64(0)
	for _, t := range auditTables {
		unresolved += vr.countWhere(vr.v2, t, "created_by IS NOT NULL AND created_by NOT IN (SELECT uuid FROM user_idp_references)")
	}
	vr.eq("E", "all created_by resolve in user_idp_references", unresolved, 0, "across audited tables")
}

func (vr *verifier) checkCarriedHandles(v1table, v2table string) {
	v1 := loadKV(vr.v1, "SELECT uuid, handle FROM "+v1table)
	rows, err := vr.v2.Query("SELECT uuid, handle FROM " + v2table)
	if err != nil {
		vr.fail("E", "carried handle "+v2table, err.Error())
		return
	}
	defer rows.Close()
	mismatch, suffixed := 0, 0
	for rows.Next() {
		var uuid, h string
		_ = rows.Scan(&uuid, &h)
		src, ok := v1[uuid]
		if !ok {
			continue
		}
		if h != slug(src) {
			// A collision suffix or degenerate-short pad is expected/allowed; only a
			// wholesale divergence from the slug prefix is a real problem.
			if len(h) >= 5 && len(slug(src)) >= 3 && h[:min2(len(slug(src)), len(h))] != slug(src)[:min2(len(slug(src)), len(h))] {
				mismatch++
			} else {
				suffixed++
			}
		}
	}
	st := statusPass
	if mismatch > 0 {
		st = statusFail
	}
	vr.add(Check{Layer: "E", Name: "carried handle == slug(v1) in " + v2table, Status: st,
		Detail: fmt.Sprintf("%d divergent, %d collision/degenerate-suffixed (tolerated)", mismatch, suffixed)})
}

// ---- Layer F: drop reconciliation + quarantine sign-off gate ----

func (vr *verifier) layerF_dropReconcile() {
	drops := vr.loadDropsByFeature()

	// Intentional row-drops reconcile with v1 source counts.
	vr.eq("F", "drop devportals == v1 devportals", drops["devportals"], vr.count(vr.v1, "devportals"), "")
	vr.eq("F", "drop publication_mappings == v1 publication_mappings", drops["publication_mappings"], vr.count(vr.v1, "publication_mappings"), "")
	vr.eq("F", "drop dev_portal assoc == v1", drops["dev_portal_association"], vr.countWhere(vr.v1, "association_mappings", "association_type = 'dev_portal'"), "")
	// Field-drops reconcile with v1 source counts.
	vr.eq("F", "drop billing_plan == v1 plans with billing_plan", drops["billing_plan"], vr.countWhere(vr.v1, "subscription_plans", "billing_plan IS NOT NULL AND billing_plan <> ''"), "")
	vr.eq("F", "drop llm_provider_status == v1", drops["llm_provider_status"], vr.countWhere(vr.v1, "llm_providers", "status IS NOT NULL AND status <> ''"), "")
	vr.eq("F", "drop llm_proxy_status == v1", drops["llm_proxy_status"], vr.countWhere(vr.v1, "llm_proxies", "status IS NOT NULL AND status <> ''"), "")
	vr.eq("F", "drop mcp_status == v1", drops["mcp_status"], vr.countWhere(vr.v1, "mcp_proxies", "status IS NOT NULL AND status <> ''"), "")

	// 1:1 / renamed table reconciliation: v2 + quarantine == v1.
	recon := []struct{ v1t, v2t string }{
		{"organizations", "organizations"},
		{"projects", "projects"},
		{"applications", "applications"},
		{"subscription_plans", "subscription_plans"},
		{"subscriptions", "subscriptions"},
		{"gateways", "gateways"},
		{"gateway_custom_policies", "gateway_custom_policies"},
		{"gateway_custom_policy_usages", "gateway_custom_policy_usages"},
		{"gateway_tokens", "gateway_tokens"},
		{"deployments", "deployments"},
		{"deployment_status", "deployment_status"},
		{"api_keys", "api_keys"},
		{"application_api_keys", "application_api_key_mappings"},
		{"application_artifacts", "application_artifact_mappings"},
	}
	for _, r := range recon {
		v1c := vr.count(vr.v1, r.v1t)
		v2c := vr.count(vr.v2, r.v2t)
		quar := vr.quarByTable[r.v2t]
		vr.add(Check{Layer: "F", Name: "reconcile " + r.v1t, Status: passIf(v2c+quar == v1c),
			V1: i64(v1c), V2: i64(v2c), Expected: i64(v1c),
			Detail: fmt.Sprintf("v2(%d)+quarantine(%d)==v1(%d)", v2c, quar, v1c)})
	}

	// Quarantine sign-off gate: every quarantined key must be signed off
	// (quarantine-signoff.jsonl). "Resolved by a re-run" is handled implicitly — a
	// key fixed at source no longer appears in the latest run's quarantine file, so
	// it never reaches this gate; what remains here must be explicitly accepted as loss.
	unresolved := 0
	var sample []string
	for key := range vr.quarKeys {
		if vr.signoff[key] {
			continue
		}
		unresolved++
		if len(sample) < 20 {
			sample = append(sample, key)
		}
	}
	st := statusPass
	detail := fmt.Sprintf("all %d quarantined key(s) signed off", len(vr.quarKeys))
	if unresolved > 0 {
		st = statusFail
		detail = fmt.Sprintf("%d quarantined key(s) NOT signed off (e.g. %v)", unresolved, sample)
	}
	vr.add(Check{Layer: "F", Name: "quarantine sign-off", Status: st, Detail: detail})
}

func (vr *verifier) loadDropsByFeature() map[string]int64 {
	m := map[string]int64{}
	path := filepathJoin(vr.opts.OutDir, suffixed("drops", vr.opts.RunID, false, "jsonl"))
	forEachJSONL(path, func(rec map[string]any) {
		if f, ok := rec["feature"].(string); ok {
			m[f]++
		}
	})
	return m
}

// ---- small helpers ----

func passIf(ok bool) string {
	if ok {
		return statusPass
	}
	return statusFail
}

func loadKV(db *database.DB, query string) map[string]string {
	m := map[string]string{}
	rows, err := db.Query(query)
	if err != nil {
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var v sql.NullString
		if rows.Scan(&k, &v) == nil {
			m[k] = v.String
		}
	}
	return m
}

func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sa := append([]string(nil), a...)
	sb := append([]string(nil), b...)
	sort.Strings(sa)
	sort.Strings(sb)
	for i := range sa {
		if sa[i] != sb[i] {
			return false
		}
	}
	return true
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// filepathJoin builds an output-file path (out-dir is always a plain local dir).
func filepathJoin(dir, name string) string { return dir + "/" + name }
