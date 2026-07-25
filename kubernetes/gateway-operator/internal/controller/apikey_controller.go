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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"log/slog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/secretsource"
)

const (
	apiKeyFinalizer = "gateway.api-platform.wso2.com/apikey-finalizer"
	// apiKeySecretBindingFingerprintAnn records the Kubernetes Secret backing
	// apiKey.valueFrom at last successful programmatic deploy so value-only Secret
	// rotations are observed even though ApiKey metadata.generation is unchanged.
	apiKeySecretBindingFingerprintAnn = "gateway.api-platform.wso2.com/last-applied-apikey-secret-rv"
)

// ApiKeyReconciler reconciles ApiKey CRs against the gateway-controller
// management API at /<parent>/{parentName}/api-keys/{name}.
type ApiKeyReconciler struct {
	GenericReconciler
}

// NewApiKeyReconciler constructs a fully wired ApiKey reconciler.
func NewApiKeyReconciler(c client.Client, cfg *config.OperatorConfig, logger *slog.Logger, tracker *ResourceTracker) *ApiKeyReconciler {
	r := &ApiKeyReconciler{}
	r.Client = c
	r.Config = cfg
	r.Logger = logger
	r.Tracker = tracker
	r.Adapter = &apiKeyAdapter{}
	return r
}

//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=apikeys,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=apikeys/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=apikeys/finalizers,verbs=update

// SetupWithManager registers the controller with mgr.
func (r *ApiKeyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles(r.Config.Reconciliation.MaxConcurrentReconciles)}
	return ctrl.NewControllerManagedBy(mgr).
		Named("apikey").
		WithOptions(opts).
		For(&apiv1.ApiKey{}).
		Watches(&apiv1.APIGateway{},
			handler.EnqueueRequestsFromMapFunc(enqueueAllOfKind(r.Client, &apiv1.ApiKeyList{})),
			builder.WithPredicates(gatewayWatchPredicate())).
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(enqueueApiKeysReferencingSecret(r.Client, r.Logger))).
		Complete(r)
}

// enqueueApiKeysReferencingSecret maps a Secret event to reconcile requests
// for ApiKeys in the same namespace whose spec.apiKey.valueFrom names that Secret.
// On list failure, logs and fails open by enqueueing every ApiKey in the cluster so
// a transient cache/RBAC/List error cannot drop Secret-driven reconciliation.
func enqueueApiKeysReferencingSecret(c client.Client, log *slog.Logger) handlerMapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		s, ok := obj.(*corev1.Secret)
		if !ok || s == nil {
			return nil
		}
		ns := s.GetNamespace()
		if ns == "" {
			return nil
		}
		var list apiv1.ApiKeyList
		if err := c.List(ctx, &list, client.InNamespace(ns)); err != nil {
			if log != nil {
				log.Error("Secret watch: listing ApiKeys in namespace failed, fail-open to full ApiKey reconcile",
					slog.Any("error", err),
					slog.String("secretNamespace", ns),
					slog.String("secretName", s.GetName()))
			}
			return enqueueAllOfKind(c, &apiv1.ApiKeyList{})(ctx, obj)
		}
		out := make([]reconcile.Request, 0)
		for i := range list.Items {
			key := &list.Items[i]
			if key.Spec.ApiKey == nil || key.Spec.ApiKey.ValueFrom == nil {
				continue
			}
			if key.Spec.ApiKey.ValueFrom.Name != s.GetName() {
				continue
			}
			out = append(out, reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: key.Namespace,
				Name:      key.Name,
			}})
		}
		return out
	}
}

type apiKeyAdapter struct{}

func (a *apiKeyAdapter) Kind() string             { return "ApiKey" }
func (a *apiKeyAdapter) FinalizerName() string    { return apiKeyFinalizer }
func (a *apiKeyAdapter) NewObject() client.Object { return &apiv1.ApiKey{} }
func (a *apiKeyAdapter) IsUUIDKeyed() bool        { return false }

func (a *apiKeyAdapter) Handle(obj client.Object) string {
	return obj.GetName()
}

func (a *apiKeyAdapter) GetStatus(obj client.Object) *apiv1.ResourceStatus {
	return &obj.(*apiv1.ApiKey).Status
}

func (a *apiKeyAdapter) SetStatusId(obj client.Object, id string) {
	obj.(*apiv1.ApiKey).Status.Id = id
}

func (a *apiKeyAdapter) GatewaySelectionKey(obj client.Object) (string, map[string]string) {
	cr := obj.(*apiv1.ApiKey)
	return cr.Namespace, cr.Labels
}

func (a *apiKeyAdapter) Deploy(ctx context.Context, k8sClient client.Client, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) (DeployResult, error) {
	cr := obj.(*apiv1.ApiKey)

	apiKeyVal, _, err := secretsource.ResolveOptional(ctx, k8sClient, "spec.apiKey", cr.Spec.ApiKey, cr.Namespace)
	if err != nil {
		return DeployResult{}, classifySecretSourceResolveError(err)
	}

	payload := gatewayclient.APIKeyCreatePayload{
		Name:          cr.Name,
		ApiKey:        apiKeyVal,
		ExternalRefId: stringDeref(cr.Spec.ExternalRefId),
		Issuer:        stringDeref(cr.Spec.Issuer),
		MaskedApiKey:  stringDeref(cr.Spec.MaskedApiKey),
	}
	if cr.Spec.DisplayName != nil {
		payload.DisplayName = *cr.Spec.DisplayName
	}
	if cr.Spec.ExpiresAt != nil {
		t := cr.Spec.ExpiresAt.Time
		payload.ExpiresAt = (*time.Time)(&t)
	} else if cr.Spec.ExpiresIn != nil {
		payload.ExpiresIn = &gatewayclient.APIKeyExpiry{
			Duration: cr.Spec.ExpiresIn.Duration,
			Unit:     cr.Spec.ExpiresIn.Unit,
		}
	}

	exists, err := gatewayclient.APIKeyExists(ctx, gatewayEndpoint, cr.Spec.ParentRef.Kind, cr.Spec.ParentRef.Name, cr.Name, authFn)
	if err != nil {
		return DeployResult{}, err
	}
	if err := gatewayclient.DeployAPIKey(ctx, gatewayEndpoint, cr.Spec.ParentRef.Kind, cr.Spec.ParentRef.Name, cr.Name, payload, exists, authFn); err != nil {
		return DeployResult{}, err
	}
	return DeployResult{}, nil
}

func (a *apiKeyAdapter) Delete(ctx context.Context, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) error {
	cr := obj.(*apiv1.ApiKey)
	return gatewayclient.DeleteAPIKey(ctx, gatewayEndpoint, cr.Spec.ParentRef.Kind, cr.Spec.ParentRef.Name, cr.Name, authFn)
}

func stringDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func apiKeySecretBindingFingerprint(ctx context.Context, c client.Client, cr *apiv1.ApiKey) (string, error) {
	if cr.Spec.ApiKey == nil || cr.Spec.ApiKey.ValueFrom == nil {
		return "", nil
	}
	vf := cr.Spec.ApiKey.ValueFrom
	var sec corev1.Secret
	if err := c.Get(ctx, types.NamespacedName{Namespace: cr.Namespace, Name: vf.Name}, &sec); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s@%s", vf.Name, vf.Key, sec.ResourceVersion), nil
}

func (a *apiKeyAdapter) needsRedeployForExternalDeps(ctx context.Context, c client.Client, obj client.Object) (bool, error) {
	cr := obj.(*apiv1.ApiKey)
	fp, err := apiKeySecretBindingFingerprint(ctx, c, cr)
	if err != nil {
		return false, err
	}
	if fp == "" {
		return false, nil
	}
	cur := ""
	if obj.GetAnnotations() != nil {
		cur = obj.GetAnnotations()[apiKeySecretBindingFingerprintAnn]
	}
	return cur != fp, nil
}

func (a *apiKeyAdapter) onExternalDepsApplied(ctx context.Context, c client.Client, obj client.Object, _ string) error {
	var latest apiv1.ApiKey
	if err := c.Get(ctx, types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}, &latest); err != nil {
		return err
	}
	fp, err := apiKeySecretBindingFingerprint(ctx, c, &latest)
	if err != nil {
		return err
	}
	if fp == "" {
		return nil
	}
	base := latest.DeepCopy()
	ann := base.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
	} else {
		cp := make(map[string]string, len(ann)+1)
		for k, v := range ann {
			cp[k] = v
		}
		ann = cp
	}
	ann[apiKeySecretBindingFingerprintAnn] = fp
	latest.SetAnnotations(ann)
	return c.Patch(ctx, &latest, client.MergeFrom(base))
}
