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

package xdsclient

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/kernel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

// =============================================================================
// Test helpers
// =============================================================================

// skipPolicy is a minimal policy.Policy that declares no processing (SKIP modes)
// so chain-build produces no warnings.
type skipPolicy struct{}

func (skipPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		ResponseHeaderMode: policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

// regWithCounters builds a registry whose factories count how many times each
// policy's GetPolicy factory is invoked. names are "name:version" keys.
func regWithCounters(t *testing.T, names ...string) (*registry.PolicyRegistry, map[string]*int) {
	t.Helper()
	reg := &registry.PolicyRegistry{Policies: make(map[string]*registry.PolicyEntry)}
	require.NoError(t, reg.SetConfig(map[string]interface{}{}))
	counts := make(map[string]*int)
	for _, nv := range names {
		c := new(int)
		counts[nv] = c
		parts := strings.SplitN(nv, ":", 2)
		require.Len(t, parts, 2, "policy key must be name:version")
		reg.Policies[nv] = &registry.PolicyEntry{
			Definition: &policy.PolicyDefinition{Name: parts[0], Version: parts[1]},
			Factory: func(_ policy.PolicyMetadata, _ map[string]interface{}) (policy.Policy, error) {
				*c++
				return skipPolicy{}, nil
			},
		}
	}
	return reg, counts
}

// mustResource wraps a StoredPolicyConfig map as the double-wrapped *anypb.Any
// the ADS stream delivers.
func mustResource(t *testing.T, stored map[string]interface{}) *anypb.Any {
	t.Helper()
	ps, err := structpb.NewStruct(stored)
	require.NoError(t, err)
	sb, err := proto.Marshal(ps)
	require.NoError(t, err)
	inner := &anypb.Any{TypeUrl: "type.googleapis.com/google.protobuf.Struct", Value: sb}
	ib, err := proto.Marshal(inner)
	require.NoError(t, err)
	return &anypb.Any{TypeUrl: PolicyChainTypeURL, Value: ib}
}

// storedConfig builds a StoredPolicyConfig map with the given routes. updatedAt
// feeds updated_at/resource_version — volatile fields that must NOT affect the
// signature.
func storedConfig(id, apiID, apiName, apiVersion string, updatedAt int, routes ...interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":      id,
		"version": 1,
		"configuration": map[string]interface{}{
			"metadata": map[string]interface{}{
				"api_id":           apiID,
				"api_name":         apiName,
				"version":          apiVersion,
				"created_at":       0,
				"updated_at":       updatedAt,
				"resource_version": updatedAt,
				"context":          "/" + apiName,
			},
			"routes": routes,
		},
	}
}

func route(routeKey string, policies ...interface{}) interface{} {
	return map[string]interface{}{"route_key": routeKey, "policies": policies}
}

func pol(name, version string, params map[string]interface{}) interface{} {
	if params == nil {
		params = map[string]interface{}{}
	}
	return map[string]interface{}{
		"name":       name,
		"version":    version,
		"enabled":    true,
		"parameters": params,
	}
}

// =============================================================================
// Reconciliation behaviour (handler level)
// =============================================================================

// The original bug: redeploying one API re-ran every other API's GetPolicy.
func TestHandlePolicyChainUpdate_ReusesUnchangedRoutesOnUnrelatedRedeploy(t *testing.T) {
	metrics.Init()
	reg, counts := regWithCounters(t, "polA:v1", "polB:v1")
	k := kernel.NewKernel()
	h := NewResourceHandler(k, reg)
	ctx := context.Background()

	rA := route("rA", pol("polA", "v1", map[string]interface{}{"x": "1"}))
	rB := route("rB", pol("polB", "v1", map[string]interface{}{"y": "1"}))

	require.NoError(t, h.HandlePolicyChainUpdate(ctx, []*anypb.Any{
		mustResource(t, storedConfig("A", "apiA", "A", "v1", 1, rA)),
		mustResource(t, storedConfig("B", "apiB", "B", "v1", 1, rB)),
	}, "1"))
	require.Equal(t, 1, *counts["polA:v1"])
	require.Equal(t, 1, *counts["polB:v1"])
	chainA1 := k.GetPolicyChain("rA")
	require.NotNil(t, chainA1)

	// Redeploy B (param changed); A is byte-identical, B's metadata bumped.
	rBChanged := route("rB", pol("polB", "v1", map[string]interface{}{"y": "2"}))
	require.NoError(t, h.HandlePolicyChainUpdate(ctx, []*anypb.Any{
		mustResource(t, storedConfig("A", "apiA", "A", "v1", 1, rA)),
		mustResource(t, storedConfig("B", "apiB", "B", "v1", 2, rBChanged)),
	}, "2"))

	assert.Equal(t, 1, *counts["polA:v1"], "A reused: its GetPolicy must NOT re-run on B's redeploy")
	assert.Equal(t, 2, *counts["polB:v1"], "B changed: must rebuild")
	assert.Same(t, chainA1, k.GetPolicyChain("rA"), "reused chain pointer must be identical")
	assert.NotSame(t, chainA1, k.GetPolicyChain("rB"))
}

// Bumping only volatile metadata (updated_at/resource_version) must not rebuild.
func TestHandlePolicyChainUpdate_ReuseAcrossVolatileMetadataBump(t *testing.T) {
	metrics.Init()
	reg, counts := regWithCounters(t, "polA:v1")
	k := kernel.NewKernel()
	h := NewResourceHandler(k, reg)
	ctx := context.Background()

	rA := route("rA", pol("polA", "v1", map[string]interface{}{"x": "1"}))
	require.NoError(t, h.HandlePolicyChainUpdate(ctx,
		[]*anypb.Any{mustResource(t, storedConfig("A", "apiA", "A", "v1", 10, rA))}, "1"))
	require.Equal(t, 1, *counts["polA:v1"])

	// Identical behavioural config, only volatile metadata differs.
	require.NoError(t, h.HandlePolicyChainUpdate(ctx,
		[]*anypb.Any{mustResource(t, storedConfig("A", "apiA", "A", "v1", 999, rA))}, "2"))
	assert.Equal(t, 1, *counts["polA:v1"], "volatile metadata bump must not trigger a rebuild")
}

// Reordering the policies within a chain must rebuild (execution order matters).
func TestHandlePolicyChainUpdate_ReorderRebuilds(t *testing.T) {
	metrics.Init()
	reg, counts := regWithCounters(t, "polA:v1", "polB:v1")
	k := kernel.NewKernel()
	h := NewResourceHandler(k, reg)
	ctx := context.Background()

	pA := pol("polA", "v1", map[string]interface{}{"x": "1"})
	pB := pol("polB", "v1", map[string]interface{}{"y": "1"})

	require.NoError(t, h.HandlePolicyChainUpdate(ctx,
		[]*anypb.Any{mustResource(t, storedConfig("A", "apiA", "A", "v1", 1, route("rA", pA, pB)))}, "1"))
	require.Equal(t, 1, *counts["polA:v1"])
	require.Equal(t, 1, *counts["polB:v1"])

	// Same policies, swapped order.
	require.NoError(t, h.HandlePolicyChainUpdate(ctx,
		[]*anypb.Any{mustResource(t, storedConfig("A", "apiA", "A", "v1", 1, route("rA", pB, pA)))}, "2"))
	assert.Equal(t, 2, *counts["polA:v1"], "reorder must rebuild the chain")
	assert.Equal(t, 2, *counts["polB:v1"])
}

// Routes absent from a later snapshot must be dropped from the kernel.
func TestHandlePolicyChainUpdate_RemovesAbsentRoutes(t *testing.T) {
	metrics.Init()
	reg, _ := regWithCounters(t, "polA:v1")
	k := kernel.NewKernel()
	h := NewResourceHandler(k, reg)
	ctx := context.Background()

	rA := route("rA", pol("polA", "v1", nil))
	require.NoError(t, h.HandlePolicyChainUpdate(ctx,
		[]*anypb.Any{mustResource(t, storedConfig("A", "apiA", "A", "v1", 1, rA))}, "1"))
	require.NotNil(t, k.GetPolicyChain("rA"))

	// Empty snapshot: A is gone.
	require.NoError(t, h.HandlePolicyChainUpdate(ctx, []*anypb.Any{}, "2"))
	assert.Nil(t, k.GetPolicyChain("rA"), "absent route must be dropped from the kernel")
}

// =============================================================================
// routeSignature — field sensitivity
// =============================================================================

func baseChainAndMeta() (*policyenginev1.PolicyChain, policyenginev1.Metadata) {
	cfg := &policyenginev1.PolicyChain{
		RouteKey: "rA",
		Policies: []policyenginev1.PolicyInstance{
			{Name: "polA", Version: "v1", Enabled: true, Parameters: map[string]interface{}{"x": "1"}},
			{Name: "polB", Version: "v1", Enabled: true, Parameters: map[string]interface{}{"y": "1"}},
		},
	}
	md := policyenginev1.Metadata{APIId: "apiA", APIName: "A", Version: "v1"}
	return cfg, md
}

func sig(t *testing.T, cfg *policyenginev1.PolicyChain, md policyenginev1.Metadata) string {
	t.Helper()
	s, err := routeSignature(cfg, md)
	require.NoError(t, err)
	return s
}

func TestRouteSignature_StableAndSensitive(t *testing.T) {
	cfg, md := baseChainAndMeta()
	base := sig(t, cfg, md)

	// Identical inputs -> identical signature.
	cfg2, md2 := baseChainAndMeta()
	assert.Equal(t, base, sig(t, cfg2, md2), "identical config must yield identical signature")

	strPtr := func(s string) *string { return &s }

	// Each of these must CHANGE the signature.
	changed := []struct {
		name  string
		apply func(*policyenginev1.PolicyChain, *policyenginev1.Metadata)
	}{
		{"param value", func(c *policyenginev1.PolicyChain, _ *policyenginev1.Metadata) { c.Policies[0].Parameters["x"] = "2" }},
		{"param added", func(c *policyenginev1.PolicyChain, _ *policyenginev1.Metadata) { c.Policies[0].Parameters["z"] = "1" }},
		{"param removed", func(c *policyenginev1.PolicyChain, _ *policyenginev1.Metadata) { delete(c.Policies[0].Parameters, "x") }},
		{"reorder", func(c *policyenginev1.PolicyChain, _ *policyenginev1.Metadata) {
			c.Policies[0], c.Policies[1] = c.Policies[1], c.Policies[0]
		}},
		{"name", func(c *policyenginev1.PolicyChain, _ *policyenginev1.Metadata) { c.Policies[0].Name = "polC" }},
		{"version", func(c *policyenginev1.PolicyChain, _ *policyenginev1.Metadata) { c.Policies[0].Version = "v2" }},
		{"enabled", func(c *policyenginev1.PolicyChain, _ *policyenginev1.Metadata) { c.Policies[0].Enabled = false }},
		{"executionCondition", func(c *policyenginev1.PolicyChain, _ *policyenginev1.Metadata) {
			c.Policies[0].ExecutionCondition = strPtr("x == 1")
		}},
		{"routeKey", func(c *policyenginev1.PolicyChain, _ *policyenginev1.Metadata) { c.RouteKey = "rB" }},
		{"apiId", func(_ *policyenginev1.PolicyChain, m *policyenginev1.Metadata) { m.APIId = "apiB" }},
		{"apiName", func(_ *policyenginev1.PolicyChain, m *policyenginev1.Metadata) { m.APIName = "B" }},
		{"apiVersion", func(_ *policyenginev1.PolicyChain, m *policyenginev1.Metadata) { m.Version = "v2" }},
	}
	for _, tc := range changed {
		c, m := baseChainAndMeta()
		tc.apply(c, &m)
		assert.NotEqualf(t, base, sig(t, c, m), "%s must change the signature", tc.name)
	}

	// Volatile / non-behavioural metadata must NOT change the signature.
	unchanged := []struct {
		name  string
		apply func(*policyenginev1.Metadata)
	}{
		{"created_at", func(m *policyenginev1.Metadata) { m.CreatedAt = 123 }},
		{"updated_at", func(m *policyenginev1.Metadata) { m.UpdatedAt = 456 }},
		{"resource_version", func(m *policyenginev1.Metadata) { m.ResourceVersion = 789 }},
		{"context", func(m *policyenginev1.Metadata) { m.Context = "/changed" }},
	}
	for _, tc := range unchanged {
		c, m := baseChainAndMeta()
		tc.apply(&m)
		assert.Equalf(t, base, sig(t, c, m), "%s must NOT change the signature", tc.name)
	}
}

// Completeness guard: every field of routeSignatureView must influence the
// signature, and every field must have a mutator here. If someone adds a field
// to the view without a mutator, this fails — forcing them to keep coverage.
func TestRouteSignatureView_Completeness(t *testing.T) {
	base := routeSignatureView{
		RouteKey:   "r",
		Policies:   []policyenginev1.PolicyInstance{{Name: "p", Version: "v1", Enabled: true, Parameters: map[string]interface{}{"a": "b"}}},
		APIId:      "i",
		APIName:    "n",
		APIVersion: "v",
	}

	mutators := map[string]func(*routeSignatureView){
		"RouteKey":   func(v *routeSignatureView) { v.RouteKey = "r2" },
		"Policies":   func(v *routeSignatureView) { v.Policies = []policyenginev1.PolicyInstance{{Name: "other"}} },
		"APIId":      func(v *routeSignatureView) { v.APIId = "i2" },
		"APIName":    func(v *routeSignatureView) { v.APIName = "n2" },
		"APIVersion": func(v *routeSignatureView) { v.APIVersion = "v2" },
	}

	require.Equal(t, reflect.TypeOf(base).NumField(), len(mutators),
		"add a mutator for every routeSignatureView field so completeness stays guaranteed")

	baseSig, err := signatureOf(base)
	require.NoError(t, err)
	for name, mutate := range mutators {
		v := base
		mutate(&v)
		got, err := signatureOf(v)
		require.NoError(t, err)
		assert.NotEqualf(t, baseSig, got, "mutating field %s must change the signature", name)
	}
}
