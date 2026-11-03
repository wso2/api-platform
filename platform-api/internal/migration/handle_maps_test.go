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
	"strings"
	"testing"
)

func TestParseHandleMap_OrgExportShape(t *testing.T) {
	src := "org_uuid,org_handle,org_status,org_name\n" +
		"4DDD1F0E-d881-4f0a-b7a6-e25448a9c4f1,saasorg,ACTIVE,SaasOrg\n" +
		"0b5e4ca5-e20e-4397-8351-f72464ab5896,dewni.com,INACTIVE,\"Dewni, Inc\"\n"
	m, err := parseHandleMap("organizations", "o.csv", strings.NewReader(src), orgUUIDHeaders, orgHandleHeaders)
	if err != nil {
		t.Fatal(err)
	}
	if m.Rows != 2 || m.Len() != 2 {
		t.Fatalf("rows=%d len=%d", m.Rows, m.Len())
	}
	// uuid lookup is case-insensitive; the handle is verbatim.
	if h, ok := m.Lookup("4ddd1f0e-d881-4f0a-b7a6-e25448a9c4f1"); !ok || h != "saasorg" {
		t.Fatalf("lookup: %q %v", h, ok)
	}
	if h, _ := m.Lookup("0b5e4ca5-e20e-4397-8351-f72464ab5896"); h != "dewni.com" {
		t.Fatalf("handle must be verbatim, got %q", h)
	}
	if s := m.StatusOf("0b5e4ca5-e20e-4397-8351-f72464ab5896"); s != "INACTIVE" {
		t.Fatalf("status: %q", s)
	}
	// org_name is carried for display_name — quoted commas survive, and it is verbatim.
	if n := m.NameOf("0b5e4ca5-e20e-4397-8351-f72464ab5896"); n != "Dewni, Inc" {
		t.Fatalf("name: %q", n)
	}
	if n := m.NameOf("4ddd1f0e-d881-4f0a-b7a6-e25448a9c4f1"); n != "SaasOrg" {
		t.Fatalf("name: %q", n)
	}
	if _, ok := m.Lookup("missing"); ok {
		t.Fatal("unexpected hit")
	}
}

func TestParseHandleMap_ProjectExportShape_EmptyHandlerIsUnmapped(t *testing.T) {
	src := "project_uuid,project_handler\n" +
		"85b7274d-90f1-402d-95c4-f1cd5c42000c,thushani-hybrdgw\n" +
		"58e081c7-9ece-4096-acc3-8cbd802d3ca1,\n" +
		"1313e957-8e67-4fc2-99b1-02f6733f49cc,tjZD\n"
	m, err := parseHandleMap("projects", "p.csv", strings.NewReader(src), projUUIDHeaders, projHandleHeaders)
	if err != nil {
		t.Fatal(err)
	}
	if m.Len() != 2 || m.EmptyCount() != 1 {
		t.Fatalf("len=%d empty=%d", m.Len(), m.EmptyCount())
	}
	if n := m.NameOf("85b7274d-90f1-402d-95c4-f1cd5c42000c"); n != "" {
		t.Fatalf("project export has no name column; got %q", n)
	}
	if _, ok := m.Lookup("58e081c7-9ece-4096-acc3-8cbd802d3ca1"); ok {
		t.Fatal("empty handler must be unmapped")
	}
	// An uppercase Choreo handler is kept verbatim and only flagged (WARN, not an error).
	if h, _ := m.Lookup("1313e957-8e67-4fc2-99b1-02f6733f49cc"); h != "tjZD" {
		t.Fatalf("got %q", h)
	}
	if handleFormatIssue("tjZD") == "" || handleFormatIssue("dewni.com") == "" {
		t.Fatal("expected v2 format issues for uppercase / dotted handles")
	}
	if issue := handleFormatIssue("thushani-hybrdgw"); issue != "" {
		t.Fatalf("clean handle flagged: %s", issue)
	}
}

func TestParseHandleMap_Rejects(t *testing.T) {
	cases := map[string]string{
		"missing columns": "uuid_x,handler_x\na,b\n",
		"over-width":      "project_uuid,project_handler\nu1," + strings.Repeat("a", 41) + "\n",
		"conflicting dup": "project_uuid,project_handler\nu1,one\nu1,two\n",
	}
	for name, src := range cases {
		if _, err := parseHandleMap("projects", name, strings.NewReader(src), projUUIDHeaders, projHandleHeaders); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	// An exact duplicate row is harmless.
	if _, err := parseHandleMap("projects", "dup", strings.NewReader("project_uuid,project_handler\nu1,one\nu1,one\n"), projUUIDHeaders, projHandleHeaders); err != nil {
		t.Fatalf("exact dup: %v", err)
	}
}

func TestPlanProjectHandles_ChoreoWinsAndFallbackIsSuffixed(t *testing.T) {
	k := testKernels(t)
	m, err := parseHandleMap("projects", "p.csv", strings.NewReader(
		"project_uuid,project_handler\n"+
			"1313e957-8e67-4fc2-99b1-02f6733f49cc,default\n"+
			"85b7274d-90f1-402d-95c4-f1cd5c42000c,thushani-hybrdgw\n"), projUUIDHeaders, projHandleHeaders)
	if err != nil {
		t.Fatal(err)
	}
	items := []projectRow{
		// v1's own auto-created "default" project — unknown to Choreo — listed FIRST on purpose.
		{uuid: "019df76d-9a9b-788d-bea5-4a4bc5f2d125", name: "default", orgUUID: "org-A"},
		// The Choreo project whose handler is "default" (its v1 name is a random string).
		{uuid: "1313e957-8e67-4fc2-99b1-02f6733f49cc", name: "nvwqb", orgUUID: "org-A"},
		{uuid: "85b7274d-90f1-402d-95c4-f1cd5c42000c", name: "irhdl", orgUUID: "org-B"},
		// Unmapped "default" in an org with no Choreo "default" keeps the plain slug.
		{uuid: "b575d723-9ed8-4931-91a7-720507247f79", name: "default", orgUUID: "org-B"},
	}
	p, err := k.planProjectHandles(items, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.mapped != 2 || len(p.fallback) != 2 {
		t.Fatalf("mapped=%d fallback=%d", p.mapped, len(p.fallback))
	}
	if h := p.handles["1313e957-8e67-4fc2-99b1-02f6733f49cc"]; h != "default" {
		t.Fatalf("Choreo handle must win: %q", h)
	}
	if p.handles["85b7274d-90f1-402d-95c4-f1cd5c42000c"] != "thushani-hybrdgw" {
		t.Fatal("mapped handle not verbatim")
	}
	v1def := p.handles["019df76d-9a9b-788d-bea5-4a4bc5f2d125"]
	if v1def == "default" || !strings.HasPrefix(v1def, "default-") {
		t.Fatalf("v1 default must be suffixed, got %q", v1def)
	}
	if len(p.suffixed) != 1 || p.suffixed[0] != "019df76d-9a9b-788d-bea5-4a4bc5f2d125" {
		t.Fatalf("suffixed=%v", p.suffixed)
	}
	if p.handles["b575d723-9ed8-4931-91a7-720507247f79"] != "default" {
		t.Fatal("unmapped default in another org must keep the plain slug")
	}
	// Re-planning reuses the checkpointed suffix (resume-safe).
	p2, err := k.planProjectHandles(items, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p2.handles["019df76d-9a9b-788d-bea5-4a4bc5f2d125"] != v1def {
		t.Fatal("checkpointed mint not reused")
	}
}

func TestPlanProjectHandles_DuplicateChoreoHandleInOrgFails(t *testing.T) {
	k := testKernels(t)
	m, err := parseHandleMap("projects", "p.csv", strings.NewReader("project_uuid,project_handler\np1,same\np2,same\n"), projUUIDHeaders, projHandleHeaders)
	if err != nil {
		t.Fatal(err)
	}
	items := []projectRow{{uuid: "p1", name: "alpha", orgUUID: "o"}, {uuid: "p2", name: "beta", orgUUID: "o"}}
	if _, err := k.planProjectHandles(items, m, nil); err == nil {
		t.Fatal("expected a duplicate-claim error")
	}
	// The same handle in DIFFERENT orgs is fine (uniqueness is per org).
	items[1].orgUUID = "o2"
	if _, err := k.planProjectHandles(items, m, nil); err != nil {
		t.Fatalf("cross-org same handle must be allowed: %v", err)
	}
}

func TestExternalHandle_Org(t *testing.T) {
	k := testKernels(t)
	m, err := parseHandleMap("organizations", "o.csv", strings.NewReader("org_uuid,org_handle\n4ddd1f0e-d881-4f0a-b7a6-e25448a9c4f1,saasorg\n"), orgUUIDHeaders, orgHandleHeaders)
	if err != nil {
		t.Fatal(err)
	}
	h, ok, err := k.externalHandle(m, "4ddd1f0e-d881-4f0a-b7a6-e25448a9c4f1")
	if err != nil || !ok || h != "saasorg" {
		t.Fatalf("%q %v %v", h, ok, err)
	}
	if _, ok, _ := k.externalHandle(m, "unknown"); ok {
		t.Fatal("unknown uuid must not resolve")
	}
	if _, ok, _ := k.externalHandle(nil, "4ddd1f0e-d881-4f0a-b7a6-e25448a9c4f1"); ok {
		t.Fatal("nil map must not resolve")
	}
}

// A checkpoint from another run (or an older export) can hand the planner a
// handle that a Choreo project now owns in the same org. mintHandle returns the
// checkpointed value without the exists-check, so the planner must catch the
// collision itself instead of planning a duplicate (which a real run would
// reject on UNIQUE(organization_uuid, handle) and a dry-run would hide).
func TestPlanProjectHandles_StaleCheckpointHandleCollisionFails(t *testing.T) {
	k := testKernels(t)
	if err := k.cp.PutHandle("projects", "019df76d-9a9b-788d-bea5-4a4bc5f2d125", "default"); err != nil {
		t.Fatal(err)
	}
	m, err := parseHandleMap("projects", "p.csv", strings.NewReader(
		"project_uuid,project_handler\n"+
			"1313e957-8e67-4fc2-99b1-02f6733f49cc,default\n"), projUUIDHeaders, projHandleHeaders)
	if err != nil {
		t.Fatal(err)
	}
	items := []projectRow{
		{uuid: "1313e957-8e67-4fc2-99b1-02f6733f49cc", name: "nvwqb", orgUUID: "org-A"},   // Choreo "default"
		{uuid: "019df76d-9a9b-788d-bea5-4a4bc5f2d125", name: "default", orgUUID: "org-A"}, // v1 default, stale ckpt says "default"
	}
	_, err = k.planProjectHandles(items, m, nil)
	if err == nil || !strings.Contains(err.Error(), "checkpoint") {
		t.Fatalf("expected a stale-checkpoint collision error, got %v", err)
	}
}
