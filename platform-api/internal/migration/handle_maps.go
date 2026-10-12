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
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// HandleMap is an externally supplied uuid → handle map (DB_MAPPING §B.14): the
// AUTHORITATIVE Choreo handles for organizations / projects, exported from
// Choreo's own stores (cr/test/export-org-handles.sh → App Service
// dbo.organization; cr/test/export-project-handlers.js → Project Manager
// am_projects). v1's own values are NOT Choreo's — v1 org handles are random
// 7-letter strings and v1 project names are "default"/random 5-letter strings —
// but the AI Workspace v2 URL and the v2 REST API address both resources BY
// HANDLE, so a migrated handle must equal Choreo's or every link breaks.
//
// Handles are carried VERBATIM (never slugified, cased or truncated): the URL
// must match Choreo exactly. v2 only runs ValidateHandle on its create/PUT
// paths — GET / path params are served as stored — so a Choreo handle that
// v2's own validation would reject (uppercase, a '.') is written as-is and
// merely WARNED about in §B.0 Gate 5.
type HandleMap struct {
	Kind string // "organizations" | "projects" — for messages only
	Path string
	Rows int // data rows read (header excluded)

	byUUID map[string]string // uuid -> handle (non-empty handles only)
	empty  map[string]bool   // uuids present in the file with an EMPTY handle
	status map[string]string // optional per-row status column (org_status)
	name   map[string]string // optional per-row display-name column (org_name)
}

// Accepted header spellings (case-insensitive). Extra columns are ignored.
var (
	orgUUIDHeaders    = []string{"org_uuid", "organization_uuid", "uuid"}
	orgHandleHeaders  = []string{"org_handle", "organization_handle", "handle"}
	projUUIDHeaders   = []string{"project_uuid", "uuid"}
	projHandleHeaders = []string{"project_handler", "project_handle", "handler", "handle"}
	statusHeaders     = []string{"org_status", "status"}
	nameHeaders       = []string{"org_name", "display_name", "name"}
)

// LoadOrgHandleMap reads an export-org-handles.sh CSV
// (org_uuid,org_handle[,org_status,org_name]).
func LoadOrgHandleMap(path string) (*HandleMap, error) {
	return loadHandleMap("organizations", path, orgUUIDHeaders, orgHandleHeaders)
}

// LoadProjectHandleMap reads an export-project-handlers.js CSV
// (project_uuid,project_handler).
func LoadProjectHandleMap(path string) (*HandleMap, error) {
	return loadHandleMap("projects", path, projUUIDHeaders, projHandleHeaders)
}

func loadHandleMap(kind, path string, uuidHdrs, handleHdrs []string) (*HandleMap, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%s handle map: %w", kind, err)
	}
	defer f.Close()
	return parseHandleMap(kind, path, f, uuidHdrs, handleHdrs)
}

func parseHandleMap(kind, path string, src io.Reader, uuidHdrs, handleHdrs []string) (*HandleMap, error) {
	r := csv.NewReader(src)
	r.FieldsPerRecord = -1 // tolerate ragged rows; columns are indexed by header
	r.TrimLeadingSpace = true

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("%s handle map %s: cannot read header: %w", kind, path, err)
	}
	uuidIdx, handleIdx, statusIdx, nameIdx := indexOfAny(header, uuidHdrs), indexOfAny(header, handleHdrs),
		indexOfAny(header, statusHeaders), indexOfAny(header, nameHeaders)
	if uuidIdx < 0 || handleIdx < 0 {
		return nil, fmt.Errorf("%s handle map %s: header must contain a uuid column (%s) and a handle column (%s); got %q",
			kind, path, strings.Join(uuidHdrs, "|"), strings.Join(handleHdrs, "|"), header)
	}

	m := &HandleMap{Kind: kind, Path: path, byUUID: map[string]string{}, empty: map[string]bool{}, status: map[string]string{}, name: map[string]string{}}
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s handle map %s: row %d: %w", kind, path, m.Rows+2, err)
		}
		m.Rows++
		if uuidIdx >= len(rec) {
			continue
		}
		uuid := strings.ToLower(strings.TrimSpace(rec[uuidIdx]))
		if uuid == "" {
			continue
		}
		handle := ""
		if handleIdx < len(rec) {
			handle = strings.TrimSpace(rec[handleIdx]) // VERBATIM — never lowercased/slugified
		}
		if handle == "" {
			m.empty[uuid] = true
			continue
		}
		// A handle is an external id: over-width cannot be truncated (§B.0 Gate 1).
		if len(handle) > maxHandleLen {
			return nil, fmt.Errorf("%s handle map %s: row %d: handle %q for %s is %d chars > v2 limit %d — fix the export",
				kind, path, m.Rows+1, handle, uuid, len(handle), maxHandleLen)
		}
		if prev, dup := m.byUUID[uuid]; dup && prev != handle {
			return nil, fmt.Errorf("%s handle map %s: uuid %s appears twice with different handles (%q vs %q) — fix the export",
				kind, path, uuid, prev, handle)
		}
		m.byUUID[uuid] = handle
		if statusIdx >= 0 && statusIdx < len(rec) {
			if s := strings.TrimSpace(rec[statusIdx]); s != "" {
				m.status[uuid] = s
			}
		}
		if nameIdx >= 0 && nameIdx < len(rec) {
			if n := strings.TrimSpace(rec[nameIdx]); n != "" {
				m.name[uuid] = n
			}
		}
	}
	return m, nil
}

// indexOfAny returns the index of the first header cell matching any candidate
// (case-insensitive, BOM/space-trimmed), or -1.
func indexOfAny(header, candidates []string) int {
	for i, h := range header {
		h = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\xEF\xBB\xBF")))
		for _, c := range candidates {
			if h == c {
				return i
			}
		}
	}
	return -1
}

// Lookup returns the Choreo handle for uuid. A nil map (no CSV configured, e.g.
// reverse or a bare verify) never matches.
func (m *HandleMap) Lookup(uuid string) (string, bool) {
	if m == nil {
		return "", false
	}
	h, ok := m.byUUID[strings.ToLower(strings.TrimSpace(uuid))]
	return h, ok
}

// Len is the number of uuids with a non-empty handle.
func (m *HandleMap) Len() int {
	if m == nil {
		return 0
	}
	return len(m.byUUID)
}

// EmptyCount is the number of uuids the export listed with an EMPTY handle —
// they are treated as unmapped.
func (m *HandleMap) EmptyCount() int {
	if m == nil {
		return 0
	}
	return len(m.empty)
}

// NameOf returns the export's display-name column for uuid ("" if absent) —
// Choreo's org name at export time, the same value a freshly provisioned org
// would be registered with (§B.14).
func (m *HandleMap) NameOf(uuid string) string {
	if m == nil {
		return ""
	}
	return m.name[strings.ToLower(strings.TrimSpace(uuid))]
}

// StatusOf returns the export's status column for uuid ("" if absent).
func (m *HandleMap) StatusOf(uuid string) string {
	if m == nil {
		return ""
	}
	return m.status[strings.ToLower(strings.TrimSpace(uuid))]
}

// handleFormatIssue returns v2's own ValidateHandle complaint for a handle that
// v2's CREATE/PUT paths would reject (uppercase, '.', <3 chars …), or "" if it
// is clean. Reads / path params are served as stored, so such a handle is still
// written verbatim — this only feeds the Gate-5 WARN.
func handleFormatIssue(h string) string {
	if err := utils.ValidateHandle(h); err != nil {
		return err.Error()
	}
	return ""
}

// ---------------------------------------------------------------------------
// Kernel 2b: external (Choreo) handles — organizations & projects (§B.14)
// ---------------------------------------------------------------------------

// externalHandle resolves sourceID through a Choreo handle map. ok=false means
// Choreo does not know the row (the caller falls back; Gate 5 has WARNed).
func (k *Kernels) externalHandle(m *HandleMap, sourceID string) (string, bool, error) {
	h, ok := m.Lookup(sourceID)
	if !ok {
		return "", false, nil
	}
	if len(h) > maxHandleLen { // defensive; the loader already rejects these
		return "", true, fmt.Errorf("QUARANTINE %s/%s: Choreo handle %q is %d chars > v2 limit %d", m.Kind, sourceID, h, len(h), maxHandleLen)
	}
	return h, true, nil
}

// projectHandlePlan is the pre-computed v2 handle for every v1 project.
type projectHandlePlan struct {
	handles  map[string]string // project uuid -> final v2 handle
	mapped   int               // handles taken verbatim from the Choreo export
	fallback []string          // uuids minted from the v1 name (Choreo does not know them)
	suffixed []string          // fallback uuids whose base slug collided inside the org and got a suffix
}

// planProjectHandles decides every project's v2 handle BEFORE any insert so a
// Choreo handle always wins inside its org: pass 1 claims the export's handles
// verbatim; pass 2 mints the rest from the v1 name (v1's auto-created "default"
// projects are NOT Choreo projects — 43 dev orgs hold both a Choreo project
// whose handler is "default" and a v1 "default" row) with an exists-check that
// sees the claimed set as well as the target DB, so the v1 row becomes
// "default-xxxx", never the Choreo one. Minted handles stay checkpointed (§B.2).
//
// dbExists may be nil (dry-run / no target); the in-memory claim set still
// applies, so a dry-run reports the same suffixing a real run would.
func (k *Kernels) planProjectHandles(items []projectRow, m *HandleMap, dbExists func(orgUUID, handle string) bool) (*projectHandlePlan, error) {
	p := &projectHandlePlan{handles: make(map[string]string, len(items))}
	claimed := map[string]map[string]bool{} // org -> handle set
	claim := func(org, h string) {
		if claimed[org] == nil {
			claimed[org] = map[string]bool{}
		}
		claimed[org][h] = true
	}

	// Pass 1 — Choreo handles, verbatim.
	for _, r := range items {
		h, ok, err := k.externalHandle(m, r.uuid)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if claimed[r.orgUUID][h] {
			return nil, fmt.Errorf("Choreo handle %q is claimed by two projects in org %s (second: %s) — fix the export (§B.0 Gate 5)",
				h, r.orgUUID, r.uuid)
		}
		claim(r.orgUUID, h)
		p.handles[r.uuid] = h
		p.mapped++
	}

	// Pass 2 — fallback: mint from the v1 name for projects Choreo does not know.
	for _, r := range items {
		if _, done := p.handles[r.uuid]; done {
			continue
		}
		org := r.orgUUID
		exists := func(h string) bool {
			if claimed[org][h] {
				return true
			}
			return dbExists != nil && dbExists(org, h)
		}
		h, err := k.mintHandle("projects", r.uuid, r.name, exists)
		if err != nil {
			return nil, err
		}
		// A fresh mint can never collide (existsCheck above), so a collision here
		// means the CHECKPOINT supplied the handle and it clashes with a handle
		// planned in this run (a Choreo handle claimed in pass 1, or an earlier
		// mint): the checkpoint is from another run or an older export. Writing
		// it would hit projects UNIQUE(organization_uuid, handle); a dry-run
		// would silently plan a duplicate. Hard error.
		if claimed[org][h] {
			return nil, fmt.Errorf("handle %q for project %s (v1 name %q) is already planned for another project in org %s — "+
				"it came from the checkpoint (fresh mints never collide), so the checkpoint belongs to another run or an older export: "+
				"use a NEW --checkpoint-file for a new migration, or resume with the export that run used (§B.14)",
				h, r.uuid, r.name, org)
		}
		if base, berr := utils.GenerateHandle(r.name, nil); berr == nil && base != h {
			p.suffixed = append(p.suffixed, r.uuid)
		}
		claim(org, h)
		p.handles[r.uuid] = h
		p.fallback = append(p.fallback, r.uuid)
	}
	sort.Strings(p.fallback)
	sort.Strings(p.suffixed)
	return p, nil
}

// verifyHandleParity asserts (§9) that every migrated row Choreo knows carries
// exactly Choreo's handle — the AI Workspace v2 URL contract. Writes are
// insert-only (ON CONFLICT DO NOTHING), so an export that changed AFTER a row's
// first migration is NOT re-applied; this is the check that surfaces it.
func verifyHandleParity(ctx context.Context, rc *RunContext, rep *ResourceReport, table string, m *HandleMap) {
	if rc.Tgt == nil || rc.DryRun || m == nil {
		return
	}
	rows, err := rc.Tgt.QueryContext(ctx, "SELECT uuid, handle FROM "+table)
	if err != nil {
		rep.warn("handle parity check failed: %v", err)
		return
	}
	defer rows.Close()
	var total, matched, mismatched int
	var examples []string
	for rows.Next() {
		var uuid, handle string
		if err := rows.Scan(&uuid, &handle); err != nil {
			rep.warn("handle parity scan: %v", err)
			return
		}
		want, ok := m.Lookup(uuid)
		if !ok {
			continue // not a Choreo row (fallback handle) — nothing to compare
		}
		total++
		if want == handle {
			matched++
			continue
		}
		mismatched++
		if len(examples) < 5 {
			examples = append(examples, fmt.Sprintf("%s: have %q want %q", uuid, handle, want))
		}
	}
	if mismatched > 0 {
		rep.fail("%d %s handle(s) differ from the Choreo export (%s) — rows are insert-only, so a changed export is not re-applied; delete and re-migrate those rows",
			mismatched, table, strings.Join(examples, "; "))
		return
	}
	rep.addf("handle parity: %d/%d Choreo-known %s rows carry the exported handle", matched, total, table)
}
