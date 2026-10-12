/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package it_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/actor"
	"github.com/wso2/api-platform/tests/framework/core/catalog"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/components"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/suiterun"
	"github.com/wso2/api-platform/tests/framework/core/topology"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/suites/it/steps"
	"github.com/wso2/api-platform/tests/framework/suites/it/steps/platformgateway"
)

// options describes the integration suite to the shared suite runner.
func options() suiterun.Options {
	return suiterun.Options{
		Steps: func(topo *frameworkruntime.Topology, featureRoot string) (frameworkruntime.StepRegistrar, error) {
			suite, err := steps.New(topo, featureRoot)
			if err != nil {
				return nil, err
			}
			return suite.Register, nil
		},
		CleanupDeleters: registerDeleters,
	}
}

func TestMain(m *testing.M) {
	suiterun.Main(m, options())
}

// TestIntegrationSuite runs the integration suite declared in YAML.
func TestIntegrationSuite(t *testing.T) {
	suiterun.Run(t, options())
}

func TestGatewayDatabaseCompatibilityExcludesSQLServerForGatewayV110(t *testing.T) {
	dir, err := os.Getwd()
	require.NoError(t, err)
	registry, err := catalog.Registry()
	require.NoError(t, err)
	resolved, err := topology.LoadFile(filepath.Join(dir, "it-suite.yaml"), registry)
	require.NoError(t, err)

	selected, err := (topology.Selection{GatewayVersion: "1.1.0"}).Apply(resolved)
	require.NoError(t, err)
	require.NotEmpty(t, selected.SkippedBlocks)
	for _, block := range selected.Blocks {
		for _, component := range block.Components {
			if component.Def != nil && component.Def.Name == "platform-gateway" {
				require.NotEqual(t, components.SQLServer, component.DB, "unsupported Gateway 1.1.0 block %q was retained", block.Name)
			}
		}
	}
	for _, skipped := range selected.SkippedBlocks {
		require.Contains(t, skipped.Block, "/sqlserver")
		require.Contains(t, skipped.Reason, "Gateway version 1.1.0")
	}

	_, err = (topology.Selection{
		GatewayVersion: "1.1.0",
		Blocks:         []string{"gateway-core/sqlserver"},
	}).Apply(resolved)
	require.ErrorContains(t, err, `selected block "gateway-core/sqlserver" is incompatible`)

	for _, selection := range []topology.Selection{{GatewayVersion: "1.2.0"}, {}} {
		selected, err := selection.Apply(resolved)
		require.NoError(t, err)
		foundSQLServer := false
		for _, block := range selected.Blocks {
			for _, component := range block.Components {
				if component.Def != nil && component.Def.Name == "platform-gateway" && component.DB == components.SQLServer {
					foundSQLServer = true
				}
			}
		}
		require.True(t, foundSQLServer, "compatible Gateway selection unexpectedly removed every SQL Server variant")
		require.Empty(t, selected.SkippedBlocks)
	}
}

func TestBasicAuthForUsesTopologyCredentials(t *testing.T) {
	topo := &frameworkruntime.Topology{Admin: actor.Credentials{
		Username: "admin", Password: "secret",
	}}

	got := basicAuthFor(topo)
	want := steps.BasicAuthHeader("admin", "secret")
	if got != want {
		t.Fatalf("basicAuthFor() = %q, want %q", got, want)
	}
}

// registerDeleters configures cleanup handlers for gateway resources.
func registerDeleters(reg *cleanup.Registry, topo *frameworkruntime.Topology) {
	client := httpx.NewClient(httpx.Options{MaxRetries: 1, InsecureSkipVerify: true})

	reg.RegisterDeleter(cleanup.KindAPI, func(ctx context.Context, res cleanup.Resource) error {
		base, err := topo.URL("platform-gateway", "rest")
		if err != nil {
			return err
		}
		version, err := topo.ComponentVersion("platform-gateway")
		if err != nil {
			return err
		}

		resp, err := client.Do(ctx, httpx.Request{
			Method: http.MethodDelete,
			URL:    base + platformgateway.ManagementBasePathForVersion(version) + "/rest-apis/" + res.ID,
			Headers: map[string]string{
				"Authorization": basicAuthFor(topo),
			},
		}, 1, 0)
		if err != nil {
			return err
		}
		// A 404 means it is already gone, which is success for a sweep. Anything else is a
		// real leak signal and is reported.
		if resp.StatusCode == http.StatusNotFound || resp.Succeeded() {
			return awaitDeletionApplied(ctx, topo, client, version, res.ID)
		}
		return errFromResponse(resp)
	})

	registerControllerDeleter(reg, topo, client, cleanup.KindLLMProvider, "/llm-providers")
	registerControllerDeleter(reg, topo, client, cleanup.KindLLMProxy, "/llm-proxies")
	registerControllerDeleter(reg, topo, client, cleanup.KindLLMProviderTemplate, "/llm-provider-templates")
	registerControllerDeleter(reg, topo, client, cleanup.KindMCPProxy, "/mcp-proxies")
	registerControllerDeleter(reg, topo, client, cleanup.KindAgent, "/agents")
	registerControllerDeleter(reg, topo, client, cleanup.KindCertificate, "/certificates")
	registerControllerDeleter(reg, topo, client, cleanup.KindSecret, "/secrets")
}

func registerControllerDeleter(
	reg *cleanup.Registry,
	topo *frameworkruntime.Topology,
	client *httpx.Client,
	kind cleanup.Kind,
	collection string,
) {
	reg.RegisterDeleter(kind, func(ctx context.Context, res cleanup.Resource) error {
		base, err := topo.URL("platform-gateway", "rest")
		if err != nil {
			return err
		}
		version, err := topo.ComponentVersion("platform-gateway")
		if err != nil {
			return err
		}
		resp, err := client.Do(ctx, httpx.Request{
			Method: http.MethodDelete,
			URL:    base + platformgateway.ManagementBasePathForVersion(version) + collection + "/" + res.ID,
			Headers: map[string]string{
				"Authorization": basicAuthFor(topo),
			},
		}, 1, 0)
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusNotFound || resp.Succeeded() {
			return awaitDeletionApplied(ctx, topo, client, version, res.ID)
		}
		return errFromResponse(resp)
	})
}

// awaitDeletionApplied waits until the controller has applied a gateway deletion, on releases
// where the next scenario's resource could otherwise lose its route to the snapshot race.
func awaitDeletionApplied(
	ctx context.Context, topo *frameworkruntime.Topology, client *httpx.Client, version, handle string,
) error {
	admin, err := topo.URL("platform-gateway", "admin")
	if err != nil {
		return err
	}
	return platformgateway.AwaitDeletionApplied(ctx, client, admin, version, basicAuthFor(topo), handle)
}

func basicAuthFor(topo *frameworkruntime.Topology) string {
	return steps.BasicAuthHeader(topo.Admin.Username, topo.Admin.Password)
}

// TestEveryBlockSweepsEveryEngine verifies that gateway coverage includes every database engine.
func TestEveryBlockSweepsEveryEngine(t *testing.T) {
	resolved := loadSuiteCoverage(t)
	variants := map[string][]components.DBType{}
	for i := range resolved.Blocks {
		block := &resolved.Blocks[i]
		for _, component := range block.Components {
			if component.Def.Name == coverageSubject {
				variants[block.Source] = append(variants[block.Source], component.DB)
			}
		}
	}
	require.NotEmpty(t, variants)
	for source, got := range variants {
		if source == "devportal-webhook" || source == "multigateway" || source == "ai-workspace-cli" {
			require.Len(t, got, 1, "single-engine block %q", source)
			continue
		}
		sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
		require.Equal(t, coverageEngines, got, "block %q database coverage", source)
	}
}

const coverageSubject = "platform-gateway"

var coverageEngines = []components.DBType{components.Postgres, components.SQLite, components.SQLServer}

func loadSuiteCoverage(t *testing.T) *topology.Resolved {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	registry, err := catalog.Registry()
	require.NoError(t, err)
	resolved, err := topology.LoadFile(filepath.Join(dir, "it-suite.yaml"), registry)
	require.NoError(t, err)
	return resolved
}

func errFromResponse(resp *httpx.Response) error {
	return &deleteError{resp: resp}
}

type deleteError struct{ resp *httpx.Response }

func (e *deleteError) Error() string { return e.resp.Describe() }
