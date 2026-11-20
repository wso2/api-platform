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

package selfhosted

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

func TestCheckAwaitingConnection(t *testing.T) {
	valid := func() map[string]any {
		return map[string]any{
			"id": "development-shgw-1", "environment": "development",
			"isActive": false, "endpoints": []any{pendingEndpoint},
		}
	}
	require.NoError(t, checkAwaitingConnection(valid(), "development-shgw-1", "development"))

	for name, mutate := range map[string]func(map[string]any){
		"wrong id":           func(d map[string]any) { d["id"] = "other" },
		"wrong environment":  func(d map[string]any) { d["environment"] = "production" },
		"active":             func(d map[string]any) { d["isActive"] = true },
		"isActive missing":   func(d map[string]any) { delete(d, "isActive") },
		"real endpoint":      func(d map[string]any) { d["endpoints"] = []any{"https://gw.example.com"} },
		"extra endpoint":     func(d map[string]any) { d["endpoints"] = []any{pendingEndpoint, "https://gw.example.com"} },
		"endpoints missing":  func(d map[string]any) { delete(d, "endpoints") },
		"isActive as string": func(d map[string]any) { d["isActive"] = "false" },
	} {
		document := valid()
		mutate(document)
		require.Error(t, checkAwaitingConnection(document, "development-shgw-1", "development"), name)
	}
}

func TestMintedTokenIDNeverQuotesTheBody(t *testing.T) {
	id, err := mintedTokenID([]byte(`{"id":"tok-1","token":"plain-text-secret"}`))
	require.NoError(t, err)
	require.Equal(t, "tok-1", id)

	for _, body := range []string{
		`{"token":"plain-text-secret"}`,
		`{"id":"tok-1","token":""}`,
		`plain-text-secret`,
	} {
		_, err := mintedTokenID([]byte(body))
		require.Error(t, err, body)
		require.NotContains(t, err.Error(), "plain-text-secret")
	}
}

func TestActiveTokenIDsFollowsTheConsoleRule(t *testing.T) {
	require.Equal(t, []string{"active-1", "no-status"}, activeTokenIDs([]map[string]any{
		{"id": "active-1", "status": "active"},
		{"id": "no-status"},
		{"id": "revoked-1", "status": "revoked"},
		{"id": "", "status": "active"},
		{"status": "active"},
	}))
	require.Empty(t, activeTokenIDs(nil))
}

func TestStillActive(t *testing.T) {
	require.Equal(t, []string{"old-2"}, stillActive([]string{"old-1", "old-2"}, []string{"new-1", "old-2"}))
	require.Empty(t, stillActive([]string{"old-1"}, []string{"new-1"}))
}

func TestSelfHostedGatewayPathsEscapeID(t *testing.T) {
	require.Equal(t, "/self-hosted-gateways/id%2Fwith%20separator", selfHostedGatewayPath("id/with separator"))
	require.Equal(t, "/self-hosted-gateways/gw-1/tokens", selfHostedGatewayTokensPath("gw-1"))
}

func TestRemoveUnregisteredSelfHostedGatewayDeletesItAndReportsWhenItMayRemain(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	for _, tc := range []struct {
		status  int
		wantErr bool
	}{
		{status: http.StatusNoContent},
		{status: http.StatusNotFound},
		{status: http.StatusInternalServerError, wantErr: true},
	} {
		var deleted []string
		topo := cloudbase.CloudAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodDelete, r.Method)
			deleted = append(deleted, r.URL.Path)
			w.WriteHeader(tc.status)
		})
		steps := &Steps{Steps: &cloudbase.Steps{Topo: topo, Client: httpx.NewClient(httpx.Options{Timeout: 5 * time.Second})}}
		ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))

		err := steps.removeUnregisteredSelfHostedGateway(ctx, "development-shgw-1")
		require.Equal(t, []string{"/self-hosted-gateways/development-shgw-1"}, deleted, "status %d", tc.status)
		if tc.wantErr {
			require.ErrorContains(t, err, `"development-shgw-1"`)
		} else {
			require.NoError(t, err, "status %d", tc.status)
		}
	}
}

func TestControlPlaneHostIsTheConnectEndpointWithoutItsScheme(t *testing.T) {
	host, err := controlPlaneHost("https://cloud-console-development.gateway.dev.example.test/api-management/connect/")
	require.NoError(t, err)
	require.Equal(t, "cloud-console-development.gateway.dev.example.test/api-management/connect", host)
	for _, endpoint := range []string{
		"http://cloud-console.example.test/api-management/connect",
		"https://user:pass@cloud-console.example.test/api-management/connect",
		"https://cloud-console.example.test/api-management/connect?x=1",
		"/api-management/connect",
		"",
	} {
		_, err := controlPlaneHost(endpoint)
		require.Error(t, err, endpoint)
	}
}

func TestLocalGatewayEnvVerifiesTheControlPlaneCertificate(t *testing.T) {
	require.Equal(t, map[string]string{
		"APIP_GW_CONTROLLER_CONTROLPLANE_HOST":                 "console.example.test/api-management/connect",
		"APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN":                "registration-token",
		"APIP_GW_CONTROLLER_CONTROLPLANE_INSECURE_SKIP_VERIFY": "false",
	}, localGatewayEnv("console.example.test/api-management/connect", "registration-token"))
}

func TestParseMintedTokenReturnsTheValueWithoutQuotingTheBody(t *testing.T) {
	id, token, err := parseMintedToken([]byte(`{"id":"tok-1","token":" secret-value "}`))
	require.NoError(t, err)
	require.Equal(t, "tok-1", id)
	require.Equal(t, "secret-value", token)
	_, _, err = parseMintedToken([]byte(`{"token":"secret-value"}`))
	require.ErrorContains(t, err, "no token id")
	require.NotContains(t, err.Error(), "secret-value")
}

// fakeStack is a local gateway stack whose stop fails with err.
type fakeStack struct {
	stops int
	err   error
}

func (f *fakeStack) Stop(context.Context) error {
	f.stops++
	return f.err
}

// runnerWithLocalGateway returns a runner context holding a running local gateway on origin.
func runnerWithLocalGateway(t *testing.T, runner, suffix, origin string, stack cloudbase.Stopper) context.Context {
	t.Helper()
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal(runner))
	registry, err := cloudbase.LocalGatewaysOf(ctx, true)
	require.NoError(t, err)
	registry.Add(suffix, stack)
	registry.Approve(suffix, origin)
	return ctx
}

func TestOnlyTheOriginOfAStartedLocalGatewayIsAnApprovedDataPlaneTarget(t *testing.T) {
	stack := &fakeStack{}
	ctx := runnerWithLocalGateway(t, "runner-a", "-shgw-1", "http://localhost:49153", stack)

	target, err := cloudbase.ResolveCloudDataPlaneURL(ctx, "http://localhost:49153/api-1/v1/posts/1")
	require.NoError(t, err)
	require.Equal(t, "http://localhost:49153/api-1/v1/posts/1", target)
	for _, refused := range []string{
		"http://localhost:49154/api-1/v1/posts/1",
		"https://localhost:49153/api-1/v1/posts/1",
		"http://user@localhost:49153/api-1/v1/posts/1",
		"http://127.0.0.1:49153/api-1/v1/posts/1",
	} {
		_, err := cloudbase.ResolveCloudDataPlaneURL(ctx, refused)
		require.Error(t, err, refused)
	}

	other := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner-b"))
	_, err = cloudbase.ResolveCloudDataPlaneURL(other, "http://localhost:49153/api-1/v1/posts/1")
	require.Error(t, err, "another runner's gateway is not approved")

	require.NoError(t, stopLocalGateway(ctx, "-shgw-1"))
	require.Equal(t, 1, stack.stops)
	_, err = cloudbase.ResolveCloudDataPlaneURL(ctx, "http://localhost:49153/api-1/v1/posts/1")
	require.Error(t, err, "a stopped gateway's origin is no longer approved")
}

func TestALocalGatewayThatFailsToStopStaysApproved(t *testing.T) {
	stack := &fakeStack{err: errors.New("compose down failed")}
	ctx := runnerWithLocalGateway(t, "runner-a", "-shgw-1", "http://localhost:49153", stack)

	require.ErrorContains(t, stopLocalGateway(ctx, "-shgw-1"), "compose down failed")
	_, err := cloudbase.ResolveCloudDataPlaneURL(ctx, "http://localhost:49153/api-1/v1/posts/1")
	require.NoError(t, err, "it may still be serving")

	stack.err = nil
	require.NoError(t, LocalGatewayDeleter(ctx, cleanup.Resource{Kind: KindLocalGateway, ID: "-shgw-1"}))
	require.Equal(t, 2, stack.stops, "cleanup stops it again")
}

func TestSelfHostedGatewayStepsUseTheLocalGatewayAsTheActiveGateway(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	steps := &Steps{Steps: &cloudbase.Steps{}}
	require.ErrorContains(t, steps.selectActiveGatewayEnvironment(ctx), "find an active APIP gateway first")

	local, _ := tcontext.LocalOf(ctx)
	local.Set(cloudbase.KeyGatewayID, "managed-gw")
	local.Set(cloudbase.KeyGatewayEnv, "development")
	local.Set(cloudbase.KeyGatewayEndpoint, "https://managed.example.test")
	require.NoError(t, steps.selectActiveGatewayEnvironment(ctx))
	environment, err := cloudbase.RequiredValue(ctx, cloudbase.KeyEnvironmentName)
	require.NoError(t, err)
	require.Equal(t, "development", environment)

	local.Set(keySelfHostedGatewayID, "development-syn-shgw-1")
	require.ErrorContains(t, steps.useLocalGatewayAsActive(ctx), "start a local APIP gateway first")
	local.Set(keyLocalGatewayEndpoint, "http://localhost:49153")
	require.NoError(t, steps.useLocalGatewayAsActive(ctx))
	for key, want := range map[string]string{
		cloudbase.KeyGatewayID: "development-syn-shgw-1", cloudbase.KeyGatewayEnv: "development", cloudbase.KeyGatewayEndpoint: "http://localhost:49153",
	} {
		got, err := cloudbase.RequiredValue(ctx, key)
		require.NoError(t, err, key)
		require.Equal(t, want, got, key)
	}
}

func TestAwaitSelfHostedGatewayStateWaitsForTheReportedState(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	reads := 0
	topo := cloudbase.CloudAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/self-hosted-gateways/development-syn-shgw-1", r.URL.Path)
		reads++
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "development-syn-shgw-1", "isActive": reads > 1})
	})
	steps := &Steps{Steps: &cloudbase.Steps{Topo: topo, Client: httpx.NewClient(httpx.Options{Timeout: 5 * time.Second})}}
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	local, _ := tcontext.LocalOf(ctx)
	local.Set(keySelfHostedGatewayID, "development-syn-shgw-1")

	require.NoError(t, steps.awaitSelfHostedGatewayState(ctx, "active"))
	require.Equal(t, 2, reads, "the first, inactive answer is not accepted")

	_, err := selfHostedGatewayActive([]byte(`{"id":"development-syn-shgw-1"}`))
	require.ErrorContains(t, err, "no boolean isActive")
}

func TestStoppingALocalGatewayThatIsNotRunningSucceeds(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	require.NoError(t, stopLocalGateway(ctx, "-shgw-unknown"))
	require.NoError(t, LocalGatewayDeleter(ctx, cleanup.Resource{Kind: KindLocalGateway, ID: "-shgw-unknown"}))
	require.Less(t, KindLocalGateway.Order, KindSelfHostedGateway.Order, "the local gateway stops before its registration is deleted")
	require.Greater(t, KindLocalGateway.Order, cleanup.KindAPI.Order, "the APIs deployed to it are deleted while it still runs")
}
