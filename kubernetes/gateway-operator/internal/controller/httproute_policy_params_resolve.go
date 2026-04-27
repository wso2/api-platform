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

// Reserved keys for the k8s-native valueFrom shape used inside policy params:
//
//	valueFrom:
//	  secretKeyRef:     { name, key, namespace? }
//	  configMapKeyRef:  { name, key, namespace? }
//
// Exactly one of secretKeyRef / configMapKeyRef must be set; both paths resolve to a
// single string value that replaces the `{ valueFrom: {...} }` object in the JSON tree
// before gateway-controller is called.
const (
	valueFromKey        = "valueFrom"
	secretKeyRefKey     = "secretKeyRef"
	configMapKeyRefKey  = "configMapKeyRef"
	keyRefFieldName     = "name"
	keyRefFieldKey      = "key"
	keyRefFieldNamespac = "namespace"
)

// resolveAPIConfigPolicyParamsValueFrom replaces policy params `valueFrom` objects with
// string values read from either a Secret or a ConfigMap, so gateway-controller receives
// the same JSON shape as inline RestApi policies.
func resolveAPIConfigPolicyParamsValueFrom(ctx context.Context, c client.Client, routeNS string, spec *apiv1.APIConfigData, log *zap.Logger) error {
	for i := range spec.Policies {
		if err := resolvePolicyParamsValueFrom(ctx, c, routeNS, &spec.Policies[i], "api-level", log); err != nil {
			return err
		}
	}
	for i := range spec.Operations {
		for j := range spec.Operations[i].Policies {
			scope := string(spec.Operations[i].Method) + " " + spec.Operations[i].Path
			if err := resolvePolicyParamsValueFrom(ctx, c, routeNS, &spec.Operations[i].Policies[j], scope, log); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolvePolicyParamsValueFrom(ctx context.Context, c client.Client, defaultNS string, p *apiv1.Policy, scope string, log *zap.Logger) error {
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

// tryResolveValueFromMap resolves a leaf map that is exclusively
//
//	{ "valueFrom": { "secretKeyRef":    { name, key, namespace? } } }
//	{ "valueFrom": { "configMapKeyRef": { name, key, namespace? } } }
//
// Returns (value, true, nil) when the map was consumed as a value-from reference,
// (_, false, nil) when it should be treated as a regular nested object and recursed
// into, or (_, true, err) when the shape is malformed or the resource/key is missing.
func tryResolveValueFromMap(ctx context.Context, c client.Client, defaultNS string, m map[string]interface{}, policyName, scope string, log *zap.Logger) (string, bool, error) {
	if len(m) != 1 {
		return "", false, nil
	}
	rawVF, ok := m[valueFromKey]
	if !ok {
		return "", false, nil
	}
	inner, ok := rawVF.(map[string]interface{})
	if !ok {
		return "", true, newInvalidHTTPRouteConfigError("policy %q params: %q must be an object", policyName, valueFromKey)
	}

	kind, ref, err := selectValueFromRef(inner, policyName)
	if err != nil {
		return "", true, err
	}

	name, key, ns, err := readKeyRef(ref, defaultNS, policyName, kind)
	if err != nil {
		return "", true, err
	}

	switch kind {
	case secretKeyRefKey:
		return resolveSecretKeyRef(ctx, c, ns, name, key, policyName, scope, log)
	case configMapKeyRefKey:
		return resolveConfigMapKeyRef(ctx, c, ns, name, key, policyName, scope, log)
	default:
		return "", true, newInvalidHTTPRouteConfigError("policy %q params: unsupported valueFrom kind %q", policyName, kind)
	}
}

// selectValueFromRef enforces that exactly one of secretKeyRef / configMapKeyRef is set
// inside a valueFrom object, and that no unknown sibling keys are present. Returns the
// kind string (secretKeyRefKey | configMapKeyRefKey) and the ref map.
func selectValueFromRef(inner map[string]interface{}, policyName string) (string, map[string]interface{}, error) {
	var selectedKind string
	var selectedRef map[string]interface{}
	var unknown []string
	for k, v := range inner {
		switch k {
		case secretKeyRefKey, configMapKeyRefKey:
			m, ok := v.(map[string]interface{})
			if !ok {
				return "", nil, newInvalidHTTPRouteConfigError("policy %q params: %s must be an object", policyName, k)
			}
			if selectedKind != "" {
				return "", nil, newInvalidHTTPRouteConfigError(
					"policy %q params: valueFrom requires exactly one of %s/%s, got both",
					policyName, secretKeyRefKey, configMapKeyRefKey)
			}
			selectedKind = k
			selectedRef = m
		default:
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		return "", nil, newInvalidHTTPRouteConfigError(
			"policy %q params: unknown valueFrom field(s) %v (expected one of %s, %s)",
			policyName, unknown, secretKeyRefKey, configMapKeyRefKey)
	}
	if selectedKind == "" {
		return "", nil, newInvalidHTTPRouteConfigError(
			"policy %q params: valueFrom requires one of %s or %s",
			policyName, secretKeyRefKey, configMapKeyRefKey)
	}
	return selectedKind, selectedRef, nil
}

// readKeyRef validates {name, key, namespace?} on a secretKeyRef/configMapKeyRef map.
// Returns the resolved (name, key, namespace) where namespace falls back to defaultNS
// when omitted.
func readKeyRef(ref map[string]interface{}, defaultNS, policyName, kind string) (string, string, string, error) {
	name, _ := ref[keyRefFieldName].(string)
	key, _ := ref[keyRefFieldKey].(string)
	name = strings.TrimSpace(name)
	key = strings.TrimSpace(key)
	if name == "" || key == "" {
		return "", "", "", newInvalidHTTPRouteConfigError(
			"policy %q params: %s requires non-empty %q and %q",
			policyName, kind, keyRefFieldName, keyRefFieldKey)
	}
	ns := defaultNS
	if rawNS, ok := ref[keyRefFieldNamespac]; ok {
		s, ok := rawNS.(string)
		if !ok {
			return "", "", "", newInvalidHTTPRouteConfigError(
				"policy %q params: %s.namespace must be a string", policyName, kind)
		}
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			ns = trimmed
		}
	}
	// Reject unknown fields on the ref so typos surface.
	for k := range ref {
		switch k {
		case keyRefFieldName, keyRefFieldKey, keyRefFieldNamespac:
		default:
			return "", "", "", newInvalidHTTPRouteConfigError(
				"policy %q params: unknown field %q on %s (expected %s, %s, %s)",
				policyName, k, kind, keyRefFieldName, keyRefFieldKey, keyRefFieldNamespac)
		}
	}
	return name, key, ns, nil
}

func resolveSecretKeyRef(ctx context.Context, c client.Client, ns, name, key, policyName, scope string, log *zap.Logger) (string, bool, error) {
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
		log.Debug("resolved policy param from Secret (valueFrom.secretKeyRef)",
			zap.String("policy", policyName),
			zap.String("scope", scope),
			zap.String("secretNamespace", ns),
			zap.String("secret", name),
			zap.String("key", key),
			zap.Int("valueLength", len(data)))
	}
	return string(data), true, nil
}

func resolveConfigMapKeyRef(ctx context.Context, c client.Client, ns, name, key, policyName, scope string, log *zap.Logger) (string, bool, error) {
	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, cm); err != nil {
		if apierrors.IsNotFound(err) {
			return "", true, newTransientHTTPRouteConfigError("ConfigMap %s/%s not found: %w", ns, name, err)
		}
		return "", true, err
	}
	if v, ok := cm.Data[key]; ok {
		if v == "" {
			return "", true, newTransientHTTPRouteConfigError("ConfigMap %s/%s has empty data key %q", ns, name, key)
		}
		if log != nil {
			log.Debug("resolved policy param from ConfigMap (valueFrom.configMapKeyRef)",
				zap.String("policy", policyName),
				zap.String("scope", scope),
				zap.String("configMapNamespace", ns),
				zap.String("configMap", name),
				zap.String("key", key),
				zap.String("source", "data"),
				zap.Int("valueLength", len(v)))
		}
		return v, true, nil
	}
	if v, ok := cm.BinaryData[key]; ok {
		if len(v) == 0 {
			return "", true, newTransientHTTPRouteConfigError("ConfigMap %s/%s has empty binaryData key %q", ns, name, key)
		}
		if log != nil {
			log.Debug("resolved policy param from ConfigMap (valueFrom.configMapKeyRef)",
				zap.String("policy", policyName),
				zap.String("scope", scope),
				zap.String("configMapNamespace", ns),
				zap.String("configMap", name),
				zap.String("key", key),
				zap.String("source", "binaryData"),
				zap.Int("valueLength", len(v)))
		}
		return string(v), true, nil
	}
	return "", true, newTransientHTTPRouteConfigError(
		"ConfigMap %s/%s has no key %q in data or binaryData",
		ns, name, key)
}
