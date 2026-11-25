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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

// fakeIdentityPlatform is one organisation's Identity Platform API. Its control plane holds the
// portal users and the administrator role's assignments; its Data Plane holds what was deployed.
// A deploy behaves as the Identity Platform's does: it refuses a change that is not pending, and a
// user's removal while the Data Plane's role still names the user unless the role goes along; it
// adds users before it removes them, so a second user with the same username fails; and it answers
// 200 even when an item fails.
type fakeIdentityPlatform struct {
	t      *testing.T
	server *httptest.Server

	mu         sync.Mutex
	next       int
	cpUsers    map[string]string
	dpUsers    map[string]string
	cpAssigned map[string]bool
	dpAssigned map[string]bool
	dpHasRole  bool
	applies    [][]string
	// busy is how many deploys are refused as held up by another before one is taken.
	busy int
	// failItem makes the next deploy report one failed item.
	failItem bool
}

const (
	fakeAdminRoleID   = "role-dp-admin"
	fakeUsersOU       = "ou-default"
	fakeLoginEnv      = "development"
	fakeIdentityToken = "iam-token"
)

func newFakeIdentityPlatform(t *testing.T) *fakeIdentityPlatform {
	t.Helper()
	f := &fakeIdentityPlatform{t: t, cpUsers: map[string]string{}, dpUsers: map[string]string{},
		cpAssigned: map[string]bool{}, dpAssigned: map[string]bool{}, dpHasRole: true}
	f.server = httptest.NewTLSServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeIdentityPlatform) platform() identityPlatform {
	pool := x509.NewCertPool()
	pool.AddCert(f.server.Certificate())
	client := httpx.NewClient(httpx.Options{Timeout: 5 * time.Second, TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}})
	return identityPlatform{client: client, base: f.server.URL, token: fakeIdentityToken, pollInterval: retry.BaseInterval}
}

// holdAdmin makes the control plane, and when deployed the Data Plane, hold an administrator.
func (f *fakeIdentityPlatform) holdAdmin(id string, deployed bool) {
	f.cpUsers[id], f.cpAssigned[id] = portalAdminUsername, true
	if deployed {
		f.dpUsers[id], f.dpAssigned[id] = portalAdminUsername, true
	}
}

func (f *fakeIdentityPlatform) diff() []configChange {
	var changes []configChange
	for id, name := range f.cpUsers {
		change := "added"
		if _, ok := f.dpUsers[id]; ok {
			change = "unchanged"
		}
		changes = append(changes, configChange{Change: change, Key: "user/" + id, Name: name})
	}
	for id, name := range f.dpUsers {
		if _, ok := f.cpUsers[id]; !ok {
			changes = append(changes, configChange{Change: "deleted", Key: "user/" + id, Name: name})
		}
	}
	role := "unchanged"
	switch {
	case !f.dpHasRole:
		role = "added"
	case !maps.Equal(f.cpAssigned, f.dpAssigned):
		role = "updated"
	}
	return append(changes, configChange{Change: role, Key: "role/" + fakeAdminRoleID, Name: portalAdminRole})
}

func (f *fakeIdentityPlatform) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	require.Equal(f.t, "Bearer "+fakeIdentityToken, r.Header.Get("Authorization"))
	var body map[string]any
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/organization-units/tree/default":
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"id": fakeUsersOU, "handle": "default"})
	case r.Method == http.MethodGet && r.URL.Path == "/organization-units/tree/default/roles":
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"roles": []map[string]any{
			{"id": "role-dp-subscriber", "name": "dp_subscriber"}, {"id": fakeAdminRoleID, "name": portalAdminRole}}})
	case r.Method == http.MethodGet && r.URL.Path == "/users":
		var users []map[string]any
		for id, name := range f.cpUsers {
			if r.URL.Query().Get("filter") == fmt.Sprintf("username eq %q", name) {
				users = append(users, map[string]any{"id": id})
			}
		}
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"users": users, "totalResults": len(users)})
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/users/"):
		id := strings.TrimPrefix(r.URL.Path, "/users/")
		delete(f.cpUsers, id)
		delete(f.cpAssigned, id)
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPost && r.URL.Path == "/users":
		attributes, _ := body["attributes"].(map[string]any)
		require.Equal(f.t, fakeUsersOU, body["ouId"])
		require.Equal(f.t, portalUserType, body["type"])
		require.NotEmpty(f.t, attributes["password"])
		require.NotEmpty(f.t, attributes["email"])
		f.next++
		id := fmt.Sprintf("user-%d", f.next)
		f.cpUsers[id] = attributes["username"].(string)
		cloudbase.WriteJSON(w, http.StatusCreated, map[string]any{"id": id})
	case r.Method == http.MethodPost && r.URL.Path == "/roles/"+fakeAdminRoleID+"/assignments/add":
		assignments, _ := body["assignments"].([]any)
		require.Len(f.t, assignments, 1)
		assignment := assignments[0].(map[string]any)
		require.Equal(f.t, "user", assignment["type"])
		f.cpAssigned[assignment["id"].(string)] = true
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPost && r.URL.Path == "/configuration-versions":
		cloudbase.WriteJSON(w, http.StatusCreated, map[string]any{"version": "v1"})
	case r.Method == http.MethodGet && r.URL.Path == "/environments/"+fakeLoginEnv+"/diff":
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"changes": f.diff(), "summary": map[string]any{}, "toVersion": "v1"})
	case r.Method == http.MethodPost && r.URL.Path == "/environments/"+fakeLoginEnv+"/apply":
		f.apply(w, body)
	default:
		f.t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
		w.WriteHeader(http.StatusNotImplemented)
	}
}

func (f *fakeIdentityPlatform) apply(w http.ResponseWriter, body map[string]any) {
	if f.busy > 0 {
		f.busy--
		cloudbase.WriteJSON(w, http.StatusConflict, map[string]any{"code": gatewayApplyInProgress})
		return
	}
	pending := map[string]string{}
	for _, change := range f.diff() {
		if change.pending() {
			pending[change.Key] = change.Change
		}
	}
	var selection []string
	for _, key := range body["selection"].([]any) {
		selection = append(selection, key.(string))
		if pending[key.(string)] == "" {
			cloudbase.WriteJSON(w, http.StatusBadRequest, map[string]any{"code": "GTW-1025", "message": key.(string) + " holds no change"})
			return
		}
	}
	roleKey := "role/" + fakeAdminRoleID
	for _, key := range selection {
		id := strings.TrimPrefix(key, "user/")
		if pending[key] == "deleted" && f.dpAssigned[id] && !slices.Contains(selection, roleKey) {
			cloudbase.WriteJSON(w, http.StatusConflict, map[string]any{"code": "GTW-1033", "message": key + " is needed by " + roleKey})
			return
		}
	}
	f.applies = append(f.applies, selection)
	var results []map[string]any
	failed := 0
	fail := func(kind, id, operation, code string) {
		failed++
		results = append(results, map[string]any{"resourceType": kind, "resourceId": id, "operation": operation, "status": "failed", "code": code})
	}
	for _, key := range selection {
		if id, ok := strings.CutPrefix(key, "user/"); ok && pending[key] == "added" {
			if slices.Contains(slices.Collect(maps.Values(f.dpUsers)), f.cpUsers[id]) {
				fail("user", id, "create", "USR-1014")
				continue
			}
			f.dpUsers[id] = f.cpUsers[id]
		}
	}
	if slices.Contains(selection, roleKey) {
		f.dpHasRole = true
		f.dpAssigned = map[string]bool{}
		for id := range f.cpAssigned {
			if _, ok := f.dpUsers[id]; ok {
				f.dpAssigned[id] = true
			} else if !slices.Contains(selection, "user/"+id) || pending["user/"+id] != "deleted" {
				fail("role", fakeAdminRoleID, "update", "ROL-1007")
			}
		}
	}
	for _, key := range selection {
		if pending[key] == "deleted" {
			delete(f.dpUsers, strings.TrimPrefix(key, "user/"))
		}
	}
	if f.failItem {
		f.failItem = false
		fail("user", "x", "update", "USR-1999")
	}
	cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"recorded": true,
		"import": map[string]any{"summary": map[string]any{"failed": failed}, "results": results}})
}

func TestEnsurePortalAdminFirstRunCreatesAndDeploysTheAdministrator(t *testing.T) {
	fake := newFakeIdentityPlatform(t)
	fake.dpHasRole = false

	admin, err := ensurePortalAdmin(context.Background(), fake.platform(), fakeLoginEnv, slog.Default())
	require.NoError(t, err)

	require.Equal(t, portalAdminUsername, admin.username)
	require.NotEmpty(t, admin.password)
	require.Equal(t, [][]string{{"user/user-1", "role/" + fakeAdminRoleID}}, fake.applies, "one deploy of the user and the role")
	require.Equal(t, map[string]string{"user-1": portalAdminUsername}, fake.dpUsers)
	require.Equal(t, map[string]bool{"user-1": true}, fake.dpAssigned, "the Data Plane gives the administrator the role")
}

func TestEnsurePortalAdminRemovesThePreviousAdministratorBeforeDeployingTheNewOne(t *testing.T) {
	fake := newFakeIdentityPlatform(t)
	fake.holdAdmin("user-old", true)
	fake.dpUsers["user-other"] = "someone-else"

	first, err := ensurePortalAdmin(context.Background(), fake.platform(), fakeLoginEnv, slog.Default())
	require.NoError(t, err)
	require.Equal(t, [][]string{
		{"user/user-old", "role/" + fakeAdminRoleID},
		{"user/user-1", "role/" + fakeAdminRoleID},
	}, fake.applies, "the removal goes with the role, before the new administrator")
	require.Equal(t, map[string]bool{"user-1": true}, fake.dpAssigned)
	require.Equal(t, map[string]string{"user-1": portalAdminUsername, "user-other": "someone-else"}, fake.dpUsers,
		"only the administrator is replaced")

	second, err := ensurePortalAdmin(context.Background(), fake.platform(), fakeLoginEnv, slog.Default())
	require.NoError(t, err)
	require.NotEqual(t, first.password, second.password, "every run makes a new password")
	require.Equal(t, map[string]string{"user-2": portalAdminUsername, "user-other": "someone-else"}, fake.dpUsers)
	require.Equal(t, map[string]bool{"user-2": true}, fake.dpAssigned)
}

func TestEnsurePortalAdminDeploysNoRemovalOfAnAdministratorThatNeverReachedTheDataPlane(t *testing.T) {
	fake := newFakeIdentityPlatform(t)
	fake.holdAdmin("user-old", false)

	_, err := ensurePortalAdmin(context.Background(), fake.platform(), fakeLoginEnv, slog.Default())
	require.NoError(t, err)
	require.Equal(t, [][]string{{"user/user-1", "role/" + fakeAdminRoleID}}, fake.applies)
	require.NotContains(t, fake.cpUsers, "user-old")
}

func TestEnsurePortalAdminRemovesAnAdministratorAnEarlierRunLeftOnTheDataPlane(t *testing.T) {
	fake := newFakeIdentityPlatform(t)
	fake.holdAdmin("user-old", true)
	delete(fake.cpUsers, "user-old")
	delete(fake.cpAssigned, "user-old")

	_, err := ensurePortalAdmin(context.Background(), fake.platform(), fakeLoginEnv, slog.Default())
	require.NoError(t, err)
	require.Equal(t, [][]string{
		{"user/user-old", "role/" + fakeAdminRoleID},
		{"user/user-1", "role/" + fakeAdminRoleID},
	}, fake.applies, "the leftover's removal goes first, or the new username would be refused")
	require.Equal(t, map[string]string{"user-1": portalAdminUsername}, fake.dpUsers)
}

func TestEnsurePortalAdminWaitsForADeployInProgress(t *testing.T) {
	fake := newFakeIdentityPlatform(t)
	fake.busy = 2

	_, err := ensurePortalAdmin(context.Background(), fake.platform(), fakeLoginEnv, slog.Default())
	require.NoError(t, err)
	require.Len(t, fake.applies, 1)
	require.Contains(t, fake.dpUsers, "user-1")
}

func TestEnsurePortalAdminFailsWhenTheDeployReportsAFailedItem(t *testing.T) {
	fake := newFakeIdentityPlatform(t)
	fake.failItem = true

	_, err := ensurePortalAdmin(context.Background(), fake.platform(), fakeLoginEnv, slog.Default())
	require.ErrorContains(t, err, "1 item(s) failed")
	require.ErrorContains(t, err, "USR-1999")
}

func TestEnsurePortalAdminNeedsTheEnvironmentPortalUsersSignInOn(t *testing.T) {
	fake := newFakeIdentityPlatform(t)
	_, err := ensurePortalAdmin(context.Background(), fake.platform(), " ", slog.Default())
	require.ErrorContains(t, err, "names no environment")
	require.Empty(t, fake.applies)
}

func TestIdentityPlatformFromEnvNeedsTheTokenAndAnHTTPSURL(t *testing.T) {
	t.Setenv(cloudbase.CloudLocalDataPlaneEnv, "")
	client := httpx.NewClient(httpx.Options{})

	t.Setenv(cloudIAMAccessTokenEnv, "")
	t.Setenv(cloudIAMAPIURLEnv, "https://api.example/iam/v1")
	_, err := identityPlatformFromEnv(client)
	require.ErrorContains(t, err, cloudIAMAccessTokenEnv)

	t.Setenv(cloudIAMAccessTokenEnv, "token")
	t.Setenv(cloudIAMAPIURLEnv, "")
	_, err = identityPlatformFromEnv(client)
	require.ErrorContains(t, err, cloudIAMAPIURLEnv)

	t.Setenv(cloudIAMAPIURLEnv, "http://api.example/iam/v1")
	_, err = identityPlatformFromEnv(client)
	require.ErrorContains(t, err, "does not use https")

	t.Setenv(cloudIAMAPIURLEnv, " https://api.example/iam/v1/ ")
	platform, err := identityPlatformFromEnv(client)
	require.NoError(t, err)
	require.Equal(t, "https://api.example/iam/v1", platform.base)
}

// fakePortalAdminAPI is a portal's REST API as its administrator sees it: plans and API listings,
// deleted only with the session's CSRF token. A plan or listing in use is refused.
type fakePortalAdminAPI struct {
	t      *testing.T
	server *httptest.Server

	mu       sync.Mutex
	plans    []string
	listings []string
	inUse    map[string]bool
	// created is when a plan was made; a plan not named here is two hours old.
	created map[string]time.Time
	// sessionEnded makes every request answer as a portal that no longer takes the session, and
	// endSessionAfter ends it after that many removals.
	sessionEnded    bool
	endSessionAfter int
	lists           int
}

func newFakePortalAdminAPI(t *testing.T, plans, listings []string) *fakePortalAdminAPI {
	t.Helper()
	f := &fakePortalAdminAPI{t: t, plans: plans, listings: listings, inUse: map[string]bool{}, created: map[string]time.Time{}}
	f.server = httptest.NewTLSServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakePortalAdminAPI) browser(t *testing.T) *portalBrowser {
	t.Helper()
	pool := x509.NewCertPool()
	pool.AddCert(f.server.Certificate())
	client := httpx.NewClient(httpx.Options{Timeout: 5 * time.Second, TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}})
	portal, err := url.Parse(f.server.URL)
	require.NoError(t, err)
	browser, err := newPortalBrowser(client, portal)
	require.NoError(t, err)
	browser.jar.SetCookies(portal, []*http.Cookie{{Name: portalCSRFCookie, Value: "csrf-1", Path: "/"}})
	return browser
}

func (f *fakePortalAdminAPI) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sessionEnded {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	collection := map[string]*[]string{portalAPIBasePath + "/subscription-plans": &f.plans, portalAPIBasePath + "/apis": &f.listings}
	switch {
	case r.Method == http.MethodGet && collection[r.URL.Path] != nil:
		f.lists++
		items := *collection[r.URL.Path]
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		require.LessOrEqual(f.t, limit, 50)
		end := min(offset+limit, len(items))
		var page []map[string]any
		for _, id := range items[min(offset, end):end] {
			created, ok := f.created[id]
			if !ok {
				created = time.Now().Add(-2 * time.Hour)
			}
			page = append(page, map[string]any{"id": id, "createdAt": created.UTC().Format(time.RFC3339Nano)})
		}
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"list": page, "count": len(page),
			"pagination": map[string]any{"total": len(items), "limit": limit, "offset": offset}})
	case r.Method == http.MethodDelete:
		if r.Header.Get(portalCSRFHeader) != "csrf-1" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		dir, id := r.URL.Path[:strings.LastIndex(r.URL.Path, "/")], r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		items := collection[dir]
		switch {
		case items == nil:
			f.t.Errorf("unexpected delete %s", r.URL.Path)
			w.WriteHeader(http.StatusNotImplemented)
		case f.inUse[id]:
			w.WriteHeader(http.StatusConflict)
		case !slices.Contains(*items, id):
			w.WriteHeader(http.StatusNotFound)
		default:
			*items = slices.DeleteFunc(*items, func(item string) bool { return item == id })
			w.WriteHeader(http.StatusNoContent)
			if f.endSessionAfter > 0 {
				f.endSessionAfter--
				f.sessionEnded = f.endSessionAfter == 0
			}
		}
	default:
		f.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotImplemented)
	}
}

func TestSweepPortalRemovesTheLeftoversOfEveryRunAndNothingElse(t *testing.T) {
	plans := []string{"AsyncUnlimited", "Bronze", "Gold"}
	for run, suffix := range []string{"abcde234", "zzzzz222"} {
		for n := range 30 + run {
			plans = append(plans, fmt.Sprintf("syn-plan-%s-%d", suffix, n))
		}
	}
	plans = append(plans, "syn-plan-qqqqq333-0", "syn-plan-ownrun12-0")
	listings := []string{"weather-api", "syn-api-abcde234-1", "syn-api-zzzzz222-2", "syn-graphql-zzzzz222-3"}
	fake := newFakePortalAdminAPI(t, plans, listings)
	fake.inUse["syn-plan-zzzzz222-7"] = true
	fake.created["syn-plan-qqqqq333-0"] = time.Now().Add(-5 * time.Minute)
	fake.created["syn-plan-ownrun12-0"] = time.Now().Add(-time.Minute)
	held := func(_ context.Context, id string) (bool, error) {
		return id == "syn-api-abcde234-1", nil
	}
	scope := sweepScope{ownedByRun: func(id string) bool { return strings.Contains(id, "ownrun12") }, now: time.Now()}

	sweep, err := sweepPortal(context.Background(), fake.browser(t), held, scope)
	require.NoError(t, err)

	require.Equal(t, []string{"AsyncUnlimited", "Bronze", "Gold", "syn-plan-zzzzz222-7", "syn-plan-qqqqq333-0"}, fake.plans,
		"every run's old synthetic plans and this run's go, across pages; the portal's own plans, one in use and another run's recent one stay")
	require.Equal(t, []string{"weather-api", "syn-api-abcde234-1"}, fake.listings,
		"only synthetic listings the platform no longer holds go")
	require.Len(t, sweep.removed, 60+1+2)
	require.Contains(t, sweep.removed, "api/syn-graphql-zzzzz222-3")
	require.Contains(t, sweep.removed, "subscription-plan/syn-plan-ownrun12-0", "this run's plan goes however recent")
	require.Equal(t, []string{"subscription-plan/syn-plan-zzzzz222-7"}, sweep.inUse)
	require.Equal(t, []string{"subscription-plan/syn-plan-qqqqq333-0"}, sweep.recent, "another run may still publish with it")
	require.Empty(t, sweep.failed)
	require.Equal(t, "removed 63, kept 1 in use and 1 recent, 0 failed", sweep.String())
}

func TestSweepPortalRecordsWhatThePlatformCannotTellAndCarriesOn(t *testing.T) {
	fake := newFakePortalAdminAPI(t, []string{"syn-plan-abcde234-1"}, []string{"syn-api-abcde234-1"})
	held := func(context.Context, string) (bool, error) { return false, errors.New("platform unreachable") }

	sweep, err := sweepPortal(context.Background(), fake.browser(t), held, sweepScope{now: time.Now()})
	require.NoError(t, err)
	require.Equal(t, []string{"api/syn-api-abcde234-1: platform unreachable"}, sweep.failed)
	require.Equal(t, []string{"syn-api-abcde234-1"}, fake.listings, "a listing whose API cannot be checked stays")
	require.Equal(t, []string{"subscription-plan/syn-plan-abcde234-1"}, sweep.removed)
}

func TestSweepPortalStopsWhenThePortalNoLongerTakesTheSession(t *testing.T) {
	fake := newFakePortalAdminAPI(t, []string{"syn-plan-abcde234-1"}, nil)
	fake.sessionEnded = true

	_, err := sweepPortal(context.Background(), fake.browser(t), func(context.Context, string) (bool, error) { return false, nil }, sweepScope{now: time.Now()})
	require.ErrorIs(t, err, errPortalSessionEnded)
}

func TestSweepPortalKeepsWhatItRemovedWhenTheSessionEndsAndTheAdministratorCannotSignInAgain(t *testing.T) {
	fake := newFakePortalAdminAPI(t, []string{"syn-plan-abcde234-1", "syn-plan-abcde234-2"}, nil)
	fake.endSessionAfter = 1
	browser := fake.browser(t)
	admin := &Administrator{client: httpx.NewClient(httpx.Options{}), browser: browser, portal: browser.portal,
		admin: portalAdmin{username: portalAdminUsername, password: "Pw9!"}}

	removed, err := admin.Sweep(context.Background(), nil)
	require.ErrorContains(t, err, "signing the portal administrator in again")
	require.Equal(t, []string{"subscription-plan/syn-plan-abcde234-1"}, removed, "what the first attempt removed is reported")
	require.Len(t, admin.sweeps, 1)
	require.Equal(t, removed, admin.sweeps[0].removed)
}

func TestPortalSweepMergeAddsAnotherAttemptsFindings(t *testing.T) {
	first := portalSweep{removed: []string{"a"}, inUse: []string{"b"}}
	merged := first.merge(portalSweep{removed: []string{"c"}, recent: []string{"d"}, failed: []string{"e"}})
	require.Equal(t, portalSweep{removed: []string{"a", "c"}, inUse: []string{"b"}, recent: []string{"d"}, failed: []string{"e"}}, merged)
}

func TestSignInAsSignsAPortalUserInAndReportsARefusal(t *testing.T) {
	site := newFakePortalSite(t)
	site.users[portalAdminUsername] = "Right-Pw9!"
	portal, err := url.Parse(site.portal.URL)
	require.NoError(t, err)

	browser, err := signInAs(context.Background(), site.client(), portal, portalAdminUsername, "Right-Pw9!")
	require.NoError(t, err)
	api, err := portal.Parse(portalAPIBasePath)
	require.NoError(t, err)
	token := browser.cookie(api, portalCSRFCookie)
	require.NotEmpty(t, token)
	require.NotEqual(t, "csrf-anonymous", token, "the browser holds the signed-in session's CSRF token")

	_, err = signInAs(context.Background(), site.client(), portal, portalAdminUsername, "Wrong-Pw9!")
	require.ErrorContains(t, err, "refused the sign-in of "+portalAdminUsername)
}
