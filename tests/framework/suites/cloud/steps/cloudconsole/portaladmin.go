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

package cloudconsole

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
)

const (
	// cloudIAMAccessTokenEnv holds a token of the organisation's administrator for the Identity
	// Platform API, and cloudIAMAPIURLEnv that API's base URL.
	cloudIAMAccessTokenEnv = "CLOUD_IAM_ACCESS_TOKEN"
	// cloudIAMAccessTokenFileEnv names a file holding that token, replaced before the token expires.
	cloudIAMAccessTokenFileEnv = "CLOUD_IAM_ACCESS_TOKEN_FILE"
	cloudIAMAPIURLEnv          = "CLOUD_IAM_API_URL"

	// portalAdminUsername is the suite's administrator of the organisation's managed API Portals.
	portalAdminUsername = testResourcePrefix + "portal-admin"
	// portalAdminRole is the role a portal maps to its administrator, and portalUserType the only
	// user type a portal's sign-in accepts.
	portalAdminRole = "dp_admin"
	portalUserType  = "apip-portal-subscriber"
	// portalUsersOU is the organization unit a portal's users and roles belong to.
	portalUsersOU = "default"

	// syntheticListingPrefix starts the handle of every API a scenario creates, and so of its
	// listing on a portal.
	syntheticListingPrefix = "syn-"

	portalListPageSize = 50
	portalListMaxPages = 200

	// portalSweepMinAge is how old another run's synthetic plan must be before a sweep removes
	// it: longer than a run, so a plan a run is still publishing with is never taken from it.
	portalSweepMinAge = 30 * time.Minute

	// portalAdminDeployTimeout bounds the wait for a change to be pending and for a deploy that
	// another deploy holds up.
	portalAdminDeployTimeout = 2 * time.Minute

	// gatewayApplyInProgress is the Identity Platform's answer to a deploy while another one runs.
	gatewayApplyInProgress = "GTW-1018"
)

var (
	// errDeployInProgress marks a deploy refused because another deploy to the gateway runs.
	errDeployInProgress = errors.New("another deploy to the environment is in progress")
	// errPortalSessionEnded marks a portal answer that no longer takes the administrator's session.
	errPortalSessionEnded = errors.New("the portal no longer takes the administrator's session")
)

// identityPlatform calls the Identity Platform API as the organisation's administrator.
type identityPlatform struct {
	client *httpx.Client
	base   string
	token  string
	// pollInterval is how often a pending change or a deploy held up by another is checked; zero
	// selects the default.
	pollInterval time.Duration
}

// identityPlatformFromEnv returns the Identity Platform API the run may call, or why it may not.
func identityPlatformFromEnv(client *httpx.Client) (identityPlatform, error) {
	token, err := currentToken(cloudIAMAccessTokenEnv, cloudIAMAccessTokenFileEnv)
	if err != nil {
		return identityPlatform{}, err
	}
	raw := strings.TrimRight(strings.TrimSpace(os.Getenv(cloudIAMAPIURLEnv)), "/")
	if raw == "" {
		return identityPlatform{}, fmt.Errorf("%s is not set", cloudIAMAPIURLEnv)
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return identityPlatform{}, fmt.Errorf("%s does not parse: %w", cloudIAMAPIURLEnv, err)
	}
	if err := requireSecureURL(parsed); err != nil {
		return identityPlatform{}, fmt.Errorf("%s: %w", cloudIAMAPIURLEnv, err)
	}
	return identityPlatform{client: client, base: raw, token: token}, nil
}

func (p identityPlatform) do(ctx context.Context, method, path string, body any) (*httpx.Response, error) {
	headers := map[string]string{"Authorization": "Bearer " + liveToken(p.token, cloudIAMAccessTokenFileEnv), "Accept": "application/json"}
	var raw []byte
	if body != nil {
		var err error
		if raw, err = json.Marshal(body); err != nil {
			return nil, fmt.Errorf("encoding the request to %s: %w", path, err)
		}
		headers["Content-Type"] = "application/json"
	}
	return p.client.Do(ctx, httpx.Request{Method: method, URL: p.base + path, Headers: headers, Body: raw}, 0, 0)
}

// decodeAnswer reads a JSON answer of one of the expected statuses into out.
func decodeAnswer(resp *httpx.Response, out any, what string, statuses ...int) error {
	if !slices.Contains(statuses, resp.StatusCode) {
		return fmt.Errorf("%s: %w", what, responseError(resp))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(resp.Body, out); err != nil {
		return fmt.Errorf("%s: decoding the answer: %w", what, err)
	}
	return nil
}

// organizationUnit returns the id of the organization unit a portal's users belong to.
func (p identityPlatform) organizationUnit(ctx context.Context) (string, error) {
	resp, err := p.do(ctx, http.MethodGet, "/organization-units/tree/"+url.PathEscape(portalUsersOU), nil)
	if err != nil {
		return "", err
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := decodeAnswer(resp, &out, "reading the "+portalUsersOU+" organization unit", http.StatusOK); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", fmt.Errorf("the %s organization unit has no id", portalUsersOU)
	}
	return out.ID, nil
}

// role returns the id of the named role of the organization unit a portal's users belong to.
func (p identityPlatform) role(ctx context.Context, name string) (string, error) {
	resp, err := p.do(ctx, http.MethodGet, "/organization-units/tree/"+url.PathEscape(portalUsersOU)+"/roles?limit=100&offset=0", nil)
	if err != nil {
		return "", err
	}
	var out struct {
		Roles []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"roles"`
	}
	if err := decodeAnswer(resp, &out, "listing the roles of the "+portalUsersOU+" organization unit", http.StatusOK); err != nil {
		return "", err
	}
	for _, role := range out.Roles {
		if role.Name == name && role.ID != "" {
			return role.ID, nil
		}
	}
	return "", fmt.Errorf("the %s organization unit holds no %q role", portalUsersOU, name)
}

// usersNamed returns the ids of the users with the username.
func (p identityPlatform) usersNamed(ctx context.Context, username string) ([]string, error) {
	filter := url.QueryEscape(fmt.Sprintf("username eq %q", username))
	resp, err := p.do(ctx, http.MethodGet, "/users?limit=100&filter="+filter, nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Users []struct {
			ID string `json:"id"`
		} `json:"users"`
	}
	if err := decodeAnswer(resp, &out, "looking up the user "+username, http.StatusOK); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(out.Users))
	for _, user := range out.Users {
		if user.ID != "" {
			ids = append(ids, user.ID)
		}
	}
	return ids, nil
}

func (p identityPlatform) deleteUser(ctx context.Context, id string) error {
	resp, err := p.do(ctx, http.MethodDelete, "/users/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	return decodeAnswer(resp, nil, "deleting the user "+id, http.StatusNoContent, http.StatusOK, http.StatusNotFound)
}

// createUser creates a portal user in the organization unit and returns its id.
func (p identityPlatform) createUser(ctx context.Context, unit, username, password string) (string, error) {
	resp, err := p.do(ctx, http.MethodPost, "/users", map[string]any{
		"ouId": unit,
		"type": portalUserType,
		"attributes": map[string]string{
			"username": username,
			"email":    username + "@example.com",
			"password": password,
		},
	})
	if err != nil {
		return "", err
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := decodeAnswer(resp, &out, "creating the user "+username, http.StatusCreated, http.StatusOK); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", fmt.Errorf("creating the user %s: the answer names no id", username)
	}
	return out.ID, nil
}

// assignRole assigns the role to the user. A user the role already holds is not an error.
func (p identityPlatform) assignRole(ctx context.Context, roleID, userID string) error {
	resp, err := p.do(ctx, http.MethodPost, "/roles/"+url.PathEscape(roleID)+"/assignments/add",
		map[string]any{"assignments": []map[string]string{{"type": "user", "id": userID}}})
	if err != nil {
		return err
	}
	return decodeAnswer(resp, nil, "assigning the role "+roleID, http.StatusNoContent, http.StatusOK, http.StatusConflict)
}

// configChange is one entry of an environment's diff: added, updated, deleted or unchanged.
type configChange struct {
	Change string `json:"change"`
	Key    string `json:"key"`
	Name   string `json:"name"`
}

func (c configChange) pending() bool {
	return c.Change == "added" || c.Change == "updated" || c.Change == "deleted"
}

// changes returns the environment's diff: every resource of the latest version against what its
// Data Plane holds.
func (p identityPlatform) changes(ctx context.Context, environment string) (map[string]configChange, error) {
	resp, err := p.do(ctx, http.MethodGet, "/environments/"+url.PathEscape(environment)+"/diff", nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Changes []configChange `json:"changes"`
	}
	if err := decodeAnswer(resp, &out, "reading the pending changes of "+environment, http.StatusOK); err != nil {
		return nil, err
	}
	byKey := make(map[string]configChange, len(out.Changes))
	for _, change := range out.Changes {
		byKey[change.Key] = change
	}
	return byKey, nil
}

// capture makes the organisation's current configuration a version and returns it.
func (p identityPlatform) capture(ctx context.Context) (string, error) {
	resp, err := p.do(ctx, http.MethodPost, "/configuration-versions",
		map[string]string{"name": portalAdminUsername, "note": "the APIP cloud integration tests' portal administrator"})
	if err != nil {
		return "", err
	}
	var out struct {
		Version string `json:"version"`
	}
	if err := decodeAnswer(resp, &out, "capturing a configuration version", http.StatusCreated, http.StatusOK); err != nil {
		return "", err
	}
	if out.Version == "" {
		return "", errors.New("capturing a configuration version: the answer names no version")
	}
	return out.Version, nil
}

// apply deploys only the selected changes of the version to the environment's Data Plane. The
// Identity Platform answers 200 even when items of the import failed, so the import's results are
// checked too.
func (p identityPlatform) apply(ctx context.Context, environment, version string, selection []string) error {
	resp, err := p.do(ctx, http.MethodPost, "/environments/"+url.PathEscape(environment)+"/apply",
		map[string]any{"version": version, "selection": selection})
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusConflict && strings.Contains(string(resp.Body), gatewayApplyInProgress) {
		return errDeployInProgress
	}
	var out struct {
		Import struct {
			Summary struct {
				Failed int `json:"failed"`
			} `json:"summary"`
			Results []struct {
				ResourceType string `json:"resourceType"`
				ResourceID   string `json:"resourceId"`
				Operation    string `json:"operation"`
				Status       string `json:"status"`
				Code         string `json:"code"`
				Message      string `json:"message"`
			} `json:"results"`
		} `json:"import"`
	}
	what := fmt.Sprintf("deploying %s to %s", strings.Join(selection, ", "), environment)
	if err := decodeAnswer(resp, &out, what, http.StatusOK); err != nil {
		return err
	}
	var failed []string
	for _, result := range out.Import.Results {
		if result.Status == "failed" {
			failed = append(failed, fmt.Sprintf("%s %s/%s: %s %s", result.Operation, result.ResourceType, result.ResourceID, result.Code, result.Message))
		}
	}
	if len(failed) > 0 || out.Import.Summary.Failed > 0 {
		return fmt.Errorf("%s: %d item(s) failed: %s", what, max(len(failed), out.Import.Summary.Failed), strings.Join(failed, "; "))
	}
	return nil
}

// deploy sends the given changes, and no other, to the environment's Data Plane. It waits until
// every change in must is pending, and sends each change in also that is pending along with them.
func (p identityPlatform) deploy(ctx context.Context, environment string, must, also []string) error {
	var version string
	var selection []string
	ready, err := retry.Until(ctx, retry.Options{Timeout: portalAdminDeployTimeout, Interval: p.pollInterval,
		Retryable: func(error) bool { return false }},
		func(ctx context.Context) (bool, error) {
			captured, err := p.capture(ctx)
			if err != nil {
				return false, err
			}
			changes, err := p.changes(ctx, environment)
			if err != nil {
				return false, err
			}
			for _, key := range must {
				if !changes[key].pending() {
					return false, nil
				}
			}
			version, selection = captured, nil
			for _, key := range append(slices.Clone(must), also...) {
				if changes[key].pending() && !slices.Contains(selection, key) {
					selection = append(selection, key)
				}
			}
			return true, nil
		}, func(ready bool) bool { return ready })
	if err != nil {
		return err
	}
	if !ready {
		return fmt.Errorf("%s never became pending for %s", strings.Join(must, ", "), environment)
	}
	if len(selection) == 0 {
		return nil
	}
	_, err = retry.Until(ctx, retry.Options{Timeout: portalAdminDeployTimeout, Interval: p.pollInterval,
		Retryable: func(err error) bool { return errors.Is(err, errDeployInProgress) }},
		func(ctx context.Context) (bool, error) {
			return true, p.apply(ctx, environment, version, selection)
		}, func(bool) bool { return true })
	return err
}

// portalAdmin is a portal user with the administrator's role, and the password it signs in with.
// The password stays in memory and is never logged.
type portalAdmin struct {
	username string
	password string
}

// ensurePortalAdmin makes the suite's portal administrator again, with a new password, and deploys
// it to the environment portal users sign in on. The Identity Platform deploys a user's password
// only with the user itself, so the previous administrator is removed and a new one created:
//  1. the previous administrator is deleted, and its removal deployed with the administrator's
//     role, which still names it on the Data Plane until then;
//  2. the new administrator is created and given the role, and both are deployed.
//
// The removal goes first because the Data Plane refuses a second user with the same username.
func ensurePortalAdmin(ctx context.Context, platform identityPlatform, environment string, log *slog.Logger) (portalAdmin, error) {
	if strings.TrimSpace(environment) == "" {
		return portalAdmin{}, errors.New("the portal names no environment its users sign in on")
	}
	unit, err := platform.organizationUnit(ctx)
	if err != nil {
		return portalAdmin{}, err
	}
	roleID, err := platform.role(ctx, portalAdminRole)
	if err != nil {
		return portalAdmin{}, err
	}
	roleKey := "role/" + roleID

	previous, err := platform.usersNamed(ctx, portalAdminUsername)
	if err != nil {
		return portalAdmin{}, err
	}
	held, err := platform.changes(ctx, environment)
	if err != nil {
		return portalAdmin{}, err
	}
	var removals []string
	for _, id := range previous {
		if err := platform.deleteUser(ctx, id); err != nil {
			return portalAdmin{}, err
		}
		// A user the diff lists as added was never deployed, so there is nothing to remove.
		if change, ok := held["user/"+id]; ok && change.Change != "added" {
			removals = append(removals, "user/"+id)
		}
	}
	// An administrator an earlier run deleted without deploying the removal is still on the Data
	// Plane, and would refuse the new one's username.
	for key, change := range held {
		if strings.HasPrefix(key, "user/") && change.Change == "deleted" && change.Name == portalAdminUsername && !slices.Contains(removals, key) {
			removals = append(removals, key)
		}
	}
	slices.Sort(removals)
	if len(removals) > 0 {
		if err := platform.deploy(ctx, environment, removals, []string{roleKey}); err != nil {
			return portalAdmin{}, fmt.Errorf("removing the previous portal administrator from %s: %w", environment, err)
		}
	}

	password, err := newConsumerPassword()
	if err != nil {
		return portalAdmin{}, err
	}
	userID, err := platform.createUser(ctx, unit, portalAdminUsername, password)
	if err != nil {
		return portalAdmin{}, err
	}
	if err := platform.assignRole(ctx, roleID, userID); err != nil {
		return portalAdmin{}, err
	}
	if err := platform.deploy(ctx, environment, []string{"user/" + userID}, []string{roleKey}); err != nil {
		return portalAdmin{}, fmt.Errorf("deploying the portal administrator to %s: %w", environment, err)
	}
	log.Info("made the portal administrator", "user", portalAdminUsername, "environment", environment)
	return portalAdmin{username: portalAdminUsername, password: password}, nil
}

// signInAs opens the portal in a new browser and signs the user in with the password. It returns
// the signed-in browser.
func signInAs(ctx context.Context, client *httpx.Client, portal *url.URL, username, password string) (*portalBrowser, error) {
	browser, err := newPortalBrowser(client, portal)
	if err != nil {
		return nil, err
	}
	if _, err := browser.openSignIn(ctx); err != nil {
		return nil, err
	}
	base, authID, step, err := browser.beginFlow(ctx)
	if err != nil {
		return nil, err
	}
	actions := []flowAction{{action: flowActionSubmit, inputs: map[string]string{"username": username, "password": password}}}
	if err := browser.completeSignIn(ctx, base, authID, step, actions, nil); err != nil {
		return nil, err
	}
	if browser.refusal != "" {
		return nil, fmt.Errorf("the identity provider refused the sign-in of %s: %s", username, browser.refusal)
	}
	return browser, nil
}

// portalSweep says what a sweep of a portal removed, kept because it is in use or too recent to be
// a leftover, and could not remove. Each entry is "<kind>/<id>".
type portalSweep struct {
	removed []string
	inUse   []string
	recent  []string
	failed  []string
}

func (s portalSweep) String() string {
	return fmt.Sprintf("removed %d, kept %d in use and %d recent, %d failed", len(s.removed), len(s.inUse), len(s.recent), len(s.failed))
}

// merge adds what another attempt of the same sweep found.
func (s portalSweep) merge(other portalSweep) portalSweep {
	return portalSweep{
		removed: append(s.removed, other.removed...),
		inUse:   append(s.inUse, other.inUse...),
		recent:  append(s.recent, other.recent...),
		failed:  append(s.failed, other.failed...),
	}
}

// apiHeld reports whether the platform still holds an API.
type apiHeld func(ctx context.Context, id string) (bool, error)

// sweepScope says which synthetic plans a sweep may remove: those of this run, whatever their
// age, and another run's once they are older than portalSweepMinAge, so a run still publishing
// keeps its plans. ownedByRun is nil before the run has made any.
type sweepScope struct {
	ownedByRun func(id string) bool
	now        time.Time
}

func (s sweepScope) removable(item portalItem) bool {
	if s.ownedByRun != nil && s.ownedByRun(item.ID) {
		return true
	}
	return item.CreatedAt.IsZero() || s.now.Sub(item.CreatedAt) >= portalSweepMinAge
}

// sweepPortal removes the leftovers of every run from the portal, whichever run made them: the
// listings of synthetic APIs the platform no longer holds, then the synthetic subscription plans
// scope allows. The portal refuses to remove a plan an API offers or a subscription uses, and a
// listing a subscription uses; those are kept and reported. Nothing without a synthetic name is
// touched.
func sweepPortal(ctx context.Context, browser *portalBrowser, held apiHeld, scope sweepScope) (portalSweep, error) {
	var sweep portalSweep
	listings, err := browser.listItems(ctx, "/apis")
	if err != nil {
		return sweep, err
	}
	for _, listing := range listings {
		id := listing.ID
		if !strings.HasPrefix(id, syntheticListingPrefix) {
			continue
		}
		kept, err := held(ctx, id)
		if err != nil {
			sweep.failed = append(sweep.failed, "api/"+id+": "+err.Error())
			continue
		}
		if kept {
			continue
		}
		if err := sweep.remove(ctx, browser, "api", "/apis/", id); err != nil {
			return sweep, err
		}
	}
	plans, err := browser.listItems(ctx, "/subscription-plans")
	if err != nil {
		return sweep, err
	}
	for _, plan := range plans {
		if !strings.HasPrefix(plan.ID, syntheticPlanPrefix) {
			continue
		}
		if !scope.removable(plan) {
			sweep.recent = append(sweep.recent, "subscription-plan/"+plan.ID)
			continue
		}
		if err := sweep.remove(ctx, browser, "subscription-plan", "/subscription-plans/", plan.ID); err != nil {
			return sweep, err
		}
	}
	return sweep, nil
}

// remove deletes one leftover and records the outcome. It returns an error only when the portal no
// longer takes the administrator's session.
func (s *portalSweep) remove(ctx context.Context, browser *portalBrowser, kind, path, id string) error {
	entry := kind + "/" + id
	resp, err := browser.apiRequest(ctx, http.MethodDelete, path+url.PathEscape(id), nil)
	if err != nil {
		s.failed = append(s.failed, entry+": "+err.Error())
		return nil
	}
	switch {
	case resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK:
		s.removed = append(s.removed, entry)
	case resp.StatusCode == http.StatusNotFound:
	case resp.StatusCode == http.StatusConflict:
		s.inUse = append(s.inUse, entry)
	case sessionEnded(resp.StatusCode):
		return errPortalSessionEnded
	default:
		s.failed = append(s.failed, fmt.Sprintf("%s: answered %d", entry, resp.StatusCode))
	}
	return nil
}

func sessionEnded(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden || (status >= 300 && status < 400)
}

// portalItem is an item of a portal collection: its id, and when it was created when the portal
// says.
type portalItem struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
}

// listItems returns every item of a portal collection, page by page.
func (b *portalBrowser) listItems(ctx context.Context, collection string) ([]portalItem, error) {
	var items []portalItem
	for page := range portalListMaxPages {
		offset := page * portalListPageSize
		resp, err := b.apiRequest(ctx, http.MethodGet, fmt.Sprintf("%s?limit=%d&offset=%d", collection, portalListPageSize, offset), nil)
		if err != nil {
			return nil, err
		}
		if sessionEnded(resp.StatusCode) {
			return nil, errPortalSessionEnded
		}
		var out struct {
			List       []portalItem `json:"list"`
			Pagination struct {
				Total int `json:"total"`
			} `json:"pagination"`
		}
		if err := decodeAnswer(resp, &out, "listing the portal's "+strings.TrimPrefix(collection, "/"), http.StatusOK); err != nil {
			return nil, err
		}
		for _, item := range out.List {
			if item.ID != "" {
				items = append(items, item)
			}
		}
		if len(out.List) < portalListPageSize || offset+len(out.List) >= out.Pagination.Total {
			return items, nil
		}
	}
	return nil, fmt.Errorf("the portal's %s run past %d pages", strings.TrimPrefix(collection, "/"), portalListMaxPages)
}

// platformHoldsAPI reports whether the platform still holds a REST or GraphQL API of the id.
func platformHoldsAPI(platform organisationAPI) apiHeld {
	return func(ctx context.Context, id string) (bool, error) {
		for _, collection := range []string{restAPIsCollection, graphQLAPIsCollection} {
			resp, err := platform.get(ctx, collection+"/"+url.PathEscape(id))
			if err != nil {
				return false, err
			}
			switch resp.StatusCode {
			case http.StatusOK:
				return true, nil
			case http.StatusNotFound:
			default:
				return false, fmt.Errorf("reading %s%s: %w", collection, "/"+id, responseError(resp))
			}
		}
		return false, nil
	}
}

// readPortalRecord returns the URL of a managed API Portal and the environment its users sign in on.
func readPortalRecord(ctx context.Context, platform organisationAPI, id string) (*url.URL, string, error) {
	resp, err := platform.get(ctx, portalPath(id))
	if err != nil {
		return nil, "", err
	}
	var out struct {
		URL              string `json:"url"`
		LoginEnvironment string `json:"loginEnvironment"`
	}
	if err := decodeAnswer(resp, &out, "reading the managed API Portal "+id, http.StatusOK); err != nil {
		return nil, "", err
	}
	parsed, err := url.Parse(out.URL)
	if err != nil {
		return nil, "", fmt.Errorf("the managed API Portal %s reports the URL %q, which does not parse: %w", id, out.URL, err)
	}
	if err := requireSecureURL(parsed); err != nil {
		return nil, "", fmt.Errorf("the managed API Portal %s: %w", id, err)
	}
	return parsed, out.LoginEnvironment, nil
}
