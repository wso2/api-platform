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

package service

import (
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/policysecret"
)

// stubResolver declares a fixed set of paths write-only for one policy name,
// standing in for the index an organization's gateway manifests would build.
type stubResolver struct {
	policyName string
	paths      []policysecret.Path
}

func (r stubResolver) WriteOnlyPaths(policyName, _ string) []policysecret.Path {
	if policyName != r.policyName {
		return nil
	}
	return r.paths
}

func awsResolver() stubResolver {
	return stubResolver{
		policyName: "aws-authentication",
		paths:      []policysecret.Path{{"awsSecretAccessKey"}},
	}
}

// awsManifestStore is a store backed by a manifest declaring the AWS secret
// write-only, so a service method resolves through the real index rather than a
// stub.
func awsManifestStore() *policysecret.Store {
	manifest := []byte(`[{"name": "aws-authentication", "version": "v0",
	  "writeOnlyParams": [["awsSecretAccessKey"]]}]`)
	return policysecret.NewStore(func(string) ([][]byte, error) {
		return [][]byte{manifest}, nil
	})
}

func paramsOf(t *testing.T, policy api.Policy) map[string]interface{} {
	t.Helper()
	if policy.Params == nil {
		t.Fatalf("policy %q carries no params", policy.Name)
	}
	return *policy.Params
}

func TestBlankPolicySecretsEmptiesDeclaredParameters(t *testing.T) {
	params := map[string]interface{}{"awsSecretAccessKey": "a-stored-secret", "region": "us-east-1"}
	policies := []api.Policy{{Name: "aws-authentication", Version: "v0", Params: &params}}

	blankPolicySecrets(awsResolver(), &policies)

	got := paramsOf(t, policies[0])
	if got["awsSecretAccessKey"] != "" {
		t.Errorf("awsSecretAccessKey = %v, want the empty string", got["awsSecretAccessKey"])
	}
	if got["region"] != "us-east-1" {
		t.Errorf("region = %v, want it untouched", got["region"])
	}
	if params["awsSecretAccessKey"] != "a-stored-secret" {
		t.Error("the stored parameter map was mutated; a response must be built from a copy")
	}
}

func TestBlankPolicySecretsLeavesUndeclaredPoliciesAlone(t *testing.T) {
	params := map[string]interface{}{"allowOrigins": "*"}
	policies := []api.Policy{{Name: "cors", Version: "v1", Params: &params}}

	blankPolicySecrets(awsResolver(), &policies)

	if paramsOf(t, policies[0])["allowOrigins"] != "*" {
		t.Error("a policy with no write-only declarations was altered")
	}
}

func TestBlankOperationPolicySecretsCoversEveryPath(t *testing.T) {
	policies := []api.OperationPolicy{{
		Name:    "aws-authentication",
		Version: "v0",
		Paths: []api.OperationPolicyPath{
			{Path: "/one", Params: map[string]interface{}{"awsSecretAccessKey": "one-secret"}},
			{Path: "/two", Params: map[string]interface{}{"awsSecretAccessKey": "two-secret"}},
		},
	}}

	blankOperationPolicySecrets(awsResolver(), &policies)

	for i, path := range policies[0].Paths {
		if path.Params["awsSecretAccessKey"] != "" {
			t.Errorf("paths[%d] secret = %v, want the empty string", i, path.Params["awsSecretAccessKey"])
		}
	}
}

func TestPreservePolicySecretsCarriesStoredValueForward(t *testing.T) {
	storedParams := map[string]interface{}{"awsSecretAccessKey": "a-stored-secret", "region": "us-east-1"}
	updatedParams := map[string]interface{}{"awsSecretAccessKey": "", "region": "eu-west-1"}
	stored := []model.Policy{{Name: "aws-authentication", Version: "v0", Params: &storedParams}}
	updated := []model.Policy{{Name: "aws-authentication", Version: "v0", Params: &updatedParams}}

	preservePolicySecrets(awsResolver(), stored, updated)

	if updatedParams["awsSecretAccessKey"] != "a-stored-secret" {
		t.Errorf("awsSecretAccessKey = %v, want the stored value", updatedParams["awsSecretAccessKey"])
	}
	if updatedParams["region"] != "eu-west-1" {
		t.Errorf("region = %v, want the update's own value", updatedParams["region"])
	}
}

// A policy upgrade is a different parameter schema, so a secret stored against
// the old version is not reattached to the new one.
func TestPreservePolicySecretsSkipsAVersionChange(t *testing.T) {
	storedParams := map[string]interface{}{"awsSecretAccessKey": "a-stored-secret"}
	updatedParams := map[string]interface{}{"awsSecretAccessKey": ""}
	stored := []model.Policy{{Name: "aws-authentication", Version: "v0", Params: &storedParams}}
	updated := []model.Policy{{Name: "aws-authentication", Version: "v1", Params: &updatedParams}}

	preservePolicySecrets(awsResolver(), stored, updated)

	if updatedParams["awsSecretAccessKey"] != "" {
		t.Errorf("awsSecretAccessKey = %v, want it left empty across a version change",
			updatedParams["awsSecretAccessKey"])
	}
}

// An update that mentions the policy but sends no parameters at all must still
// keep the stored credential.
func TestPreservePolicySecretsRebuildsAnAbsentParameterMap(t *testing.T) {
	storedParams := map[string]interface{}{"awsSecretAccessKey": "a-stored-secret"}
	stored := []model.Policy{{Name: "aws-authentication", Version: "v0", Params: &storedParams}}
	updated := []model.Policy{{Name: "aws-authentication", Version: "v0"}}

	preservePolicySecrets(awsResolver(), stored, updated)

	if updated[0].Params == nil {
		t.Fatal("params were left nil; the stored secret had nowhere to go")
	}
	if (*updated[0].Params)["awsSecretAccessKey"] != "a-stored-secret" {
		t.Errorf("awsSecretAccessKey = %v, want the stored value", (*updated[0].Params)["awsSecretAccessKey"])
	}
}

func TestPreserveOperationPolicySecretsMatchesOnThePathExpression(t *testing.T) {
	stored := []model.OperationPolicy{{
		Name:    "aws-authentication",
		Version: "v0",
		Paths: []model.OperationPolicyPath{
			{Path: "/one", Params: map[string]interface{}{"awsSecretAccessKey": "one-secret"}},
			{Path: "/two", Params: map[string]interface{}{"awsSecretAccessKey": "two-secret"}},
		},
	}}
	// Reordered, as a client is free to send them.
	updated := []model.OperationPolicy{{
		Name:    "aws-authentication",
		Version: "v0",
		Paths: []model.OperationPolicyPath{
			{Path: "/two", Params: map[string]interface{}{"awsSecretAccessKey": ""}},
			{Path: "/one", Params: map[string]interface{}{"awsSecretAccessKey": ""}},
		},
	}}

	preserveOperationPolicySecrets(awsResolver(), stored, updated)

	if got := updated[0].Paths[0].Params["awsSecretAccessKey"]; got != "two-secret" {
		t.Errorf("/two secret = %v, want the value stored for /two", got)
	}
	if got := updated[0].Paths[1].Params["awsSecretAccessKey"]; got != "one-secret" {
		t.Errorf("/one secret = %v, want the value stored for /one", got)
	}
}

// A provider written before the canonical lists existed is read back as global
// and operation policies, so that is the shape an update echoes and the shape
// the stored side has to be matched in.
func TestStoredPolicyScopesSplitsALegacyList(t *testing.T) {
	legacy := []model.LLMPolicy{{
		Name:    "aws-authentication",
		Version: "v0",
		Paths: []model.LLMPolicyPath{
			{Path: "/*", Methods: []string{"*"}, Params: map[string]interface{}{"awsSecretAccessKey": "a-stored-secret"}},
		},
	}}

	global, operation := storedPolicyScopes(nil, nil, legacy)

	if len(global) != 1 {
		t.Fatalf("global policies = %d, want the wildcard path lifted into one", len(global))
	}
	if global[0].Params["awsSecretAccessKey"] != "a-stored-secret" {
		t.Errorf("secret = %v, want it carried through the split", global[0].Params["awsSecretAccessKey"])
	}
	if len(operation) != 0 {
		t.Errorf("operation policies = %d, want none for a wildcard-only legacy list", len(operation))
	}
}

func TestStoredPolicyScopesPrefersTheCanonicalLists(t *testing.T) {
	global := []model.GlobalPolicy{{Name: "aws-authentication", Version: "v0"}}
	legacy := []model.LLMPolicy{{Name: "cors", Version: "v1"}}

	gotGlobal, _ := storedPolicyScopes(global, nil, legacy)

	if len(gotGlobal) != 1 || gotGlobal[0].Name != "aws-authentication" {
		t.Errorf("global policies = %v, want the canonical list used as it is", gotGlobal)
	}
}

func TestPreserveAPIPolicySecretsMatchesOperationsOnMethodAndPath(t *testing.T) {
	storedParams := map[string]interface{}{"awsSecretAccessKey": "a-stored-secret"}
	updatedParams := map[string]interface{}{"awsSecretAccessKey": ""}
	existing := &model.API{Configuration: model.RestAPIConfig{
		Operations: []model.Operation{{Request: &model.OperationRequest{
			Method:   "POST",
			Path:     "/chat",
			Policies: []model.Policy{{Name: "aws-authentication", Version: "v0", Params: &storedParams}},
		}}},
	}}
	updated := &model.API{Configuration: model.RestAPIConfig{
		Operations: []model.Operation{{Request: &model.OperationRequest{
			Method:   "POST",
			Path:     "/chat",
			Policies: []model.Policy{{Name: "aws-authentication", Version: "v0", Params: &updatedParams}},
		}}},
	}}

	svc := &APIService{}
	svc.SetPolicySecrets(awsManifestStore())

	if err := svc.preserveAPIPolicySecrets("org-a", existing, updated); err != nil {
		t.Fatalf("preserveAPIPolicySecrets returned %v", err)
	}

	if updatedParams["awsSecretAccessKey"] != "a-stored-secret" {
		t.Errorf("awsSecretAccessKey = %v, want the stored value", updatedParams["awsSecretAccessKey"])
	}
	if findStoredOperationRequest(existing.Configuration.Operations,
		&model.OperationRequest{Method: "GET", Path: "/chat"}) != nil {
		t.Error("an operation was matched on path alone; method is part of its identity")
	}
}

// A service constructed without a store resolves nothing rather than failing.
// Server startup always wires one; this is the path tests take.
func TestPolicySecretResolverWithoutAStoreResolvesNothing(t *testing.T) {
	resolver, err := policySecretResolver(nil, "org-a")
	if err != nil {
		t.Fatalf("policySecretResolver returned %v", err)
	}
	if paths := resolver.WriteOnlyPaths("aws-authentication", "v0"); paths != nil {
		t.Errorf("paths = %v, want none", paths)
	}
}
