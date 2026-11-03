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

package gatewaytranslator

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/internal/constants"
)

func TestKindMapping_AgentProxyIsAgentOnTheGateway(t *testing.T) {
	gatewayKind, ok := GatewayKindForPlatformKind(constants.AgentProxy)
	require.True(t, ok)
	assert.Equal(t, "Agent", gatewayKind)

	platformKind, ok := PlatformKindForGatewayKind("Agent")
	require.True(t, ok)
	assert.Equal(t, constants.AgentProxy, platformKind)

	_, ok = PlatformKindForGatewayKind(constants.AgentProxy)
	assert.False(t, ok, "AgentProxy is not a gateway document kind")
	_, ok = GatewayKindForPlatformKind("Agent")
	assert.False(t, ok, "Agent is not a control-plane kind")
}

func TestKindMapping_SameNameKindsAreExplicit(t *testing.T) {
	for _, kind := range []string{
		constants.RestApi, constants.WebSubApi, constants.WebBrokerApi,
		constants.MCPProxy, constants.LLMProxy, constants.LLMProvider,
	} {
		gatewayKind, ok := GatewayKindForPlatformKind(kind)
		require.Truef(t, ok, "%s has no explicit mapping", kind)
		assert.Equal(t, kind, gatewayKind)
		platformKind, ok := PlatformKindForGatewayKind(kind)
		require.True(t, ok)
		assert.Equal(t, kind, platformKind)
	}
}

func TestKindMapping_UnknownKindIsNotPassedThrough(t *testing.T) {
	_, ok := GatewayKindForPlatformKind("SomeFutureKind")
	assert.False(t, ok)
	_, ok = PlatformKindForGatewayKind("SomeFutureKind")
	assert.False(t, ok)
}

// Every mapped kind must carry a data-version entry, so a kind registered for
// deployment cannot silently compute the "1.0" default.
func TestKindMapping_EveryMappedKindHasADataVersion(t *testing.T) {
	for platformKind := range platformToGatewayKind {
		_, ok := platformDataMinorVersions[platformKind]
		assert.Truef(t, ok, "%s is mapped but has no platformDataMinorVersions entry", platformKind)
	}
	for platformKind := range platformDataMinorVersions {
		_, ok := platformToGatewayKind[platformKind]
		assert.Truef(t, ok, "%s has a data version but no gateway kind mapping", platformKind)
	}
}

func TestComputeDataVersion_AgentProxyIsDeclared(t *testing.T) {
	minor, ok := platformDataMinorVersions[constants.AgentProxy]
	require.True(t, ok, "AgentProxy must be declared, not resolved by the unknown-kind fallback")
	assert.Equal(t, 0, minor)
	assert.Equal(t, PlatformDataVersion("1.0"), ComputeDataVersion(constants.AgentProxy, constants.GatewayApiVersion))
}

func TestComputeDataVersionForGatewayKind(t *testing.T) {
	assert.Equal(t,
		ComputeDataVersion(constants.AgentProxy, constants.GatewayApiVersion),
		ComputeDataVersionForGatewayKind("Agent", constants.GatewayApiVersion))
	assert.Equal(t, PlatformDataVersion("1.1"),
		ComputeDataVersionForGatewayKind(constants.LLMProxy, constants.GatewayApiVersion))
	assert.Equal(t, defaultPlatformDataVersion,
		ComputeDataVersionForGatewayKind("SomeFutureKind", "gateway.api-platform.wso2.com/v2"))
}

// gatewayModelsDir is the gateway controller's models package, read as source:
// platform-api does not (and must not) depend on the gateway module, so the
// agreement check parses the gateway's table instead of importing it.
var gatewayModelsDir = filepath.Join("..", "..", "..", "gateway", "gateway-controller", "pkg", "models")

// gatewayDataMinorVersions parses the gateway's dataMinorVersions table, keyed
// by the kind's string value.
func gatewayDataMinorVersions(t *testing.T) map[string]int {
	t.Helper()
	fset := token.NewFileSet()
	kindValues := map[string]string{}
	var table *ast.CompositeLit
	for _, name := range []string{"stored_config.go", "data_version.go"} {
		file, err := parser.ParseFile(fset, filepath.Join(gatewayModelsDir, name), nil, 0)
		require.NoError(t, err)
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.ValueSpec:
				for i, ident := range node.Names {
					if i >= len(node.Values) {
						continue
					}
					if lit, ok := node.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						if v, err := strconv.Unquote(lit.Value); err == nil {
							kindValues[ident.Name] = v
						}
					}
					if ident.Name == "dataMinorVersions" {
						table, _ = node.Values[i].(*ast.CompositeLit)
					}
				}
			}
			return true
		})
	}
	require.NotNil(t, table, "gateway dataMinorVersions table not found")

	out := map[string]int{}
	for _, elt := range table.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		require.True(t, ok)
		key, ok := kv.Key.(*ast.Ident)
		require.True(t, ok)
		kind, ok := kindValues[key.Name]
		require.Truef(t, ok, "gateway kind constant %s not resolved", key.Name)
		lit, ok := kv.Value.(*ast.BasicLit)
		require.True(t, ok)
		minor, err := strconv.Atoi(lit.Value)
		require.NoError(t, err)
		out[kind] = minor
	}
	return out
}

// The control plane and the gateway must compute the same data version for an
// Agent. The gateway's table is keyed by its document kind (Agent), the CP's by
// its own kind (AgentProxy); compare them through the explicit kind mapping.
func TestAgentDataVersionMatchesGateway(t *testing.T) {
	if _, err := os.Stat(gatewayModelsDir); err != nil {
		t.Skipf("gateway controller source not available at %s: %v", gatewayModelsDir, err)
	}
	gatewayTable := gatewayDataMinorVersions(t)

	gatewayKind, ok := GatewayKindForPlatformKind(constants.AgentProxy)
	require.True(t, ok)
	gatewayMinor, ok := gatewayTable[gatewayKind]
	require.Truef(t, ok, "gateway dataMinorVersions has no %s entry", gatewayKind)
	assert.Equal(t, gatewayMinor, platformDataMinorVersions[constants.AgentProxy])
}
