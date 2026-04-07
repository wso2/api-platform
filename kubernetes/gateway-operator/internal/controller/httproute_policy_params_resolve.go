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
	"strings"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
)

// resolveAPIConfigPolicyParamsSecrets replaces params.valueFrom objects with string values read from Secrets
// so gateway-controller receives the same JSON shape as inline RestApi policies.
func resolveAPIConfigPolicyParamsSecrets(ctx context.Context, c client.Client, routeNS string, spec *apiv1.APIConfigData, log *zap.Logger) error {
	for i := range spec.Policies {
		if err := resolvePolicyParamsSecrets(ctx, c, routeNS, &spec.Policies[i], "api-level", log); err != nil {
			return err
		}
	}
	for i := range spec.Operations {
		for j := range spec.Operations[i].Policies {
			scope := string(spec.Operations[i].Method) + " " + spec.Operations[i].Path
			if err := resolvePolicyParamsSecrets(ctx, c, routeNS, &spec.Operations[i].Policies[j], scope, log); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolvePolicyParamsSecrets(ctx context.Context, c client.Client, defaultNS string, p *apiv1.Policy, scope string, log *zap.Logger) error {
	if p.Params == nil || len(p.Params.Raw) == 0 {
		return nil
	}
	var root interface{}
	if err := json.Unmarshal(p.Params.Raw, &root); err != nil {
		return newInvalidHTTPRouteConfigError("policy %q params: invalid JSON: %w", p.Name, err)
	}
	resolved, err := resolveValueFromInJSON(ctx, c, defaultNS, root, p.Name, scope, log)
	if err != nil {
		return err
	}
	out, err := json.Marshal(resolved)
	if err != nil {
		return newInvalidHTTPRouteConfigError("policy %q params: %w", p.Name, err)
	}
	p.Params = &runtime.RawExtension{Raw: out}
	return nil
}

func resolveValueFromInJSON(ctx context.Context, c client.Client, defaultNS string, v interface{}, policyName, scope string, log *zap.Logger) (interface{}, error) {
	switch x := v.(type) {
	case map[string]interface{}:
		if s, ok, err := tryResolveValueFromMap(ctx, c, defaultNS, x, policyName, scope, log); ok || err != nil {
			return s, err
		}
		out := make(map[string]interface{}, len(x))
		for k, child := range x {
			r, err := resolveValueFromInJSON(ctx, c, defaultNS, child, policyName, scope, log)
			if err != nil {
				return nil, err
			}
			out[k] = r
		}
		return out, nil
	case []interface{}:
		out := make([]interface{}, len(x))
		for i, el := range x {
			r, err := resolveValueFromInJSON(ctx, c, defaultNS, el, policyName, scope, log)
			if err != nil {
				return nil, err
			}
			out[i] = r
		}
		return out, nil
	default:
		return v, nil
	}
}

// tryResolveValueFromMap resolves maps that are exclusively { "valueFrom": { name, valueKey [, namespace] } }.
func tryResolveValueFromMap(ctx context.Context, c client.Client, defaultNS string, m map[string]interface{}, policyName, scope string, log *zap.Logger) (string, bool, error) {
	if len(m) != 1 {
		return "", false, nil
	}
	vf, ok := m["valueFrom"]
	if !ok {
		return "", false, nil
	}
	inner, ok := vf.(map[string]interface{})
	if !ok {
		return "", false, nil
	}
	name, _ := inner["name"].(string)
	key, _ := inner["valueKey"].(string)
	if strings.TrimSpace(name) == "" || strings.TrimSpace(key) == "" {
		return "", false, newInvalidHTTPRouteConfigError(`valueFrom requires "name" and "valueKey"`)
	}
	ns := defaultNS
	if n, ok := inner["namespace"].(string); ok && strings.TrimSpace(n) != "" {
		ns = strings.TrimSpace(n)
	}
	sec := &corev1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, sec); err != nil {
		if apierrors.IsNotFound(err) {
			return "", true, newTransientHTTPRouteConfigError("Secret %s/%s not found: %w", ns, name, err)
		}
		return "", true, err
	}
	data, ok := sec.Data[key]
	if !ok || len(data) == 0 {
		return "", true, newTransientHTTPRouteConfigError("Secret %s/%s has no non-empty data key %q", ns, name, key)
	}
	if log != nil {
		log.Debug("resolved policy param from Secret (valueFrom)",
			zap.String("policy", policyName),
			zap.String("scope", scope),
			zap.String("secretNamespace", ns),
			zap.String("secret", name),
			zap.String("valueKey", key),
			zap.Int("valueLength", len(data)))
	}
	return string(data), true, nil
}
