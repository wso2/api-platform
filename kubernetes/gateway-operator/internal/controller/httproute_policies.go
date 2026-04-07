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
	"sort"
	"strings"

	"go.uber.org/zap"
	yamlv3 "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
)

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

func loadPolicyListFromConfigMap(ctx context.Context, c client.Client, namespace, cmName string, log *zap.Logger) ([]apiv1.Policy, error) {
	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cmName}, cm); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, newTransientHTTPRouteConfigError("policies ConfigMap %s/%s not found: %w", namespace, cmName, err)
		}
		return nil, err
	}
	raw := configMapPolicyPayload(cm)
	if raw == "" {
		return nil, newInvalidHTTPRouteConfigError("ConfigMap %q has no policies.yaml, policies.yml, or policies.json data key", cmName)
	}
	pl, err := parsePolicyList(raw)
	if err != nil {
		return nil, err
	}
	if log != nil {
		log.Debug("loaded policy list from ConfigMap",
			zap.String("namespace", namespace),
			zap.String("configMap", cmName),
			zap.Int("policyCount", len(pl)))
	}
	return pl, nil
}

func loadHTTPRouteAPIPolicies(ctx context.Context, c client.Client, route *gatewayv1.HTTPRoute, log *zap.Logger) ([]apiv1.Policy, error) {
	cmName := strings.TrimSpace(route.Annotations[AnnHTTPRouteAPIPoliciesConfigMap])
	if cmName != "" {
		pl, err := loadPolicyListFromConfigMap(ctx, c, route.Namespace, cmName, log)
		if err != nil {
			return nil, err
		}
		if log != nil {
			log.Info("API-level policies from HTTPRoute annotation (ConfigMap)",
				zap.String("configMap", cmName),
				zap.Int("policyCount", len(pl)))
		}
		return pl, nil
	}
	if inline := strings.TrimSpace(route.Annotations[AnnHTTPRouteAPIPolicies]); inline != "" {
		pl, err := parsePolicyList(inline)
		if err != nil {
			return nil, err
		}
		if log != nil {
			log.Info("API-level policies from HTTPRoute annotation (inline)",
				zap.Int("policyCount", len(pl)))
		}
		return pl, nil
	}
	return apiPoliciesFromLabeledAPIPolicies(ctx, c, route, log)
}

func apiPoliciesFromLabeledAPIPolicies(ctx context.Context, c client.Client, route *gatewayv1.HTTPRoute, log *zap.Logger) ([]apiv1.Policy, error) {
	list := &apiv1.APIPolicyList{}
	if err := c.List(ctx, list,
		client.InNamespace(route.Namespace),
		client.MatchingLabels{LabelAPIPolicyScope: LabelAPIPolicyScopeApiLevel},
	); err != nil {
		return nil, err
	}
	var crNames []string
	byCR := make(map[string][]apiv1.Policy)
	for i := range list.Items {
		ap := &list.Items[i]
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
			log.Debug("no APIPolicy CRs with ApiLevel scope target this HTTPRoute",
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
		log.Info("API-level policies from APIPolicy CRs (label scope ApiLevel)",
			zap.Strings("apiPolicies", crNames),
			zap.Int("embeddedPolicyCount", len(out)))
	}
	return out, nil
}

func apiPolicyTargetRefMatchesHTTPRoute(ap *apiv1.APIPolicy, route *gatewayv1.HTTPRoute) bool {
	ref := ap.Spec.TargetRef
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

// policiesFromHTTPRouteRuleExtensionRefs loads policies from rule.filters where type is ExtensionRef:
//   - group "" / kind ConfigMap (legacy): policies data keys as today;
//   - group gateway.api-platform.wso2.com / kind APIPolicy: all entries in spec.policies from that CR (targetRef must match the HTTPRoute).
//
// The merged list is applied to every Operation derived from that rule's matches. Other filter types are ignored.
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
		case "ConfigMap":
			if string(ref.Group) != "" {
				continue
			}
			pl, err := loadPolicyListFromConfigMap(ctx, c, route.Namespace, name, log)
			if err != nil {
				return nil, err
			}
			if log != nil {
				log.Debug("rule ExtensionRef merged policies",
					zap.Int("ruleIndex", ruleIdx),
					zap.String("refKind", "ConfigMap"),
					zap.String("refName", name),
					zap.Int("policyCount", len(pl)))
			}
			merged = append(merged, pl...)
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
	if !apiPolicyTargetRefMatchesHTTPRoute(ap, route) {
		return nil, newInvalidHTTPRouteConfigError(
			"APIPolicy %q spec.targetRef must name this HTTPRoute (%s/%s) with group %s kind HTTPRoute",
			policyName, route.Namespace, route.Name, gatewayv1.GroupName,
		)
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

func configMapPolicyPayload(cm *corev1.ConfigMap) string {
	for _, k := range []string{"policies.yaml", "policies.yml", "policies.json"} {
		if v := cm.Data[k]; v != "" {
			return v
		}
	}
	return ""
}

func configMapOperationPolicyPayload(cm *corev1.ConfigMap) string {
	for _, k := range []string{
		"operation-policies.yaml",
		"operation-policies.yml",
		"operation-policies.json",
	} {
		if v := cm.Data[k]; v != "" {
			return v
		}
	}
	return ""
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

func loadHTTPRouteOperationPolicyMap(ctx context.Context, c client.Client, route *gatewayv1.HTTPRoute, log *zap.Logger) (map[string][]apiv1.Policy, error) {
	cmName := strings.TrimSpace(route.Annotations[AnnHTTPRouteOperationPoliciesConfigMap])
	if cmName != "" {
		cm := &corev1.ConfigMap{}
		if err := c.Get(ctx, types.NamespacedName{Namespace: route.Namespace, Name: cmName}, cm); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, newTransientHTTPRouteConfigError("operation-policies ConfigMap %s/%s not found: %w", route.Namespace, cmName, err)
			}
			return nil, err
		}
		raw := configMapOperationPolicyPayload(cm)
		if raw == "" {
			return nil, newInvalidHTTPRouteConfigError("ConfigMap %q has no operation-policies.yaml, operation-policies.yml, or operation-policies.json data key", cmName)
		}
		m, err := parseOperationPoliciesMap(raw)
		if err != nil {
			return nil, err
		}
		if log != nil {
			log.Info("operation-level policies from ConfigMap",
				zap.String("configMap", cmName),
				zap.Int("operationKeyCount", len(m)))
		}
		return m, nil
	}

	inline := strings.TrimSpace(route.Annotations[AnnHTTPRouteOperationPolicies])
	if inline == "" {
		return nil, nil
	}
	m, err := parseOperationPoliciesMap(inline)
	if err != nil {
		return nil, err
	}
	if log != nil {
		log.Info("operation-level policies from HTTPRoute annotation (inline)",
			zap.Int("operationKeyCount", len(m)))
	}
	return m, nil
}

func parseOperationPoliciesMap(data string) (map[string][]apiv1.Policy, error) {
	data = strings.TrimSpace(data)
	if strings.HasPrefix(data, "{") {
		return parseOperationPoliciesMapJSON([]byte(data))
	}
	return parseOperationPoliciesMapYAML([]byte(data))
}

func parseOperationPoliciesMapJSON(b []byte) (map[string][]apiv1.Policy, error) {
	var raw map[string][]policyYAML
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, newInvalidHTTPRouteConfigError("operation policies JSON: %w", err)
	}
	return operationPolicyMapToAPI(raw)
}

func parseOperationPoliciesMapYAML(b []byte) (map[string][]apiv1.Policy, error) {
	var raw map[string][]policyYAML
	if err := yamlv3.Unmarshal(b, &raw); err != nil {
		return nil, newInvalidHTTPRouteConfigError("operation policies YAML: %w", err)
	}
	return operationPolicyMapToAPI(raw)
}

func operationPolicyMapToAPI(raw map[string][]policyYAML) (map[string][]apiv1.Policy, error) {
	out := make(map[string][]apiv1.Policy, len(raw))
	for key, docs := range raw {
		normKey, err := normalizeOperationPolicyKey(key)
		if err != nil {
			return nil, err
		}
		pols, err := policyYAMLSliceToAPI(docs)
		if err != nil {
			return nil, newInvalidHTTPRouteConfigError("operation %q: %w", key, err)
		}
		out[normKey] = pols
	}
	return out, nil
}

// normalizeOperationPolicyKey accepts "METHOD:path" or "METHOD:/path" and uppercases the method segment.
func normalizeOperationPolicyKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	idx := strings.Index(key, ":")
	if idx <= 0 {
		return "", newInvalidHTTPRouteConfigError(
			"invalid operation key %q: expected canonical METHOD:/path format so applyOperationPolicies can match operations",
			key,
		)
	}
	method := strings.ToUpper(strings.TrimSpace(key[:idx]))
	path := strings.TrimSpace(key[idx+1:])
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return method + ":" + path, nil
}

func applyOperationPolicies(ops []apiv1.Operation, byKey map[string][]apiv1.Policy) {
	if len(byKey) == 0 {
		return
	}
	for i := range ops {
		k := HTTPRouteOperationPolicyKey(ops[i].Method, ops[i].Path)
		if pols, ok := byKey[k]; ok {
			ops[i].Policies = append(append([]apiv1.Policy(nil), ops[i].Policies...), pols...)
		}
	}
}
