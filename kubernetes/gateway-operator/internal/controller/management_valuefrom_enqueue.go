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
	"strings"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func enqueueLlmProvidersForSecret(c client.Client) handlerMapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		sec, ok := obj.(*corev1.Secret)
		if !ok {
			return nil
		}
		list := &apiv1.LlmProviderList{}
		if err := c.List(ctx, list, client.InNamespace(sec.Namespace)); err != nil {
			return nil
		}
		var reqs []reconcile.Request
		for i := range list.Items {
			cr := &list.Items[i]
			if llmProviderReferencesSecret(cr, sec.Name) {
				reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: cr.Namespace, Name: cr.Name}})
			}
		}
		return reqs
	}
}

func enqueueLlmProvidersForConfigMap(c client.Client) handlerMapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return nil
		}
		list := &apiv1.LlmProviderList{}
		if err := c.List(ctx, list); err != nil {
			return nil
		}
		var reqs []reconcile.Request
		for i := range list.Items {
			cr := &list.Items[i]
			if llmProviderReferencesValueFromKind(cr, configMapKeyRefKey, cm.Namespace, cm.Name) {
				reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: cr.Namespace, Name: cr.Name}})
			}
		}
		return reqs
	}
}

func enqueueMcpsForSecret(c client.Client) handlerMapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		sec, ok := obj.(*corev1.Secret)
		if !ok {
			return nil
		}
		list := &apiv1.McpList{}
		if err := c.List(ctx, list); err != nil {
			return nil
		}
		var reqs []reconcile.Request
		for i := range list.Items {
			cr := &list.Items[i]
			if mcpReferencesSecret(cr, sec.Namespace, sec.Name) {
				reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: cr.Namespace, Name: cr.Name}})
			}
		}
		return reqs
	}
}

func enqueueMcpsForConfigMap(c client.Client) handlerMapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return nil
		}
		list := &apiv1.McpList{}
		if err := c.List(ctx, list); err != nil {
			return nil
		}
		var reqs []reconcile.Request
		for i := range list.Items {
			cr := &list.Items[i]
			if mcpReferencesValueFromKind(cr, configMapKeyRefKey, cm.Namespace, cm.Name) {
				reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: cr.Namespace, Name: cr.Name}})
			}
		}
		return reqs
	}
}

func enqueueLlmProxiesForSecret(c client.Client) handlerMapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		sec, ok := obj.(*corev1.Secret)
		if !ok {
			return nil
		}
		list := &apiv1.LlmProxyList{}
		if err := c.List(ctx, list); err != nil {
			return nil
		}
		var reqs []reconcile.Request
		for i := range list.Items {
			cr := &list.Items[i]
			if llmProxyReferencesSecret(cr, sec.Namespace, sec.Name) {
				reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: cr.Namespace, Name: cr.Name}})
			}
		}
		return reqs
	}
}

func enqueueLlmProxiesForConfigMap(c client.Client) handlerMapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return nil
		}
		list := &apiv1.LlmProxyList{}
		if err := c.List(ctx, list); err != nil {
			return nil
		}
		var reqs []reconcile.Request
		for i := range list.Items {
			cr := &list.Items[i]
			if llmProxyReferencesValueFromKind(cr, configMapKeyRefKey, cm.Namespace, cm.Name) {
				reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: cr.Namespace, Name: cr.Name}})
			}
		}
		return reqs
	}
}

func llmProviderReferencesSecret(cr *apiv1.LlmProvider, secretName string) bool {
	if cr.Spec.Upstream.Auth != nil && cr.Spec.Upstream.Auth.Value.ValueFrom != nil {
		if strings.TrimSpace(cr.Spec.Upstream.Auth.Value.ValueFrom.Name) == secretName {
			return true
		}
	}
	// valueFrom.secretKeyRef inside policy params can optionally set a namespace, so
	// a Secret event in this namespace can still be referenced explicitly.
	return llmProviderReferencesValueFromKind(cr, secretKeyRefKey, cr.Namespace, secretName)
}

func llmProviderReferencesValueFromKind(cr *apiv1.LlmProvider, kind, targetNS, targetName string) bool {
	defaultNS := cr.Namespace
	for i := range cr.Spec.Policies {
		pol := &cr.Spec.Policies[i]
		for j := range pol.Paths {
			if pol.Paths[j].Params == nil || len(pol.Paths[j].Params.Raw) == 0 {
				continue
			}
			var root interface{}
			if err := json.Unmarshal(pol.Paths[j].Params.Raw, &root); err != nil {
				continue
			}
			if jsonTreeReferencesValueFrom(root, kind, targetNS, targetName, defaultNS) {
				return true
			}
		}
	}
	return false
}

func mcpReferencesSecret(cr *apiv1.Mcp, secretNS, secretName string) bool {
	if cr.Spec.Upstream.Auth != nil && cr.Spec.Upstream.Auth.Value.ValueFrom != nil {
		if cr.Namespace == secretNS && strings.TrimSpace(cr.Spec.Upstream.Auth.Value.ValueFrom.Name) == secretName {
			return true
		}
	}
	return mcpReferencesValueFromKind(cr, secretKeyRefKey, secretNS, secretName)
}

func mcpReferencesValueFromKind(cr *apiv1.Mcp, kind, targetNS, targetName string) bool {
	defaultNS := cr.Namespace
	for i := range cr.Spec.Policies {
		p := &cr.Spec.Policies[i]
		if p.Params == nil || len(p.Params.Raw) == 0 {
			continue
		}
		var root interface{}
		if err := json.Unmarshal(p.Params.Raw, &root); err != nil {
			continue
		}
		if jsonTreeReferencesValueFrom(root, kind, targetNS, targetName, defaultNS) {
			return true
		}
	}
	return false
}

func llmProxyReferencesSecret(cr *apiv1.LlmProxy, secretNS, secretName string) bool {
	if cr.Spec.Provider.Auth != nil && cr.Spec.Provider.Auth.Value.ValueFrom != nil {
		if cr.Namespace == secretNS && strings.TrimSpace(cr.Spec.Provider.Auth.Value.ValueFrom.Name) == secretName {
			return true
		}
	}
	return llmProxyReferencesValueFromKind(cr, secretKeyRefKey, secretNS, secretName)
}

func llmProxyReferencesValueFromKind(cr *apiv1.LlmProxy, kind, targetNS, targetName string) bool {
	defaultNS := cr.Namespace
	for i := range cr.Spec.Policies {
		pol := &cr.Spec.Policies[i]
		for j := range pol.Paths {
			if pol.Paths[j].Params == nil || len(pol.Paths[j].Params.Raw) == 0 {
				continue
			}
			var root interface{}
			if err := json.Unmarshal(pol.Paths[j].Params.Raw, &root); err != nil {
				continue
			}
			if jsonTreeReferencesValueFrom(root, kind, targetNS, targetName, defaultNS) {
				return true
			}
		}
	}
	return false
}
