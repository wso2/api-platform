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

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
)

// enqueueLlmProvidersReferencingTemplate returns a mapper that enqueues every
// LlmProvider in the template's namespace whose spec.template equals the
// template's metadata.name.
func enqueueLlmProvidersReferencingTemplate(c client.Client) handlerMapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		tmpl, ok := obj.(*apiv1.LlmProviderTemplate)
		if !ok {
			return nil
		}
		list := &apiv1.LlmProviderList{}
		if err := c.List(ctx, list, client.InNamespace(tmpl.Namespace)); err != nil {
			return nil
		}
		out := make([]reconcile.Request, 0)
		for i := range list.Items {
			if list.Items[i].Spec.Template == tmpl.Name {
				out = append(out, reconcile.Request{NamespacedName: types.NamespacedName{
					Namespace: list.Items[i].Namespace,
					Name:      list.Items[i].Name,
				}})
			}
		}
		return out
	}
}

// enqueueLlmProxiesReferencingProvider enqueues LlmProxy CRs whose
// spec.provider.id matches the reconciled LlmProvider name.
func enqueueLlmProxiesReferencingProvider(c client.Client) handlerMapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		prov, ok := obj.(*apiv1.LlmProvider)
		if !ok {
			return nil
		}
		list := &apiv1.LlmProxyList{}
		if err := c.List(ctx, list, client.InNamespace(prov.Namespace)); err != nil {
			return nil
		}
		out := make([]reconcile.Request, 0)
		for i := range list.Items {
			if list.Items[i].Spec.Provider.Id == prov.Name {
				out = append(out, reconcile.Request{NamespacedName: types.NamespacedName{
					Namespace: list.Items[i].Namespace,
					Name:      list.Items[i].Name,
				}})
			}
		}
		return out
	}
}
