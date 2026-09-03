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
	"os"
	"path/filepath"
	"testing"
)

func TestLoadKeyFilter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.txt")
	content := "# a comment\n" +
		"upsert rest_apis uuid-1\n" +
		"upsert gateways uuid-2\n" +
		"delete artifacts uuid-3\n" +
		"delete deployment_status org|art|gw\n" +
		"\n" +
		"upsert rest_apis uuid-4\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	kf, err := loadKeyFilter(path)
	if err != nil {
		t.Fatalf("loadKeyFilter: %v", err)
	}
	if !kf.upserts["rest_apis"]["uuid-1"] || !kf.upserts["rest_apis"]["uuid-4"] {
		t.Errorf("rest_apis upsert keys missing: %v", kf.upserts["rest_apis"])
	}
	if !kf.upserts["gateways"]["uuid-2"] {
		t.Errorf("gateways upsert key missing: %v", kf.upserts["gateways"])
	}
	if len(kf.deletes) != 2 {
		t.Fatalf("deletes = %d, want 2 (%v)", len(kf.deletes), kf.deletes)
	}
	if kf.deletes[0].table != "artifacts" || kf.deletes[0].key != "uuid-3" {
		t.Errorf("deletes[0] = %+v, want {artifacts uuid-3}", kf.deletes[0])
	}
	if kf.deletes[1].table != "deployment_status" || kf.deletes[1].key != "org|art|gw" {
		t.Errorf("deletes[1] = %+v, want {deployment_status org|art|gw}", kf.deletes[1])
	}

	// want() matches only listed upsert keys; anything else (and the delete keys) is skipped.
	mc := &migCtx{only: kf}
	if !mc.want("rest_apis", "uuid-1") {
		t.Error("want(rest_apis, uuid-1) should be true")
	}
	if mc.want("rest_apis", "uuid-3") {
		t.Error("want(rest_apis, uuid-3) should be false (it is a delete, not an upsert)")
	}
	// Batch mode (only == nil) always processes.
	if !(&migCtx{}).want("anything", "anykey") {
		t.Error("want() must return true in batch mode (only == nil)")
	}
}

// TestLoadKeyFilterAcceptsJSONL proves the live path's JSONL failure log can be fed to
// -only-keys verbatim (each line is a JSON object with op/table/key).
func TestLoadKeyFilterAcceptsJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "failures.jsonl")
	content := `{"code":"V2_DUAL_WRITE_FAILURE","entity":"rest_api","op":"upsert","table":"rest_apis","key":"uuid-1","occurred_at":"2026-01-01T00:00:00Z"}
{"code":"V2_DUAL_WRITE_FAILURE","entity":"gateway","op":"delete","table":"gateways","key":"gw-1"}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	kf, err := loadKeyFilter(path)
	if err != nil {
		t.Fatalf("loadKeyFilter: %v", err)
	}
	if !kf.upserts["rest_apis"]["uuid-1"] {
		t.Error("JSONL upsert line not parsed")
	}
	if len(kf.deletes) != 1 || kf.deletes[0].table != "gateways" || kf.deletes[0].key != "gw-1" {
		t.Errorf("JSONL delete line not parsed: %v", kf.deletes)
	}
}

func TestLoadKeyFilterRejectsBadLines(t *testing.T) {
	write := func(content string) string {
		p := filepath.Join(t.TempDir(), "k.txt")
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if _, err := loadKeyFilter(write("upsert rest_apis\n")); err == nil {
		t.Error("expected an error for a 2-field line")
	}
	if _, err := loadKeyFilter(write("frob rest_apis uuid\n")); err == nil {
		t.Error("expected an error for an unknown op")
	}
}
