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
	"strings"
	"testing"
)

// v1 api_keys as the migrator reads them: one key whose name is a valid v2
// handle and one whose 59-char name cannot survive the 40-char handle mint.
const v1APIKeysSchema = `
CREATE TABLE artifacts (uuid TEXT, handle TEXT, name TEXT, version TEXT, organization_uuid TEXT,
	created_at TIMESTAMP, updated_at TIMESTAMP);
CREATE TABLE api_keys (uuid TEXT, artifact_uuid TEXT, name TEXT, masked_api_key TEXT, api_key_hashes TEXT,
	status TEXT, created_at TIMESTAMP, created_by TEXT, updated_at TIMESTAMP, expires_at TIMESTAMP,
	issuer TEXT, allowed_targets TEXT);
INSERT INTO artifacts VALUES ('art-1', 'rest-h', 'Rest', 'v1', 'org-1', '2026-01-01 00:00:00', '2026-01-01 00:00:00');
INSERT INTO api_keys VALUES
	('key-short', 'art-1', 'e2e-key-1791861881-29309', '***1234', '{"sha256":"a"}', 'active',
	 '2026-01-01 00:00:00', NULL, '2026-01-01 00:00:00', NULL, NULL, 'ALL'),
	('key-long', 'art-1', 'e2e-migrated-key-with-a-very-long-name-exceeding-forty-chars', '***5678', '{"sha256":"b"}', 'active',
	 '2026-01-01 00:00:00', NULL, '2026-01-01 00:00:00', NULL, NULL, 'ALL');
`

func newAPIKeysRunContext(t *testing.T) *RunContext {
	t.Helper()
	src := openSQLite(t, "v1.db")
	if _, err := src.Exec(v1APIKeysSchema); err != nil {
		t.Fatalf("v1 schema: %v", err)
	}
	tgt := openSQLite(t, "v2.db")
	execFile(t, tgt, filepath.Join("..", "database", "schema.sqlite.sql"))
	execFile(t, tgt, filepath.Join("..", "..", "plugins", "eventgateway", "schema", "schema.sqlite.sql"))
	k := testKernels(t)
	return &RunContext{Cfg: k.cfg, Log: k.log, Kernels: k, CP: k.cp, Src: src, Tgt: tgt,
		Direction: DirectionForward, BatchSize: 10, Handles: &HandleReport{}}
}

// A key whose v1 name is longer than v2's 40-char handle is still migrated,
// under a truncated handle, but the gateways that hold it know only the v1
// name: the report must say so, naming the key, and stay silent about keys
// whose name is their handle.
func TestAPIKeysMigrator_WarnsWhenTheNameDoesNotSurviveTheHandleMint(t *testing.T) {
	rc := newAPIKeysRunContext(t)
	rep, err := (&apiKeysMigrator{baseMigrator{name: "api_keys"}}).Migrate(context.Background(), rc)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if rep.Status != StatusWarn {
		t.Fatalf("status = %s, want WARN; messages: %v", rep.Status, rep.Messages)
	}
	joined := strings.Join(rep.Messages, "\n")
	for _, want := range []string{"key-long", "e2e-migrated-key-with-a-very-long-name-exceeding-forty-chars", "e2e-migrated-key-with-a-very-long-name-e", "cannot be revoked"} {
		if !strings.Contains(joined, want) {
			t.Errorf("report does not mention %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "key-short") {
		t.Errorf("report mentions the key whose name is its handle:\n%s", joined)
	}

	var short, long string
	if err := rc.Tgt.QueryRow(`SELECT handle FROM api_keys WHERE uuid = 'key-short'`).Scan(&short); err != nil {
		t.Fatalf("read key-short: %v", err)
	}
	if err := rc.Tgt.QueryRow(`SELECT handle FROM api_keys WHERE uuid = 'key-long'`).Scan(&long); err != nil {
		t.Fatalf("read key-long: %v", err)
	}
	if short != "e2e-key-1791861881-29309" {
		t.Errorf("key-short handle = %q, want the v1 name", short)
	}
	if long != "e2e-migrated-key-with-a-very-long-name-e" {
		t.Errorf("key-long handle = %q, want the 40-char mint", long)
	}
}

// A migrated gateway has not connected to v2: is_active starts false even
// when v1 had it true.
func TestGatewaysMigrator_MigratedGatewayStartsInactive(t *testing.T) {
	src := openSQLite(t, "v1.db")
	if _, err := src.Exec(v1SourceSchema); err != nil { // gw-1 has is_active = 1
		t.Fatalf("v1 schema: %v", err)
	}
	tgt := openSQLite(t, "v2.db")
	execFile(t, tgt, filepath.Join("..", "database", "schema.sqlite.sql"))
	execFile(t, tgt, filepath.Join("..", "..", "plugins", "eventgateway", "schema", "schema.sqlite.sql"))
	k := testKernels(t)
	rc := &RunContext{Cfg: k.cfg, Log: k.log, Kernels: k, CP: k.cp, Src: src, Tgt: tgt,
		Direction: DirectionForward, BatchSize: 10, Handles: &HandleReport{}}

	if _, err := (&gatewaysMigrator{baseMigrator{name: "gateways"}}).Migrate(context.Background(), rc); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var active int
	if err := tgt.QueryRow(`SELECT is_active FROM gateways WHERE uuid = 'gw-1'`).Scan(&active); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if active != 0 {
		t.Errorf("is_active = %d, want 0 (the gateway has not connected to v2)", active)
	}
}
