/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	annLlmProviderPolicyValueFromFingerprint = "gateway.api-platform.wso2.com/last-applied-llmprovider-policy-valuefrom-fingerprint"
	annLlmProxyPolicyValueFromFingerprint    = "gateway.api-platform.wso2.com/last-applied-llmproxy-policy-valuefrom-fingerprint"
	annMcpPolicyValueFromFingerprint         = "gateway.api-platform.wso2.com/last-applied-mcp-policy-valuefrom-fingerprint"
)

func computeBackingFingerprint(ctx context.Context, c client.Client, backing map[string]valueFromBackingID) (string, error) {
	if len(backing) == 0 {
		return "", nil
	}
	var lines []string
	for _, id := range backing {
		rv, err := fetchBackingResourceVersion(ctx, c, id)
		if err != nil {
			return "", err
		}
		tag := "configmap"
		if id.secret {
			tag = "secret"
		}
		lines = append(lines, fmt.Sprintf("%s:%s/%s@%s", tag, id.ns, id.name, rv))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n"), nil
}

func accumulateBackingFromRawParams(raw []byte, defaultNS string, backing map[string]valueFromBackingID) {
	if len(raw) == 0 {
		return
	}
	var root interface{}
	if err := json.Unmarshal(raw, &root); err != nil {
		return
	}
	jsonTreeAccumulateValueFromBackingIDs(root, defaultNS, backing)
}

func accumulateBackingFromSecretValueSource(v apiv1.SecretValueSource, defaultNS string, backing map[string]valueFromBackingID) {
	if v.ValueFrom == nil {
		return
	}
	name := strings.TrimSpace(v.ValueFrom.Name)
	if name == "" || strings.TrimSpace(defaultNS) == "" {
		return
	}
	id := valueFromBackingID{secret: true, ns: defaultNS, name: name}
	backing[id.key()] = id
}

func llmProviderExternalDepsFingerprint(ctx context.Context, c client.Client, cr *apiv1.LlmProvider) (string, error) {
	backing := map[string]valueFromBackingID{}
	if cr.Spec.Upstream.Auth != nil {
		accumulateBackingFromSecretValueSource(cr.Spec.Upstream.Auth.Value, cr.Namespace, backing)
	}
	for i := range cr.Spec.Policies {
		for j := range cr.Spec.Policies[i].Paths {
			if p := cr.Spec.Policies[i].Paths[j].Params; p != nil {
				accumulateBackingFromRawParams(p.Raw, cr.Namespace, backing)
			}
		}
	}
	return computeBackingFingerprint(ctx, c, backing)
}

func mcpExternalDepsFingerprint(ctx context.Context, c client.Client, cr *apiv1.Mcp) (string, error) {
	backing := map[string]valueFromBackingID{}
	if cr.Spec.Upstream.Auth != nil {
		accumulateBackingFromSecretValueSource(cr.Spec.Upstream.Auth.Value, cr.Namespace, backing)
	}
	for i := range cr.Spec.Policies {
		if p := cr.Spec.Policies[i].Params; p != nil {
			accumulateBackingFromRawParams(p.Raw, cr.Namespace, backing)
		}
	}
	return computeBackingFingerprint(ctx, c, backing)
}

func llmProxyExternalDepsFingerprint(ctx context.Context, c client.Client, cr *apiv1.LlmProxy) (string, error) {
	backing := map[string]valueFromBackingID{}
	if cr.Spec.Provider.Auth != nil {
		accumulateBackingFromSecretValueSource(cr.Spec.Provider.Auth.Value, cr.Namespace, backing)
	}
	for i := range cr.Spec.AdditionalProviders {
		if cr.Spec.AdditionalProviders[i].Auth != nil {
			accumulateBackingFromSecretValueSource(cr.Spec.AdditionalProviders[i].Auth.Value, cr.Namespace, backing)
		}
	}
	for i := range cr.Spec.Policies {
		for j := range cr.Spec.Policies[i].Paths {
			if p := cr.Spec.Policies[i].Paths[j].Params; p != nil {
				accumulateBackingFromRawParams(p.Raw, cr.Namespace, backing)
			}
		}
	}
	return computeBackingFingerprint(ctx, c, backing)
}

func resolveRawExtensionValueFrom(ctx context.Context, c client.Client, defaultNS string, raw *runtime.RawExtension, policyName, scope string) error {
	if raw == nil || len(raw.Raw) == 0 {
		return nil
	}
	var root interface{}
	if err := json.Unmarshal(raw.Raw, &root); err != nil {
		return newInvalidHTTPRouteConfigError("policy %q params: invalid JSON: %w", policyName, err)
	}
	resolved, err := resolveValueFromInJSON(ctx, c, defaultNS, root, policyName, scope, nil)
	if err != nil {
		return err
	}
	out, err := json.Marshal(resolved)
	if err != nil {
		return newInvalidHTTPRouteConfigError("policy %q params: %w", policyName, err)
	}
	raw.Raw = out
	return nil
}
