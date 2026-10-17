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
	"path/filepath"
	"testing"
	"time"
)

// v1 holds applications, API keys and the application→key mappings. There is
// deliberately NO application_artifacts table: AI Workspace v1 never fills it and
// older v1 schemas lack it, so the migrator must not read it.
const v1AppMappingsSchema = `
CREATE TABLE applications (uuid TEXT, handle TEXT, project_uuid TEXT, organization_uuid TEXT, created_by TEXT,
	name TEXT, description TEXT, type TEXT, created_at TIMESTAMP, updated_at TIMESTAMP);
CREATE TABLE api_keys (uuid TEXT, artifact_uuid TEXT, name TEXT, created_at TIMESTAMP);
CREATE TABLE application_api_keys (application_uuid TEXT, api_key_id TEXT, created_at TIMESTAMP, updated_at TIMESTAMP);

INSERT INTO applications VALUES
	('app-1', 'app-one', 'proj-1', 'org-1', 'u1', 'App One', NULL, 'genai', '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
	('app-2', 'app-two', 'proj-1', 'org-1', 'u1', 'App Two', NULL, 'genai', '2026-01-01 00:00:00', '2026-01-01 00:00:00');
INSERT INTO api_keys VALUES
	('key-a', 'art-prov', 'key-a', '2026-01-01 00:00:00'),
	('key-b', 'art-prov', 'key-b', '2026-01-01 00:00:00'),
	('key-c', 'art-prox', 'key-c', '2026-01-01 00:00:00'),
	('key-d', 'art-prov', 'key-d', '2026-01-01 00:00:00');
-- app-1 holds two keys of the same provider (mapped at different times) and one
-- of a proxy; app-2 holds one provider key. Rows are inserted newest first so the
-- ORDER BY, not insertion order, must pick the earliest mapping per pair.
INSERT INTO application_api_keys VALUES
	('app-1', 'key-b', '2026-03-01 00:00:00', '2026-03-01 00:00:00'),
	('app-1', 'key-a', '2026-02-01 00:00:00', '2026-02-01 00:00:00'),
	('app-1', 'key-c', '2026-04-01 00:00:00', '2026-04-01 00:00:00'),
	('app-2', 'key-d', '2026-05-01 00:00:00', '2026-05-01 00:00:00');
`

// application_artifact_mappings is derived from the key mappings: one row per
// (application, artifact) an application holds a key for, created_at = the
// earliest of those mappings, idempotent, and verified against the same derivation.
func TestAppArtifactMappings_DerivedFromKeyMappings(t *testing.T) {
	src := openSQLite(t, "v1.db")
	if _, err := src.Exec(v1AppMappingsSchema); err != nil {
		t.Fatalf("v1 schema: %v", err)
	}
	tgt := openSQLite(t, "v2.db")
	execFile(t, tgt, filepath.Join("..", "database", "schema.sqlite.sql"))

	k := testKernels(t)
	rc := &RunContext{Cfg: k.cfg, Log: k.log, Kernels: k, CP: k.cp, Src: src, Tgt: tgt,
		Direction: DirectionForward, BatchSize: 10, Handles: &HandleReport{}}
	m := &appArtifactMappingsMigrator{baseMigrator{name: "application_artifact_mappings"}}

	for pass := 1; pass <= 2; pass++ {
		rep, err := m.Migrate(context.Background(), rc)
		if err != nil {
			t.Fatalf("migrate pass %d: %v", pass, err)
		}
		if rep.SrcCount != 3 {
			t.Errorf("pass %d: SrcCount = %d, want 3 derived pairs (4 key mappings)", pass, rep.SrcCount)
		}
	}

	rows, err := tgt.Query(`SELECT application_uuid, artifact_uuid, created_at FROM application_artifact_mappings ORDER BY application_uuid, artifact_uuid`)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	defer rows.Close()
	type pair struct {
		app, art string
		at       time.Time
	}
	var got []pair
	for rows.Next() {
		var p pair
		if err := rows.Scan(&p.app, &p.art, &p.at); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, p)
	}
	want := []pair{
		{"app-1", "art-prov", time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)}, // earliest of key-a (Feb) and key-b (Mar)
		{"app-1", "art-prox", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)},
		{"app-2", "art-prov", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)},
	}
	if len(got) != len(want) {
		t.Fatalf("rows = %+v, want %d rows (second pass must add none)", got, len(want))
	}
	for i := range want {
		if got[i].app != want[i].app || got[i].art != want[i].art || !got[i].at.Equal(want[i].at) {
			t.Errorf("row %d = %s/%s@%s, want %s/%s@%s", i, got[i].app, got[i].art, got[i].at.UTC(),
				want[i].app, want[i].art, want[i].at)
		}
	}

	vrep, err := m.Verify(context.Background(), rc)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if vrep.Status != StatusPass || vrep.SrcCount != 3 || vrep.TgtCount != 3 {
		t.Errorf("verify = %s src=%d tgt=%d %v, want PASS 3/3", vrep.Status, vrep.SrcCount, vrep.TgtCount, vrep.Messages)
	}
}

func TestDeriveAppArtifactPairs_FirstSeenWins(t *testing.T) {
	in := []appArtifactRow{
		{applicationUUID: "a", artifactUUID: "x"},
		{applicationUUID: "a", artifactUUID: "y"},
		{applicationUUID: "a", artifactUUID: "x"},
		{applicationUUID: "b", artifactUUID: "x"},
	}
	out := deriveAppArtifactPairs(in)
	if len(out) != 3 || out[0].artifactUUID != "x" || out[1].artifactUUID != "y" || out[2].applicationUUID != "b" {
		t.Fatalf("derived = %+v, want a/x, a/y, b/x", out)
	}
	if len(deriveAppArtifactPairs(nil)) != 0 {
		t.Fatal("nil input must derive no pairs")
	}
}
