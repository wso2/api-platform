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
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
)

// gatewayWatchPredicate mirrors the predicate used in restapi_controller.go
// SetupWithManager: it filters APIGateway events down to those that are
// likely to affect API selection (creation/deletion always pass; updates
// pass when generation or status changes).
func gatewayWatchPredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return true },
		UpdateFunc: func(evt event.UpdateEvent) bool {
			newGw, okN := evt.ObjectNew.(*apiv1.APIGateway)
			oldGw, okO := evt.ObjectOld.(*apiv1.APIGateway)
			if !okN || !okO {
				return true
			}
			if newGw.Generation != oldGw.Generation {
				return true
			}
			return !equality.Semantic.DeepEqual(newGw.Status, oldGw.Status)
		},
	}
}

// enqueueAllOfKind is a generic mapper that turns an APIGateway event into
// reconcile requests for every CR of the supplied list type. This mirrors
// RestApiReconciler.enqueueAPIsForGateway for the new kinds; per-CR label
// filtering still happens inside the reconcile loop via the selector
// configured on the gateway.
func enqueueAllOfKind(c client.Client, list client.ObjectList) handlerMapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		logger := log.FromContext(ctx)
		// Allocate a fresh list instance per call to avoid races when
		// multiple APIGateway watch callbacks run concurrently.
		newList := newObjectListSameType(list)
		if newList == nil {
			logger.Error(nil, "failed to allocate list instance for gateway event", "listType", fmt.Sprintf("%T", list))
			return nil
		}
		if err := c.List(ctx, newList); err != nil {
			logger.Error(err, "failed to list resources for gateway event",
				"namespace", obj.GetNamespace(),
				"name", obj.GetName())
			return nil
		}
		items := extractItems(newList)
		out := make([]reconcile.Request, 0, len(items))
		for _, item := range items {
			out = append(out, reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: item.GetNamespace(),
				Name:      item.GetName(),
			}})
		}
		return out
	}
}

func newObjectListSameType(template client.ObjectList) client.ObjectList {
	switch template.(type) {
	case *apiv1.LlmProviderList:
		return &apiv1.LlmProviderList{}
	case *apiv1.LlmProviderTemplateList:
		return &apiv1.LlmProviderTemplateList{}
	case *apiv1.LlmProxyList:
		return &apiv1.LlmProxyList{}
	case *apiv1.McpList:
		return &apiv1.McpList{}
	case *apiv1.ManagedSecretList:
		return &apiv1.ManagedSecretList{}
	case *apiv1.CertificateList:
		return &apiv1.CertificateList{}
	case *apiv1.ApiKeyList:
		return &apiv1.ApiKeyList{}
	case *apiv1.SubscriptionPlanList:
		return &apiv1.SubscriptionPlanList{}
	case *apiv1.SubscriptionList:
		return &apiv1.SubscriptionList{}
	default:
		return nil
	}
}

// handlerMapFunc matches the signature expected by handler.EnqueueRequestsFromMapFunc.
type handlerMapFunc = func(context.Context, client.Object) []reconcile.Request

// extractItems pulls the *.Items slice off any of the new v1alpha1 list
// types so a single helper can drive all per-kind controller watches.
func extractItems(list client.ObjectList) []client.Object {
	switch v := list.(type) {
	case *apiv1.LlmProviderList:
		out := make([]client.Object, 0, len(v.Items))
		for i := range v.Items {
			out = append(out, &v.Items[i])
		}
		return out
	case *apiv1.LlmProviderTemplateList:
		out := make([]client.Object, 0, len(v.Items))
		for i := range v.Items {
			out = append(out, &v.Items[i])
		}
		return out
	case *apiv1.LlmProxyList:
		out := make([]client.Object, 0, len(v.Items))
		for i := range v.Items {
			out = append(out, &v.Items[i])
		}
		return out
	case *apiv1.McpList:
		out := make([]client.Object, 0, len(v.Items))
		for i := range v.Items {
			out = append(out, &v.Items[i])
		}
		return out
	case *apiv1.ManagedSecretList:
		out := make([]client.Object, 0, len(v.Items))
		for i := range v.Items {
			out = append(out, &v.Items[i])
		}
		return out
	case *apiv1.CertificateList:
		out := make([]client.Object, 0, len(v.Items))
		for i := range v.Items {
			out = append(out, &v.Items[i])
		}
		return out
	case *apiv1.ApiKeyList:
		out := make([]client.Object, 0, len(v.Items))
		for i := range v.Items {
			out = append(out, &v.Items[i])
		}
		return out
	case *apiv1.SubscriptionPlanList:
		out := make([]client.Object, 0, len(v.Items))
		for i := range v.Items {
			out = append(out, &v.Items[i])
		}
		return out
	case *apiv1.SubscriptionList:
		out := make([]client.Object, 0, len(v.Items))
		for i := range v.Items {
			out = append(out, &v.Items[i])
		}
		return out
	}
	return nil
}

// maxConcurrentReconciles returns a sane positive value for the controller
// MaxConcurrentReconciles option, falling back to 1 when unset.
func maxConcurrentReconciles(n int) int {
	if n <= 0 {
		return 1
	}
	return n
}

// deployEnvelopeResource handles the existence-probe + POST/PUT for any
// envelope-shaped (apiVersion/kind/metadata/spec) management-API resource.
// It exists so per-kind adapters need only construct the YAML payload.
func deployEnvelopeResource(
	ctx context.Context,
	gatewayEndpoint, resourcePath, handle string,
	body []byte,
	auth gatewayclient.AuthHeaderFunc,
) error {
	exists, err := gatewayclient.ResourceExists(ctx, gatewayEndpoint, resourcePath, handle, auth)
	if err != nil {
		return err
	}
	return gatewayclient.DeployResource(ctx, gatewayEndpoint, resourcePath, handle, body, exists, gatewayclient.PayloadContentTypeYAML, auth)
}

// crSetupOptions records shared SetupWithManager parameters used by every
// per-kind controller for the new management-API CRDs.
type crSetupOptions struct {
	mgr ctrl.Manager
}

// init dummy use to silence unused-warning for ctrl pkg in helpers when
// only manager type is referenced via crSetupOptions.
var _ = crSetupOptions{}
