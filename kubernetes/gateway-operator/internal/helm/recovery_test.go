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

package helm

import (
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/release"
)

// rel builds a history entry at the given revision and status.
func rel(revision int, status release.Status) *release.Release {
	return &release.Release{
		Name:    "test-gw",
		Version: revision,
		Info:    &release.Info{Status: status},
	}
}

func TestPlanRelease(t *testing.T) {
	tests := []struct {
		name             string
		history          []*release.Release
		wantOperation    releaseOperation
		wantRollbackRev  int
		wantErrSubstring string
	}{
		{
			// Nothing stored: there is nothing to preserve or upgrade from.
			name:          "empty history installs",
			history:       nil,
			wantOperation: operationInstall,
		},
		{
			name:          "deployed upgrades",
			history:       []*release.Release{rel(1, release.StatusDeployed)},
			wantOperation: operationUpgrade,
		},
		{
			// Existing behaviour: Helm can upgrade a failed release in place.
			name:          "failed upgrades",
			history:       []*release.Release{rel(1, release.StatusFailed)},
			wantOperation: operationUpgrade,
		},
		{
			name:          "superseded upgrades",
			history:       []*release.Release{rel(1, release.StatusSuperseded)},
			wantOperation: operationUpgrade,
		},
		{
			// History kept after an uninstall leaves no revision to upgrade from.
			name:          "uninstalled installs",
			history:       []*release.Release{rel(1, release.StatusUninstalled)},
			wantOperation: operationInstall,
		},
		{
			// The interrupted first install: purging discards nothing successful.
			name:          "revision 1 pending install is purged and reinstalled",
			history:       []*release.Release{rel(1, release.StatusPendingInstall)},
			wantOperation: operationPurgeThenInstall,
		},
		{
			name: "pending upgrade rolls back to latest successful revision",
			history: []*release.Release{
				rel(1, release.StatusSuperseded),
				rel(2, release.StatusSuperseded),
				rel(3, release.StatusPendingUpgrade),
			},
			wantOperation:   operationRollbackThenUpgrade,
			wantRollbackRev: 2,
		},
		{
			name: "pending rollback rolls back to latest successful revision",
			history: []*release.Release{
				rel(1, release.StatusSuperseded),
				rel(2, release.StatusPendingRollback),
			},
			wantOperation:   operationRollbackThenUpgrade,
			wantRollbackRev: 1,
		},
		{
			// Helm sorts history before returning it, but the planner must not depend on it.
			name: "unordered history selects the highest revision",
			history: []*release.Release{
				rel(3, release.StatusPendingUpgrade),
				rel(1, release.StatusSuperseded),
				rel(2, release.StatusDeployed),
			},
			wantOperation:   operationRollbackThenUpgrade,
			wantRollbackRev: 2,
		},
		{
			// A failed revision is not a rollback target, so nothing safe remains.
			name: "pending upgrade with no successful revision fails",
			history: []*release.Release{
				rel(1, release.StatusFailed),
				rel(2, release.StatusPendingUpgrade),
			},
			wantErrSubstring: "no successful revision to recover to",
		},
		{
			// Never purge here: revision 2 means an earlier revision might have succeeded.
			name: "pending install beyond revision 1 with no successful revision fails",
			history: []*release.Release{
				rel(1, release.StatusFailed),
				rel(2, release.StatusPendingInstall),
			},
			wantErrSubstring: "no successful revision to recover to",
		},
		{
			name: "pending install beyond revision 1 rolls back when a successful revision exists",
			history: []*release.Release{
				rel(1, release.StatusSuperseded),
				rel(2, release.StatusPendingInstall),
			},
			wantOperation:   operationRollbackThenUpgrade,
			wantRollbackRev: 1,
		},
		{
			// Recorded intent was removal, so neither upgrading nor purging is inferable.
			name:             "uninstalling fails rather than guessing",
			history:          []*release.Release{rel(1, release.StatusUninstalling)},
			wantErrSubstring: "stuck in uninstalling",
		},
		{
			// Defensive: a nil Info must not panic and must not be read as successful.
			name:          "missing info is treated as upgradeable",
			history:       []*release.Release{{Name: "test-gw", Version: 1}},
			wantOperation: operationUpgrade,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := planRelease(tc.history)

			if tc.wantErrSubstring != "" {
				if err == nil {
					t.Fatalf("expected an error containing %q, got plan %v", tc.wantErrSubstring, plan.operation)
				}
				if !strings.Contains(err.Error(), tc.wantErrSubstring) {
					t.Fatalf("expected error containing %q, got %q", tc.wantErrSubstring, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if plan.operation != tc.wantOperation {
				t.Errorf("operation = %v, want %v", plan.operation, tc.wantOperation)
			}
			if plan.rollbackRevision != tc.wantRollbackRev {
				t.Errorf("rollbackRevision = %d, want %d", plan.rollbackRevision, tc.wantRollbackRev)
			}
			if plan.reason == "" {
				t.Error("expected a non-empty reason so the decision is auditable in logs")
			}
		})
	}
}

func TestPlanRelease_nilEntriesIgnored(t *testing.T) {
	// A nil entry must not panic or mask the real latest revision.
	plan, err := planRelease([]*release.Release{nil, rel(1, release.StatusDeployed), nil})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.operation != operationUpgrade {
		t.Errorf("operation = %v, want %v", plan.operation, operationUpgrade)
	}
}

func TestLatestRevision_numericNotPositional(t *testing.T) {
	// Revision 10 must beat revision 9 ; a string comparison would not.
	history := []*release.Release{rel(9, release.StatusSuperseded), rel(10, release.StatusDeployed)}
	latest := latestRevision(history)
	if latest == nil || latest.Version != 10 {
		t.Fatalf("latestRevision = %v, want revision 10", latest)
	}
}

func TestLatestSuccessfulRevision(t *testing.T) {
	tests := []struct {
		name    string
		history []*release.Release
		want    int
	}{
		{name: "none", history: []*release.Release{rel(1, release.StatusFailed)}, want: 0},
		{name: "deployed", history: []*release.Release{rel(4, release.StatusDeployed)}, want: 4},
		{name: "superseded counts", history: []*release.Release{rel(2, release.StatusSuperseded)}, want: 2},
		{
			name: "highest successful wins over later failure",
			history: []*release.Release{
				rel(1, release.StatusSuperseded),
				rel(2, release.StatusDeployed),
				rel(3, release.StatusFailed),
			},
			want: 2,
		},
		{
			name: "pending revisions are not rollback targets",
			history: []*release.Release{
				rel(1, release.StatusDeployed),
				rel(2, release.StatusPendingUpgrade),
			},
			want: 1,
		},
		{name: "nil safe", history: []*release.Release{nil, {Name: "x", Version: 1}}, want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := latestSuccessfulRevision(tc.history); got != tc.want {
				t.Errorf("latestSuccessfulRevision = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestReleaseOperation_String(t *testing.T) {
	// The reason strings are logged before a destructive step, so the names must be stable.
	cases := map[releaseOperation]string{
		operationInstall:             "install",
		operationUpgrade:             "upgrade",
		operationPurgeThenInstall:    "purge-then-install",
		operationRollbackThenUpgrade: "rollback-then-upgrade",
	}
	for op, want := range cases {
		if got := op.String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}
}

func TestMaxHistoryRevisions_isPositive(t *testing.T) {
	// action.History treats Max == 0 as "return nothing", which would make every release
	// look absent and turn an upgrade into a fresh install.
	if maxHistoryRevisions <= 0 {
		t.Fatalf("maxHistoryRevisions = %d, want a positive limit", maxHistoryRevisions)
	}
}
