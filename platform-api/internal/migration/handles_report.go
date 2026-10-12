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
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"sync"
)

// handleSource says where a migrated organization/project handle came from.
type handleSource string

const (
	// sourceChoreoExport: taken verbatim from the Choreo export (the normal case).
	sourceChoreoExport handleSource = "choreo-export"
	// sourceV1HandleKept: organization not in the export — v1's random handle
	// was kept (Gate 5 WARNs; the org is NOT reachable under any Choreo URL).
	sourceV1HandleKept handleSource = "v1-handle-kept"
	// sourceMinted: project not in the export — handle slugged from the v1 name
	// (v1's own auto-created "default" projects are the expected case).
	sourceMinted handleSource = "minted-from-v1-name"
	// sourceMintedSuffixed: as sourceMinted, plus a suffix because the slug
	// collided with a Choreo handle (or another minted one) inside the org.
	sourceMintedSuffixed handleSource = "minted-from-v1-name-suffixed"
)

// handleReportRow is one organization or project as the forward run handled it.
type handleReportRow struct {
	kind          string // "organization" | "project"
	uuid          string
	orgUUID       string // projects only (empty for organizations)
	v1Value       string // v1 handle (orgs) / v1 name (projects)
	v2Handle      string
	v2DisplayName string
	source        handleSource
}

// HandleReport collects every organization/project handle decision of a
// forward run (dry-run included) and writes them as ONE CSV, so the operator
// can filter the rows that did NOT get a Choreo handle, resolve them against
// Choreo by uuid, and either fix the export and re-run or UPDATE v2 afterwards.
// Rows are keyed by (kind, uuid): re-adding replaces, so a resumed run never
// duplicates. It is safe for concurrent use.
type HandleReport struct {
	mu   sync.Mutex
	rows map[string]handleReportRow
}

// HandleReportSummary is what Write returns for the run's final log line.
type HandleReportSummary struct {
	Orgs, OrgsNonChoreo         int
	Projects, ProjectsNonChoreo int
	ProjectsSuffixed            int
}

func (h *HandleReport) add(r handleReportRow) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rows == nil {
		h.rows = map[string]handleReportRow{}
	}
	h.rows[r.kind+"\x00"+r.uuid] = r
}

// Len is the number of collected rows.
func (h *HandleReport) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.rows)
}

// Write writes the CSV (header + one row per organization/project) to path and
// returns the summary counts. Order: organizations first, then projects; within
// a kind the non-Choreo rows come first (they are the actionable ones), then by
// organization uuid, then uuid — so the file is stable across runs and the rows
// that need attention are at the top of each block.
func (h *HandleReport) Write(path string) (HandleReportSummary, error) {
	h.mu.Lock()
	rows := make([]handleReportRow, 0, len(h.rows))
	for _, r := range h.rows {
		rows = append(rows, r)
	}
	h.mu.Unlock()

	kindRank := func(k string) int {
		if k == "organization" {
			return 0
		}
		return 1
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if ka, kb := kindRank(a.kind), kindRank(b.kind); ka != kb {
			return ka < kb
		}
		if na, nb := a.source != sourceChoreoExport, b.source != sourceChoreoExport; na != nb {
			return na // non-Choreo first
		}
		if a.orgUUID != b.orgUUID {
			return a.orgUUID < b.orgUUID
		}
		return a.uuid < b.uuid
	})

	var sum HandleReportSummary
	for _, r := range rows {
		switch r.kind {
		case "organization":
			sum.Orgs++
			if r.source != sourceChoreoExport {
				sum.OrgsNonChoreo++
			}
		case "project":
			sum.Projects++
			if r.source != sourceChoreoExport {
				sum.ProjectsNonChoreo++
			}
			if r.source == sourceMintedSuffixed {
				sum.ProjectsSuffixed++
			}
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) // handles are PII
	if err != nil {
		return sum, fmt.Errorf("handles report: %w", err)
	}
	w := csv.NewWriter(f)
	if err := w.Write([]string{"kind", "uuid", "organization_uuid", "v1_value", "v2_handle", "v2_display_name", "source"}); err != nil {
		f.Close()
		return sum, fmt.Errorf("handles report: %w", err)
	}
	for _, r := range rows {
		if err := w.Write([]string{r.kind, r.uuid, r.orgUUID, r.v1Value, r.v2Handle, r.v2DisplayName, string(r.source)}); err != nil {
			f.Close()
			return sum, fmt.Errorf("handles report: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		f.Close()
		return sum, fmt.Errorf("handles report: %w", err)
	}
	if err := f.Close(); err != nil {
		return sum, fmt.Errorf("handles report: %w", err)
	}
	return sum, nil
}
