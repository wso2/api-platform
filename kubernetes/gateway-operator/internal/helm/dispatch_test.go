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
	"context"
	"errors"
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
)

// recordingExecutor records the operations planAndExecute dispatches, in order, so the
// sequence and arguments can be asserted. Any operation can be made to fail.
type recordingExecutor struct {
	calls             []string
	rollbackRevisions []int
	contexts          []context.Context

	installErr  error
	upgradeErr  error
	purgeErr    error
	rollbackErr error
}

func (e *recordingExecutor) install(ctx context.Context, _ InstallOrUpgradeOptions) error {
	e.calls = append(e.calls, "install")
	e.contexts = append(e.contexts, ctx)
	return e.installErr
}

func (e *recordingExecutor) upgrade(ctx context.Context, _ InstallOrUpgradeOptions) error {
	e.calls = append(e.calls, "upgrade")
	e.contexts = append(e.contexts, ctx)
	return e.upgradeErr
}

func (e *recordingExecutor) purge(ctx context.Context, _ InstallOrUpgradeOptions) error {
	e.calls = append(e.calls, "purge")
	e.contexts = append(e.contexts, ctx)
	return e.purgeErr
}

func (e *recordingExecutor) rollback(ctx context.Context, _ InstallOrUpgradeOptions, revision int) error {
	e.calls = append(e.calls, "rollback")
	e.rollbackRevisions = append(e.rollbackRevisions, revision)
	e.contexts = append(e.contexts, ctx)
	return e.rollbackErr
}

func historyReturning(history []*release.Release, err error) func(string) ([]*release.Release, error) {
	return func(string) ([]*release.Release, error) { return history, err }
}

func joined(calls []string) string { return strings.Join(calls, ",") }

var dispatchOpts = InstallOrUpgradeOptions{ReleaseName: "test-gw", Namespace: "gw-ns"}

func TestPlanAndExecute_dispatchOrder(t *testing.T) {
	tests := []struct {
		name          string
		history       []*release.Release
		historyErr    error
		wantCalls     string
		wantRollbacks []int
		wantErrSubstr string
	}{
		{
			// Absent release: install, and never touch a destructive operation.
			name:       "missing history installs",
			historyErr: driver.ErrReleaseNotFound,
			wantCalls:  "install",
		},
		{
			name:      "deployed upgrades",
			history:   []*release.Release{rel(1, release.StatusDeployed)},
			wantCalls: "upgrade",
		},
		{
			// The interrupted first install: purge must precede install, in that order.
			name:      "revision 1 pending install purges then installs",
			history:   []*release.Release{rel(1, release.StatusPendingInstall)},
			wantCalls: "purge,install",
		},
		{
			// The bug the planner suite missed: a plain install here is rejected by Helm.
			name:      "uninstalled retained history purges then installs",
			history:   []*release.Release{rel(1, release.StatusUninstalled)},
			wantCalls: "purge,install",
		},
		{
			name: "pending upgrade rolls back to the planned revision then upgrades",
			history: []*release.Release{
				rel(1, release.StatusSuperseded),
				rel(2, release.StatusDeployed),
				rel(3, release.StatusPendingUpgrade),
			},
			wantCalls:     "rollback,upgrade",
			wantRollbacks: []int{2},
		},
		{
			name: "pending rollback rolls back then upgrades",
			history: []*release.Release{
				rel(1, release.StatusSuperseded),
				rel(2, release.StatusPendingRollback),
			},
			wantCalls:     "rollback,upgrade",
			wantRollbacks: []int{1},
		},
		{
			// No safe revision: nothing destructive may run.
			name: "unsafe pending state dispatches nothing",
			history: []*release.Release{
				rel(1, release.StatusFailed),
				rel(2, release.StatusPendingUpgrade),
			},
			wantCalls:     "",
			wantErrSubstr: "no successful revision to recover to",
		},
		{
			name:          "uninstalling dispatches nothing",
			history:       []*release.Release{rel(1, release.StatusUninstalling)},
			wantCalls:     "",
			wantErrSubstr: "stuck in uninstalling",
		},
		{
			// A transient storage or unreachable-cluster error must never look like "absent",
			// or a fresh install would run over a live release.
			name:          "non not-found history error dispatches nothing",
			historyErr:    errors.New("etcdserver: request timed out"),
			wantCalls:     "",
			wantErrSubstr: "failed to read history",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exec := &recordingExecutor{}
			err := planAndExecute(context.Background(), historyReturning(tc.history, tc.historyErr), exec, dispatchOpts)

			if tc.wantErrSubstr != "" {
				if err == nil {
					t.Fatalf("expected an error containing %q, got nil (calls: %q)", tc.wantErrSubstr, joined(exec.calls))
				}
				if !strings.Contains(err.Error(), tc.wantErrSubstr) {
					t.Fatalf("error = %q, want it to contain %q", err.Error(), tc.wantErrSubstr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := joined(exec.calls); got != tc.wantCalls {
				t.Errorf("dispatched %q, want %q", got, tc.wantCalls)
			}
			if tc.wantRollbacks != nil {
				if len(exec.rollbackRevisions) != len(tc.wantRollbacks) {
					t.Fatalf("rollback revisions = %v, want %v", exec.rollbackRevisions, tc.wantRollbacks)
				}
				for i, want := range tc.wantRollbacks {
					if exec.rollbackRevisions[i] != want {
						t.Errorf("rollback revision[%d] = %d, want %d", i, exec.rollbackRevisions[i], want)
					}
				}
			}
		})
	}
}

func TestPlanAndExecute_noInstallWhenPurgeFails(t *testing.T) {
	// Installing after a failed purge would hit the very rejection the purge exists to avoid.
	exec := &recordingExecutor{purgeErr: errors.New("purge boom")}
	err := planAndExecute(context.Background(),
		historyReturning([]*release.Release{rel(1, release.StatusPendingInstall)}, nil), exec, dispatchOpts)

	if err == nil {
		t.Fatal("expected the purge error to be returned")
	}
	if got := joined(exec.calls); got != "purge" {
		t.Errorf("dispatched %q, want only %q", got, "purge")
	}
}

func TestPlanAndExecute_noUpgradeWhenRollbackFails(t *testing.T) {
	// The release is still pending, so upgrading would be rejected by Helm.
	exec := &recordingExecutor{rollbackErr: errors.New("rollback boom")}
	err := planAndExecute(context.Background(),
		historyReturning([]*release.Release{
			rel(1, release.StatusDeployed),
			rel(2, release.StatusPendingUpgrade),
		}, nil), exec, dispatchOpts)

	if err == nil {
		t.Fatal("expected the rollback error to be returned")
	}
	if got := joined(exec.calls); got != "rollback" {
		t.Errorf("dispatched %q, want only %q", got, "rollback")
	}
}

func TestPlanAndExecute_operationErrorsPropagate(t *testing.T) {
	installBoom := errors.New("install boom")
	exec := &recordingExecutor{installErr: installBoom}
	err := planAndExecute(context.Background(), historyReturning(nil, driver.ErrReleaseNotFound), exec, dispatchOpts)
	if !errors.Is(err, installBoom) {
		t.Errorf("error = %v, want it to wrap %v", err, installBoom)
	}
}

func TestPlanAndExecute_threadsCallerContext(t *testing.T) {
	// The caller's context must reach the operations so a cancelled reconcile or an operator
	// shutdown can abort the Helm wait rather than blocking for the whole timeout.
	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("marker"), "present")

	exec := &recordingExecutor{}
	if err := planAndExecute(ctx,
		historyReturning([]*release.Release{rel(1, release.StatusPendingUpgrade), rel(2, release.StatusDeployed)}, nil),
		exec, dispatchOpts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(exec.contexts) == 0 {
		t.Fatal("expected at least one dispatched operation")
	}
	for i, got := range exec.contexts {
		if got.Value(ctxKey("marker")) != "present" {
			t.Errorf("operation %d did not receive the caller's context", i)
		}
	}
}

func TestPlanAndExecute_cancelledContextStillPropagates(t *testing.T) {
	// planAndExecute does not itself check cancellation; it must hand the cancelled context
	// to the operation, which is where the Helm SDK observes it via RunWithContext.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	exec := &recordingExecutor{}
	if err := planAndExecute(ctx, historyReturning(nil, driver.ErrReleaseNotFound), exec, dispatchOpts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(exec.contexts) != 1 {
		t.Fatalf("dispatched %d operations, want 1", len(exec.contexts))
	}
	if !errors.Is(exec.contexts[0].Err(), context.Canceled) {
		t.Errorf("operation context error = %v, want context.Canceled", exec.contexts[0].Err())
	}
}

// --- action configuration -----------------------------------------------------------------

func TestNewPurgeAction_dropsHistory(t *testing.T) {
	// KeepHistory=false is what removes the retained history holding the release name, and
	// what lets an already-uninstalled release be purged instead of erroring "already
	// deleted". A true value here would silently break both purge paths.
	client := newPurgeAction(&action.Configuration{}, InstallOrUpgradeOptions{Wait: true, Timeout: 90})
	if client.KeepHistory {
		t.Error("KeepHistory = true, want false so the retained history is removed")
	}
	if !client.Wait {
		t.Error("Wait was not carried over from the options")
	}
	if client.Timeout.Seconds() != 90 {
		t.Errorf("Timeout = %v, want 90s", client.Timeout)
	}
}

func TestNewRollbackAction_targetsPlannedRevision(t *testing.T) {
	client := newRollbackAction(&action.Configuration{}, InstallOrUpgradeOptions{Wait: true, Timeout: 30}, 7)
	if client.Version != 7 {
		t.Errorf("Version = %d, want the planned revision 7", client.Version)
	}
	if !client.CleanupOnFail {
		t.Error("CleanupOnFail = false, want true so a failed rollback does not leave new resources")
	}
	if !client.Wait {
		t.Error("Wait was not carried over from the options")
	}
	if client.Timeout.Seconds() != 30 {
		t.Errorf("Timeout = %v, want 30s", client.Timeout)
	}
}

func TestNewRollbackAction_zeroRevisionIsNotPlanned(t *testing.T) {
	// Guard on the planner contract rather than the action: revision 0 means "previous" to
	// Helm, which would be an unintended target, so planRelease must never emit it.
	plan, err := planRelease([]*release.Release{
		rel(1, release.StatusSuperseded),
		rel(2, release.StatusPendingUpgrade),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.rollbackRevision <= 0 {
		t.Errorf("rollbackRevision = %d, want a concrete revision", plan.rollbackRevision)
	}
}
