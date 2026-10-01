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
	"fmt"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/database"
)

// insertRow issues a single idempotent INSERT. conflict is a raw ON CONFLICT
// target such as "(uuid)" or "(organization_uuid, artifact_uuid, gateway_uuid)";
// empty means no ON CONFLICT clause. Values are passed as bind parameters (native
// BYTEA, zero escaping). At this data scale (hundreds–thousands of rows/table)
// autocommit per-row inserts are simpler and fully resumable; batching/COPY would
// be premature optimization.
func insertRow(db *database.DB, table string, cols []string, args []any, conflict string) error {
	if len(cols) != len(args) {
		return fmt.Errorf("insert %s: %d cols but %d args", table, len(cols), len(args))
	}
	ph := make([]string, len(cols))
	for i := range cols {
		ph[i] = "?"
	}
	q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(cols, ", "), strings.Join(ph, ", "))
	if conflict != "" {
		q += " ON CONFLICT " + conflict + " DO NOTHING"
	}
	_, err := db.Exec(db.Rebind(q), args...)
	if err != nil {
		return fmt.Errorf("insert %s: %w", table, err)
	}
	return nil
}

// rowExists reports whether the parameterized query returns at least one row.
func rowExists(db *database.DB, query string, args ...any) (bool, error) {
	var one int
	err := db.QueryRow(db.Rebind("SELECT 1 FROM ("+query+") _sub LIMIT 1"), args...).Scan(&one)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// loadStringSet runs a single-column query and collects the results into a set.
func loadStringSet(db *database.DB, query string, args ...any) (map[string]bool, error) {
	rows, err := db.Query(db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	set := map[string]bool{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		set[s] = true
	}
	return set, rows.Err()
}

// ParentStatus is the outcome of an FK-parent lookup.
type ParentStatus int

const (
	// ParentOK: the parent was migrated into v2.
	ParentOK ParentStatus = iota
	// ParentCascade: the parent exists in v1 but was NOT migrated (quarantined/
	// dropped) — the child should cascade-quarantine, not fail-fast.
	ParentCascade
	// ParentMissing: the parent uuid is absent from v1 entirely — genuine dangling
	// reference (fail-fast if v1 enforced the FK, else quarantine).
	ParentMissing
)

// ParentSet tracks, per parent table, which uuids were seen in v1 and which were
// actually inserted into v2, so children can classify an FK reference precisely.
type ParentSet struct {
	v1 map[string]bool
	v2 map[string]bool
}

func newParentSet() *ParentSet {
	return &ParentSet{v1: map[string]bool{}, v2: map[string]bool{}}
}

func (p *ParentSet) seenV1(uuid string)     { p.v1[uuid] = true }
func (p *ParentSet) insertedV2(uuid string) { p.v2[uuid] = true }

func (p *ParentSet) status(uuid string) ParentStatus {
	if p.v2[uuid] {
		return ParentOK
	}
	if p.v1[uuid] {
		return ParentCascade
	}
	return ParentMissing
}

// Sets bundles the in-memory FK parent sets used during a run. UUIDs are preserved
// end-to-end, so FK resolution is a set membership test against already-migrated
// parents — never a live DB round-trip, never a mid-batch Postgres FK error.
type Sets struct {
	orgs        *ParentSet
	projects    *ParentSet
	artifacts   *ParentSet
	gateways    *ParentSet
	plans       *ParentSet
	providers   *ParentSet
	templates   *ParentSet
	applications *ParentSet
	apiKeys     *ParentSet
	deployments *ParentSet
	policies    *ParentSet
}

func newSets() *Sets {
	return &Sets{
		orgs:         newParentSet(),
		projects:     newParentSet(),
		artifacts:    newParentSet(),
		gateways:     newParentSet(),
		plans:        newParentSet(),
		providers:    newParentSet(),
		templates:    newParentSet(),
		applications: newParentSet(),
		apiKeys:      newParentSet(),
		deployments:  newParentSet(),
		policies:     newParentSet(),
	}
}
