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
	"os"
	"path/filepath"
	"testing"
)

func TestHandleReport_WriteOrderSummaryAndEscaping(t *testing.T) {
	var h HandleReport
	h.add(handleReportRow{kind: "project", uuid: "p2", orgUUID: "o1", v1Value: "default", v2Handle: "default-ab12", v2DisplayName: "default-ab12", source: sourceMintedSuffixed})
	h.add(handleReportRow{kind: "project", uuid: "p1", orgUUID: "o1", v1Value: "default", v2Handle: "default", v2DisplayName: "default", source: sourceChoreoExport})
	h.add(handleReportRow{kind: "organization", uuid: "o2", v1Value: "ebfoyrc", v2Handle: "ebfoyrc", v2DisplayName: "ebfoyrc", source: sourceV1HandleKept})
	h.add(handleReportRow{kind: "organization", uuid: "o1", v1Value: "ttnayys", v2Handle: "saasorg", v2DisplayName: "Saas, Inc", source: sourceChoreoExport})
	h.add(handleReportRow{kind: "project", uuid: "p3", orgUUID: "o2", v1Value: "kyrbv", v2Handle: "kyrbv", v2DisplayName: "kyrbv", source: sourceMinted})
	// re-adding the same (kind, uuid) replaces instead of duplicating
	h.add(handleReportRow{kind: "project", uuid: "p3", orgUUID: "o2", v1Value: "kyrbv", v2Handle: "kyrbv", v2DisplayName: "kyrbv", source: sourceMinted})

	path := filepath.Join(t.TempDir(), "handles.csv")
	sum, err := h.Write(path)
	if err != nil {
		t.Fatal(err)
	}
	if sum != (HandleReportSummary{Orgs: 2, OrgsNonChoreo: 1, Projects: 3, ProjectsNonChoreo: 2, ProjectsSuffixed: 1}) {
		t.Fatalf("summary = %+v", sum)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	recs, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 6 {
		t.Fatalf("rows = %d, want header + 5", len(recs))
	}
	if recs[0][0] != "kind" || recs[0][6] != "source" {
		t.Fatalf("header = %v", recs[0])
	}
	// organizations first (non-Choreo first), then projects (non-Choreo first, by org, by uuid)
	wantOrder := []string{"o2", "o1", "p2", "p3", "p1"}
	for i, want := range wantOrder {
		if recs[i+1][1] != want {
			t.Fatalf("row %d uuid = %q, want %q (order %v)", i, recs[i+1][1], want, recs)
		}
	}
	if recs[2][5] != "Saas, Inc" { // comma survived CSV quoting
		t.Fatalf("display name = %q", recs[2][5])
	}
	if recs[3][6] != string(sourceMintedSuffixed) {
		t.Fatalf("source = %q", recs[3][6])
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("report mode = %o, want 0600 (handles are PII)", info.Mode().Perm())
	}
}
