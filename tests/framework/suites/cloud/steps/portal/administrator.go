/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package portal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

// Administrator is the suite's administrator of the shared portal, signed in, and what its sweeps
// removed; or why the run made none.
type Administrator struct {
	client      *httpx.Client
	mu          sync.Mutex
	browser     *portalBrowser
	admin       portalAdmin
	portal      *url.URL
	platform    cloudbase.OrganisationAPI
	environment string
	sweeps      []portalSweep
	unavailable error
}

// PrepareAdministrator makes the suite's portal administrator, signs it in and sweeps the leftovers
// of every run from the shared portal. It never fails: without an administrator the scenarios still
// run, the leftovers stay, and Report says why. pollInterval is how often the Identity Platform is
// asked for a change that is pending; zero selects the default.
func PrepareAdministrator(ctx context.Context, client *httpx.Client, platform cloudbase.OrganisationAPI,
	portalID string, pollInterval time.Duration, log *slog.Logger) *Administrator {
	state := &Administrator{client: client, platform: platform}
	state.unavailable = state.make(ctx, portalID, pollInterval, log)
	if state.unavailable != nil {
		log.Warn("the portal administrator was not made; leftovers in the portal stay", "portal", portalID, "reason", state.unavailable)
	}
	return state
}

func (a *Administrator) make(ctx context.Context, portalID string, pollInterval time.Duration, log *slog.Logger) error {
	identity, err := identityPlatformFromEnv(a.client)
	if err != nil {
		return err
	}
	identity.pollInterval = pollInterval
	portal, environment, err := readPortalRecord(ctx, a.platform, portalID)
	if err != nil {
		return err
	}
	admin, err := ensurePortalAdmin(ctx, identity, environment, log)
	if err != nil {
		return err
	}
	browser, err := signInAs(ctx, a.client, portal, admin.username, admin.password)
	if err != nil {
		return err
	}
	a.browser, a.admin, a.portal, a.environment = browser, admin, portal, environment
	sweep, err := sweepPortal(ctx, browser, platformHoldsAPI(a.platform), sweepScope{now: time.Now()})
	a.sweeps = append(a.sweeps, sweep)
	if err != nil {
		log.Warn("the portal sweep at the start stopped; the one at the end tries again", "portal", portalID, "reason", err)
	}
	log.Info("swept the portal's leftovers of every run", "portal", portalID, "result", sweep.String())
	return nil
}

// Report describes, one line each, what the administrator swept, or why there is none.
func (a *Administrator) Report() []string {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	var lines []string
	switch {
	case a.unavailable != nil:
		lines = append(lines, fmt.Sprintf("the portal administrator was not made, so the portal's leftovers were not removed: %v", a.unavailable))
	case a.browser != nil:
		for i, sweep := range a.sweeps {
			when := "at the start"
			if i > 0 {
				when = "at the end"
			}
			lines = append(lines, fmt.Sprintf("portal administrator %s on %s swept the portal %s: %s", a.admin.username, a.environment, when, sweep))
		}
	}
	return lines
}

// Sweep removes the leftovers of every run from the shared portal with the administrator, signing it
// in again when the portal no longer takes its session. ownedByRun says which synthetic names this
// run made; their plans go whatever their age. It does nothing when there is no administrator, and
// returns what it removed.
func (a *Administrator) Sweep(ctx context.Context, ownedByRun func(id string) bool) ([]string, error) {
	if a == nil {
		return nil, nil
	}
	a.mu.Lock()
	browser := a.browser
	a.mu.Unlock()
	if browser == nil {
		return nil, nil
	}
	scope := sweepScope{ownedByRun: ownedByRun, now: time.Now()}
	sweep, err := sweepPortal(ctx, browser, platformHoldsAPI(a.platform), scope)
	if errors.Is(err, errPortalSessionEnded) {
		signedIn, signErr := signInAs(ctx, a.client, a.portal, a.admin.username, a.admin.password)
		if signErr != nil {
			err = fmt.Errorf("signing the portal administrator in again: %w", signErr)
		} else {
			browser = signedIn
			var again portalSweep
			again, err = sweepPortal(ctx, browser, platformHoldsAPI(a.platform), scope)
			sweep = sweep.merge(again)
		}
	}
	a.mu.Lock()
	a.browser = browser
	a.sweeps = append(a.sweeps, sweep)
	a.mu.Unlock()
	if err == nil && (len(sweep.inUse) > 0 || len(sweep.failed) > 0) {
		err = fmt.Errorf("kept in use: %s; failed: %s", strings.Join(sweep.inUse, ", "), strings.Join(sweep.failed, ", "))
	}
	return sweep.removed, err
}
