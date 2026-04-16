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

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.uber.org/zap"
	yamlv3 "gopkg.in/yaml.v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
)

// policyVersionPattern matches APIPolicy / RestApi policy version (e.g. v1), aligned with CRD schema pattern ^v\d+$.
var policyVersionPattern = regexp.MustCompile(`^v\d+$`)

// policyYAML is a loose document shape for inline YAML/JSON (mirrors RestApi CR policy fields).
type policyYAML struct {
	Name               string                 `yaml:"name" json:"name"`
	Version            string                 `yaml:"version" json:"version"`
	ExecutionCondition *string                `yaml:"executionCondition,omitempty" json:"executionCondition,omitempty"`
	Params             map[string]interface{} `yaml:"params,omitempty" json:"params,omitempty"`
}

// HTTPRouteOperationPolicyKey returns the canonical map key for operation-level policies.
// Format: "METHOD:/path" (method as stored on the Operation, path with leading slash;
// must match values produced from HTTPRoute matches in BuildAPIConfigFromHTTPRoute).
func HTTPRouteOperationPolicyKey(method apiv1.OperationMethod, path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		p = "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return fmt.Sprintf("%s:%s", string(method), p)
}

func loadHTTPRouteAPIPolicies(ctx context.Context, c client.Client, route *gatewayv1.HTTPRoute, log *zap.Logger) ([]apiv1.Policy, error) {
	return apiPoliciesFromTargetRef(ctx, c, route, log)
}

func apiPoliciesFromTargetRef(ctx context.Context, c client.Client, route *gatewayv1.HTTPRoute, log *zap.Logger) ([]apiv1.Policy, error) {
	list := &apiv1.APIPolicyList{}
	if err := c.List(ctx, list, client.InNamespace(route.Namespace)); err != nil {
		return nil, err
	}
	var crNames []string
	byCR := make(map[string][]apiv1.Policy)
	for i := range list.Items {
		ap := &list.Items[i]
		if ap.Spec.TargetRef == nil {
			continue
		}
		if !apiPolicyTargetRefMatchesHTTPRoute(ap, route) {
			continue
		}
		pols, err := embeddedPoliciesFromAPIPolicySpec(&ap.Spec)
		if err != nil {
			return nil, newInvalidHTTPRouteConfigError("APIPolicy %q: %w", ap.Name, err)
		}
		byCR[ap.Name] = pols
		crNames = append(crNames, ap.Name)
	}
	if len(crNames) == 0 {
		if log != nil {
			log.Debug("no APIPolicy CRs with spec.targetRef for this HTTPRoute",
				zap.String("httpRoute", route.Namespace+"/"+route.Name))
		}
		return nil, nil
	}
	sort.Strings(crNames)
	var out []apiv1.Policy
	for _, n := range crNames {
		out = append(out, byCR[n]...)
	}
	if log != nil {
		log.Info("API-level policies from APIPolicy CRs (spec.targetRef)",
			zap.Strings("apiPolicies", crNames),
			zap.Int("embeddedPolicyCount", len(out)))
	}
	return out, nil
}

func apiPolicyTargetRefMatchesHTTPRoute(ap *apiv1.APIPolicy, route *gatewayv1.HTTPRoute) bool {
	if ap.Spec.TargetRef == nil {
		return false
	}
	ref := *ap.Spec.TargetRef
	if strings.TrimSpace(ref.Kind) != "HTTPRoute" {
		return false
	}
	if strings.TrimSpace(ref.Group) != gatewayv1.GroupName {
		return false
	}
	if ref.Name != route.Name {
		return false
	}
	if ref.Namespace != nil && strings.TrimSpace(*ref.Namespace) != "" && *ref.Namespace != route.Namespace {
		return false
	}
	return true
}

func embeddedPoliciesFromAPIPolicySpec(spec *apiv1.APIPolicySpec) ([]apiv1.Policy, error) {
	if len(spec.Policies) == 0 {
		return nil, fmt.Errorf("spec.policies must contain at least one policy")
	}
	out := make([]apiv1.Policy, len(spec.Policies))
	for i := range spec.Policies {
		p := spec.Policies[i]
		if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Version) == "" {
			return nil, fmt.Errorf("policies[%d]: name and version are required", i)
		}
		out[i] = p
	}
	return out, nil
}

// policiesFromHTTPRouteRuleExtensionRefs loads policies from rule.filters where type is ExtensionRef.
// Only APIPolicy ExtensionRefs are supported for Gateway API integration.
func policiesFromHTTPRouteRuleExtensionRefs(ctx context.Context, c client.Client, route *gatewayv1.HTTPRoute, rule gatewayv1.HTTPRouteRule, ruleIdx int, log *zap.Logger) ([]apiv1.Policy, error) {
	var merged []apiv1.Policy
	for _, f := range rule.Filters {
		if f.Type != gatewayv1.HTTPRouteFilterExtensionRef || f.ExtensionRef == nil {
			continue
		}
		ref := f.ExtensionRef
		name := strings.TrimSpace(string(ref.Name))
		if name == "" {
			return nil, newInvalidHTTPRouteConfigError("HTTPRoute rule filter ExtensionRef requires metadata.name")
		}
		switch string(ref.Kind) {
		case "APIPolicy":
			if string(ref.Group) != apiv1.GroupVersion.Group {
				continue
			}
			pl, err := policiesFromAPIPolicyRef(ctx, c, route, name, log)
			if err != nil {
				return nil, err
			}
			if log != nil {
				log.Debug("rule ExtensionRef merged policies",
					zap.Int("ruleIndex", ruleIdx),
					zap.String("refKind", "APIPolicy"),
					zap.String("refName", name),
					zap.Int("policyCount", len(pl)))
			}
			merged = append(merged, pl...)
		default:
			continue
		}
	}
	return merged, nil
}

func policiesFromAPIPolicyRef(ctx context.Context, c client.Client, route *gatewayv1.HTTPRoute, policyName string, log *zap.Logger) ([]apiv1.Policy, error) {
	ap := &apiv1.APIPolicy{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: route.Namespace, Name: policyName}, ap); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, newTransientHTTPRouteConfigError("APIPolicy %s/%s not found: %w", route.Namespace, policyName, err)
		}
		return nil, err
	}
	if ap.Spec.TargetRef != nil {
		if !apiPolicyTargetRefMatchesHTTPRoute(ap, route) {
			return nil, newInvalidHTTPRouteConfigError(
				"APIPolicy %q spec.targetRef must name this HTTPRoute (%s/%s) with group %s kind HTTPRoute when targetRef is set",
				policyName, route.Namespace, route.Name, gatewayv1.GroupName,
			)
		}
	}
	pols, err := embeddedPoliciesFromAPIPolicySpec(&ap.Spec)
	if err != nil {
		return nil, newInvalidHTTPRouteConfigError("APIPolicy %q: %w", policyName, err)
	}
	if log != nil {
		log.Debug("loaded policies from APIPolicy for HTTPRoute rule scope",
			zap.String("namespace", route.Namespace),
			zap.String("apiPolicy", policyName),
			zap.Int("policyCount", len(pols)))
	}
	return pols, nil
}

func copyPolicies(p []apiv1.Policy) []apiv1.Policy {
	if len(p) == 0 {
		return nil
	}
	out := make([]apiv1.Policy, len(p))
	copy(out, p)
	return out
}

func parsePolicyList(data string) ([]apiv1.Policy, error) {
	data = strings.TrimSpace(data)
	if data == "" {
		return nil, nil
	}
	if strings.HasPrefix(data, "[") || strings.HasPrefix(data, "{") {
		return parsePolicyListJSON([]byte(data))
	}
	return parsePolicyListYAML([]byte(data))
}

func parsePolicyListJSON(b []byte) ([]apiv1.Policy, error) {
	trim := strings.TrimSpace(string(b))
	if strings.HasPrefix(trim, "[") {
		var docs []policyYAML
		if err := json.Unmarshal(b, &docs); err != nil {
			return nil, newInvalidHTTPRouteConfigError("api policies JSON: %w", err)
		}
		return policyYAMLSliceToAPI(docs)
	}
	var wrap struct {
		Policies []policyYAML `json:"policies"`
	}
	if err := json.Unmarshal(b, &wrap); err != nil {
		return nil, newInvalidHTTPRouteConfigError("api policies JSON: %w", err)
	}
	if wrap.Policies == nil {
		return nil, newInvalidHTTPRouteConfigError(`api policies JSON object must contain "policies"`)
	}
	return policyYAMLSliceToAPI(wrap.Policies)
}

func parsePolicyListYAML(b []byte) ([]apiv1.Policy, error) {
	var asList []policyYAML
	if err := yamlv3.Unmarshal(b, &asList); err == nil {
		return policyYAMLSliceToAPI(asList)
	}
	var wrap struct {
		Policies []policyYAML `yaml:"policies"`
	}
	if err := yamlv3.Unmarshal(b, &wrap); err != nil {
		return nil, newInvalidHTTPRouteConfigError("api policies YAML: %w", err)
	}
	if wrap.Policies == nil {
		return nil, newInvalidHTTPRouteConfigError(`api policies YAML object must contain "policies"`)
	}
	return policyYAMLSliceToAPI(wrap.Policies)
}

func policyYAMLSliceToAPI(docs []policyYAML) ([]apiv1.Policy, error) {
	out := make([]apiv1.Policy, 0, len(docs))
	for i := range docs {
		p, err := policyYAMLToAPI(docs[i])
		if err != nil {
			return nil, newInvalidHTTPRouteConfigError("policy[%d]: %w", i, err)
		}
		out = append(out, p)
	}
	return out, nil
}

func policyYAMLToAPI(p policyYAML) (apiv1.Policy, error) {
	if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Version) == "" {
		return apiv1.Policy{}, fmt.Errorf("name and version are required")
	}
	if !policyVersionPattern.MatchString(strings.TrimSpace(p.Version)) {
		return apiv1.Policy{}, fmt.Errorf(
			"invalid version format %q: must be v followed by digits only (e.g. v1), matching pattern ^v\\d+$",
			strings.TrimSpace(p.Version),
		)
	}
	out := apiv1.Policy{
		Name:               p.Name,
		Version:            p.Version,
		ExecutionCondition: p.ExecutionCondition,
	}
	if len(p.Params) > 0 {
		raw, err := json.Marshal(p.Params)
		if err != nil {
			return apiv1.Policy{}, fmt.Errorf("params: %w", err)
		}
		out.Params = &runtime.RawExtension{Raw: raw}
	}
	return out, nil
}
