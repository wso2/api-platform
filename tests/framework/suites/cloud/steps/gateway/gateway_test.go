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

package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/require"
	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

func TestDeployToGatewayRejectsAResponseThatListsNoDeploymentOnTheGateway(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	body := `{"list":[{"gatewayId":"other","deploymentId":"d"},{"gatewayId":"gw","deploymentId":""}]}`
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(body))
	})
	local.Set(cloudbase.KeyProjectID, "default")

	_, err := steps.deployToGateway(ctx, "api", "development", "gw")
	require.ErrorContains(t, err, `no deployment on gateway "gw"`)

	body = `not json`
	_, err = steps.deployToGateway(ctx, "api", "development", "gw")
	require.ErrorContains(t, err, "decoding the deployment response")
}

func listedManagedGateway(id, environment, status string, active bool) map[string]any {
	return map[string]any{
		"id": id, "environment": environment, "status": status, "isActive": active,
		"isCritical": false, "isDefault": false,
	}
}

func TestBuildManagedGatewayRequest(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	local, _ := tcontext.LocalOf(ctx)

	_, err := buildManagedGatewayRequest(ctx, map[string]string{"environment": "dev"})
	require.ErrorContains(t, err, "no gateway name chosen")
	local.Set(keyManagedGatewayName, "syn-gw-1")
	local.Set("cloudGatewayEnvironment", "dev")

	body, err := buildManagedGatewayRequest(ctx, map[string]string{
		"environment": "${CTX:cloudGatewayEnvironment}", "isCritical": "true", "properties": `{"owner":"it"}`,
		"version": "1.0",
	})
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"displayName": "syn-gw-1", "environment": "dev", "isCritical": true,
		"properties": map[string]any{"owner": "it"}, "version": "1.0", "description": cloudbase.ScenarioGatewayDescription,
	}, body, "a gateway a scenario creates is marked as the suite's own")
	named, err := buildManagedGatewayRequest(ctx, map[string]string{"environment": "dev", "description": "mine"})
	require.NoError(t, err)
	require.Equal(t, "mine", named["description"], "a description the scenario names is kept")

	for name, values := range map[string]map[string]string{
		"no environment":     {"displayName": "x"},
		"unknown key":        {"environment": "dev", "host": "x"},
		"bad bool":           {"environment": "dev", "isDefault": "maybe"},
		"bad properties":     {"environment": "dev", "properties": "[1]"},
		"unresolved context": {"environment": "${CTX:nothing}"},
	} {
		_, err := buildManagedGatewayRequest(ctx, values)
		require.Error(t, err, name)
	}
}

func TestGatewayListingSteps(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{
			listedManagedGateway("dev-gw", "dev", "ACTIVE", true),
			listedManagedGateway("dev-other", "dev", "DISCONNECTED", false),
		}})
	})
	local.Set(cloudbase.KeyGatewayID, "dev-gw")

	require.NoError(t, steps.listManagedGateways(ctx))
	require.NoError(t, steps.assertListingIsValid(ctx))
	require.NoError(t, steps.assertListingIncludesActiveGateway(ctx))

	local.Set(cloudbase.KeyGatewayID, "dev-missing")
	require.Error(t, steps.assertListingIncludesActiveGateway(ctx))
}

func TestValidateGatewayListing(t *testing.T) {
	good := []map[string]any{
		listedManagedGateway("dev-default", "dev", "ACTIVE", true),
		listedManagedGateway("dev-new", "dev", "PROVISIONING", false),
		listedManagedGateway("dev-built", "dev", "DISCONNECTED", false),
	}
	good[1]["endpoints"] = []any{"https://gw.example.com"}
	good[1]["functionalityType"] = "ai"
	require.NoError(t, validateGatewayListing(good))

	failed := listedManagedGateway("dev-failed", "dev", "FAILED", false)
	require.ErrorContains(t, validateGatewayListing([]map[string]any{failed}), "without a status reason")
	failed["statusReason"] = "The gateway could not be provisioned."
	require.NoError(t, validateGatewayListing([]map[string]any{failed}))

	require.ErrorContains(t, validateGatewayListing(nil), "lists no managed gateway")
	for name, mutate := range map[string]func(map[string]any){
		"no id":              func(g map[string]any) { delete(g, "id") },
		"no environment":     func(g map[string]any) { g["environment"] = " " },
		"unknown status":     func(g map[string]any) { g["status"] = "" },
		"missing flag":       func(g map[string]any) { delete(g, "isDefault") },
		"reason while built": func(g map[string]any) { g["statusReason"] = "boom" },
		"connected but idle": func(g map[string]any) { g["isActive"] = true; g["status"] = "DISCONNECTED" },
		"active but idle":    func(g map[string]any) { g["status"] = "ACTIVE" },
		"unknown kind":       func(g map[string]any) { g["functionalityType"] = "mcp" },
		"endpoints not list": func(g map[string]any) { g["endpoints"] = "https://gw.example.com" },
		"blank endpoint":     func(g map[string]any) { g["endpoints"] = []any{" "} },
	} {
		gateway := listedManagedGateway("dev-x", "dev", "DISCONNECTED", false)
		mutate(gateway)
		require.Error(t, validateGatewayListing([]map[string]any{gateway}), name)
	}
}

func TestValidateGatewayListingShowsTheItemWithoutAStatus(t *testing.T) {
	err := validateGatewayListing([]map[string]any{{
		"id": "dev-orphan", "environment": "dev", "isActive": false, "isCritical": false, "isDefault": false, "status": "",
	}})
	require.ErrorContains(t, err, "the listing gives it as")
	require.ErrorContains(t, err, "dev-orphan")
}

func TestAssertSelectedEnvironmentListed(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	environments := []map[string]any{{"id": "dev", "name": "dev", "isProduction": false}}
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"list": environments})
	})
	require.Error(t, steps.assertSelectedEnvironmentListed(ctx), "nothing was selected")
	local.Set(cloudbase.KeyEnvironmentName, "dev")
	require.NoError(t, steps.assertSelectedEnvironmentListed(ctx))
	environments[0]["id"] = "other"
	require.ErrorContains(t, steps.assertSelectedEnvironmentListed(ctx), "own name")
	environments[0]["id"] = "dev"
	delete(environments[0], "isProduction")
	require.ErrorContains(t, steps.assertSelectedEnvironmentListed(ctx), "isProduction")
	local.Set(cloudbase.KeyEnvironmentName, "absent")
	require.ErrorContains(t, steps.assertSelectedEnvironmentListed(ctx), "not in the platform's list")
}

func TestRequestManagedGatewayReconcilesTheRegisteredID(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		cloudbase.WriteJSON(w, http.StatusCreated, map[string]string{"id": "assigned-by-core"})
	})
	local.Set(keyManagedGatewayName, "syn-gw-1")
	require.NoError(t, steps.requestManagedGatewayWith(ctx, map[string]string{"environment": "dev"}))
	pending := registry.Pending()
	require.Len(t, pending, 1)
	require.Equal(t, "assigned-by-core", pending[0].ID)
}

func TestRequestManagedGatewayRegistersASuffixedIDTheRepeatWasAnsweredWith(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var answers []string
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		cloudbase.WriteJSON(w, http.StatusCreated, map[string]string{"id": answers[0]})
		answers = answers[1:]
	})
	local.Set(keyManagedGatewayName, "syn-gw-1")
	values := map[string]string{"environment": "dev"}
	answers = []string{"dev-syn-gw-1", "dev-syn-gw-1-ow9j"}
	require.NoError(t, steps.requestManagedGatewayWith(ctx, values))
	require.NoError(t, steps.requestManagedGatewayWith(ctx, values))

	var pending []string
	for _, resource := range registry.Pending() {
		pending = append(pending, resource.ID)
	}
	require.ElementsMatch(t, []string{"dev-syn-gw-1", "dev-syn-gw-1-ow9j"}, pending, "the second gateway must not be left behind")
}

func TestRequestManagedGatewayOnceItsNameIsFreeDeletesTheSuffixedGatewayAndTriesAgain(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	gatewayNameReleaseInterval = time.Millisecond
	defer func() { gatewayNameReleaseInterval = 30 * time.Second }()
	answers := []string{"dev-syn-gw-1-ow9j", "dev-syn-gw-1"}
	var deleted []string
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = append(deleted, r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:])
			w.WriteHeader(http.StatusNoContent)
			return
		}
		cloudbase.WriteJSON(w, http.StatusCreated, map[string]string{"id": answers[0]})
		answers = answers[1:]
	})
	local.Set(keyManagedGatewayName, "syn-gw-1")

	require.NoError(t, steps.requestManagedGatewayOnceItsNameIsFreeWith(ctx, map[string]string{"environment": "dev"}))

	require.Equal(t, []string{"dev-syn-gw-1-ow9j"}, deleted, "only the gateway under the suffixed id is deleted")
	var pending []string
	for _, resource := range registry.Pending() {
		pending = append(pending, resource.ID)
	}
	require.Equal(t, []string{"dev-syn-gw-1"}, pending)
}

func TestRequestManagedGatewayRegistersForCleanupAndKeepsOnlyWhatWasCreated(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	status := http.StatusCreated
	var bodies []map[string]any
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		if status != http.StatusCreated {
			cloudbase.WriteJSON(w, status, map[string]string{"code": "conflict", "message": "no"})
			return
		}
		cloudbase.WriteJSON(w, status, map[string]string{"id": body["environment"].(string) + "-" + body["displayName"].(string)})
	})
	local.Set(keyManagedGatewayName, "syn-gw-1")
	values := map[string]string{"environment": "dev"}

	require.NoError(t, steps.requestManagedGatewayWith(ctx, values))
	require.Len(t, registry.Pending(), 1)

	// A repeat that is refused must not drop the registration the first request made.
	status = http.StatusConflict
	require.NoError(t, steps.requestManagedGatewayWith(ctx, values))
	require.Len(t, registry.Pending(), 1, "the first gateway stays registered")

	// A refused request for a different gateway leaves nothing registered.
	local.Set(keyManagedGatewayName, "syn-gw-2")
	require.NoError(t, steps.requestManagedGatewayWith(ctx, values))
	require.Len(t, registry.Pending(), 1)
	require.Len(t, bodies, 3)

	// A repeat that succeeds again is one gateway, registered once.
	status = http.StatusCreated
	local.Set(keyManagedGatewayName, "syn-gw-1")
	require.NoError(t, steps.requestManagedGatewayWith(ctx, values))
	require.Len(t, registry.Pending(), 1)

	// Once the gateway was deleted and deregistered, creating the name again registers it again.
	registry.Deregister(cleanup.KindGateway, "dev-syn-gw-1")
	require.NoError(t, steps.requestManagedGatewayWith(ctx, values))
	require.Len(t, registry.Pending(), 1)
}

func TestAttemptSpecs(t *testing.T) {
	specs, values, err := attemptSpecs(cloudbase.ValuesTable(t,
		[]string{"path", "value", "message"},
		[]string{"a.b", "5", " must be low "},
		[]string{"a.b", `"5"`, ""}))
	require.NoError(t, err)
	require.Equal(t, []configAttempt{{Path: "a.b", Want: "must be low"}, {Path: "a.b"}}, specs)
	require.Equal(t, []any{float64(5), "5"}, values)

	specs, _, err = attemptSpecs(cloudbase.ValuesTable(t, []string{"path", "value"}, []string{"a", "1"}))
	require.NoError(t, err)
	require.Equal(t, []configAttempt{{Path: "a"}}, specs)

	for name, table := range map[string]*godog.Table{
		"no table":     nil,
		"header only":  cloudbase.ValuesTable(t, []string{"path", "value"}),
		"wrong header": cloudbase.ValuesTable(t, []string{"key", "value"}, []string{"a", "1"}),
		"third header": cloudbase.ValuesTable(t, []string{"path", "value", "why"}, []string{"a", "1", "x"}),
		"short row":    cloudbase.ValuesTable(t, []string{"path", "value", "message"}, []string{"a", "1"}),
		"no path":      cloudbase.ValuesTable(t, []string{"path", "value"}, []string{" ", "1"}),
	} {
		_, _, err := attemptSpecs(table)
		require.Error(t, err, name)
	}
}

func TestJudgeRejection(t *testing.T) {
	good := configAttempt{Path: "a.b", Status: 400, Code: "invalid_request", Message: "a.b must be low", Want: "must be low"}
	require.Empty(t, judgeRejection(good))
	for name, mutate := range map[string]func(*configAttempt){
		"accepted":       func(a *configAttempt) { a.Status = 200 },
		"server fault":   func(a *configAttempt) { a.Status = 500 },
		"wrong code":     func(a *configAttempt) { a.Code = "conflict" },
		"unnamed":        func(a *configAttempt) { a.Message = "must be low" },
		"other message":  func(a *configAttempt) { a.Message = "a.b must be high" },
		"empty response": func(a *configAttempt) { *a = configAttempt{Path: "a.b", Status: 400} },
	} {
		attempt := good
		mutate(&attempt)
		require.NotEmpty(t, judgeRejection(attempt), name)
	}
	good.Want = ""
	good.Message = "a.b is wrong"
	require.Empty(t, judgeRejection(good), "no expected fragment means any message that names the setting")
}

func TestValidateConfigurationDocument(t *testing.T) {
	encode := func(document map[string]any) []byte {
		raw, err := json.Marshal(document)
		require.NoError(t, err)
		return raw
	}
	require.NoError(t, validateConfigurationDocument(encode(configurationBody()), "dev-gw"))
	require.ErrorContains(t, validateConfigurationDocument(encode(configurationBody()), "other"), "want \"other\"")
	require.Error(t, validateConfigurationDocument([]byte("nope"), "dev-gw"))

	for name, mutate := range map[string]func(map[string]any){
		"no environment": func(d map[string]any) { d["environment"] = "" },
		"no values":      func(d map[string]any) { delete(d, "values") },
		"unknown phase":  func(d map[string]any) { d["status"] = map[string]any{"phase": "ready"} },
		"bad timestamp": func(d map[string]any) {
			d["status"] = map[string]any{"phase": "healthy", "lastTransitionTime": "yesterday"}
		},
		"no editable":        func(d map[string]any) { d["editable"] = []any{} },
		"value not editable": func(d map[string]any) { d["values"] = map[string]any{"gateway.hidden": 1} },
		"null value":         func(d map[string]any) { d["values"] = map[string]any{"gateway.config.router.access_logs.enabled": nil} },
		"duplicate path":     func(d map[string]any) { d["editable"] = append(d["editable"].([]any), d["editable"].([]any)[0]) },
		"unknown constraint": func(d map[string]any) { d["constraints"] = []any{map[string]any{"type": "equal"}} },
		"constraint off list": func(d map[string]any) {
			d["constraints"] = []any{map[string]any{"type": "notGreaterThan", "path": "x", "than": "y"}}
		},
		"unknown setting type": func(d map[string]any) { d["editable"] = []any{map[string]any{"path": "p", "type": "float"}} },
		"enum without values":  func(d map[string]any) { d["editable"] = []any{map[string]any{"path": "p", "type": "enum"}} },
		"integer without max": func(d map[string]any) {
			d["editable"] = []any{map[string]any{"path": "p", "type": "integer", "min": "1"}}
		},
		"string without max":    func(d map[string]any) { d["editable"] = []any{map[string]any{"path": "p", "type": "string"}} },
		"editable without path": func(d map[string]any) { d["editable"] = []any{map[string]any{"type": "boolean"}} },
	} {
		document := configurationBody()
		mutate(document)
		require.Error(t, validateConfigurationDocument(encode(document), "dev-gw"), name)
	}
}

func TestReadAndValidateGatewayConfiguration(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	status := http.StatusOK
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/managed-gateways/dev-gw/configuration", r.URL.Path)
		cloudbase.WriteJSON(w, status, configurationBody())
	})
	require.Error(t, steps.readGatewayConfiguration(ctx), "no active gateway was found")
	local.Set(cloudbase.KeyGatewayID, "dev-gw")
	require.NoError(t, steps.readGatewayConfiguration(ctx))
	require.NoError(t, steps.assertConfigurationDocument(ctx))

	local.Set(keyConfigGatewayID, "another")
	require.ErrorContains(t, steps.assertConfigurationDocument(ctx), "another")

	status = http.StatusNotFound
	require.NoError(t, steps.readGatewayConfiguration(ctx))
	require.Error(t, steps.assertConfigurationDocument(ctx))
}

func TestAwaitConfigurationReadable(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var reads int
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		reads++
		if reads < 2 {
			cloudbase.WriteJSON(w, http.StatusNotFound, map[string]string{"code": "not_found"})
			return
		}
		cloudbase.WriteJSON(w, http.StatusOK, configurationBody())
	})
	require.Error(t, steps.awaitConfigurationReadable(ctx), "no active gateway was found")
	local.Set(cloudbase.KeyGatewayID, "dev-gw")
	require.NoError(t, steps.awaitConfigurationReadable(ctx))
	require.Equal(t, 2, reads, "a gateway whose release does not exist yet is waited for")
}

func TestWriteLongString(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var bodies []map[string]map[string]any
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		cloudbase.WriteJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request", "message": "no"})
	})
	require.Error(t, steps.writeLongString(ctx, "gateway.config_toml", 8193), "no active gateway was found")
	local.Set(cloudbase.KeyGatewayID, "dev-gw")
	require.NoError(t, steps.writeLongString(ctx, "gateway.config_toml", 8193))
	require.Len(t, bodies[0]["values"]["gateway.config_toml"], 8193)
	require.NoError(t, steps.AssertCloudResponseStatus(ctx, http.StatusBadRequest))
	require.Error(t, steps.writeLongString(ctx, "gateway.config_toml", 0))
}

func TestWriteEachValueRecordsEveryAnswer(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var sent []map[string]any
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/managed-gateways/dev-gw/configuration", r.URL.Path)
		var body map[string]map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		sent = append(sent, body["values"])
		if _, accepted := body["values"]["gateway.ok"]; accepted {
			cloudbase.WriteJSON(w, http.StatusOK, configurationBody())
			return
		}
		cloudbase.WriteJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request", "message": "gateway.bad must be low"})
	})
	local.Set(cloudbase.KeyGatewayID, "dev-gw")
	table := cloudbase.ValuesTable(t, []string{"path", "value", "message"}, []string{"gateway.bad", "5", "must be low"}, []string{"gateway.bad", `"5"`, ""})
	require.NoError(t, steps.writeEachValue(ctx, table))
	require.Equal(t, []map[string]any{{"gateway.bad": float64(5)}, {"gateway.bad": "5"}}, sent, "each value is a write of its own")
	require.NoError(t, steps.assertEveryWriteRejected(ctx))

	table = cloudbase.ValuesTable(t, []string{"path", "value"}, []string{"gateway.bad", "1"}, []string{"gateway.ok", "1"})
	require.NoError(t, steps.writeEachValue(ctx, table))
	err := steps.assertEveryWriteRejected(ctx)
	require.ErrorContains(t, err, "1 of 2 writes")
	require.ErrorContains(t, err, "gateway.ok: answered 200")

	local.Set(keyConfigAttempts, "")
	require.ErrorContains(t, steps.assertEveryWriteRejected(ctx), "no gateway configuration writes")
	require.Error(t, steps.writeEachValue(ctx, cloudbase.ValuesTable(t, []string{"path"})))
}

func TestSelectOtherGatewayEnvironmentPassesOverTheActiveProductionAndSyntheticOnes(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	environments := []map[string]any{
		{"name": "development"},
		{"name": "production", "isProduction": true},
		{"name": "e-abcdefg2-1"},
		{"name": "qa"},
		{"name": "apip-test-environment"},
		{"name": "apip-test-s1"},
	}
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /environments":
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"list": environments})
		case "POST /managed-gateways":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(cloudbase.HandleRefusal(body["environment"].(string))))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	require.ErrorContains(t, steps.SelectOtherGatewayEnvironment(ctx), "does not report its environment")
	local.Set(cloudbase.KeyGatewayEnv, "development")
	require.NoError(t, steps.SelectOtherGatewayEnvironment(ctx))
	require.Equal(t, "apip-test-s1", cloudbase.OptionalValue(ctx, cloudbase.KeyEnvironmentName),
		"qa belongs to other consoles and apip-test-environment leaves no room for a gateway name")

	environments = environments[:4]
	require.ErrorContains(t, steps.SelectOtherGatewayEnvironment(ctx), `besides "development"`)
}

func configurationBody() map[string]any {
	return map[string]any{
		"id": "dev-gw", "name": "gw", "environment": "dev",
		"values": map[string]any{"gateway.config.controller.logging.level": "debug"},
		"editable": []any{
			map[string]any{"path": "gateway.config.controller.logging.level", "type": "enum", "values": []any{"debug", "info"}},
			map[string]any{"path": "gateway.controller.deployment.replicaCount", "type": "integer", "min": "1", "max": "5"},
			map[string]any{"path": "gateway.controller.deployment.resources.requests.cpu", "type": "quantity", "min": "50m", "max": "4"},
			map[string]any{"path": "gateway.controller.deployment.resources.limits.cpu", "type": "quantity", "min": "50m", "max": "4"},
			map[string]any{"path": "gateway.config.router.access_logs.enabled", "type": "boolean"},
			map[string]any{"path": "gateway.config.policy_configurations.ratelimit_v1.memory.cleanup_interval", "type": "duration", "min": "30s", "max": "1h"},
			map[string]any{"path": "gateway.config_toml", "type": "string", "max": "8192"},
		},
		"constraints": []any{map[string]any{
			"type": "notGreaterThan", "path": "gateway.controller.deployment.resources.requests.cpu",
			"than": "gateway.controller.deployment.resources.limits.cpu",
		}},
		"status": map[string]any{"phase": "healthy", "lastTransitionTime": "2026-09-03T08:12:34Z"},
	}
}

func TestHandleOfLengthKeepsTheTailAndPadsShortNames(t *testing.T) {
	require.Equal(t, "sg-abcde234-1xx", handleOfLength("sg-abcde234-1", 15), "a short name is padded with a letter")
	require.Equal(t, "abcde234-12", handleOfLength("sg-abcde234-12", 11), "a long name keeps its tail, which is what makes it unique")
	require.Equal(t, "axxxxxxx-12", handleOfLength("-xxxxxxx-12", 11), "a handle never starts with a dash")
	require.Empty(t, handleOfLength("name", 0))
}

func TestRunnerGatewayIsRegisteredBeforeTheRequestAndNumbered(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var pending func() []cleanup.Resource
	registeredBefore := false
	var sent map[string]any
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/managed-gateways", r.URL.Path)
		registeredBefore = len(pending()) == 1
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		cloudbase.WriteJSON(w, http.StatusAccepted, map[string]any{"id": "development-" + sent["handle"].(string), "status": "PROVISIONING"})
	})
	pending = registry.Pending

	require.NoError(t, steps.requestRunnerManagedGateway(ctx, cloudbase.ValuesTable(t,
		[]string{"environment", "development"}, []string{"displayName", "sg-abcde234-1"}, []string{"handle", "sg-abcde234-1"})))
	require.True(t, registeredBefore, "the gateway must be registered for cleanup before it is requested")
	require.NoError(t, steps.assertGatewayRequestAccepted(ctx))

	id, err := cloudbase.RequiredValue(ctx, "cloudSyntheticManagedGateway1")
	require.NoError(t, err)
	require.Equal(t, "development-sg-abcde234-1", id)
	require.Equal(t, "development", cloudbase.OptionalValue(ctx, "cloudSyntheticManagedGatewayEnvironment1"))

	local.Set(keyManagedGatewayID, "")
	require.NoError(t, steps.selectSyntheticGateway(ctx, 1))
	current, environment, err := currentGateway(ctx)
	require.NoError(t, err)
	require.Equal(t, id, current)
	require.Equal(t, "development", environment)
	require.Equal(t, cloudbase.ScenarioGatewayDescription, cloudbase.OptionalValue(ctx, keyScenarioGatewayMark))
	require.ErrorContains(t, steps.selectSyntheticGateway(ctx, 2), "does not exist")

	// A runner-owned gateway is swept with the runner, not with the scenario.
	require.NoError(t, cleanup.Sweep(ctx))
	require.Len(t, registry.Pending(), 1)
}

func TestScenarioGatewayRequestAnsweredWithAnErrorIsNoLongerRegistered(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		cloudbase.WriteJSON(w, http.StatusConflict, map[string]string{"code": "conflict"})
	})
	require.NoError(t, steps.requestManagedGatewayWith(ctx, map[string]string{
		"environment": "development", "displayName": "sg-abcde234-1", "handle": "sg-abcde234-1"}))
	require.NoError(t, steps.AssertCloudResponseStatus(ctx, http.StatusConflict))
	require.Empty(t, registry.Pending(), "a refused request created nothing to clean up")
	require.Empty(t, syntheticGatewayIDs(ctx))
}

func TestAGatewayDeletionIsAskedAgainWhileTheGatewayIsStillChanging(t *testing.T) {
	var calls atomic.Int32
	var refusals atomic.Int32
	refusals.Store(2)
	topo := cloudbase.CloudAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		if calls.Add(1) <= refusals.Load() {
			cloudbase.WriteJSON(w, http.StatusConflict, map[string]string{"code": "conflict", "message": "The request conflicts with the current state of the gateway."})
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)
	client := httpx.NewClient(httpx.Options{Timeout: 5 * time.Second})

	require.NoError(t, deleteGatewayWhenItCanBe(context.Background(), client, base, "console-token", "development-sg-abcde234-1", 10*time.Second, 5*time.Millisecond))
	require.EqualValues(t, 3, calls.Load(), "the deletion is asked again until the platform accepts it")

	// A refusal that waiting does not change is returned at once.
	calls.Store(0)
	permanent := cloudbase.CloudAPIStub(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		cloudbase.WriteJSON(w, http.StatusConflict, map[string]string{"code": "gateway_is_default", "message": "The default gateway cannot be deleted."})
	})
	permanentBase, err := permanent.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	require.NoError(t, err)
	err = deleteGatewayWhenItCanBe(context.Background(), client, permanentBase, "console-token", "development-default", 10*time.Second, 5*time.Millisecond)
	require.ErrorContains(t, err, "gateway_is_default")
	require.EqualValues(t, 1, calls.Load())

	// A gateway that keeps refusing is given up on at the timeout.
	calls.Store(0)
	refusals.Store(1 << 20)
	err = deleteGatewayWhenItCanBe(context.Background(), client, base, "console-token", "development-sg-abcde234-2", 200*time.Millisecond, 20*time.Millisecond)
	require.Error(t, err)
	require.Greater(t, calls.Load(), int32(1))
}

func TestAScenarioGatewayTheProductBuiltAnywayIsNotNumberedForTheRunner(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		var sent map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		cloudbase.WriteJSON(w, http.StatusAccepted, map[string]any{"id": "development-" + sent["handle"].(string), "status": "PROVISIONING"})
	})

	// A request the scenario expects to be refused, which the platform builds a gateway for.
	require.NoError(t, steps.requestManagedGatewayWith(ctx, map[string]string{
		"environment": "development", "displayName": "sg-abcde234-2", "handle": "sg-abcde234-2"}))
	require.Empty(t, syntheticGatewayIDs(ctx), "a gateway the scenario cleans up itself takes no number")
	require.Len(t, registry.Pending(), 1, "it is still cleaned up with the scenario")

	require.NoError(t, steps.requestRunnerManagedGateway(ctx, cloudbase.ValuesTable(t,
		[]string{"environment", "development"}, []string{"displayName", "sg-abcde234-3"}, []string{"handle", "sg-abcde234-3"})))
	require.Equal(t, []string{"development-sg-abcde234-3"}, syntheticGatewayIDs(ctx), "the runner's first gateway is the one it asked to keep")
}

func TestProbeHandleBudgetReadsTheLimitAndCreatesNothing(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var handles []string
	status := http.StatusBadRequest
	steps, ctx, _, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		var sent map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		handles = append(handles, sent["handle"].(string))
		if status == http.StatusBadRequest {
			cloudbase.WriteJSON(w, status, map[string]string{"code": "invalid_request", "message": "its handle must be 15 characters or fewer."})
			return
		}
		cloudbase.WriteJSON(w, http.StatusAccepted, map[string]any{"id": "development-" + sent["handle"].(string)})
	})
	budget, err := steps.probeHandleBudget(ctx, "development")
	require.NoError(t, err)
	require.Equal(t, 15, budget)
	require.Len(t, handles[0], cloudbase.HandleProbeLength)
	require.Empty(t, registry.Pending(), "a refused probe leaves nothing registered")

	status = http.StatusAccepted
	_, err = steps.probeHandleBudget(ctx, "development")
	require.ErrorContains(t, err, "was not refused")
	require.NotEmpty(t, registry.Pending(), "a probe that was accepted created a gateway, which is cleaned up")
}

func TestChooseGatewayNameAtLimitUsesTheTighterEnvironment(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		var sent map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		limit := map[string]int{"development": 15, "e-abcde234-1": 13}[sent["environment"].(string)]
		cloudbase.WriteJSON(w, http.StatusBadRequest, map[string]string{"message": "must be " + strconv.Itoa(limit) + " characters or fewer."})
	})
	local.Set(cloudbase.KeyGatewayEnv, "development")
	local.Set(cloudbase.KeyEnvironmentName, "e-abcde234-1")
	require.NoError(t, steps.chooseGatewayNameAtLimit(ctx))
	name, err := cloudbase.RequiredValue(ctx, keyManagedGatewayName)
	require.NoError(t, err)
	require.Len(t, name, 13, "the one name has to fit the tighter environment")
	require.Regexp(t, `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`, name)
	require.NotEmpty(t, cloudbase.NotSyntheticGateway(map[string]any{"displayName": name}), "the finalizer and the gateway choice must still recognise it")
}

func TestGatewayEntryFallsBackToTheListingWhileTheGatewayIsUnbound(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	bound := false
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/managed-gateways/gw-1":
			if bound {
				cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"id": "gw-1", "status": "DISCONNECTED"})
				return
			}
			cloudbase.WriteJSON(w, http.StatusNotFound, map[string]string{"code": "not_found"})
		case "/managed-gateways":
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{{"id": "gw-1", "status": "PROVISIONING"}}})
		}
	})
	entry, err := steps.gatewayEntry(ctx, "gw-1")
	require.NoError(t, err)
	require.Equal(t, "PROVISIONING", entry["status"])
	bound = true
	entry, err = steps.gatewayEntry(ctx, "gw-1")
	require.NoError(t, err)
	require.Equal(t, "DISCONNECTED", entry["status"])
	_, err = steps.fetchManagedGateway(ctx, "gw-2")
	require.Error(t, err)
}

func TestStatusProgression(t *testing.T) {
	var seen statusProgression
	for _, status := range []string{"PROVISIONING", "PROVISIONING", "DISCONNECTED", "ACTIVE"} {
		seen.add(status)
	}
	require.Equal(t, statusProgression{"PROVISIONING", "DISCONNECTED", "ACTIVE"}, seen)
	require.NoError(t, seen.check())
	require.ErrorContains(t, statusProgression{"PROVISIONING", "FAILED"}.check(), "FAILED")
	require.ErrorContains(t, statusProgression{"DISCONNECTED", "PROVISIONING"}.check(), "went back")
}

func TestFirstReadStatus(t *testing.T) {
	require.NoError(t, firstReadStatus(map[string]any{"id": "gw", "status": "PROVISIONING"}))
	require.ErrorContains(t, firstReadStatus(map[string]any{"id": "gw", "status": ""}), "recorded provisioning state")
	require.ErrorContains(t, firstReadStatus(map[string]any{"id": "gw", "status": "FAILED", "statusReason": "boom"}), "boom")
}

func TestGatewayDocumentProblem(t *testing.T) {
	document := map[string]any{"id": "gw", "environment": "development", "status": "ACTIVE"}
	require.Empty(t, gatewayDocumentProblem(document, "gw", "development"))
	require.Contains(t, gatewayDocumentProblem(document, "other", "development"), "id")
	require.Contains(t, gatewayDocumentProblem(document, "gw", "elsewhere"), "environment")
	require.Contains(t, gatewayDocumentProblem(map[string]any{"id": "gw", "environment": "development", "status": "?"}, "gw", "development"), "status")
	require.Contains(t, gatewayDocumentProblem(map[string]any{"id": "gw", "environment": "development", "status": "ACTIVE", "functionalityType": "ai"},
		"gw", "development"), "functionality type")
}

func TestDefaultGatewayOfAndUpdateBody(t *testing.T) {
	items := []map[string]any{
		{"id": "a", "environment": "development", "isDefault": true},
		{"id": "b", "environment": "development", "isDefault": false},
		{"id": "c", "environment": "development", "isDefault": true, "functionalityType": "ai"},
		{"id": "d", "environment": "other", "isDefault": true},
	}
	id, err := defaultGatewayOf(items, "development", cloudbase.RegularGatewayKind)
	require.NoError(t, err)
	require.Equal(t, "a", id)
	_, err = defaultGatewayOf(items, "empty", cloudbase.RegularGatewayKind)
	require.ErrorContains(t, err, "no default")
	items[1]["isDefault"] = true
	_, err = defaultGatewayOf(items, "development", cloudbase.RegularGatewayKind)
	require.ErrorContains(t, err, "want one")

	body := updateBodyOf(map[string]any{"displayName": "Name", "description": "d", "isCritical": true, "properties": map[string]any{"k": "v"}})
	require.Equal(t, map[string]any{"isDefault": true, "displayName": "Name", "description": "d", "isCritical": true,
		"properties": map[string]any{"k": "v"}}, body)
	require.Equal(t, map[string]any{"isDefault": true}, updateBodyOf(map[string]any{"isCritical": false}))
}

func TestMakeGatewayDefaultRegistersTheReturnBeforeTheChange(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var pending func() []cleanup.Resource
	registeredBefore := false
	var put map[string]any
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/managed-gateways":
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{
				{"id": "development-held", "environment": "development", "isDefault": true},
				{"id": "development-mine", "environment": "development", "isDefault": false}}})
		case r.Method == http.MethodGet:
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"id": "development-mine", "displayName": "Mine", "isDefault": false})
		case r.Method == http.MethodPut:
			registeredBefore = len(pending()) == 1
			require.NoError(t, json.NewDecoder(r.Body).Decode(&put))
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"id": "development-mine", "isDefault": true})
		}
	})
	pending = registry.Pending
	local.Set(keyManagedGatewayID, "development-mine")
	local.Set(keyManagedGatewayEnv, "development")

	require.NoError(t, steps.makeGatewayDefault(ctx))
	require.True(t, registeredBefore, "the return of the default must be registered before it moves")
	require.Equal(t, map[string]any{"isDefault": true, "displayName": "Mine"}, put)
	held, _ := cloudbase.RequiredValue(ctx, keyDefaultGatewayID)
	require.Equal(t, "development-held", held)
	require.Equal(t, KindDefaultRestore.Name, registry.Pending()[0].Kind.Name)
	require.Less(t, KindDefaultRestore.Order, cleanup.KindGateway.Order, "the default goes back before any gateway is deleted")
	handedBack := 0
	require.NoError(t, registry.RegisterDeleter(KindDefaultRestore, func(context.Context, cleanup.Resource) error {
		handedBack++
		return nil
	}))
	require.NoError(t, registry.SweepRunner(ctx))
	require.Zero(t, handedBack, "the default is not left to the end of the runner")
	require.NoError(t, registry.Sweep(ctx))
	require.Equal(t, 1, handedBack, "the default goes back when the scenario ends, however it ends")

	local.Set(keyManagedGatewayID, "development-held")
	require.ErrorContains(t, steps.makeGatewayDefault(ctx), "already the default")
}

func TestRestoreDefaultKeepsFieldsAndSkipsAGatewayThatIsGoneOrAlreadyDefault(t *testing.T) {
	var calls []string
	answer := func(status int, body string) func(context.Context, string, string, any) (*httpx.Response, error) {
		return func(_ context.Context, method, path string, sent any) (*httpx.Response, error) {
			calls = append(calls, method+" "+path)
			if method == http.MethodPut {
				require.Equal(t, map[string]any{"isDefault": true, "displayName": "Held", "isCritical": true}, sent)
				return &httpx.Response{StatusCode: http.StatusOK}, nil
			}
			return &httpx.Response{StatusCode: status, Body: []byte(body)}, nil
		}
	}
	require.NoError(t, restoreDefault(context.Background(), answer(http.StatusOK, `{"displayName":"Held","isCritical":true,"isDefault":false}`), "gw"))
	require.Equal(t, []string{"GET /managed-gateways/gw", "PUT /managed-gateways/gw"}, calls)
	calls = nil
	require.NoError(t, restoreDefault(context.Background(), answer(http.StatusNotFound, ""), "gw"))
	require.NoError(t, restoreDefault(context.Background(), answer(http.StatusOK, `{"isDefault":true}`), "gw"))
	require.Equal(t, []string{"GET /managed-gateways/gw", "GET /managed-gateways/gw"}, calls)
	require.Error(t, restoreDefault(context.Background(), answer(http.StatusInternalServerError, ""), "gw"))
}

func TestVerifyManagedGatewayDeletedWaitsForTheRecordToGo(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, local, registry := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/managed-gateways" {
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{}})
			return
		}
		cloudbase.WriteJSON(w, http.StatusNotFound, map[string]string{"code": "not_found"})
	})
	require.NoError(t, registerGateway(ctx, cleanup.RunnerScope, "development-gone"))
	local.Set(keyManagedGatewayID, "development-gone")
	require.NoError(t, steps.verifyManagedGatewayDeleted(ctx))
	require.Empty(t, registry.Pending(), "a deleted gateway is no longer waiting to be cleaned up")
}

func TestDeleteManagedGatewayAcceptsOnlyAcceptedAndNoContent(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	status := http.StatusAccepted
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(status)
	})
	local.Set(keyManagedGatewayID, "development-gw")
	for _, accepted := range []int{http.StatusAccepted, http.StatusNoContent} {
		status = accepted
		require.NoError(t, steps.deleteManagedGateway(ctx))
	}
	status = http.StatusConflict
	require.ErrorContains(t, steps.deleteManagedGateway(ctx), "409")
	require.NoError(t, steps.requestGatewayDeletion(ctx), "a refusal is for the scenario to assert on")
	require.NoError(t, steps.AssertCloudResponseStatus(ctx, http.StatusConflict))
}

func TestUndeployFromSyntheticGatewayUndeploysOnlyWhatIsHeld(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var undeployed []string
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"list": []map[string]any{
				{"deploymentId": "held", "status": "DEPLOYED"}, {"deploymentId": "gone", "status": "UNDEPLOYED"}}})
			return
		}
		undeployed = append(undeployed, r.URL.Path+"?"+r.URL.RawQuery)
		w.WriteHeader(http.StatusNoContent)
	})
	local.Set(cloudbase.KeyAPIID, "api-1")
	local.Set(cloudbase.KeyProjectID, "default")
	require.NoError(t, recordSyntheticGateway(ctx, "development-gw", "development"))
	require.NoError(t, steps.undeployFromSyntheticGateway(ctx, 1))
	require.Equal(t, []string{"/projects/default/apis/api-1/deployments/held/undeploy?environment=development&gatewayId=development-gw"}, undeployed)
}

func TestConfigurationStepsTargetTheSelectedGatewayAndNotTheSharedOne(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	var paths []string
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"values": map[string]any{"a.b": "debug"}, "status": map[string]any{"phase": "applying"}})
	})
	local.Set(cloudbase.KeyGatewayID, "development-shared")
	require.NoError(t, recordSyntheticGateway(ctx, "e-1-throwaway", "e-1"))

	require.NoError(t, steps.saveGatewayConfiguration(ctx, cloudbase.ValuesTable(t, []string{"a.b", "debug"})))
	require.Equal(t, []string{"PUT /managed-gateways/development-shared/configuration"}, paths, "without a selection the shared gateway is the target")

	paths = nil
	require.NoError(t, steps.selectConfigurationGateway(ctx, 1))
	require.NoError(t, steps.saveGatewayConfiguration(ctx, cloudbase.ValuesTable(t, []string{"a.b", "debug"})))
	require.NoError(t, steps.assertConfigurationPhase(ctx, "applying"))
	require.ErrorContains(t, steps.assertConfigurationPhase(ctx, "healthy"), "want \"healthy\"")
	require.NoError(t, steps.assertConfigurationReports(ctx, cloudbase.ValuesTable(t, []string{"a.b", "debug"})))
	require.Equal(t, "PUT /managed-gateways/e-1-throwaway/configuration", paths[0])
	for _, path := range paths {
		require.Contains(t, path, "e-1-throwaway")
	}
	require.ErrorContains(t, steps.selectConfigurationGateway(ctx, 2), "does not exist")
}

func TestAssertConfigurationOmitsWaitsForTheSettingToGo(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	reads := 0
	steps, ctx, local, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		reads++
		values := map[string]any{"other": 1}
		if reads == 1 {
			values["gone.soon"] = "x"
		}
		cloudbase.WriteJSON(w, http.StatusOK, map[string]any{"values": values})
	})
	local.Set(cloudbase.KeyGatewayID, "development-shared")
	require.NoError(t, steps.assertConfigurationOmits(ctx, "gone.soon"))
	require.Greater(t, reads, 1)
}

func TestJudgeRoundAcceptsWritesAndConflictsAndFlagsEverythingElse(t *testing.T) {
	ok := &httpx.Response{StatusCode: http.StatusOK}
	conflict := &httpx.Response{StatusCode: http.StatusConflict, Body: []byte(`{"code":"conflict"}`)}
	round := judgeRound([]*httpx.Response{ok, conflict, ok})
	require.Equal(t, []int{200, 409, 200}, round.statuses)
	require.Equal(t, conflict, round.conflict)
	require.Empty(t, round.unwanted)

	round = judgeRound([]*httpx.Response{ok, {StatusCode: http.StatusInternalServerError, Body: []byte("boom")},
		{StatusCode: http.StatusConflict, Body: []byte(`{"code":"other"}`)}})
	require.Len(t, round.unwanted, 2)
}

// cloudSteps wires the area's steps to a stub cloud API.
func cloudSteps(t *testing.T, handler http.HandlerFunc) (*Steps, context.Context, *tcontext.Local, *cleanup.Registry) {
	t.Helper()
	base, ctx, local, registry := cloudbase.CloudSteps(t, handler)
	return &Steps{Steps: base}, ctx, local, registry
}
