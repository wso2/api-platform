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
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/catalog"
	"github.com/wso2/api-platform/tests/framework/core/components"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/suiterun"
	"github.com/wso2/api-platform/tests/framework/core/topology"
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
		CleanupDeleters: platformgateway.RegisterDeleters,
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
	for source, engine := range singleEngineBlocks {
		require.Contains(t, variants, source, "single-engine block %q is not in the suite", source)
		require.Equal(t, []components.DBType{engine}, variants[source], "single-engine block %q", source)
	}
	for source, got := range variants {
		if _, single := singleEngineBlocks[source]; single {
			continue
		}
		sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
		require.Equal(t, coverageEngines, got, "block %q database coverage", source)
	}
}

// singleEngineBlocks are the gateway blocks that run one engine by design, with that engine.
// The webhook block follows its control plane's PostgreSQL store, the publishing block follows
// the API Portal's PostgreSQL store, the multigateway block
// runs two gateway stacks rather than an engine matrix, the AI Workspace CLI block runs one
// SQLite stack, and the two-controller block needs
// a database server both controllers share. The hostname blocks that set their own
// downstream_tls settings run one engine: the settings do not depend on the database, and the
// default block of the feature repeats on every engine. So do the trust-any, observability and
// default identity blocks, whose subjects are the data plane's behaviour, not the store.
var singleEngineBlocks = map[string]components.DBType{
	"devportal-webhook":                               components.Postgres,
	"api-portal-publishing":                           components.Postgres,
	"multigateway":                                    components.SQLite,
	"ai-workspace-cli":                                components.SQLite,
	"gateway-mtls-ha":                                 components.Postgres,
	"gateway-mtls-hostnames-required":                 components.SQLite,
	"gateway-mtls-hostnames-all-connections":          components.SQLite,
	"gateway-mtls-hostnames-all-connections-required": components.SQLite,
	"gateway-mtls-trust-any":                          components.SQLite,
	"gateway-mtls-observability":                      components.SQLite,
	"gateway-mtls-default-identity-off":               components.SQLite,
	"gateway-mtls-default-identity-on":                components.SQLite,
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
