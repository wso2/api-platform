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
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"log/slog"
	"maps"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

// fastPortalPolls lowers the portal poll interval to the retry floor for one test.
func fastPortalPolls(t *testing.T) {
	t.Helper()
	previous := cloudbase.PortalPollInterval
	cloudbase.PortalPollInterval = retry.BaseInterval
	t.Cleanup(func() { cloudbase.PortalPollInterval = previous })
}

func registeredPortals(registry *cleanup.Registry) []string {
	var ids []string
	for _, resource := range registry.Pending() {
		if resource.Kind == KindAPIPortal {
			ids = append(ids, resource.ID)
		}
	}
	return ids
}

func TestRequestRunnerPortalRegistersThePortalBeforeTheRequest(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var sent map[string]any
	var registeredAtRequest []string
	var registry *cleanup.Registry
	answer := http.StatusAccepted
	steps, ctx, _, reg := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, cloudbase.ManagedPortalsCollection, r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		registeredAtRequest = registeredPortals(registry)
		cloudbase.WriteJSON(w, answer, map[string]any{"id": sent["handle"], "name": sent["name"], "status": cloudbase.PortalStatusPending})
	})
	registry = reg

	require.ErrorContains(t, steps.assertPortalAccepted(ctx), "requested no managed API Portal")
	require.NoError(t, steps.requestRunnerPortal(ctx))
	handle, err := runnerPortal(ctx)
	require.NoError(t, err)
	require.Equal(t, handle, sent["handle"])
	require.Equal(t, "Synthetic "+handle, sent["name"])
	require.Equal(t, "development", sent["loginEnvironment"], "consumers sign in on the development environment")
	require.LessOrEqual(t, len(handle), portalHandleMaxLength)
	require.Equal(t, []string{handle}, registeredAtRequest, "the portal is registered before the request that creates it")
	require.NoError(t, steps.assertPortalAccepted(ctx))

	answer = http.StatusCreated
	require.NoError(t, steps.requestRunnerPortal(ctx))
	require.NoError(t, steps.assertPortalAccepted(ctx), "a portal the platform built before answering is accepted too")
}

func TestRequestRunnerPortalDropsTheRegistrationOfARefusedPortal(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		cloudbase.WriteJSON(w, http.StatusPaymentRequired, map[string]string{"code": "payment_required"})
	})
	require.NoError(t, steps.requestRunnerPortal(ctx))
	require.Empty(t, registeredPortals(registry))
	require.ErrorContains(t, steps.assertPortalAccepted(ctx), "not accepted")
}

func TestAssertPortalAcceptedChecksTheHandleAndThePendingStatus(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	body := map[string]any{}
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		cloudbase.WriteJSON(w, http.StatusAccepted, body)
	})
	local.Set(keyAPIPortalID, "syn-portal-1")
	for _, tc := range []struct {
		body map[string]any
		want string
	}{
		{map[string]any{"status": cloudbase.PortalStatusPending}, "reading the id"},
		{map[string]any{"id": "another-portal", "status": cloudbase.PortalStatusPending}, "want the requested handle"},
		{map[string]any{"id": "syn-portal-1", "status": cloudbase.PortalStatusActive}, "should read as pending"},
		{map[string]any{"id": "syn-portal-1"}, "should read as pending"},
	} {
		body = tc.body
		_, err := steps.DoAuthenticated(ctx, http.MethodGet, "/answer", nil)
		require.NoError(t, err)
		require.ErrorContains(t, steps.assertPortalAccepted(ctx), tc.want)
	}
}

func TestAwaitPortalActiveWaitsForTheBuildAndRecordsTheURL(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	fastPortalPolls(t)
	reads := 0
	activeAt := 3
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, portalPath("syn-portal-1"), r.URL.Path)
		reads++
		if reads == 1 {
			cloudbase.WriteJSON(w, http.StatusNotFound, map[string]string{"code": "not_found"})
			return
		}
		if reads < activeAt {
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"id": "syn-portal-1", "status": cloudbase.PortalStatusPending})
			return
		}
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"id": "syn-portal-1", "status": cloudbase.PortalStatusActive,
			"url": "https://abc123-portal-p01.example.test", "loginEnvironment": "development", "selfSignup": true})
	})
	require.ErrorContains(t, steps.awaitPortalActive(ctx), "requested no managed API Portal")

	local.Set(keyAPIPortalID, "syn-portal-1")
	require.NoError(t, steps.awaitPortalActive(ctx))
	require.Equal(t, activeAt, reads, "a portal that briefly reads as not found while it is built is read again")
	portalURL, err := cloudbase.RequiredValue(ctx, keyAPIPortalURL)
	require.NoError(t, err)
	require.Equal(t, "https://abc123-portal-p01.example.test", portalURL)
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	status, err := cloudbase.StringField(published, "status")
	require.NoError(t, err)
	require.Equal(t, cloudbase.PortalStatusActive, status, "the active read is published for the assertions that follow")
	require.NoError(t, steps.assertPortalURL(ctx))

	reads = activeAt
	require.NoError(t, steps.awaitPortalActive(ctx), "an active portal passes on the first read")
	require.Equal(t, activeAt+1, reads)
}

func TestAwaitPortalActiveStopsAtAFailedPortalOrARefusedRead(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	fastPortalPolls(t)
	status, code := cloudbase.PortalStatusFailed, http.StatusOK
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		cloudbase.WriteJSON(w, code, map[string]any{"id": "syn-portal-1", "status": status})
	})
	local.Set(keyAPIPortalID, "syn-portal-1")
	started := time.Now()
	require.ErrorIs(t, steps.awaitPortalActive(ctx), errPortalFailed)
	code = http.StatusForbidden
	require.ErrorContains(t, steps.awaitPortalActive(ctx), "403")
	require.Less(t, time.Since(started), 10*time.Second, "neither a failed portal nor a refused read is waited for")
}

func TestRequireSecureURL(t *testing.T) {
	t.Setenv(cloudbase.CloudLocalDataPlaneEnv, "")
	require.Error(t, requireSecureURL(nil))
	for _, accepted := range []string{"https://abc-portal-p01.example.test", "https://127.0.0.1:8443/api-portal"} {
		parsed, err := url.Parse(accepted)
		require.NoError(t, err)
		require.NoError(t, requireSecureURL(parsed), accepted)
	}
	for _, refused := range []string{"http://abc-portal-p01.example.test", "https://user:pass@example.test", "/api-portal/login", "http://portal.localhost"} {
		parsed, err := url.Parse(refused)
		require.NoError(t, err)
		err = requireSecureURL(parsed)
		require.Error(t, err, refused)
		require.NotContains(t, err.Error(), "pass@", "an error never repeats a credential")
	}
	t.Setenv(cloudbase.CloudLocalDataPlaneEnv, "true")
	parsed, err := url.Parse("http://portal.localhost:8080")
	require.NoError(t, err)
	require.NoError(t, requireSecureURL(parsed), "a cloud on the developer's machine serves plain http on .localhost")
}

func TestAssertPortalURLRequiresAnHTTPSURL(t *testing.T) {
	t.Setenv(cloudbase.CloudLocalDataPlaneEnv, "")
	_, ctx, local, _ := cloudSteps(t, func(http.ResponseWriter, *http.Request) {})
	steps := &Steps{Steps: &cloudbase.Steps{}}
	require.ErrorContains(t, steps.assertPortalURL(ctx), "reported no URL")
	local.Set(keyAPIPortalURL, "http://abc-portal-p01.example.test")
	require.ErrorContains(t, steps.assertPortalURL(ctx), "https")
	local.Set(keyAPIPortalURL, "https://abc-portal-p01.example.test")
	require.NoError(t, steps.assertPortalURL(ctx))
}

func TestAssertPortalListedRequiresTheActivePortal(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	listed := []map[string]any{{"id": "ops-portal", "status": cloudbase.PortalStatusActive}}
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, cloudbase.ManagedPortalsCollection, r.URL.Path)
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"count": len(listed), "list": listed})
	})
	local.Set(keyAPIPortalID, "syn-portal-1")
	require.ErrorContains(t, steps.assertPortalListed(ctx), "does not include")
	listed = append(listed, map[string]any{"id": "syn-portal-1", "status": cloudbase.PortalStatusPending})
	require.ErrorContains(t, steps.assertPortalListed(ctx), "status pending")
	listed[1]["status"] = cloudbase.PortalStatusActive
	require.NoError(t, steps.assertPortalListed(ctx))
}

func TestRequestDuplicatePortalRepeatsTheHandleWithAnotherName(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var sent map[string]any
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		cloudbase.WriteJSON(w, http.StatusConflict, map[string]string{"code": "conflict"})
	})
	require.ErrorContains(t, steps.requestDuplicatePortal(ctx), "requested no managed API Portal")
	local.Set(keyAPIPortalID, "syn-portal-1")
	require.NoError(t, steps.requestDuplicatePortal(ctx))
	require.Equal(t, "syn-portal-1", sent["handle"])
	require.NotEqual(t, "Synthetic syn-portal-1", sent["name"])
	require.Empty(t, registeredPortals(registry), "the handle is the runner's and already registered")
}

func TestRequestDuplicatePortalLeavesAnyAnswerToTheStatusAssertion(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	code, body := http.StatusPaymentRequired, map[string]string{"code": "quota_exceeded", "message": "Hard limit reached: 1 of 1 used"}
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		cloudbase.WriteJSON(w, code, body)
	})
	local.Set(keyAPIPortalID, "syn-portal-1")
	require.NoError(t, steps.requestDuplicatePortal(ctx), "a 402 is not skipped; the status assertion fails it")
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusPaymentRequired, published.StatusCode)

	code, body = http.StatusConflict, map[string]string{"code": "conflict"}
	require.NoError(t, steps.requestDuplicatePortal(ctx))
	published, err = httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, published.StatusCode)
}

func TestAssertPortalStillActive(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	status, code := cloudbase.PortalStatusActive, http.StatusOK
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		cloudbase.WriteJSON(w, code, map[string]any{"id": "syn-portal-1", "status": status})
	})
	local.Set(keyAPIPortalID, "syn-portal-1")
	require.NoError(t, steps.assertPortalStillActive(ctx))
	status = cloudbase.PortalStatusPending
	require.ErrorContains(t, steps.assertPortalStillActive(ctx), "want active")
	code = http.StatusNotFound
	require.Error(t, steps.assertPortalStillActive(ctx))
}

func TestRequestDefectivePortalBreaksOneRuleAndRegistersTheHandleFirst(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var sent map[string]any
	var registeredAtRequest []string
	var registry *cleanup.Registry
	steps, ctx, _, reg := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		registeredAtRequest = registeredPortals(registry)
		cloudbase.WriteJSON(w, http.StatusBadRequest, map[string]string{"code": "bad_request"})
	})
	registry = reg
	require.ErrorContains(t, steps.requestDefectivePortal(ctx, "with a typo"), "unknown managed API Portal request defect")

	checks := map[string]func(handle, name string){
		"an uppercase handle": func(handle, _ string) {
			require.Equal(t, strings.ToUpper(handle), handle)
			require.NotEqual(t, strings.ToLower(handle), handle)
		},
		"a handle with an underscore":        func(handle, _ string) { require.Contains(t, handle, "_") },
		"a handle shorter than 3 characters": func(handle, _ string) { require.Len(t, handle, 2) },
		"a handle longer than 40 characters": func(handle, _ string) { require.Greater(t, len(handle), portalHandleMaxLength) },
		"no name": func(handle, name string) {
			require.Empty(t, name)
			require.Regexp(t, `^[a-z0-9-]{3,40}$`, handle, "only the name is wrong")
		},
		"a name longer than 255 characters": func(handle, name string) {
			require.Len(t, name, portalNameMaxLength+1)
			require.Regexp(t, `^[a-z0-9-]{3,40}$`, handle, "only the name is wrong")
		},
		"a description longer than 1023 characters": func(handle, _ string) {
			require.Regexp(t, `^[a-z0-9-]{3,40}$`, handle, "only the description is wrong")
			require.Len(t, sent["description"], portalDescriptionMaxLength+1)
		},
	}
	require.Len(t, checks, len(portalRequestDefects), "every defect is checked")
	for defect, check := range checks {
		sent, registeredAtRequest = nil, nil
		require.NoError(t, steps.requestDefectivePortal(ctx, defect), defect)
		handle, _ := sent["handle"].(string)
		name, _ := sent["name"].(string)
		check(handle, name)
		refused, err := cloudbase.RequiredValue(ctx, keyRefusedAPIPortalID)
		require.NoError(t, err)
		require.Equal(t, refused, handle, defect)
		require.Equal(t, []string{handle}, registeredAtRequest, defect+": registered before the request")
		require.Empty(t, registeredPortals(registry), defect+": a refused portal is no longer registered")
	}
}

func TestRequestDefectivePortalKeepsTheRegistrationOfAnAcceptedPortal(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		cloudbase.WriteJSON(w, http.StatusAccepted, map[string]any{"status": cloudbase.PortalStatusPending})
	})
	require.NoError(t, steps.requestDefectivePortal(ctx, "no name"))
	refused, err := cloudbase.RequiredValue(ctx, keyRefusedAPIPortalID)
	require.NoError(t, err)
	require.Equal(t, []string{refused}, registeredPortals(registry), "a portal accepted against expectation is removed by cleanup")
}

func TestRequestPortalInAbsentEnvironmentNamesAnEnvironmentNobodyCreated(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var sent map[string]any
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		cloudbase.WriteJSON(w, http.StatusBadRequest, map[string]string{"code": "bad_request"})
	})
	require.NoError(t, steps.requestPortalInAbsentEnvironment(ctx))
	environment, _ := sent["loginEnvironment"].(string)
	require.True(t, strings.HasPrefix(environment, "syn-no-env-"), environment)
	require.Regexp(t, `^[a-z0-9-]{3,40}$`, sent["handle"])
	require.NotEmpty(t, sent["name"])
	_, overwritten := tcontext.Get(ctx, cloudbase.KeyEnvironmentName)
	require.False(t, overwritten, "the environment the other steps name is left alone")
}

func TestAssertRefusedPortalNotListed(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	listed := []map[string]any{{"id": "ops-portal"}}
	code := http.StatusOK
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		cloudbase.WriteJSON(w, code, map[string]any{"list": listed})
	})
	require.ErrorContains(t, steps.assertRefusedPortalNotListed(ctx), "requested no managed API Portal")
	local.Set(keyRefusedAPIPortalID, "SYN-PORTAL-1")
	require.NoError(t, steps.assertRefusedPortalNotListed(ctx))
	listed = append(listed, map[string]any{"id": "syn-portal-1"})
	require.ErrorContains(t, steps.assertRefusedPortalNotListed(ctx), "is listed", "a handle is compared without regard to case")
	listed = listed[:1]
	code = http.StatusInternalServerError
	require.Error(t, steps.assertRefusedPortalNotListed(ctx), "a list that does not answer proves nothing")
}

func TestReadRunnerAndAbsentPortals(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var seen []string
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		cloudbase.WriteJSON(w, http.StatusNotFound, map[string]string{"code": "not_found"})
	})
	require.ErrorContains(t, steps.readRunnerPortal(ctx), "requested no managed API Portal")
	local.Set(keyAPIPortalID, "syn-portal-1")
	require.NoError(t, steps.readRunnerPortal(ctx))
	require.NoError(t, steps.readAbsentPortal(ctx))
	absent, err := cloudbase.RequiredValue(ctx, keyAbsentAPIPortalID)
	require.NoError(t, err)
	require.NotEqual(t, "syn-portal-1", absent)
	require.Equal(t, []string{"GET " + portalPath("syn-portal-1"), "GET " + portalPath(absent)}, seen)
	require.Empty(t, registry.Pending(), "a portal that is never created is never registered")
}

func TestAssertPortalDeletionAccepted(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	code := http.StatusAccepted
	body := map[string]any{"id": "syn-portal-1", "status": portalStatusDeleting}
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		if code == http.StatusNoContent {
			w.WriteHeader(code)
			return
		}
		cloudbase.WriteJSON(w, code, body)
	})
	local.Set(keyAPIPortalID, "syn-portal-1")
	require.NoError(t, steps.deleteRunnerPortal(ctx))
	require.NoError(t, steps.assertPortalDeletionAccepted(ctx))
	body["status"] = cloudbase.PortalStatusActive
	require.NoError(t, steps.deleteRunnerPortal(ctx))
	require.ErrorContains(t, steps.assertPortalDeletionAccepted(ctx), "should read as deleting")
	code = http.StatusNoContent
	require.NoError(t, steps.deleteRunnerPortal(ctx))
	require.NoError(t, steps.assertPortalDeletionAccepted(ctx))
	code = http.StatusConflict
	require.NoError(t, steps.deleteRunnerPortal(ctx))
	require.ErrorContains(t, steps.assertPortalDeletionAccepted(ctx), "not accepted")
}

func TestAwaitPortalGoneWaitsForTheTeardownAndDropsTheRegistration(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	fastPortalPolls(t)
	reads := 0
	listed := []map[string]any{}
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == cloudbase.ManagedPortalsCollection {
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"list": listed})
			return
		}
		reads++
		if reads < 3 {
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"id": "syn-portal-1", "status": portalStatusDeleting})
			return
		}
		cloudbase.WriteJSON(w, http.StatusNotFound, map[string]string{"code": "not_found"})
	})
	local.Set(keyAPIPortalID, "syn-portal-1")
	require.NoError(t, cleanup.RegisterRunner(ctx, portalResource("syn-portal-1")))
	require.NoError(t, steps.awaitPortalGone(ctx))
	require.Equal(t, 3, reads)
	require.Empty(t, registeredPortals(registry), "a deleted portal is not deleted again by cleanup")

	listed = []map[string]any{{"id": "syn-portal-1", "status": portalStatusDeleting}}
	require.ErrorContains(t, steps.awaitPortalGone(ctx), "still listed")
}

func TestDeletePortalRepeatsARefusalUntilTheBuildEnds(t *testing.T) {
	fastPortalPolls(t)
	var answers []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "Bearer console-token", r.Header.Get("Authorization"))
		require.Equal(t, portalPath("syn-portal-1"), r.URL.Path)
		code := answers[0]
		if len(answers) > 1 {
			answers = answers[1:]
		}
		w.WriteHeader(code)
	}))
	t.Cleanup(server.Close)
	client := httpx.NewClient(httpx.Options{Timeout: 5 * time.Second})

	answers = []int{http.StatusConflict, http.StatusConflict, http.StatusAccepted}
	require.NoError(t, deletePortal(context.Background(), client, server.URL, "console-token", "syn-portal-1", time.Minute))
	require.Equal(t, []int{http.StatusAccepted}, answers, "the delete is repeated while the portal is still being built")

	answers = []int{http.StatusNotFound}
	require.NoError(t, deletePortal(context.Background(), client, server.URL, "console-token", "syn-portal-1", time.Minute),
		"an absent portal counts as deleted")

	answers = []int{http.StatusForbidden}
	started := time.Now()
	err := deletePortal(context.Background(), client, server.URL, "console-token", "syn-portal-1", time.Minute)
	require.ErrorContains(t, err, "403")
	require.Less(t, time.Since(started), 10*time.Second, "a refusal that will not change is not repeated")
}

func TestRegisterDeletersDeletesARegisteredPortal(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	fastPortalPolls(t)
	var seen []string
	reads := 0
	topo := cloudbase.CloudAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodDelete {
			cloudbase.WriteJSON(w, http.StatusAccepted, map[string]any{"status": portalStatusDeleting})
			return
		}
		reads++
		if reads == 1 {
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"id": "syn-portal-1", "status": portalStatusDeleting})
			return
		}
		cloudbase.WriteJSON(w, http.StatusNotFound, map[string]string{"code": "not_found"})
	})
	registry := cleanup.NewRegistry(slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, RegisterDeleters(registry, topo, httpx.NewClient(httpx.Options{MaxRetries: 1})))
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	require.NoError(t, cleanup.Install(ctx, registry))
	require.NoError(t, cleanup.RegisterRunner(ctx, portalResource("syn-portal-1")))
	require.NoError(t, registry.SweepRunner(ctx))
	require.Equal(t, []string{"DELETE " + portalPath("syn-portal-1"), "GET " + portalPath("syn-portal-1"), "GET " + portalPath("syn-portal-1")}, seen,
		"cleanup waits until the portal is gone, which frees the quota and leaves the finalizer nothing to find")
	require.Empty(t, registry.Pending())
}

// fakePortalSite is a managed API Portal and its identity provider, each on a TLS server of its
// own, that behave as the sign-in steps expect the real ones to. The identity provider serves
// under a path prefix, as one behind a shared gateway does.
type fakePortalSite struct {
	t      *testing.T
	portal *httptest.Server
	idp    *httptest.Server

	mu       sync.Mutex
	users    map[string]string
	sessions map[string]bool
	// signIns counts the sessions the callback started, so a session is never named twice.
	signIns int
	flows   []map[string]any
	// wrongPasswordStatus is how the identity provider answers a wrong password: 200 with an
	// ERROR flow status, or a client error.
	wrongPasswordStatus int
	// accountCreatedActions, when not nil, makes the registration answer with a prompt that
	// confirms the new account and offers these actions, instead of signing the consumer in.
	accountCreatedActions []string
	// clientID and callbackHost let a test break the authorization request the portal sends.
	clientID     string
	callbackHost string
	// consumer is the portal's REST API for the signed-in consumer.
	consumer fakeConsumerAPI
	// landingStatus, when set, is how the page the sign-in lands on answers.
	landingStatus int
}

const fakeIdentityPrefix = "/t/acme"

func newFakePortalSite(t *testing.T) *fakePortalSite {
	t.Helper()
	site := &fakePortalSite{t: t, users: map[string]string{}, sessions: map[string]bool{}, clientID: "portal-client",
		wrongPasswordStatus: http.StatusOK}
	site.portal = httptest.NewTLSServer(http.HandlerFunc(site.servePortal))
	site.idp = httptest.NewTLSServer(http.HandlerFunc(site.serveIdentity))
	t.Cleanup(site.portal.Close)
	t.Cleanup(site.idp.Close)
	site.callbackHost = site.portal.URL
	return site
}

// client trusts the fake servers' certificate, which the two servers share.
func (f *fakePortalSite) client() *httpx.Client {
	pool := x509.NewCertPool()
	pool.AddCert(f.portal.Certificate())
	return httpx.NewClient(httpx.Options{Timeout: 5 * time.Second, TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}})
}

func (f *fakePortalSite) sessionOf(r *http.Request) string {
	if cookie, err := r.Cookie("portal.sid"); err == nil {
		return cookie.Value
	}
	return ""
}

func (f *fakePortalSite) servePortal(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	const view = "/api-portal/acme/views/default"
	switch r.URL.Path {
	case "/api-portal/login":
		http.Redirect(w, r, view+"/login", http.StatusFound)
	case view + "/login":
		http.SetCookie(w, &http.Cookie{Name: "portal.sid", Value: "anonymous", Path: "/api-portal"})
		query := url.Values{"response_type": {"code"}, "client_id": {f.clientID}, "state": {"st"},
			"redirect_uri": {f.callbackHost + "/api-portal/acme/callback"}}
		http.Redirect(w, r, f.idp.URL+fakeIdentityPrefix+"/oauth2/authorize?"+query.Encode(), http.StatusFound)
	case "/api-portal/acme/callback":
		if f.sessionOf(r) != "anonymous" || r.URL.Query().Get("code") != "the-code" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		session := "signed-in-" + strconv.Itoa(f.signIns)
		f.signIns++
		f.sessions[session] = true
		// Like the portal, the callback sets the replaced session's CSRF cookie; the landing page sets the new one.
		http.SetCookie(w, &http.Cookie{Name: "portal.sid", Value: session, Path: "/api-portal"})
		http.SetCookie(w, &http.Cookie{Name: portalCSRFCookie, Value: "csrf-anonymous", Path: "/api-portal"})
		http.Redirect(w, r, view, http.StatusFound)
	case view:
		if f.landingStatus != 0 {
			w.WriteHeader(f.landingStatus)
			return
		}
		if session := f.sessionOf(r); f.sessions[session] {
			http.SetCookie(w, &http.Cookie{Name: portalCSRFCookie, Value: "csrf-" + session, Path: "/api-portal"})
		}
		_, _ = w.Write([]byte("<html>home</html>"))
	case view + "/applications":
		if !f.sessions[f.sessionOf(r)] {
			http.Redirect(w, r, view+"/login", http.StatusSeeOther)
			return
		}
		_, _ = w.Write([]byte("<html>applications</html>"))
	case view + "/logout":
		delete(f.sessions, f.sessionOf(r))
		http.Redirect(w, r, f.idp.URL+fakeIdentityPrefix+"/oauth2/logout", http.StatusFound)
	default:
		if strings.HasPrefix(r.URL.Path, portalAPIBasePath+"/") {
			f.serveConsumerAPI(w, r)
			return
		}
		f.t.Errorf("unexpected portal request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *fakePortalSite) serveIdentity(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.URL.Path {
	case fakeIdentityPrefix + "/oauth2/authorize":
		http.Redirect(w, r, f.idp.URL+"/gate/signin?executionId=exec-1&authId=auth-1", http.StatusFound)
	case fakeIdentityPrefix + "/flow/execute":
		var body map[string]any
		require.NoError(f.t, json.NewDecoder(r.Body).Decode(&body))
		f.flows = append(f.flows, body)
		require.Equal(f.t, "exec-1", body["executionId"])
		inputs, _ := body["inputs"].(map[string]any)
		switch {
		case body["action"] == nil:
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"executionId": "exec-1", "flowStatus": "INCOMPLETE", "challengeToken": "ch-1"})
		case body["action"] == flowActionSignUp:
			require.Equal(f.t, "ch-1", body["challengeToken"])
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"executionId": "exec-1", "flowStatus": "INCOMPLETE", "challengeToken": "ch-2"})
		case body["action"] == flowActionSubmit && inputs["email"] != nil:
			require.Equal(f.t, "ch-2", body["challengeToken"])
			f.users[inputs["username"].(string)] = inputs["password"].(string)
			if f.accountCreatedActions != nil {
				actions := make([]map[string]string, 0, len(f.accountCreatedActions))
				for _, ref := range f.accountCreatedActions {
					actions = append(actions, map[string]string{"ref": ref, "nextNode": "authorization"})
				}
				cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"executionId": "exec-1", "flowStatus": "INCOMPLETE", "type": "VIEW",
					"challengeToken": "ch-3", "data": map[string]any{"actions": actions}})
				return
			}
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"executionId": "exec-1", "flowStatus": flowStatusComplete, "assertion": "the-assertion"})
		case body["action"] == flowActionContinue:
			require.Equal(f.t, "ch-3", body["challengeToken"])
			require.Contains(f.t, f.accountCreatedActions, flowActionContinue, "continue is taken only when the prompt offers it")
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"executionId": "exec-1", "flowStatus": flowStatusComplete, "assertion": "the-assertion"})
		case body["action"] == flowActionSubmit:
			require.Equal(f.t, "ch-1", body["challengeToken"])
			if password, known := f.users[inputs["username"].(string)]; known && password == inputs["password"] {
				cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"executionId": "exec-1", "flowStatus": flowStatusComplete, "assertion": "the-assertion"})
				return
			}
			if f.wrongPasswordStatus != http.StatusOK {
				cloudbase.WriteJSON(w, f.wrongPasswordStatus, map[string]any{"code": "FES-1001", "message": "Invalid credentials"})
				return
			}
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"executionId": "exec-1", "flowStatus": flowStatusError, "failureReason": "Invalid credentials"})
		default:
			f.t.Errorf("unexpected flow action %v", body["action"])
			w.WriteHeader(http.StatusBadRequest)
		}
	case fakeIdentityPrefix + "/oauth2/auth/callback":
		var body map[string]any
		require.NoError(f.t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(f.t, "auth-1", body["authId"])
		require.Equal(f.t, "the-assertion", body["assertion"])
		require.Equal(f.t, []any{}, body["sessionData"], "the callback carries a list even when the flow reported no session")
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"redirect_uri": f.portal.URL + "/api-portal/acme/callback?code=the-code&state=st"})
	default:
		f.t.Errorf("unexpected identity provider request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

// signInSteps returns steps whose client trusts the fake site, in a runner whose portal is the
// fake portal.
func signInSteps(t *testing.T, site *fakePortalSite) (*Steps, context.Context) {
	t.Helper()
	client := site.client()
	steps := &Steps{Steps: &cloudbase.Steps{Funnel: httpx.NewFunnel(client, 0, 0), Client: client}}
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	require.NoError(t, tcontext.Set(ctx, keyAPIPortalURL, site.portal.URL))
	registry := cleanup.NewRegistry(slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, RegisterConsumerDeleters(registry, (&Steps{Steps: &cloudbase.Steps{Client: client}}).SignInConsumer))
	require.NoError(t, cleanup.Install(ctx, registry))
	return steps, ctx
}

func TestViewSignInPage(t *testing.T) {
	org, view, ok := viewSignInPage("/api-portal/acme/views/default/login")
	require.True(t, ok)
	require.Equal(t, "acme", org)
	require.Equal(t, "default", view)
	for _, other := range []string{"/api-portal/login", "/api-portal/acme/views/default", "/portal/acme/views/default/login",
		"/api-portal//views/default/login", "/api-portal/acme/pages/default/login", "/api-portal/acme/views/default/login/extra"} {
		_, _, ok := viewSignInPage(other)
		require.False(t, ok, other)
	}
}

func TestIdentityBaseKeepsThePrefixAndDropsTheQuery(t *testing.T) {
	authorize, err := url.Parse("https://idp.example.test/t/acme/oauth2/authorize?client_id=c#x")
	require.NoError(t, err)
	base := identityBase(authorize)
	require.Equal(t, "https://idp.example.test/t/acme", base.String())
	require.Equal(t, "https://idp.example.test/t/acme/flow/execute", joinURLPath(base, identityFlowPath))
	require.Equal(t, "https://idp.example.test/t/acme/oauth2/authorize?client_id=c#x", authorize.String(), "the authorization URL is left as it was")
}

func TestOpenSignInFollowsThePortalToItsIdentityProvider(t *testing.T) {
	site := newFakePortalSite(t)
	steps, ctx := signInSteps(t, site)
	require.ErrorContains(t, steps.assertSentToIdentityProvider(ctx), "opened no managed API Portal")

	require.NoError(t, steps.openSignInAsVisitor(ctx))
	require.NoError(t, steps.assertSentToIdentityProvider(ctx))
	browser, err := scenarioBrowser(ctx)
	require.NoError(t, err)
	require.Equal(t, "acme", browser.org)
	require.Equal(t, "default", browser.view)
	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, published.StatusCode, "the answer that sent the visitor away is published")

	site.clientID = ""
	require.NoError(t, steps.openSignInAsVisitor(ctx))
	require.ErrorContains(t, steps.assertSentToIdentityProvider(ctx), "names no client")
	site.clientID, site.callbackHost = "portal-client", site.idp.URL
	require.NoError(t, steps.openSignInAsVisitor(ctx))
	require.ErrorContains(t, steps.assertSentToIdentityProvider(ctx), "not to the portal")
}

func TestOpenSignInRefusesAPortalWithoutAnHTTPSURL(t *testing.T) {
	t.Setenv(cloudbase.CloudLocalDataPlaneEnv, "")
	site := newFakePortalSite(t)
	steps, ctx := signInSteps(t, site)
	require.NoError(t, tcontext.Set(ctx, keyAPIPortalURL, strings.Replace(site.portal.URL, "https://", "http://", 1)))
	require.ErrorContains(t, steps.openSignInAsVisitor(ctx), "https")
}

func TestSignUpThenSignInKeepsTheConsumersSession(t *testing.T) {
	site := newFakePortalSite(t)
	steps, ctx := signInSteps(t, site)
	require.ErrorContains(t, steps.SignInConsumer(ctx), "signed up no consumer")

	require.NoError(t, steps.signUpConsumer(ctx))
	username, password, err := consumerCredentials(ctx)
	require.NoError(t, err)
	require.Equal(t, password, site.users[username])
	require.Equal(t, []any{nil, flowActionSignUp, flowActionSubmit}, flowActions(site.flows))
	require.Equal(t, username+"@example.com", site.flows[2]["inputs"].(map[string]any)["email"])
	require.NoError(t, steps.assertConsumerSignedIn(ctx))
	require.NoError(t, steps.assertVisitorWithoutSessionSentToSignIn(ctx))
	require.ErrorContains(t, steps.assertConsumerNotSignedIn(ctx), "was not refused")

	site.flows = nil
	require.NoError(t, steps.SignInConsumer(ctx))
	require.Equal(t, []any{nil, flowActionSubmit}, flowActions(site.flows))
	require.NoError(t, steps.assertConsumerSignedIn(ctx))
	require.ErrorContains(t, steps.assertSignInRefused(ctx), "accepted the sign-in")
}

func TestSignUpContinuesPastThePromptThatConfirmsTheAccount(t *testing.T) {
	site := newFakePortalSite(t)
	site.accountCreatedActions = []string{flowActionContinue}
	steps, ctx := signInSteps(t, site)

	require.NoError(t, steps.signUpConsumer(ctx))
	require.Equal(t, []any{nil, flowActionSignUp, flowActionSubmit, flowActionContinue}, flowActions(site.flows))
	require.Nil(t, site.flows[3]["inputs"], "continue submits no inputs")
	require.NoError(t, steps.assertConsumerSignedIn(ctx))

	site.flows = nil
	require.NoError(t, steps.SignInConsumer(ctx), "the consumer the flow created signs in again")
	require.Equal(t, []any{nil, flowActionSubmit}, flowActions(site.flows))
}

func TestSignUpFailsWhenThePromptThatConfirmsTheAccountOffersNoWayOn(t *testing.T) {
	site := newFakePortalSite(t)
	site.accountCreatedActions = []string{}
	steps, ctx := signInSteps(t, site)

	err := steps.signUpConsumer(ctx)
	require.ErrorContains(t, err, "refused the consumer's sign-in: the flow ended as INCOMPLETE")
	require.Equal(t, []any{nil, flowActionSignUp, flowActionSubmit}, flowActions(site.flows), "an action the prompt does not offer is not sent")
}

func TestOnlyOptionalActionsFollow(t *testing.T) {
	require.True(t, onlyOptional(nil), "no action follows")
	require.True(t, onlyOptional([]flowAction{{action: flowActionContinue, optional: true}}))
	require.False(t, onlyOptional([]flowAction{{action: flowActionContinue, optional: true}, {action: flowActionSubmit}}))
}

func TestFlowStepOffersOnlyTheActionsItsPromptLists(t *testing.T) {
	var step identityFlowStep
	require.NoError(t, json.Unmarshal([]byte(`{"flowStatus":"INCOMPLETE","data":{"actions":[{"ref":"action_001","nextNode":"credentials_auth"}]}}`), &step))
	require.True(t, step.offers(flowActionSubmit))
	require.False(t, step.offers(flowActionContinue))
	require.False(t, identityFlowStep{}.offers(flowActionSubmit), "a step without a prompt offers nothing")
}

func flowActions(flows []map[string]any) []any {
	actions := make([]any, 0, len(flows))
	for _, flow := range flows {
		actions = append(actions, flow["action"])
	}
	return actions
}

func TestAWrongPasswordIsRefusedAndLeavesNoSession(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusBadRequest, http.StatusUnauthorized} {
		site := newFakePortalSite(t)
		site.wrongPasswordStatus = status
		steps, ctx := signInSteps(t, site)
		require.NoError(t, steps.signUpConsumer(ctx))
		_, password, err := consumerCredentials(ctx)
		require.NoError(t, err)

		require.NoError(t, steps.trySignInWithWrongPassword(ctx), "a refusal is the identity provider's answer, not a step failure")
		require.NoError(t, steps.assertSignInRefused(ctx))
		require.NoError(t, steps.assertConsumerNotSignedIn(ctx))
		err = steps.SignInConsumer(ctx)
		require.NoError(t, err, "the right password still signs the consumer in")

		require.NoError(t, steps.trySignInWithWrongPassword(ctx))
		err = requireSignInAccepted(ctx)
		require.ErrorContains(t, err, "refused the consumer's sign-in")
		require.NotContains(t, err.Error(), password, "an error never repeats the consumer's password")
	}
}

func TestSignOutEndsTheConsumersSession(t *testing.T) {
	site := newFakePortalSite(t)
	steps, ctx := signInSteps(t, site)
	require.ErrorContains(t, steps.signOutConsumer(ctx), "opened no managed API Portal")
	require.NoError(t, steps.signUpConsumer(ctx))
	require.NoError(t, steps.assertConsumerSignedIn(ctx))
	require.NoError(t, steps.signOutConsumer(ctx))
	require.NoError(t, steps.assertConsumerNotSignedIn(ctx))
	require.ErrorContains(t, steps.assertConsumerSignedIn(ctx), "could not open")
}

func TestPlanBody(t *testing.T) {
	body := planBody("plan-1", "INACTIVE")
	require.Equal(t, "plan-1", body["id"])
	require.Equal(t, "Synthetic plan-1", body["displayName"])
	require.Equal(t, "INACTIVE", body["status"])
	require.Len(t, body["limits"], 1)
}

func TestDraftBody(t *testing.T) {
	body, err := draftBody("api-1", "plan-1", nil)
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"displayName": "Synthetic api-1", "version": "1.0.0", "subscriptionPlanIds": []string{"plan-1"},
	}, body)

	body, err = draftBody("api-1", "plan-1", map[string]string{
		"displayName": "Edited", "description": "d", "subscriptionPlanIds": `["a", "b"]`, "docIds": "[]",
	})
	require.NoError(t, err)
	require.Equal(t, "Edited", body["displayName"])
	require.Equal(t, "d", body["description"])
	require.Equal(t, []string{"a", "b"}, body["subscriptionPlanIds"])
	require.Equal(t, []string{}, body["docIds"])

	_, err = draftBody("api-1", "plan-1", map[string]string{"docIds": "doc-1"})
	require.ErrorContains(t, err, "JSON array")
	_, err = draftBody("api-1", "plan-1", map[string]string{"docIds": `[1]`})
	require.ErrorContains(t, err, "JSON array")
	_, err = draftBody("api-1", "plan-1", map[string]string{"labels": "[]"})
	require.ErrorContains(t, err, "unsupported draft field")
}

func TestASignInGoesOnThroughAViewThatOnlyAsksToContinue(t *testing.T) {
	require.True(t, asksOnlyToContinue([]byte(`{"flowStatus":"INCOMPLETE","type":"VIEW","data":{"actions":[{"ref":"action_continue","nextNode":"authorization"}]}}`)))

	for name, raw := range map[string]string{
		"a prompt that asks for input": `{"data":{"actions":[{"ref":"action_001"}],"inputs":[{"identifier":"username"}]}}`,
		"a view with another action":   `{"data":{"actions":[{"ref":"action_signup"}]}}`,
		"a view with two actions":      `{"data":{"actions":[{"ref":"action_continue"},{"ref":"action_001"}]}}`,
		"a continue that asks input":   `{"data":{"actions":[{"ref":"action_continue"}],"inputs":[{"identifier":"x"}]}}`,
		"no actions":                   `{"data":{}}`,
		"not JSON":                     `<html>`,
	} {
		require.False(t, asksOnlyToContinue([]byte(raw)), name)
	}
}

func TestFlowPromptExcerptHidesTheTokensAndIsBounded(t *testing.T) {
	excerpt := flowPromptExcerpt([]byte(`{"executionId":"e1","challengeToken":"secret-token","flowStatus":"INCOMPLETE","assertion":"signed","data":{"inputs":[{"identifier":"firstName"}]}}`))
	require.NotContains(t, excerpt, "secret-token")
	require.NotContains(t, excerpt, "signed")
	require.Contains(t, excerpt, `"flowStatus":"INCOMPLETE"`)
	require.Contains(t, excerpt, "firstName", "the inputs the flow still asks for are kept")

	require.Len(t, []rune(flowPromptExcerpt([]byte(strings.Repeat("x", 5000)))), 401)
	require.Empty(t, flowPromptExcerpt(nil))
}

func TestActiveAPIPortalID(t *testing.T) {
	id, err := activeAPIPortalID([]map[string]any{
		{"id": "apip-test-pending", "status": "pending"},
		{"id": "", "status": "active"},
		{"id": "ops-portal", "name": "Operations portal", "status": "active"},
		{"id": "syn-portal-abc-1", "status": "active"},
		{"id": "apip-test", "name": "apip-test-portal", "status": "active"},
	})
	require.NoError(t, err)
	require.Equal(t, "apip-test", id, "only a portal named with the suite's prefix is published to, never another console's or a scenario's")

	_, err = activeAPIPortalID(nil)
	require.ErrorContains(t, err, "no managed API Portal named with the prefix")
	_, err = activeAPIPortalID([]map[string]any{{"id": "ops-portal", "name": "Operations portal", "status": "active"}})
	require.ErrorContains(t, err, "no managed API Portal named with the prefix", "an active portal of another console does not count")
	_, err = activeAPIPortalID([]map[string]any{{"id": "apip-test-failed", "status": "failed"}})
	require.ErrorContains(t, err, "apip-test-failed: failed")
}

func TestTargetOf(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	_, err := targetOf(ctx, "synthetic REST API")
	require.Error(t, err)

	local, ok := tcontext.LocalOf(ctx)
	require.True(t, ok)
	local.Set(keyAPIPortalID, "portal-1")
	local.Set(cloudbase.KeyAPIID, "rest-1")
	local.Set(cloudbase.KeyGraphQLAPIID, "graphql-1")

	target, err := targetOf(ctx, "synthetic REST API")
	require.NoError(t, err)
	require.Equal(t, cloudbase.PublicationTarget{Portal: "portal-1", APIType: "rest-api", API: "rest-1"}, target)
	target, err = targetOf(ctx, "synthetic GraphQL API")
	require.NoError(t, err)
	require.Equal(t, cloudbase.PublicationTarget{Portal: "portal-1", APIType: "graphql-api", API: "graphql-1"}, target)
	_, err = targetOf(ctx, "synthetic MCP server")
	require.ErrorContains(t, err, "unsupported publication subject")
}

func TestCleanupRequests(t *testing.T) {
	method, path, settled, err := cloudbase.UnpublishRequest("portal-1/rest-api/api-1")
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, method)
	require.Equal(t, "/api-portals/portal-1/apis/rest-api/api-1/unpublish", path)
	require.True(t, settled(&httpx.Response{StatusCode: http.StatusNoContent}))

	method, path, settled, err = DeleteDocumentRequest("rest-api/api-1/doc-1")
	require.NoError(t, err)
	require.Equal(t, http.MethodDelete, method)
	require.Equal(t, "/apis/rest-api/api-1/docs/doc-1", path)
	require.True(t, settled(&httpx.Response{StatusCode: http.StatusNotFound}))
	require.False(t, settled(&httpx.Response{StatusCode: http.StatusConflict}))

	_, _, _, err = cloudbase.UnpublishRequest("malformed")
	require.Error(t, err)
	_, _, _, err = DeleteDocumentRequest("malformed")
	require.Error(t, err)
}

func TestSummaryStatusIs(t *testing.T) {
	items := []map[string]any{
		{"apiPortalId": "other", "status": "NOT_PUBLISHED"},
		{"apiPortalId": "portal-1", "status": "PUBLISHED"},
	}
	require.NoError(t, summaryStatusIs(items, "portal-1", "PUBLISHED"))
	require.ErrorContains(t, summaryStatusIs(items, "portal-1", "DEPRECATED"), "want DEPRECATED")
	require.ErrorContains(t, summaryStatusIs(items, "missing", "PUBLISHED"), "do not list")
	require.Error(t, summaryStatusIs(nil, "portal-1", "PUBLISHED"))
}

func TestSelectionIDs(t *testing.T) {
	publication := map[string]any{"subscriptionPlans": []any{
		map[string]any{"id": "plan-1"}, map[string]any{"displayName": "no id"}, "not a selection",
		map[string]any{"id": "plan-2"},
	}}
	require.Equal(t, []string{"plan-1", "plan-2"}, selectionIDs(publication, "subscriptionPlans"))
	require.Empty(t, selectionIDs(publication, "documents"))
	require.Empty(t, selectionIDs(map[string]any{"documents": "wrong"}, "documents"))
}

func TestDefinitionFor(t *testing.T) {
	body, contentType, err := definitionFor("valid", "graphql-api", "api-1")
	require.NoError(t, err)
	require.Equal(t, "application/graphql", contentType)
	require.Equal(t, cloudbase.GraphQLSchema, string(body))

	for _, tc := range []struct{ kind, apiType string }{{"valid", "rest-api"}, {"OpenAPI", "graphql-api"}} {
		body, contentType, err = definitionFor(tc.kind, tc.apiType, "api-1")
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		var document map[string]any
		require.NoError(t, json.Unmarshal(body, &document))
		require.Equal(t, "3.0.3", document["openapi"])
		require.Equal(t, "api-1", document["info"].(map[string]any)["title"])
	}

	body, _, err = definitionFor("invalid", "rest-api", "api-1")
	require.NoError(t, err)
	require.NotContains(t, string(body), "openapi")
	_, _, err = definitionFor("missing", "rest-api", "api-1")
	require.ErrorContains(t, err, "unsupported definition")
}

func TestForgedTokenIsAnUnsignedJWT(t *testing.T) {
	parts := strings.Split(forgedToken(), ".")
	require.Len(t, parts, 3)
	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	require.JSONEq(t, `{"alg":"RS256","typ":"JWT"}`, string(header))
	claims, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(claims, &decoded))
	require.Greater(t, decoded["exp"], float64(time.Now().Unix()))
	require.NotEmpty(t, parts[2])
}

func TestThumbnailFormHoldsAPNG(t *testing.T) {
	body, contentType, err := thumbnailForm()
	require.NoError(t, err)
	_, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	part, err := multipart.NewReader(bytes.NewReader(body), params["boundary"]).NextPart()
	require.NoError(t, err)
	require.Equal(t, "file", part.FormName())
	require.Equal(t, "image/png", part.Header.Get("Content-Type"))
	_, err = png.Decode(part)
	require.NoError(t, err)
}

func TestLandingPageCarriesRawHTML(t *testing.T) {
	require.Contains(t, landingPage, "<script>")
	require.Contains(t, landingPage, landingPageText)
}

func TestSendDefectiveCreateBodyRegistersThePortalAndDropsItWhenRefused(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	for _, tc := range []struct {
		defect string
		check  func(t *testing.T, raw []byte)
	}{
		{"an unknown field", func(t *testing.T, raw []byte) {
			var body map[string]any
			require.NoError(t, json.Unmarshal(raw, &body))
			require.Equal(t, true, body["unexpected"])
		}},
		{"an organization field", func(t *testing.T, raw []byte) {
			var body map[string]any
			require.NoError(t, json.Unmarshal(raw, &body))
			require.Contains(t, body, "organization")
		}},
		{"a name that is not text", func(t *testing.T, raw []byte) {
			var body map[string]any
			require.NoError(t, json.Unmarshal(raw, &body))
			require.EqualValues(t, 42, body["name"])
		}},
		{"malformed JSON", func(t *testing.T, raw []byte) {
			require.False(t, json.Valid(raw))
		}},
	} {
		t.Run(tc.defect, func(t *testing.T) {
			var registeredAtRequest []string
			var registry *cleanup.Registry
			status := http.StatusBadRequest
			steps, ctx, _, reg := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
				raw, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				tc.check(t, raw)
				registeredAtRequest = registeredPortals(registry)
				cloudbase.WriteJSON(w, status, map[string]string{"code": "invalid_request"})
			})
			registry = reg
			require.NoError(t, steps.sendDefectiveCreateBody(ctx, tc.defect))
			require.Len(t, registeredAtRequest, 1, "the portal is registered before the request")
			require.Empty(t, registeredPortals(registry), "a refused portal is dropped from cleanup")

			status = http.StatusAccepted
			require.NoError(t, steps.sendDefectiveCreateBody(ctx, tc.defect))
			require.Len(t, registeredPortals(registry), 1, "a portal the platform took against expectation stays registered")
		})
	}
	steps, ctx, _, _ := cloudSteps(t, func(http.ResponseWriter, *http.Request) {})
	require.ErrorContains(t, steps.sendDefectiveCreateBody(ctx, "no such defect"), "the defects are")
}

func TestPortalUpdateBodyTypesTheFields(t *testing.T) {
	body, err := portalUpdateBody(map[string]string{"name": "New", "description": "", "selfSignup": "false", "loginEnvironment": "dev"})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"name": "New", "description": "", "selfSignup": false, "loginEnvironment": "dev"}, body)

	_, err = portalUpdateBody(map[string]string{"selfSignup": "maybe"})
	require.ErrorContains(t, err, "true or false")
	_, err = portalUpdateBody(map[string]string{"handle": "x"})
	require.ErrorContains(t, err, "unsupported managed API Portal update field")
}

func TestUpdateRunnerPortalPutsTheTableToThePortalAfterRecordingItsSettings(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var puts []map[string]any
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, cloudbase.ManagedPortalsCollection+"/portal-1", r.URL.Path)
		if r.Method == http.MethodGet {
			require.Empty(t, puts, "the settings are read before the first change")
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"id": "portal-1", "name": "Shared", "selfSignup": true, "loginEnvironment": "development", "status": cloudbase.PortalStatusActive})
			return
		}
		require.Equal(t, http.MethodPut, r.Method)
		var sent map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		puts = append(puts, sent)
		cloudbase.WriteJSON(w, http.StatusAccepted, map[string]any{"status": cloudbase.PortalStatusPending})
	})
	local, ok := tcontext.LocalOf(ctx)
	require.True(t, ok)
	local.Set(keyAPIPortalID, "portal-1")
	require.NoError(t, steps.updateRunnerPortal(ctx, cloudbase.ValuesTable(t, []string{"selfSignup", "false"})))
	require.NoError(t, steps.updateRunnerPortal(ctx, cloudbase.ValuesTable(t, []string{"selfSignup", "true"})))
	require.Equal(t, []map[string]any{{"selfSignup": false}, {"selfSignup": true}}, puts)

	var restores []cleanup.Resource
	for _, resource := range registry.Pending() {
		if resource.Kind == KindPortalRestore {
			restores = append(restores, resource)
		}
	}
	require.Len(t, restores, 1, "one record per scenario and portal, however many updates")
	var record portalRestore
	require.NoError(t, json.Unmarshal([]byte(restores[0].ID), &record))
	require.Equal(t, portalRestore{Handle: "portal-1", Fields: map[string]any{"name": "Shared", "selfSignup": true, "loginEnvironment": "development"}}, record)
}

func TestRequireNoFailedChangeNamesTheChangeThePlatformCouldNotFinish(t *testing.T) {
	require.NoError(t, requireNoFailedChange([]byte(`{"id":"portal-1","status":"active"}`)))
	require.NoError(t, requireNoFailedChange([]byte(`{"id":"portal-1","lastChangeError":null}`)))
	err := requireNoFailedChange([]byte(`{"id":"portal-1","lastChangeError":{"op":"selfSignup","message":"Sign-up could not be turned off. The portal keeps its setting."}}`))
	require.ErrorContains(t, err, "selfSignup")
	require.ErrorContains(t, err, "keeps its setting")
	require.Error(t, requireNoFailedChange([]byte(`not json`)))
}

func TestPortalChangesNamesOnlyTheSettingsThatDiffer(t *testing.T) {
	want := portalRestore{Handle: "portal-1", Fields: map[string]any{"name": "Shared", "selfSignup": true}}
	require.Empty(t, portalChanges(map[string]any{"name": "Shared", "selfSignup": true, "status": "active"}, want))
	require.Equal(t, map[string]any{"selfSignup": true},
		portalChanges(map[string]any{"name": "Shared", "selfSignup": false}, want))
	require.Equal(t, map[string]any{"name": "Shared", "selfSignup": true}, portalChanges(map[string]any{}, want))
}

func TestRestorePortalPutsBackWhatChangedAndWaitsUntilActive(t *testing.T) {
	var mu sync.Mutex
	portal := map[string]any{"id": "portal-1", "name": "Renamed", "selfSignup": false, "status": cloudbase.PortalStatusActive}
	var puts []map[string]any
	rebuilds := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			if portal["status"] == cloudbase.PortalStatusPending {
				if rebuilds++; rebuilds > 1 {
					portal["status"] = cloudbase.PortalStatusActive
				}
			}
			cloudbase.WriteJSON(w, http.StatusOK, portal)
		case http.MethodPut:
			var sent map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
			if len(puts) == 0 {
				puts = append(puts, nil)
				cloudbase.WriteJSON(w, http.StatusConflict, map[string]any{"code": "conflict"})
				return
			}
			puts = append(puts, sent)
			for field, value := range sent {
				portal[field] = value
			}
			portal["status"] = cloudbase.PortalStatusPending
			cloudbase.WriteJSON(w, http.StatusAccepted, portal)
		}
	}))
	t.Cleanup(server.Close)
	platform := cloudbase.OrganisationAPI{Client: httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), Base: server.URL, Token: "console-token", PollInterval: retry.BaseInterval}

	record := portalRestore{Handle: "portal-1", Fields: map[string]any{"name": "Shared", "selfSignup": false}}
	require.NoError(t, restorePortal(context.Background(), platform, record))
	require.Equal(t, []map[string]any{nil, {"name": "Shared"}}, puts, "a change refused while the portal rebuilds is repeated, and only the changed setting is sent")
	require.Equal(t, cloudbase.PortalStatusActive, portal["status"], "the restore waits until the portal is active again")

	puts = nil
	require.NoError(t, restorePortal(context.Background(), platform, record))
	require.Empty(t, puts, "a portal that already has its settings is left alone")
}

func TestDeletingAPortalRefusesThePortalThatWasNotBuiltByTheScenario(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("no request may be sent: %s %s", r.Method, r.URL.Path)
	})
	local.Set(keyAPIPortalID, "dap-abcde234")
	err := steps.deleteRunnerPortal(ctx)
	require.ErrorContains(t, err, "never the shared portal")
}

func TestPortalUpdateDefectsBreakARule(t *testing.T) {
	require.Len(t, portalUpdateDefects["a name longer than 255 characters"]["name"], portalNameMaxLength+1)
	require.Len(t, portalUpdateDefects["a description longer than 1023 characters"]["description"], portalDescriptionMaxLength+1)
	require.Empty(t, portalUpdateDefects["an empty name"]["name"])
	require.Contains(t, portalUpdateDefects["a handle"], "handle")

	_, body := portalRequestDefects["a name longer than 255 characters"]("syn-portal-1")
	require.Len(t, body["name"], portalNameMaxLength+1)
	_, body = portalRequestDefects["a description longer than 1023 characters"]("syn-portal-1")
	require.Len(t, body["description"], portalDescriptionMaxLength+1)
}

func TestRequirePortalDetailsNamesWhatIsMissing(t *testing.T) {
	complete := map[string]any{}
	for _, field := range portalDetailFields {
		complete[field] = "value"
	}
	require.NoError(t, requirePortalDetails(complete))

	delete(complete, "url")
	complete["status"] = " "
	err := requirePortalDetails(complete)
	require.ErrorContains(t, err, "url")
	require.ErrorContains(t, err, "status")
}

func TestRequireNoSecretChecksFieldNamesAndNamesTheMarkerNotTheBody(t *testing.T) {
	for _, body := range []string{
		`{"id":"syn-portal-1","name":"Synthetic","status":"active"}`,
		`{"name":"Secrets documentation","description":"The password policy and the shared key rules."}`,
		`{"list":[{"id":"a","description":"client_secret is rotated yearly"}]}`,
	} {
		require.NoError(t, requireNoSecret([]byte(body)), body)
	}
	for _, body := range []string{
		`{"sharedKey":"x"}`, `{"shared_key":"x"}`, `{"clientSecret":"x"}`, `{"dbPassword":"x"}`,
		`{"list":[{"id":"a","sharedKey":"x"}]}`, `{"nested":{"deeper":{"client_secret":"x"}}}`,
	} {
		err := requireNoSecret([]byte(body))
		require.Error(t, err, body)
		require.NotContains(t, err.Error(), `"x"`, "the error never repeats the body")
	}
	require.ErrorContains(t, requireNoSecret([]byte("not json")), "not JSON")
}

func TestAssertPortalLocationNamesTheRunnerPortal(t *testing.T) {
	steps, ctx, _, _ := cloudSteps(t, func(http.ResponseWriter, *http.Request) {})
	local, ok := tcontext.LocalOf(ctx)
	require.True(t, ok)
	local.Set(keyAPIPortalID, "syn-portal-1")
	for location, wantErr := range map[string]bool{
		"/api/v0.9/managed-api-portals/syn-portal-1": false,
		"/api/v0.9/managed-api-portals/other":        true,
		"":                                           true,
	} {
		header := http.Header{}
		if location != "" {
			header.Set("Location", location)
		}
		require.NoError(t, steps.Funnel.Publish(ctx, &httpx.Response{StatusCode: http.StatusAccepted, Headers: header}))
		err := steps.assertPortalLocation(ctx)
		if wantErr {
			require.ErrorContains(t, err, "Location header", location)
		} else {
			require.NoError(t, err, location)
		}
	}
}

func TestPortalViewPathReadsTheOrganisationAndView(t *testing.T) {
	for path, want := range map[string][2]string{
		"/api-portal/acme/views/default":       {"acme", "default"},
		"/api-portal/acme/views/default/":      {"acme", "default"},
		"/api-portal/acme/views/default/login": {"acme", "default"},
	} {
		org, view, ok := portalViewPath(path)
		require.True(t, ok, path)
		require.Equal(t, want, [2]string{org, view}, path)
	}
	for _, path := range []string{"/api-portal/", "/api-portal/acme", "/api-portal/acme/views", "/other/acme/views/default", "/api-portal//views/default"} {
		_, _, ok := portalViewPath(path)
		require.False(t, ok, path)
	}
}

func TestDraftBodyTakesObjectsAndVisibility(t *testing.T) {
	body, err := draftBody("api-1", "plan-1", map[string]string{
		"agentVisibility": "HIDDEN",
		"endpoints":       `{"productionUrl":"https://prod.example.com"}`,
		"owners":          `{"businessOwner":"Pat"}`,
	})
	require.NoError(t, err)
	require.Equal(t, "HIDDEN", body["agentVisibility"])
	require.Equal(t, map[string]any{"productionUrl": "https://prod.example.com"}, body["endpoints"])
	require.Equal(t, map[string]any{"businessOwner": "Pat"}, body["owners"])

	_, err = draftBody("api-1", "plan-1", map[string]string{"owners": "not json"})
	require.ErrorContains(t, err, "must be a JSON object")
}

func TestThumbnailDefectsAreRefusableContent(t *testing.T) {
	require.NotEqual(t, pngSignature, thumbnailDefects["text declared as PNG"]()[:1])
	large := thumbnailDefects["larger than 2 MiB"]()
	require.Greater(t, len(large), thumbnailMaxBytes)
	require.Equal(t, pngSignature, large[:len(pngSignature)])
	_, err := png.Decode(bytes.NewReader(large))
	require.NoError(t, err, "the oversized thumbnail is a real image, so only its size can refuse it")

	valid, err := thumbnailPNG()
	require.NoError(t, err)
	require.Equal(t, pngSignature, valid[:len(pngSignature)])
	require.Less(t, len(valid), thumbnailMaxBytes)
}

func TestTrySaveDraftWithLengthSendsTheFieldAtThatLength(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var sent map[string]any
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		cloudbase.WriteJSON(w, http.StatusBadRequest, map[string]string{"code": "PUBLICATION_VALIDATION_FAILED"})
	})
	local.Set(keyAPIPortalID, "portal-1")
	local.Set(cloudbase.KeyAPIID, "api-1")
	local.Set(keyPlanID, "plan-1")
	for field, length := range map[string]int{"displayName": 256, "version": 31, "description": 0} {
		require.NoError(t, steps.trySaveDraftWithLength(ctx, field, length))
		require.Len(t, sent[field], length, field)
	}
	require.ErrorContains(t, steps.trySaveDraftWithLength(ctx, "tags", 1), "must be a JSON array")
}

func TestSaveDefectiveThumbnailRejectsAnUnknownDefect(t *testing.T) {
	steps, ctx, _, _ := cloudSteps(t, func(http.ResponseWriter, *http.Request) {})
	require.ErrorContains(t, steps.saveDefectiveThumbnail(ctx, "no such defect"), "unknown thumbnail defect")
}

func TestAssertSilentSignInRedirectChecksTheAuthorizationRequest(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, func(http.ResponseWriter, *http.Request) {})
	local.Set(keyAPIPortalURL, "https://abc-portal.example.com")
	for location, want := range map[string]string{
		"https://idp.example.com/oauth2/authorize?prompt=none&response_type=code&client_id=c": "",
		"https://idp.example.com/oauth2/authorize?response_type=code&client_id=c":             "silent sign-in",
		"https://idp.example.com/login?prompt=none&response_type=code":                        "not an authorization endpoint",
		"https://abc-portal.example.com/api-portal/org/views/default/login":                   "kept the visitor on the portal",
		"": "did not redirect",
	} {
		header := http.Header{}
		if location != "" {
			header.Set("Location", location)
		}
		status := http.StatusFound
		if location == "" {
			status = http.StatusOK
		}
		require.NoError(t, steps.Funnel.Publish(ctx, &httpx.Response{StatusCode: status, Headers: header, URL: "https://abc-portal.example.com/api-portal/org/views/default"}))
		err := steps.assertSilentSignInRedirect(ctx)
		if want == "" {
			require.NoError(t, err, location)
		} else {
			require.ErrorContains(t, err, want, location)
		}
	}
}

func TestRequestPortalUnderDeletedHandleRepeatsARefusalThatMayClear(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	fastPortalPolls(t)
	for _, tc := range []struct {
		name     string
		answers  []int
		attempts int
		kept     bool
	}{
		{"a conflict and a server error clear", []int{http.StatusConflict, http.StatusInternalServerError, http.StatusAccepted}, 3, true},
		{"a client error is final", []int{http.StatusBadRequest}, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attempts := 0
			steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
				answer := tc.answers[min(attempts, len(tc.answers)-1)]
				attempts++
				cloudbase.WriteJSON(w, answer, map[string]string{"id": "syn-portal-1", "status": cloudbase.PortalStatusPending})
			})
			local.Set(keyAPIPortalID, "syn-portal-1")
			require.NoError(t, steps.requestPortalUnderDeletedHandle(ctx))
			require.Equal(t, tc.attempts, attempts)
			require.Equal(t, tc.kept, len(registeredPortals(registry)) == 1)
		})
	}
}

func TestSaveDefinitionAcceptsOnlyAnswersAContentRefusalCanGive(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var status int
	var code string
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if status == http.StatusNoContent {
			w.WriteHeader(status)
			return
		}
		cloudbase.WriteJSON(w, status, map[string]string{"code": code})
	})
	local.Set(keyAPIPortalID, "portal-1")
	local.Set(cloudbase.KeyAPIID, "api-1")
	for name, tc := range map[string]struct {
		kind    string
		status  int
		code    string
		wantErr bool
	}{
		"stored":                          {"invalid", http.StatusNoContent, "", false},
		"refused for its content":         {"invalid", http.StatusBadRequest, publicationValidationFailed, false},
		"OpenAPI refused for its content": {"OpenAPI", http.StatusBadRequest, publicationValidationFailed, false},
		"bad request for another reason":  {"invalid", http.StatusBadRequest, "API_NOT_FOUND", true},
		"no token":                        {"invalid", http.StatusUnauthorized, "unauthorized", true},
		"server error":                    {"OpenAPI", http.StatusInternalServerError, "internal_error", true},
		"a valid definition is stored":    {"valid", http.StatusNoContent, "", false},
		"a valid definition is refused":   {"valid", http.StatusBadRequest, publicationValidationFailed, true},
	} {
		status, code = tc.status, tc.code
		err := steps.saveDefinition(ctx, tc.kind, "synthetic REST API")
		if tc.wantErr {
			require.Error(t, err, name)
		} else {
			require.NoError(t, err, name)
		}
	}
}

func TestFindOtherEnvironmentRecordsAnEnvironmentThePortalDoesNotUse(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	loginEnvironment := "development"
	environments := []map[string]any{{"name": "development", "id": "development"}}
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case cloudbase.ManagedPortalsCollection + "/portal-1":
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"id": "portal-1", "loginEnvironment": loginEnvironment})
		case "/environments":
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"list": environments})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	local.Set(keyAPIPortalID, "portal-1")
	require.ErrorIs(t, steps.findOtherEnvironment(ctx), godog.ErrSkip, "a single environment cannot test a change of environment")

	environments = append(environments, map[string]any{"name": "staging", "id": "staging"})
	require.NoError(t, steps.findOtherEnvironment(ctx))
	got, err := cloudbase.RequiredValue(ctx, keyOtherEnvironment)
	require.NoError(t, err)
	require.Equal(t, "staging", got)

	loginEnvironment = "staging"
	require.NoError(t, steps.findOtherEnvironment(ctx), "the other environment is relative to the portal's own login environment")
	got, err = cloudbase.RequiredValue(ctx, keyOtherEnvironment)
	require.NoError(t, err)
	require.Equal(t, "development", got)
}

func TestDroppingThePublicationCleanupIsAnExplicitStep(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNoContent)
	})
	local.Set(keyAPIPortalID, "portal-1")
	local.Set(cloudbase.KeyAPIID, "api-1")
	require.NoError(t, registry.Register(cleanup.Resource{Kind: CloudPublicationKind, ID: "portal-1/rest-api/api-1", Actor: cloudbase.ResourceActor}))
	require.NoError(t, registry.Register(cleanup.Resource{Kind: CloudPublicationKind, ID: "portal-1/rest-api/another-api", Actor: cloudbase.ResourceActor}))

	require.NoError(t, steps.DeleteAPI(ctx))
	require.Len(t, registry.Pending(), 2, "deleting the API alone keeps the publication cleanup registered")

	require.NoError(t, steps.dropPublicationCleanup(ctx))
	pending := registry.Pending()
	require.Len(t, pending, 1, "only the deleted API's publication is dropped")
	require.Equal(t, "portal-1/rest-api/another-api", pending[0].ID)
}

// fakeConsumerAPI is the consumer's part of a portal's REST API; it checks the session and CSRF token.
type fakeConsumerAPI struct {
	// offered names the plans each API offers, and planNames each plan's display name.
	offered   map[string][]string
	planNames map[string]string

	// refuseCreate, when set, is how the portal answers an application create.
	refuseCreate int
	// secretStatus, when set, is the status of every answer that carries a credential.
	secretStatus int
	// failPageFrom, when set, fails every list page from that offset with 502.
	failPageFrom int
	// failOnce answers each listed call ("METHOD /path") once with its status. With status 0 it
	// drops the connection before the call takes effect, and with commitThenDrop after it.
	failOnce map[string]int
	// failEvery answers each listed call with its status every time.
	failEvery map[string]int
	// keyManagers, when set, replaces the portal's key managers.
	keyManagers []any
	// request is the call being answered, for the page it asks for.
	request *http.Request

	apps   map[string]map[string]any
	subs   map[string]map[string]any
	keys   map[string]map[string]any
	oauth2 map[string]map[string]any
	seq    int
	calls  []string
}

// commitThenDrop is a failOnce status that applies the call and then drops the connection, so
// the caller never sees the answer to a call that took effect.
const commitThenDrop = -1

// fakePage pages items as the portal does: limit (default 20, at most 100) items from offset.
func fakePage(r *http.Request, items []any) map[string]any {
	limit, offset := 20, 0
	if value, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil {
		limit = min(max(value, 0), 100)
	}
	if value, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil {
		offset = max(value, 0)
	}
	slices.SortFunc(items, func(a, b any) int {
		return strings.Compare(fmt.Sprint(a), fmt.Sprint(b))
	})
	page := []any{}
	if offset < len(items) {
		page = items[offset:min(offset+limit, len(items))]
	}
	return map[string]any{"count": len(page), "list": page, "pagination": map[string]any{"limit": limit, "offset": offset, "total": len(items)}}
}

func (f *fakePortalSite) serveConsumerAPI(w http.ResponseWriter, r *http.Request) {
	api := &f.consumer
	api.request = r
	if api.apps == nil {
		api.apps, api.subs, api.keys = map[string]map[string]any{}, map[string]map[string]any{}, map[string]map[string]any{}
	}
	session := f.sessionOf(r)
	if !f.sessions[session] {
		cloudbase.WriteJSON(w, http.StatusUnauthorized, map[string]any{"message": "Unauthorized"})
		return
	}
	if r.Method != http.MethodGet && r.Header.Get(portalCSRFHeader) != "csrf-"+session {
		cloudbase.WriteJSON(w, http.StatusForbidden, map[string]any{"message": "CSRF token missing or invalid"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, portalAPIBasePath)
	call := r.Method + " " + path
	api.calls = append(api.calls, call)
	if status, fail := api.failEvery[call]; fail {
		cloudbase.WriteJSON(w, status, map[string]any{"message": http.StatusText(status)})
		return
	}
	if status, fail := api.failOnce[call]; fail && status == commitThenDrop {
		delete(api.failOnce, call)
		connection := w
		w = httptest.NewRecorder()
		defer func() {
			if conn, _, err := connection.(http.Hijacker).Hijack(); err == nil {
				_ = conn.Close()
			}
		}()
	} else if fail {
		delete(api.failOnce, call)
		if status == 0 {
			if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
				_ = conn.Close()
			}
			return
		}
		cloudbase.WriteJSON(w, status, map[string]any{"message": http.StatusText(status)})
		return
	}
	if offset, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && api.failPageFrom > 0 && offset >= api.failPageFrom {
		cloudbase.WriteJSON(w, http.StatusBadGateway, map[string]any{"message": "Bad Gateway"})
		return
	}
	var body map[string]any
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			f.t.Errorf("the portal API request %s does not decode: %v", call, err)
			cloudbase.WriteJSON(w, http.StatusBadRequest, map[string]any{"message": "Bad Request"})
			return
		}
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	switch {
	case parts[0] == "applications" && len(parts) == 3 && parts[2] == "oauth2-keys" && r.Method == http.MethodGet:
		list := []any{}
		for _, key := range api.oauth2 {
			if key["applicationId"] == parts[1] {
				list = append(list, map[string]any{"keyId": key["keyId"], "name": key["name"]})
			}
		}
		cloudbase.WriteJSON(w, http.StatusOK, fakePage(r, list))
	case parts[0] == "applications":
		api.serveApplications(w, r.Method, parts[1:], body)
	case parts[0] == "subscriptions":
		api.serveSubscriptions(w, r.Method, parts[1:], body)
	case parts[0] == "api-keys" && r.Method == http.MethodGet:
		cloudbase.WriteJSON(w, http.StatusOK, api.keyList(func(map[string]any) bool { return true }))
	case parts[0] == "key-managers" && len(parts) == 2 && parts[1] == "metadata":
		managers := api.keyManagers
		if managers == nil {
			managers = []any{map[string]any{"id": "imported", "keyCreation": "import"}, map[string]any{"id": "thunder", "keyCreation": "register"}}
		}
		cloudbase.WriteJSON(w, http.StatusOK, fakePage(r, managers))
	case parts[0] == "oauth2-keys":
		api.serveOAuth2Keys(w, r.Method, parts[1:], body)
	case parts[0] == "apis" && len(parts) >= 3 && parts[2] == "api-keys" && api.offered[parts[1]] == nil:
		cloudbase.WriteJSON(w, http.StatusNotFound, map[string]any{"message": "API not found"})
	case parts[0] == "apis" && len(parts) >= 3 && parts[2] == "api-keys":
		api.serveAPIKeys(w, r.Method, parts[1], parts[3:], body)
	default:
		f.t.Errorf("unexpected portal API request %s %s", r.Method, path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func (a *fakeConsumerAPI) serveApplications(w http.ResponseWriter, method string, rest []string, body map[string]any) {
	switch {
	case len(rest) == 0 && method == http.MethodPost && a.refuseCreate != 0:
		cloudbase.WriteJSON(w, a.refuseCreate, map[string]any{"message": "refused"})
	case len(rest) == 0 && method == http.MethodPost:
		id, _ := body["id"].(string)
		if _, taken := a.apps[id]; taken || id == "" {
			cloudbase.WriteJSON(w, http.StatusConflict, map[string]any{"message": "Application already exists"})
			return
		}
		a.apps[id] = map[string]any{"id": id, "displayName": body["displayName"], "description": body["description"]}
		cloudbase.WriteJSON(w, http.StatusCreated, a.apps[id])
	case len(rest) == 0 && method == http.MethodGet:
		list := make([]any, 0, len(a.apps))
		for _, app := range a.apps {
			list = append(list, app)
		}
		cloudbase.WriteJSON(w, http.StatusOK, fakePage(a.request, list))
	case len(rest) == 2 && rest[1] == "api-keys" && method == http.MethodGet:
		cloudbase.WriteJSON(w, http.StatusOK, a.keyList(func(key map[string]any) bool { return key["appId"] == rest[0] }))
	case len(rest) == 1:
		app, known := a.apps[rest[0]]
		if !known {
			cloudbase.WriteJSON(w, http.StatusNotFound, map[string]any{"message": "Application not found"})
			return
		}
		switch method {
		case http.MethodGet:
			cloudbase.WriteJSON(w, http.StatusOK, app)
		case http.MethodPut:
			if _, renamed := body["id"]; renamed {
				cloudbase.WriteJSON(w, http.StatusBadRequest, map[string]any{"message": "an update keeps the handle"})
				return
			}
			app["displayName"], app["description"] = body["displayName"], body["description"]
			cloudbase.WriteJSON(w, http.StatusOK, app)
		case http.MethodDelete:
			delete(a.apps, rest[0])
			w.WriteHeader(http.StatusOK)
		default:
			writeMethodNotAllowed(w)
		}
	default:
		cloudbase.WriteJSON(w, http.StatusNotFound, map[string]any{"message": "Not Found"})
	}
}

func (a *fakeConsumerAPI) statusOr(status int) int {
	if a.secretStatus != 0 {
		return a.secretStatus
	}
	return status
}

func (a *fakeConsumerAPI) offers(api, plan string) bool {
	return slices.Contains(a.offered[api], plan)
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	cloudbase.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{"message": "Method Not Allowed"})
}

func (a *fakeConsumerAPI) serveSubscriptions(w http.ResponseWriter, method string, rest []string, body map[string]any) {
	if len(rest) == 0 && method == http.MethodPost {
		artifact, _ := body["artifactId"].(string)
		plan, _ := body["subscriptionPlanId"].(string)
		if !a.offers(artifact, plan) {
			cloudbase.WriteJSON(w, http.StatusBadRequest, map[string]any{"message": "Subscription plan not found for this API"})
			return
		}
		for _, sub := range a.subs {
			if sub["artifactId"] == artifact {
				cloudbase.WriteJSON(w, http.StatusConflict, map[string]any{"message": "A subscription for this API already exists"})
				return
			}
		}
		a.seq++
		id := "sub-" + strconv.Itoa(a.seq)
		a.subs[id] = map[string]any{"subscriptionId": id, "subscriptionToken": "token-" + strconv.Itoa(a.seq), "status": "ACTIVE",
			"artifactId": artifact, "subscriptionPlanName": a.planNames[plan]}
		cloudbase.WriteJSON(w, http.StatusCreated, a.subs[id])
		return
	}
	if len(rest) == 0 && method == http.MethodGet {
		artifact := a.request.URL.Query().Get("artifactId")
		if artifact != "" && a.offered[artifact] == nil {
			cloudbase.WriteJSON(w, http.StatusNotFound, map[string]any{"message": "API not found"})
			return
		}
		list := make([]any, 0, len(a.subs))
		for _, sub := range a.subs {
			if artifact == "" || sub["artifactId"] == artifact {
				list = append(list, sub)
			}
		}
		cloudbase.WriteJSON(w, http.StatusOK, fakePage(a.request, list))
		return
	}
	if len(rest) == 0 {
		writeMethodNotAllowed(w)
		return
	}
	sub, known := a.subs[rest[0]]
	if !known {
		cloudbase.WriteJSON(w, http.StatusNotFound, map[string]any{"message": "Subscription not found"})
		return
	}
	switch {
	case len(rest) == 1 && method == http.MethodGet:
		cloudbase.WriteJSON(w, http.StatusOK, sub)
	case len(rest) == 1 && method == http.MethodPut:
		sub["status"] = body["status"]
		cloudbase.WriteJSON(w, http.StatusOK, sub)
	case len(rest) == 1 && method == http.MethodDelete:
		delete(a.subs, rest[0])
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"message": "Subscription deleted successfully"})
	case len(rest) == 2 && method != http.MethodPost:
		writeMethodNotAllowed(w)
	case len(rest) == 2 && rest[1] == "change-plan":
		plan, _ := body["planId"].(string)
		if !a.offers(sub["artifactId"].(string), plan) {
			cloudbase.WriteJSON(w, http.StatusBadRequest, map[string]any{"message": "Subscription plan not found for this API"})
			return
		}
		sub["subscriptionPlanName"] = a.planNames[plan]
		cloudbase.WriteJSON(w, http.StatusOK, sub)
	case len(rest) == 2 && rest[1] == "regenerate-token":
		a.seq++
		sub["subscriptionToken"] = "token-" + strconv.Itoa(a.seq)
		cloudbase.WriteJSON(w, http.StatusOK, sub)
	default:
		cloudbase.WriteJSON(w, http.StatusNotFound, map[string]any{"message": "Not Found"})
	}
}

func (a *fakeConsumerAPI) keyList(keep func(map[string]any) bool) map[string]any {
	list := []any{}
	for _, key := range a.keys {
		if keep(key) {
			list = append(list, map[string]any{"id": key["id"], "apiId": key["apiId"], "appId": key["appId"], "status": key["status"]})
		}
	}
	return fakePage(a.request, list)
}

func (a *fakeConsumerAPI) serveAPIKeys(w http.ResponseWriter, method, apiID string, rest []string, body map[string]any) {
	if len(rest) == 0 && method == http.MethodGet {
		cloudbase.WriteJSON(w, http.StatusOK, a.keyList(func(key map[string]any) bool { return key["apiId"] == apiID }))
		return
	}
	if len(rest) != 1 || method != http.MethodPost {
		cloudbase.WriteJSON(w, http.StatusNotFound, map[string]any{"message": "Not Found"})
		return
	}
	if rest[0] == "generate" {
		a.seq++
		id := "key-" + strconv.Itoa(a.seq)
		a.keys[id] = map[string]any{"id": id, "apiId": apiID, "status": "ACTIVE", "value": "value-" + strconv.Itoa(a.seq)}
		cloudbase.WriteJSON(w, a.statusOr(http.StatusCreated), map[string]any{"id": id, "displayName": body["displayName"], "key": a.keys[id]["value"], "status": "ACTIVE"})
		return
	}
	id, _ := body["keyId"].(string)
	key, known := a.keys[id]
	if !known || key["apiId"] != apiID {
		cloudbase.WriteJSON(w, http.StatusNotFound, map[string]any{"message": "API key not found"})
		return
	}
	switch rest[0] {
	case "regenerate":
		a.seq++
		key["value"] = "value-" + strconv.Itoa(a.seq)
		cloudbase.WriteJSON(w, a.statusOr(http.StatusOK), map[string]any{"id": id, "key": key["value"], "status": key["status"]})
	case "revoke":
		if key["status"] == "REVOKED" {
			cloudbase.WriteJSON(w, http.StatusConflict, map[string]any{"message": "API key is already revoked"})
			return
		}
		key["status"] = "REVOKED"
		w.WriteHeader(http.StatusNoContent)
	case "associate":
		key["appId"] = body["appId"]
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"message": "API key associated"})
	case "dissociate":
		delete(key, "appId")
		w.WriteHeader(http.StatusNoContent)
	default:
		cloudbase.WriteJSON(w, http.StatusNotFound, map[string]any{"message": "Not Found"})
	}
}

// consumerSteps signs a consumer up on a fake portal whose api-1 offers plan-1 and, as the other
// plan, plan-2.
func consumerSteps(t *testing.T) (*fakePortalSite, *Steps, context.Context, *cleanup.Registry) {
	t.Helper()
	site := newFakePortalSite(t)
	site.consumer.offered = map[string][]string{"api-1": {"plan-1", "plan-2"}}
	site.consumer.planNames = map[string]string{"plan-1": "Synthetic plan-1", "plan-2": "Synthetic plan-2"}
	steps, ctx := signInSteps(t, site)
	registry, err := cleanup.Of(ctx)
	require.NoError(t, err)
	for key, value := range map[string]string{cloudbase.KeyAPIID: "api-1", keyPlanID: "plan-1", keySecondPlanID: "plan-2"} {
		require.NoError(t, tcontext.Set(ctx, key, value))
	}
	require.NoError(t, steps.signUpConsumer(ctx))
	return site, steps, ctx, registry
}

func publishedStatusOf(t *testing.T, ctx context.Context) int {
	t.Helper()
	resp, err := httpx.Published(ctx)
	require.NoError(t, err)
	return resp.StatusCode
}

// scenarioPending names the pending cleanup registrations other than the runner's sweep.
func scenarioPending(registry *cleanup.Registry) []string {
	ids := []string{}
	for _, resource := range registry.Pending() {
		if resource.Kind.Name != kindConsumerLeftovers.Name {
			ids = append(ids, resource.Kind.Name+":"+resource.ID)
		}
	}
	return ids
}

// calledCollections names the method and first path segment of each call the fake answered.
func calledCollections(calls []string) []string {
	collections := []string{}
	for _, call := range calls {
		method, path, _ := strings.Cut(call, " ")
		collection, _, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")
		collections = append(collections, method+" "+collection)
	}
	return collections
}

func TestConsumerCallsNeedASignedInSessionAndRepeatItsCSRFToken(t *testing.T) {
	site := newFakePortalSite(t)
	visitorSteps, visitorCtx := signInSteps(t, site)
	require.ErrorContains(t, visitorSteps.createConsumerApplication(visitorCtx), "opened no managed API Portal",
		"a consumer call needs the scenario's browser")
	_, _, err := visitorSteps.openSignIn(visitorCtx)
	require.NoError(t, err)
	visitor, err := scenarioBrowser(visitorCtx)
	require.NoError(t, err)
	_, err = visitor.apiRequest(visitorCtx, http.MethodPost, "/applications", map[string]any{})
	require.ErrorIs(t, err, errNoConsumerSession, "a visitor who never signed in has no token to repeat")

	_, _, ctx, _ := consumerSteps(t)
	browser, err := scenarioBrowser(ctx)
	require.NoError(t, err)
	resp, err := browser.apiRequest(ctx, http.MethodGet, "/applications", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "a read sends no token")
	browser.jar.SetCookies(browser.portal, []*http.Cookie{{Name: portalCSRFCookie, Value: "forged", Path: "/api-portal"}})
	resp, err = browser.apiRequest(ctx, http.MethodPost, "/applications", map[string]any{"id": "x"})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "the call repeats the token the browser holds")
}

func TestSignUpRegistersTheRunnersSweepOfTheConsumersPortal(t *testing.T) {
	_, _, ctx, registry := consumerSteps(t)
	username, _, err := consumerCredentials(ctx)
	require.NoError(t, err)
	var sweeps []cleanup.Resource
	for _, resource := range registry.Pending() {
		if resource.Kind.Name == kindConsumerLeftovers.Name {
			sweeps = append(sweeps, resource)
		}
	}
	require.Len(t, sweeps, 1)
	require.Equal(t, username, sweeps[0].ID)
	require.NoError(t, registry.Sweep(ctx))
	require.Len(t, registry.Pending(), 1, "the sweep is the runner's, so a scenario's cleanup leaves it")
}

func TestConsumerApplicationLifecycle(t *testing.T) {
	_, steps, ctx, registry := consumerSteps(t)

	require.NoError(t, steps.createConsumerApplication(ctx))
	id, err := cloudbase.RequiredValue(ctx, keyConsumerApplicationID)
	require.NoError(t, err)
	require.Equal(t, []string{"portal-application:" + id}, scenarioPending(registry))
	require.Equal(t, http.StatusCreated, publishedStatusOf(t, ctx))
	require.NoError(t, steps.assertConsumerListing(ctx, "applications", "include", "synthetic application", ""))
	require.ErrorContains(t, steps.assertConsumerListing(ctx, "applications", "not include", "synthetic application", ""), "still include")

	table := cloudbase.ValuesTable(t, []string{"displayName", "Renamed " + id}, []string{"description", "Renamed."})
	require.NoError(t, steps.updateConsumerApplication(ctx, table))
	require.Equal(t, http.StatusOK, publishedStatusOf(t, ctx), "an update keeps the handle the application was created with")
	require.NoError(t, steps.readConsumerApplication(ctx))
	read, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, read.StatusCode)
	var answer map[string]any
	require.NoError(t, json.Unmarshal(read.Body, &answer))
	require.Equal(t, id, answer["id"])
	require.Equal(t, "Renamed "+id, answer["displayName"], "the read returns the renamed application")
	require.ErrorContains(t, steps.updateConsumerApplication(ctx, cloudbase.ValuesTable(t, []string{"id", "other"})), `unsupported application field "id"`)

	require.NoError(t, steps.deleteConsumerApplication(ctx))
	require.Equal(t, http.StatusOK, publishedStatusOf(t, ctx))
	require.Empty(t, scenarioPending(registry), "an application the scenario deleted is no longer cleanup's")
	require.NoError(t, steps.readConsumerApplication(ctx))
	require.Equal(t, http.StatusNotFound, publishedStatusOf(t, ctx))
	require.NoError(t, steps.assertConsumerListing(ctx, "applications", "not include", "synthetic application", ""))
}

func TestCreateConsumerApplicationKeepsTheRegistrationOnlyOfWhatThePortalMayHaveStored(t *testing.T) {
	site, steps, ctx, registry := consumerSteps(t)
	site.consumer.refuseCreate = http.StatusConflict
	require.ErrorContains(t, steps.createConsumerApplication(ctx), "409")
	require.Empty(t, scenarioPending(registry), "a refused create stored nothing")

	site.consumer.refuseCreate = http.StatusBadGateway
	require.ErrorContains(t, steps.createConsumerApplication(ctx), "502")
	require.Len(t, scenarioPending(registry), 1, "a server error can follow a stored application, so it stays registered")
	require.NoError(t, registry.Sweep(ctx), "an application the portal never stored is already gone")
}

func TestSubscribeConsumerRegistersTheAPIsSubscriptionsBeforeTheCall(t *testing.T) {
	site, steps, ctx, registry := consumerSteps(t)
	site.consumer.offered["api-1"] = []string{"plan-1"}

	require.NoError(t, steps.subscribeConsumer(ctx, "the other"), "a refusal is left to the status assertion")
	require.Equal(t, http.StatusBadRequest, publishedStatusOf(t, ctx))
	require.Equal(t, []string{"portal-subscriptions:api-1"}, scenarioPending(registry),
		"the API's subscriptions are registered before the call, so one whose answer is lost is still removed")
	_, err := cloudbase.RequiredValue(ctx, keyConsumerSubscriptionID)
	require.Error(t, err)

	require.NoError(t, steps.subscribeConsumer(ctx, "the"))
	require.Equal(t, http.StatusCreated, publishedStatusOf(t, ctx))
	id, err := cloudbase.RequiredValue(ctx, keyConsumerSubscriptionID)
	require.NoError(t, err)
	require.NoError(t, steps.assertConsumerListing(ctx, "subscriptions", "include", "synthetic subscription", ""))

	require.NoError(t, steps.subscribeConsumer(ctx, "the"))
	require.Equal(t, http.StatusConflict, publishedStatusOf(t, ctx))
	again, err := cloudbase.RequiredValue(ctx, keyConsumerSubscriptionID)
	require.NoError(t, err)
	require.Equal(t, id, again, "a refused second subscription leaves the first one the scenario's")
	require.Equal(t, []string{"portal-subscriptions:api-1"}, scenarioPending(registry), "one registration per API")

	require.NoError(t, registry.Sweep(ctx))
	require.Empty(t, site.consumer.subs)
}

func TestConsumerSubscriptionChangesPlanStatusAndToken(t *testing.T) {
	site, steps, ctx, registry := consumerSteps(t)
	require.NoError(t, steps.subscribeConsumer(ctx, "the"))
	id, err := cloudbase.RequiredValue(ctx, keyConsumerSubscriptionID)
	require.NoError(t, err)

	require.NoError(t, steps.changeConsumerPlan(ctx, "the other"))
	require.Equal(t, http.StatusOK, publishedStatusOf(t, ctx))
	require.Equal(t, "Synthetic plan-2", site.consumer.subs[id]["subscriptionPlanName"])

	require.NoError(t, steps.setConsumerSubscriptionStatus(ctx, "INACTIVE"))
	require.Equal(t, "INACTIVE", site.consumer.subs[id]["status"])
	require.NoError(t, steps.setConsumerSubscriptionStatus(ctx, "ACTIVE"))
	require.Equal(t, "ACTIVE", site.consumer.subs[id]["status"])

	require.NoError(t, steps.readConsumerSubscription(ctx))
	require.NoError(t, steps.assertConsumerSubscriptionToken(ctx, "unchanged"))
	require.ErrorContains(t, steps.assertConsumerSubscriptionToken(ctx, "replaced"), "kept its token")
	require.NoError(t, steps.regenerateConsumerSubscriptionToken(ctx))
	err = steps.assertConsumerSubscriptionToken(ctx, "unchanged")
	require.ErrorContains(t, err, "token changed")
	require.NotContains(t, err.Error(), "token-", "an error never repeats a token")
	require.NoError(t, steps.assertConsumerSubscriptionToken(ctx, "replaced"))
	require.NoError(t, steps.readConsumerSubscription(ctx))
	require.NoError(t, steps.assertConsumerSubscriptionToken(ctx, "unchanged"), "the replaced token is the one the subscription keeps")

	require.NoError(t, steps.unsubscribeConsumer(ctx))
	require.Equal(t, http.StatusOK, publishedStatusOf(t, ctx))
	require.Equal(t, []string{"portal-subscriptions:api-1"}, scenarioPending(registry),
		"the scenario may subscribe again, so the API's subscriptions stay registered")
	require.NoError(t, steps.readConsumerSubscription(ctx))
	require.Equal(t, http.StatusNotFound, publishedStatusOf(t, ctx))
	require.NoError(t, steps.assertConsumerListing(ctx, "subscriptions", "not include", "synthetic subscription", ""))
	require.NoError(t, registry.Sweep(ctx), "an API with no subscription left is already clean")
}

func TestConsumerAPIKeyLifecycle(t *testing.T) {
	site, steps, ctx, registry := consumerSteps(t)
	require.NoError(t, steps.createConsumerApplication(ctx))
	app, err := cloudbase.RequiredValue(ctx, keyConsumerApplicationID)
	require.NoError(t, err)

	require.NoError(t, steps.generateConsumerAPIKey(ctx))
	_, err = httpx.Published(ctx)
	require.Error(t, err, "the answer that carries the key is never published")
	id, err := cloudbase.RequiredValue(ctx, keyConsumerAPIKeyID)
	require.NoError(t, err)
	first, err := cloudbase.RequiredValue(ctx, cloudbase.KeyAPIKey)
	require.NoError(t, err)
	require.Equal(t, site.consumer.keys[id]["value"], first)
	require.Contains(t, scenarioPending(registry), "portal-api-keys:api-1")

	require.NoError(t, steps.assertConsumerListing(ctx, "API keys for the synthetic REST API", "include", "API key", "ACTIVE"))
	require.NoError(t, steps.assertConsumerListing(ctx, "API keys", "include", "API key", "ACTIVE"))
	require.ErrorContains(t, steps.assertConsumerListing(ctx, "API keys", "include", "API key", "REVOKED"), `listed as "ACTIVE"`)
	require.NoError(t, steps.assertConsumerListing(ctx, "API keys of the synthetic application", "not include", "API key", ""))

	require.NoError(t, steps.changeConsumerAPIKeyApplication(ctx, "associates the API key with"))
	require.Equal(t, http.StatusOK, publishedStatusOf(t, ctx))
	require.Equal(t, app, site.consumer.keys[id]["appId"])
	require.NoError(t, steps.assertConsumerListing(ctx, "API keys of the synthetic application", "include", "API key", "ACTIVE"))
	require.NoError(t, steps.changeConsumerAPIKeyApplication(ctx, "dissociates the API key from"))
	require.Equal(t, http.StatusNoContent, publishedStatusOf(t, ctx))
	require.NoError(t, steps.assertConsumerListing(ctx, "API keys of the synthetic application", "not include", "API key", ""))

	require.NoError(t, steps.regenerateConsumerAPIKey(ctx))
	previous, err := cloudbase.RequiredValue(ctx, cloudbase.KeyPreviousAPIKey)
	require.NoError(t, err)
	current, err := cloudbase.RequiredValue(ctx, cloudbase.KeyAPIKey)
	require.NoError(t, err)
	require.Equal(t, first, previous)
	require.Equal(t, site.consumer.keys[id]["value"], current)
	require.NotEqual(t, previous, current)

	require.NoError(t, steps.revokeConsumerAPIKey(ctx))
	require.Equal(t, http.StatusNoContent, publishedStatusOf(t, ctx))
	require.NoError(t, steps.assertConsumerListing(ctx, "API keys for the synthetic REST API", "include", "API key", "REVOKED"))
	require.NoError(t, registry.Sweep(ctx), "a key the scenario revoked needs nothing more")
}

func TestConsumerCleanupRemovesWhatTheConsumerCreatedInOrder(t *testing.T) {
	site, steps, ctx, registry := consumerSteps(t)
	require.NoError(t, steps.createConsumerApplication(ctx))
	require.NoError(t, steps.subscribeConsumer(ctx, "the"))
	require.NoError(t, steps.generateConsumerAPIKey(ctx))
	require.NoError(t, steps.changeConsumerAPIKeyApplication(ctx, "associates the API key with"))
	// A subscription and a key the portal stored although their answers never arrived.
	site.consumer.subs["sub-lost"] = map[string]any{"subscriptionId": "sub-lost", "artifactId": "api-1", "status": "ACTIVE"}
	site.consumer.keys["key-lost"] = map[string]any{"id": "key-lost", "apiId": "api-1", "status": "ACTIVE"}
	site.consumer.calls = nil

	require.NoError(t, registry.Sweep(ctx))
	require.Empty(t, scenarioPending(registry))
	require.Empty(t, site.consumer.subs)
	require.Empty(t, site.consumer.apps)
	for _, key := range site.consumer.keys {
		require.Equal(t, "REVOKED", key["status"])
	}
	require.Equal(t, []string{"GET subscriptions", "DELETE subscriptions", "DELETE subscriptions",
		"GET apis", "POST apis", "POST apis", "DELETE applications"}, calledCollections(site.consumer.calls),
		"subscriptions and keys go before the application a key can name, and the lost ones go too")
}

func TestConsumerCleanupSignsTheConsumerInAgainWhenTheSessionEnded(t *testing.T) {
	site, steps, ctx, registry := consumerSteps(t)
	require.NoError(t, steps.createConsumerApplication(ctx))
	require.NoError(t, steps.subscribeConsumer(ctx, "the"))
	require.NoError(t, steps.generateConsumerAPIKey(ctx))
	for session := range site.sessions {
		delete(site.sessions, session)
	}

	require.NoError(t, registry.Sweep(ctx), "an ended session is replaced, not a cleanup failure")
	require.Empty(t, site.consumer.subs)
	require.Empty(t, site.consumer.apps)
	for _, key := range site.consumer.keys {
		require.Equal(t, "REVOKED", key["status"])
	}
}

func TestConsumerCleanupFailsWhenTheConsumerCannotSignIn(t *testing.T) {
	site := newFakePortalSite(t)
	_, ctx := signInSteps(t, site)
	registry, err := cleanup.Of(ctx)
	require.NoError(t, err)
	require.NoError(t, cleanup.Register(ctx, cleanup.Resource{Kind: kindConsumerApplication, ID: "syn-app-1", Actor: consumerActor}))
	err = registry.Sweep(ctx)
	require.ErrorContains(t, err, "signing the consumer in to clean up")
	require.ErrorContains(t, err, "signed up no consumer")
}

func TestTheRunnersSweepEmptiesTheConsumersPortal(t *testing.T) {
	site, steps, ctx, registry := consumerSteps(t)
	require.NoError(t, steps.createConsumerApplication(ctx))
	require.NoError(t, steps.subscribeConsumer(ctx, "the"))
	require.NoError(t, steps.generateConsumerAPIKey(ctx))
	for i := range 150 {
		id := "syn-app-extra-" + strconv.Itoa(i)
		site.consumer.apps[id] = map[string]any{"id": id}
	}
	// The scenario's cleanup failed, and the runner's last scenario left a visitor's browser.
	for _, resource := range registry.Pending() {
		if resource.Kind.Name != kindConsumerLeftovers.Name {
			registry.Deregister(resource.Kind, resource.ID)
		}
	}
	_, _, err := steps.openSignIn(ctx)
	require.NoError(t, err)

	require.NoError(t, registry.SweepRunner(ctx))
	require.Empty(t, registry.Pending())
	require.Empty(t, site.consumer.subs)
	require.Empty(t, site.consumer.apps, "every page of the consumer's applications is removed")
	for _, key := range site.consumer.keys {
		require.Equal(t, "REVOKED", key["status"])
	}
}

func TestListingForMatchesTheItemToItsList(t *testing.T) {
	for list, item := range map[string]string{
		"applications": "synthetic application", "subscriptions": "synthetic subscription", "API keys": "API key",
		"API keys for the synthetic REST API": "API key", "API keys of the synthetic application": "API key",
	} {
		_, err := listingFor(list, item, "")
		require.NoError(t, err, list)
	}
	_, err := listingFor("applications", "API key", "")
	require.ErrorContains(t, err, "hold no API key")
	_, err = listingFor("subscriptions", "synthetic subscription", "ACTIVE")
	require.ErrorContains(t, err, "only an API key is listed with a status")
	_, err = listingFor("webhooks", "API key", "")
	require.ErrorContains(t, err, "unsupported consumer list")
	_, steps, ctx, _ := consumerSteps(t)
	require.ErrorContains(t, steps.assertConsumerListing(ctx, "API keys", "not include", "API key", "REVOKED"), "only for an item the list should include")
}

func TestApplicationBody(t *testing.T) {
	created, err := applicationBody("syn-app-1", true, nil)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"id": "syn-app-1", "displayName": "Synthetic syn-app-1", "description": consumerResourceDescription}, created)
	updated, err := applicationBody("syn-app-1", false, map[string]string{"displayName": "Renamed"})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"displayName": "Renamed", "description": consumerResourceDescription}, updated)
	_, err = applicationBody("syn-app-1", false, map[string]string{"owner": "x"})
	require.ErrorContains(t, err, `unsupported application field "owner"`)
}

func TestRegeneratingASubscriptionTokenKeepsThePreviousOne(t *testing.T) {
	_, steps, ctx, _ := consumerSteps(t)
	require.NoError(t, steps.subscribeConsumer(ctx, "the"))
	first, err := cloudbase.RequiredValue(ctx, cloudbase.KeyConsumerSubscriptionToken)
	require.NoError(t, err)
	require.NoError(t, steps.regenerateConsumerSubscriptionToken(ctx))
	require.NoError(t, steps.assertConsumerSubscriptionToken(ctx, "replaced"))
	previous, err := cloudbase.RequiredValue(ctx, cloudbase.KeyPreviousSubscriptionToken)
	require.NoError(t, err)
	current, err := cloudbase.RequiredValue(ctx, cloudbase.KeyConsumerSubscriptionToken)
	require.NoError(t, err)
	require.Equal(t, first, previous)
	require.NotEqual(t, first, current)
}

func TestRegisteringKeyManagerPicksOneThatRegistersClients(t *testing.T) {
	id, err := registeringKeyManager([]map[string]any{
		{"id": "imported", "keyCreation": "import"}, {"id": "thunder", "keyCreation": "register"},
	})
	require.NoError(t, err)
	require.Equal(t, "thunder", id)
	_, err = registeringKeyManager([]map[string]any{{"id": "imported", "keyCreation": "import"}})
	require.ErrorContains(t, err, "none of the portal's 1 key managers registers clients")
}

// serveOAuth2Keys serves the fake portal's OAuth2 keys, registered with its one key manager.
func (a *fakeConsumerAPI) serveOAuth2Keys(w http.ResponseWriter, method string, rest []string, body map[string]any) {
	if a.oauth2 == nil {
		a.oauth2 = map[string]map[string]any{}
	}
	switch {
	case len(rest) == 0 && method == http.MethodPost:
		properties, _ := body["properties"].(map[string]any)
		if body["keyManagerId"] != "thunder" || properties["client_name"] == nil {
			cloudbase.WriteJSON(w, http.StatusBadRequest, map[string]any{"message": "keyManagerId and client_name are required"})
			return
		}
		a.seq++
		id := "oauth-" + strconv.Itoa(a.seq)
		a.oauth2[id] = map[string]any{"keyId": id, "name": properties["client_name"], "secret": "secret-" + strconv.Itoa(a.seq)}
		cloudbase.WriteJSON(w, a.statusOr(http.StatusCreated), map[string]any{"keyId": id, "name": properties["client_name"],
			"consumerKey": "client-" + strconv.Itoa(a.seq), "consumerSecret": a.oauth2[id]["secret"]})
	case len(rest) == 0 && method == http.MethodGet:
		list := []any{}
		for _, key := range a.oauth2 {
			list = append(list, map[string]any{"keyId": key["keyId"], "name": key["name"], "consumerSecret": ""})
		}
		cloudbase.WriteJSON(w, http.StatusOK, fakePage(a.request, list))
	case len(rest) == 0:
		writeMethodNotAllowed(w)
	default:
		key, known := a.oauth2[rest[0]]
		if !known {
			cloudbase.WriteJSON(w, http.StatusNotFound, map[string]any{"message": "OAuth2 key not found"})
			return
		}
		switch {
		case len(rest) == 1 && method == http.MethodDelete:
			delete(a.oauth2, rest[0])
			w.WriteHeader(http.StatusNoContent)
		case len(rest) == 2 && method != http.MethodPost:
			writeMethodNotAllowed(w)
		case len(rest) == 2 && rest[1] == "associate":
			key["applicationId"] = body["applicationId"]
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"application": map[string]any{"id": body["applicationId"]}})
		case len(rest) == 2 && rest[1] == "dissociate":
			delete(key, "applicationId")
			w.WriteHeader(http.StatusNoContent)
		case len(rest) == 2 && rest[1] == "generate-token":
			if body["consumerSecret"] != key["secret"] {
				cloudbase.WriteJSON(w, http.StatusBadRequest, map[string]any{"message": "The key manager refused these credentials."})
				return
			}
			cloudbase.WriteJSON(w, a.statusOr(http.StatusOK), map[string]any{"accessToken": "aGVhZA.Ym9keQ." + rest[0], "tokenType": "Bearer"})
		default:
			cloudbase.WriteJSON(w, http.StatusNotFound, map[string]any{"message": "Not Found"})
		}
	}
}

func TestConsumerOAuth2KeysIssueATokenAndFollowTheirApplication(t *testing.T) {
	site, steps, ctx, registry := consumerSteps(t)
	require.NoError(t, steps.createConsumerApplication(ctx))

	require.NoError(t, steps.createConsumerOAuth2Keys(ctx))
	_, err := httpx.Published(ctx)
	require.Error(t, err, "the answer that carries the client secret is never published")
	name, err := cloudbase.RequiredValue(ctx, keyConsumerOAuth2ClientName)
	require.NoError(t, err)
	require.Contains(t, scenarioPending(registry), "portal-oauth2-keys:"+name, "the client is registered for cleanup by its name")
	id, err := cloudbase.RequiredValue(ctx, keyConsumerOAuth2KeyID)
	require.NoError(t, err)
	require.NoError(t, steps.assertConsumerListing(ctx, "OAuth2 keys", "include", "OAuth2 keys", ""))
	require.NoError(t, steps.assertConsumerListing(ctx, "OAuth2 keys of the synthetic application", "not include", "OAuth2 keys", ""))

	require.NoError(t, steps.changeConsumerOAuth2KeysApplication(ctx, "associates the OAuth2 keys with"))
	require.Equal(t, http.StatusOK, publishedStatusOf(t, ctx))
	require.NoError(t, steps.assertConsumerListing(ctx, "OAuth2 keys of the synthetic application", "include", "OAuth2 keys", ""))

	require.NoError(t, steps.generateConsumerAccessToken(ctx))
	token, err := cloudbase.RequiredValue(ctx, cloudbase.KeyAccessToken)
	require.NoError(t, err)
	require.Equal(t, "aGVhZA.Ym9keQ."+id, token)

	require.NoError(t, steps.changeConsumerOAuth2KeysApplication(ctx, "dissociates the OAuth2 keys from"))
	require.Equal(t, http.StatusNoContent, publishedStatusOf(t, ctx))
	require.NoError(t, steps.assertConsumerListing(ctx, "OAuth2 keys of the synthetic application", "not include", "OAuth2 keys", ""))

	// A client the portal registered although its answer never arrived, under another name.
	site.consumer.oauth2["oauth-lost"] = map[string]any{"keyId": "oauth-lost", "name": name}
	site.consumer.oauth2["oauth-other"] = map[string]any{"keyId": "oauth-other", "name": "someone-else"}
	require.NoError(t, registry.Sweep(ctx))
	require.Equal(t, []string{"oauth-other"}, slices.Collect(maps.Keys(site.consumer.oauth2)),
		"cleanup deletes the scenario's clients, the lost one too, and no other")
}

func TestTheRunnersSweepDeletesTheConsumersOAuth2Keys(t *testing.T) {
	site, steps, ctx, registry := consumerSteps(t)
	require.NoError(t, steps.createConsumerOAuth2Keys(ctx))
	for _, resource := range registry.Pending() {
		if resource.Kind.Name != kindConsumerLeftovers.Name {
			registry.Deregister(resource.Kind, resource.ID)
		}
	}
	require.NoError(t, registry.SweepRunner(ctx))
	require.Empty(t, site.consumer.oauth2)
}

func TestCredentialAnswersWithAnUnexpectedStatusAreReportedWithoutTheirBody(t *testing.T) {
	site, steps, ctx, _ := consumerSteps(t)
	require.NoError(t, steps.generateConsumerAPIKey(ctx))
	require.NoError(t, steps.createConsumerOAuth2Keys(ctx))
	site.consumer.secretStatus = http.StatusAccepted

	for name, step := range map[string]func(context.Context) error{
		"generate an API key":      steps.generateConsumerAPIKey,
		"regenerate the API key":   steps.regenerateConsumerAPIKey,
		"create OAuth2 keys":       steps.createConsumerOAuth2Keys,
		"generate an access token": steps.generateConsumerAccessToken,
	} {
		err := step(ctx)
		require.ErrorContains(t, err, "-> 202", name)
		for _, secret := range []string{"value-", "secret-", "aGVhZA"} {
			require.NotContains(t, err.Error(), secret, "%s: an error never repeats a credential", name)
		}
	}
}

func TestPublishedSubscriptionAnswersCarryNoToken(t *testing.T) {
	site, steps, ctx, _ := consumerSteps(t)
	require.NoError(t, steps.subscribeConsumer(ctx, "the"))
	id, err := cloudbase.RequiredValue(ctx, keyConsumerSubscriptionID)
	require.NoError(t, err)
	token := site.consumer.subs[id][subscriptionTokenField].(string)

	published, err := httpx.Published(ctx)
	require.NoError(t, err)
	require.NotContains(t, string(published.Body), token)
	require.Contains(t, string(published.Body), redactedValue)
	err = steps.AssertCloudResponseStatus(ctx, http.StatusOK)
	require.Error(t, err)
	require.NotContains(t, err.Error(), token, "a failed status assertion never repeats the token")

	require.NoError(t, steps.assertConsumerListing(ctx, "subscriptions", "include", "synthetic subscription", ""))
	published, err = httpx.Published(ctx)
	require.NoError(t, err)
	require.NotContains(t, string(published.Body), token, "a list's tokens are redacted too")
	require.NoError(t, steps.readConsumerSubscription(ctx))
	require.NoError(t, steps.assertConsumerSubscriptionToken(ctx, "unchanged"), "the token assertion still sees the token")
}

func TestRedactSubscriptionTokensLeavesOtherAnswersAlone(t *testing.T) {
	for _, body := range []string{`not json`, `{"id":"syn-app-1"}`, `[1,2]`} {
		resp := &httpx.Response{StatusCode: http.StatusOK, Body: []byte(body)}
		redacted, token := redactSubscriptionTokens(resp)
		require.Empty(t, token, body)
		if json.Valid([]byte(body)) {
			require.JSONEq(t, body, string(redacted.Body))
		} else {
			require.Equal(t, body, string(redacted.Body), "an answer that is not JSON is passed through")
		}
	}
}

// fillApplicationsBefore adds n applications that the portal lists before the scenario's, which
// match it in every field the listing sorts on ahead of the id, and returns the scenario's
// application.
func fillApplicationsBefore(t *testing.T, site *fakePortalSite, ctx context.Context, n int) string {
	t.Helper()
	id, err := cloudbase.RequiredValue(ctx, keyConsumerApplicationID)
	require.NoError(t, err)
	target := site.consumer.apps[id]
	for i := range n {
		filler := "aaa-app-" + strconv.Itoa(1000+i)
		site.consumer.apps[filler] = map[string]any{"id": filler, "displayName": target["displayName"], "description": target["description"]}
	}
	return id
}

func TestConsumerListingReadsEveryPage(t *testing.T) {
	site, steps, ctx, _ := consumerSteps(t)
	require.NoError(t, steps.createConsumerApplication(ctx))
	id := fillApplicationsBefore(t, site, ctx, 150)
	all := make([]any, 0, len(site.consumer.apps))
	for _, app := range site.consumer.apps {
		all = append(all, app)
	}
	first := fakePage(httptest.NewRequest(http.MethodGet, "/applications?limit=100", nil), all)["list"].([]any)
	require.NotContains(t, first, site.consumer.apps[id], "the application is past the first page")
	require.NoError(t, steps.assertConsumerListing(ctx, "applications", "include", "synthetic application", ""),
		"the application is found on a later page")
	require.ErrorContains(t, steps.assertConsumerListing(ctx, "applications", "not include", "synthetic application", ""), "still include")
	require.NoError(t, steps.deleteConsumerApplication(ctx))
	require.NoError(t, steps.assertConsumerListing(ctx, "applications", "not include", "synthetic application", ""))
}

func TestSignInFailsWhenThePageItLandsOnFails(t *testing.T) {
	site := newFakePortalSite(t)
	steps, ctx := signInSteps(t, site)
	require.NoError(t, steps.signUpConsumer(ctx))
	site.landingStatus = http.StatusBadGateway
	require.ErrorContains(t, steps.SignInConsumer(ctx), "lands on answered 502")
}

func TestTheRunnersSweepRefusesAConsumerItCannotSignInAs(t *testing.T) {
	_, _, ctx, registry := consumerSteps(t)
	require.NoError(t, cleanup.RegisterRunner(ctx, cleanup.Resource{Kind: kindConsumerLeftovers, ID: "syn-consumer-earlier", Actor: consumerActor}))
	err := registry.SweepRunner(ctx)
	require.ErrorContains(t, err, `consumer "syn-consumer-earlier" were not swept`)
}

func TestConsumerCleanupSignsInAgainOnAStaleCSRFToken(t *testing.T) {
	site, steps, ctx, registry := consumerSteps(t)
	require.NoError(t, steps.createConsumerApplication(ctx))
	browser, err := scenarioBrowser(ctx)
	require.NoError(t, err)
	browser.jar.SetCookies(browser.portal, []*http.Cookie{{Name: portalCSRFCookie, Value: "stale", Path: "/api-portal"}})
	require.NoError(t, registry.Sweep(ctx))
	require.Empty(t, site.consumer.apps)
}

func TestASubscriptionOfAnotherAPIIsNotTheScenarios(t *testing.T) {
	_, steps, ctx, _ := consumerSteps(t)
	require.NoError(t, steps.subscribeConsumer(ctx, "the"))
	require.NoError(t, steps.readConsumerSubscription(ctx))

	require.NoError(t, tcontext.Set(ctx, cloudbase.KeyAPIID, "api-2"))
	require.ErrorContains(t, steps.readConsumerSubscription(ctx), "no subscription to the synthetic REST API api-2")
	require.NoError(t, steps.subscribeConsumer(ctx, "the"), "a refusal is left to the status assertion")
	_, err := cloudbase.RequiredValue(ctx, cloudbase.KeyConsumerSubscriptionToken)
	require.Error(t, err, "a refused subscribe drops the token of another API's subscription")
}

func TestConsumerCleanupRemovesWhatItListedBeforeAPageFailed(t *testing.T) {
	site, steps, ctx, registry := consumerSteps(t)
	require.NoError(t, steps.createConsumerApplication(ctx))
	fillApplicationsBefore(t, site, ctx, 150)
	site.consumer.failPageFrom = listPageSize
	require.NoError(t, registry.Sweep(ctx), "the scenario's application is deleted by its handle")
	err := consumerSession{signIn: steps.SignInConsumer}.removeApplications(ctx)
	require.ErrorContains(t, err, "-> 502")
	require.Len(t, site.consumer.apps, 50, "the first page is removed although the second failed")
}

func TestConsumerCleanupRepeatsARemovalThatGotNoAnswer(t *testing.T) {
	for name, status := range map[string]int{
		"a 502 answer":         http.StatusBadGateway,
		"a 429 answer":         http.StatusTooManyRequests,
		"a dropped connection": 0,
		"a connection dropped after the deletion took effect": commitThenDrop,
	} {
		t.Run(name, func(t *testing.T) {
			site, steps, ctx, registry := consumerSteps(t)
			require.NoError(t, steps.createConsumerApplication(ctx))
			require.Len(t, site.consumer.apps, 1)
			for id := range site.consumer.apps {
				site.consumer.failOnce = map[string]int{http.MethodDelete + " " + applicationPath(id): status}
			}
			site.consumer.calls = nil

			require.NoError(t, registry.Sweep(ctx))
			require.Empty(t, site.consumer.apps)
			require.Equal(t, []string{"DELETE applications", "DELETE applications"}, calledCollections(site.consumer.calls))
		})
	}
}

func TestConsumerCleanupReportsARefusedRemovalWithoutRepeatingIt(t *testing.T) {
	site, steps, ctx, registry := consumerSteps(t)
	require.NoError(t, steps.createConsumerApplication(ctx))
	require.Len(t, site.consumer.apps, 1)
	var id string
	for id = range site.consumer.apps {
		site.consumer.failOnce = map[string]int{http.MethodDelete + " " + applicationPath(id): http.StatusConflict}
	}
	site.consumer.calls = nil

	require.ErrorContains(t, registry.Sweep(ctx), "-> 409")
	require.Contains(t, site.consumer.apps, id)
	require.Equal(t, []string{"DELETE applications"}, calledCollections(site.consumer.calls))
}

func TestFailedInTransitTellsADroppedRequestFromOneThatCannotBeBuilt(t *testing.T) {
	_, parseErr := url.Parse("http://[::1")
	require.Error(t, parseErr)
	sent := &url.Error{Op: "Delete", URL: "https://portal.example/applications/a", Err: io.ErrUnexpectedEOF}

	require.True(t, failedInTransit(sent))
	require.True(t, failedInTransit(fmt.Errorf("httpx: DELETE https://portal.example/applications/a: %w", sent)))
	require.False(t, failedInTransit(parseErr))
	require.False(t, failedInTransit(errors.New("signed up no consumer")))
	require.False(t, failedInTransit(nil))
}

func TestOAuth2KeysAreDeletedByIDWhenTheirClientHasNoName(t *testing.T) {
	site, steps, ctx, registry := consumerSteps(t)
	require.NoError(t, steps.createConsumerOAuth2Keys(ctx))
	id, err := cloudbase.RequiredValue(ctx, keyConsumerOAuth2KeyID)
	require.NoError(t, err)
	require.Contains(t, scenarioPending(registry), "portal-oauth2-key:"+id)
	site.consumer.oauth2[id]["name"] = ""
	require.NoError(t, registry.Sweep(ctx))
	require.Empty(t, site.consumer.oauth2)
}

func TestCreateConsumerOAuth2KeysReadsEveryPageOfKeyManagers(t *testing.T) {
	site, steps, ctx, _ := consumerSteps(t)
	for i := range listPageSize {
		site.consumer.keyManagers = append(site.consumer.keyManagers, map[string]any{"id": "aaa-" + strconv.Itoa(100+i), "keyCreation": "import"})
	}
	site.consumer.keyManagers = append(site.consumer.keyManagers, map[string]any{"id": "thunder", "keyCreation": "register"})
	require.NoError(t, steps.createConsumerOAuth2Keys(ctx), "the key manager that registers clients is past the first page")
}

func TestTheSubscriptionTokenAssertionReadsOnlyTheLastAnswer(t *testing.T) {
	_, steps, ctx, _ := consumerSteps(t)
	require.NoError(t, steps.subscribeConsumer(ctx, "the"))
	require.NoError(t, steps.readConsumerSubscription(ctx))
	require.NoError(t, steps.assertConsumerSubscriptionToken(ctx, "unchanged"))

	require.NoError(t, steps.publishPortalAnswer(ctx, &httpx.Response{StatusCode: http.StatusOK, Body: []byte(`{"status":"UP"}`)}))
	require.ErrorContains(t, steps.assertConsumerSubscriptionToken(ctx, "unchanged"), "not a subscription answer",
		"an answer published after the subscription read is not compared with its token")
	require.NoError(t, steps.createConsumerApplication(ctx))
	require.ErrorContains(t, steps.assertConsumerSubscriptionToken(ctx, "unchanged"), "not a subscription answer",
		"a consumer answer without a token does not carry the earlier one")
}

func TestFailedInTransitRetriesOnlyWhatAnotherAttemptCanFix(t *testing.T) {
	dropped := &url.Error{Op: http.MethodDelete, URL: "https://portal.example/applications/a", Err: io.EOF}
	require.True(t, failedInTransit(dropped))
	require.True(t, failedInTransit(fmt.Errorf("httpx: reading the body of DELETE https://portal.example/applications/a: %w", io.ErrUnexpectedEOF)),
		"an answer whose body was cut off")
	require.True(t, failedInTransit(fmt.Errorf("read: %w", syscall.ECONNRESET)))
	for name, cause := range map[string]error{
		"an unknown authority":       x509.UnknownAuthorityError{},
		"an invalid certificate":     x509.CertificateInvalidError{Reason: x509.Expired},
		"another host's certificate": x509.HostnameError{Host: "portal.example"},
		"a failed verification":      &tls.CertificateVerificationError{Err: errors.New("bad chain")},
	} {
		refused := &url.Error{Op: http.MethodDelete, URL: "https://portal.example/applications/a", Err: cause}
		require.False(t, failedInTransit(refused), name)
		require.False(t, failedInTransit(fmt.Errorf("httpx: DELETE: %w", refused)), name)
	}
	require.False(t, failedInTransit(errors.New("httpx: response body of DELETE https://portal.example/applications/a exceeds the 1048576-byte limit")),
		"an answer over the size limit is the same the next time")
}

func TestConsumerCleanupStopsRepeatingARemovalAtItsDeadline(t *testing.T) {
	site, steps, ctx, _ := consumerSteps(t)
	require.NoError(t, steps.createConsumerApplication(ctx))
	id, err := cloudbase.RequiredValue(ctx, keyConsumerApplicationID)
	require.NoError(t, err)
	site.consumer.failEvery = map[string]int{http.MethodDelete + " " + applicationPath(id): http.StatusBadGateway}

	short, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	err = consumerSession{signIn: steps.SignInConsumer}.deleteApplication(short, id)
	require.ErrorContains(t, err, "last attempt")
	require.ErrorContains(t, err, "-> 502", "the failure names what the last attempt got")
}

func TestTheRunnersSweepRunsAgainAfterAFailedSweep(t *testing.T) {
	site, steps, ctx, registry := consumerSteps(t)
	require.NoError(t, steps.createConsumerApplication(ctx))
	site.consumer.failOnce = map[string]int{http.MethodGet + " /applications": http.StatusBadGateway}
	site.consumer.calls = nil

	require.NoError(t, registry.SweepRunner(ctx), "the second sweep lists and removes what the first could not")
	require.Empty(t, site.consumer.apps)
}

func TestTheRunnersSweepReportsWhatStaysAfterItsLastAttempt(t *testing.T) {
	site, steps, ctx, registry := consumerSteps(t)
	require.NoError(t, steps.createConsumerApplication(ctx))
	site.consumer.failEvery = map[string]int{http.MethodGet + " /applications": http.StatusBadGateway}
	site.consumer.calls = nil

	err := registry.SweepRunner(ctx)
	require.ErrorContains(t, err, "after 3 sweeps")
	listings := 0
	for _, call := range site.consumer.calls {
		if call == http.MethodGet+" /applications" {
			listings++
		}
	}
	require.Equal(t, leftoverSweepAttempts, listings, "each sweep lists the applications once")
	require.Len(t, site.consumer.apps, 1)
}

// cloudSteps wires the area's steps to a stub cloud API.
func cloudSteps(t *testing.T, handler http.HandlerFunc) (*Steps, context.Context, *tcontext.Local, *cleanup.Registry) {
	t.Helper()
	base, ctx, local, registry := cloudbase.CloudSteps(t, handler)
	return &Steps{Steps: base}, ctx, local, registry
}

func TestAPublicationActionIsSentAgainWhileThePlatformCannotReachThePortal(t *testing.T) {
	previousInterval, previousTimeout := publicationRetryInterval, publicationRetryTimeout
	publicationRetryInterval, publicationRetryTimeout = retry.BaseInterval, 300*time.Millisecond
	t.Cleanup(func() { publicationRetryInterval, publicationRetryTimeout = previousInterval, previousTimeout })

	for name, tc := range map[string]struct {
		answers      []int
		code         string
		wantStatus   int
		wantRequests int
	}{
		"an unreachable portal is asked again until it answers": {[]int{503, 503, 201}, portalUnavailableCode, http.StatusCreated, 3},
		"a refusal is published at once, not asked again":       {[]int{409}, "PUBLICATION_STATE_CONFLICT", http.StatusConflict, 1},
		"another unavailable answer is not asked again":         {[]int{503}, "SOMETHING_ELSE", http.StatusServiceUnavailable, 1},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
			requests := 0
			steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
				status := tc.answers[min(requests, len(tc.answers)-1)]
				requests++
				body := map[string]string{"code": tc.code}
				if status == http.StatusCreated {
					body = map[string]string{"status": "published"}
				}
				cloudbase.WriteJSON(w, status, body)
			})
			local.Set(keyAPIPortalID, "portal-1")
			local.Set(cloudbase.KeyAPIID, "api-1")

			require.NoError(t, steps.publicationAction(ctx, "publish", "synthetic REST API"))
			published, err := httpx.Published(ctx)
			require.NoError(t, err)
			require.Equal(t, tc.wantStatus, published.StatusCode)
			require.Equal(t, tc.wantRequests, requests)
		})
	}
}

func TestAPublicationActionStopsAskingWhenThePortalStaysUnreachable(t *testing.T) {
	previousInterval, previousTimeout := publicationRetryInterval, publicationRetryTimeout
	publicationRetryInterval, publicationRetryTimeout = retry.BaseInterval, 200*time.Millisecond
	t.Cleanup(func() { publicationRetryInterval, publicationRetryTimeout = previousInterval, previousTimeout })
	if testing.Short() {
		t.Skip("waits out the retry package's minimum deadline of a minute")
	}
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		cloudbase.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"code": portalUnavailableCode})
	})
	local.Set(keyAPIPortalID, "portal-1")
	local.Set(cloudbase.KeyAPIID, "api-1")

	require.NoError(t, steps.publicationAction(ctx, "publish", "synthetic REST API"), "the answer is left for the scenario to assert")
	require.ErrorContains(t, steps.AssertCloudResponseStatus(ctx, http.StatusCreated), "503")
}
