/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helm

import (
	"fmt"

	"helm.sh/helm/v3/pkg/release"
)

// maxHistoryRevisions bounds how many stored revisions are read when planning. Helm's
// action.History truncates to Max, and treats a Max of 0 as "return nothing", so this must
// be a positive number large enough to cover any retained history (Helm keeps 10 by
// default) rather than left unset.
const maxHistoryRevisions = 1024

// releaseOperation is the operation needed to move a release towards the desired chart.
type releaseOperation int

const (
	// operationInstall installs the release; there is nothing to preserve.
	operationInstall releaseOperation = iota
	// operationUpgrade upgrades the release in place.
	operationUpgrade
	// operationPurgeThenInstall removes an unusable release (history included) and installs
	// it again. Only planned when no successful revision would be discarded.
	operationPurgeThenInstall
	// operationRollbackThenUpgrade rolls the release back to a known-good revision so the
	// storage leaves the pending state, then upgrades from there.
	operationRollbackThenUpgrade
)

func (o releaseOperation) String() string {
	switch o {
	case operationInstall:
		return "install"
	case operationUpgrade:
		return "upgrade"
	case operationPurgeThenInstall:
		return "purge-then-install"
	case operationRollbackThenUpgrade:
		return "rollback-then-upgrade"
	default:
		return fmt.Sprintf("unknown(%d)", int(o))
	}
}

// recoveryPlan is the decision taken from a release's stored history. It is produced by
// planRelease, which is pure so every recovery rule is unit-testable without a cluster.
type recoveryPlan struct {
	// operation is what the caller must perform.
	operation releaseOperation
	// rollbackRevision is the revision to roll back to, set only for
	// operationRollbackThenUpgrade.
	rollbackRevision int
	// reason explains the decision and is logged before acting, so an operator can see why
	// a destructive step was taken.
	reason string
}

// planRelease decides how to reach the desired chart from the release's stored history.
//
// A Helm operation that is interrupted rather than failing cleanly leaves the release in a
// pending state. Helm refuses to upgrade such a release ("another operation is in
// progress"), so choosing install-versus-upgrade purely on whether history exists wedges
// the release permanently. Each pending state is recovered here instead:
//
//   - pending-install at revision 1 has no earlier successful revision to preserve, so the
//     release is purged and installed again;
//   - pending-upgrade and pending-rollback are rolled back to the newest successful
//     revision, which returns the storage to a usable state before the upgrade;
//   - a pending state with no successful revision to recover to is reported as an error
//     rather than resolved destructively.
//
// The caller must pass the full history; revisions are compared numerically so no ordering
// is assumed.
func planRelease(history []*release.Release) (recoveryPlan, error) {
	latest := latestRevision(history)
	if latest == nil {
		// No usable history: nothing to preserve and nothing to upgrade from.
		return recoveryPlan{operation: operationInstall, reason: "no release history found"}, nil
	}

	status := release.StatusUnknown
	if latest.Info != nil {
		status = latest.Info.Status
	}

	switch status {
	case release.StatusDeployed:
		return recoveryPlan{
			operation: operationUpgrade,
			reason:    fmt.Sprintf("latest revision %d is deployed", latest.Version),
		}, nil

	case release.StatusUninstalled:
		// History was kept after an uninstall (--keep-history). A plain install would be
		// rejected by Helm's availableName check ("cannot re-use a name that is still in
		// use") because the retained history still holds the name, and this package does not
		// set Install.Replace. Purging first drops that history; the resources were already
		// removed by the uninstall, so nothing live is discarded.
		return recoveryPlan{
			operation: operationPurgeThenInstall,
			reason:    fmt.Sprintf("latest revision %d is uninstalled with retained history", latest.Version),
		}, nil

	case release.StatusUninstalling:
		// An interrupted uninstall is ambiguous: the recorded intent was removal, so
		// neither upgrading nor purging can be shown to be what the operator wanted.
		return recoveryPlan{}, fmt.Errorf(
			"release is stuck in %s at revision %d; complete or roll back the uninstall manually before redeploying",
			status, latest.Version)

	case release.StatusPendingInstall:
		// Purging is only safe while there is no successful revision to lose. A
		// pending-install normally only exists at revision 1, so anything else is treated
		// as a pending state needing rollback rather than a purge.
		successful := latestSuccessfulRevision(history)
		if latest.Version == 1 && successful == 0 {
			return recoveryPlan{
				operation: operationPurgeThenInstall,
				reason:    "revision 1 is stuck in pending-install and no successful revision exists",
			}, nil
		}
		if successful > 0 {
			return recoveryPlan{
				operation:        operationRollbackThenUpgrade,
				rollbackRevision: successful,
				reason: fmt.Sprintf("latest revision %d is stuck in %s; recovering to revision %d",
					latest.Version, status, successful),
			}, nil
		}
		return recoveryPlan{}, fmt.Errorf(
			"release is stuck in %s at revision %d with no successful revision to recover to; resolve it manually before redeploying",
			status, latest.Version)

	case release.StatusPendingUpgrade, release.StatusPendingRollback:
		successful := latestSuccessfulRevision(history)
		if successful == 0 {
			return recoveryPlan{}, fmt.Errorf(
				"release is stuck in %s at revision %d with no successful revision to recover to; resolve it manually before redeploying",
				status, latest.Version)
		}
		return recoveryPlan{
			operation:        operationRollbackThenUpgrade,
			rollbackRevision: successful,
			reason: fmt.Sprintf("latest revision %d is stuck in %s; recovering to revision %d",
				latest.Version, status, successful),
		}, nil

	default:
		// failed, superseded and unknown are all states Helm can upgrade from. Preserving
		// the upgrade here keeps the existing behaviour for an ordinary failed release.
		return recoveryPlan{
			operation: operationUpgrade,
			reason:    fmt.Sprintf("latest revision %d has status %s", latest.Version, status),
		}, nil
	}
}

// latestRevision returns the entry with the highest revision number. Helm sorts history
// before returning it, but the order is not part of the contract this package relies on.
func latestRevision(history []*release.Release) *release.Release {
	var latest *release.Release
	for _, rel := range history {
		if rel == nil {
			continue
		}
		if latest == nil || rel.Version > latest.Version {
			latest = rel
		}
	}
	return latest
}

// latestSuccessfulRevision returns the highest revision that reached a usable state, or 0
// when there is none. Deployed is the current successful revision; superseded revisions
// were deployed successfully before a later revision replaced them, so both are valid
// rollback targets.
func latestSuccessfulRevision(history []*release.Release) int {
	best := 0
	for _, rel := range history {
		if rel == nil || rel.Info == nil {
			continue
		}
		switch rel.Info.Status {
		case release.StatusDeployed, release.StatusSuperseded:
			if rel.Version > best {
				best = rel.Version
			}
		}
	}
	return best
}
