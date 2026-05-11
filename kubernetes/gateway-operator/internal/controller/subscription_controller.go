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
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/secretsource"
)

const (
	subscriptionFinalizer                          = "gateway.api-platform.wso2.com/subscription-finalizer"
	annSubscriptionLastAppliedCreateFingerprint    = "gateway.api-platform.wso2.com/subscription-last-applied-create-fingerprint"
	annSubscriptionLastAppliedTokenSecretBindingRV = "gateway.api-platform.wso2.com/last-applied-subscription-token-secret-rv"
)

// SubscriptionReconciler reconciles Subscription CRs against /subscriptions.
// The gateway issues a UUID id on first POST that the controller persists
// to .status.id.
type SubscriptionReconciler struct {
	GenericReconciler
}

// NewSubscriptionReconciler constructs a fully wired Subscription reconciler.
func NewSubscriptionReconciler(c client.Client, cfg *config.OperatorConfig, logger *zap.Logger, tracker *ResourceTracker) *SubscriptionReconciler {
	r := &SubscriptionReconciler{}
	r.Client = c
	r.Config = cfg
	r.Logger = logger
	r.Tracker = tracker
	r.Adapter = &subscriptionAdapter{}
	return r
}

//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=subscriptions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=subscriptions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=subscriptions/finalizers,verbs=update

// SetupWithManager registers the controller with mgr.
func (r *SubscriptionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles(r.Config.Reconciliation.MaxConcurrentReconciles)}
	return ctrl.NewControllerManagedBy(mgr).
		Named("subscription").
		WithOptions(opts).
		For(&apiv1.Subscription{}).
		Watches(&apiv1.APIGateway{},
			handler.EnqueueRequestsFromMapFunc(enqueueAllOfKind(r.Client, &apiv1.SubscriptionList{})),
			builder.WithPredicates(gatewayWatchPredicate())).
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(enqueueSubscriptionsReferencingSecret(r.Client, r.Logger))).
		Complete(r)
}

// enqueueSubscriptionsReferencingSecret maps Secret events to Subscriptions in the
// same namespace that source spec.subscriptionToken from that Secret.
func enqueueSubscriptionsReferencingSecret(c client.Client, zapLog *zap.Logger) handlerMapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		s, ok := obj.(*corev1.Secret)
		if !ok || s == nil {
			return nil
		}
		ns := s.GetNamespace()
		if ns == "" {
			return nil
		}
		var list apiv1.SubscriptionList
		if err := c.List(ctx, &list, client.InNamespace(ns)); err != nil {
			if zapLog != nil {
				zapLog.Error("Secret watch: listing Subscriptions in namespace failed, fail-open to full Subscription reconcile",
					zap.Error(err),
					zap.String("secretNamespace", ns),
					zap.String("secretName", s.GetName()))
			}
			return enqueueAllOfKind(c, &apiv1.SubscriptionList{})(ctx, obj)
		}
		out := make([]reconcile.Request, 0)
		for i := range list.Items {
			sub := &list.Items[i]
			vf := sub.Spec.SubscriptionToken.ValueFrom
			if vf == nil || vf.Name != s.GetName() {
				continue
			}
			out = append(out, reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: sub.Namespace,
				Name:      sub.Name,
			}})
		}
		return out
	}
}

type subscriptionAdapter struct{}

func (a *subscriptionAdapter) Kind() string             { return "Subscription" }
func (a *subscriptionAdapter) FinalizerName() string    { return subscriptionFinalizer }
func (a *subscriptionAdapter) NewObject() client.Object { return &apiv1.Subscription{} }
func (a *subscriptionAdapter) IsUUIDKeyed() bool        { return true }

func (a *subscriptionAdapter) Handle(obj client.Object) string {
	return obj.(*apiv1.Subscription).Status.Id
}

func (a *subscriptionAdapter) GetStatus(obj client.Object) *apiv1.ResourceStatus {
	return &obj.(*apiv1.Subscription).Status
}

func (a *subscriptionAdapter) SetStatusId(obj client.Object, id string) {
	obj.(*apiv1.Subscription).Status.Id = id
}

func (a *subscriptionAdapter) GatewaySelectionKey(obj client.Object) (string, map[string]string) {
	cr := obj.(*apiv1.Subscription)
	return cr.Namespace, cr.Labels
}

func (a *subscriptionAdapter) Deploy(ctx context.Context, k8sClient client.Client, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) (DeployResult, error) {
	cr := obj.(*apiv1.Subscription)

	if cr.Status.Id == "" {
		token, err := secretsource.Resolve(ctx, k8sClient, "spec.subscriptionToken", cr.Spec.SubscriptionToken, cr.Namespace)
		if err != nil {
			return DeployResult{}, classifySecretSourceResolveError(err)
		}
		resolvedPlanID, err := resolveSubscriptionPlanIDForCreate(ctx, k8sClient, cr)
		if err != nil {
			return DeployResult{}, err
		}
		payload := gatewayclient.SubscriptionCreatePayload{
			ApiId:                 cr.Spec.ApiId,
			SubscriptionToken:     token,
			ApplicationId:         cr.Spec.ApplicationId,
			SubscriptionPlanId:    resolvedPlanID,
			BillingCustomerId:     cr.Spec.BillingCustomerId,
			BillingSubscriptionId: cr.Spec.BillingSubscriptionId,
			Status:                cr.Spec.Status,
		}
		resp, err := gatewayclient.CreateSubscription(ctx, gatewayEndpoint, payload, authFn)
		if err != nil {
			var nr *gatewayclient.NonRetryableError
			if errors.As(err, &nr) && nr.StatusCode == http.StatusConflict {
				recoveredID, recErr := gatewayclient.FindSubscriptionIDByAPIAndApplication(ctx, gatewayEndpoint, cr.Spec.ApiId, cr.Spec.ApplicationId, authFn)
				if recErr != nil {
					return DeployResult{}, recErr
				}
				if recoveredID != "" {
					if fp, fpErr := subscriptionCreateFingerprint(cr); fpErr == nil {
						_ = setSubscriptionLastAppliedCreateFingerprint(ctx, k8sClient, cr, fp)
					}
					return DeployResult{Id: recoveredID}, nil
				}
				return DeployResult{}, &gatewayclient.RetryableError{
					StatusCode: http.StatusConflict,
					Err:        fmt.Errorf("subscription create conflicted but existing id is not yet discoverable; retrying: %w", err),
				}
			}
			return DeployResult{}, err
		}
		// Persist a fingerprint of create-only fields so later reconciles can detect forbidden updates.
		if fp, fpErr := subscriptionCreateFingerprint(cr); fpErr == nil {
			// Best-effort; failure to persist the fingerprint should not cause gateway duplicate creates.
			_ = setSubscriptionLastAppliedCreateFingerprint(ctx, k8sClient, cr, fp)
		}
		return DeployResult{Id: resp.Id}, nil
	}

	// If this Subscription is already deployed (Status.Id set), then create-only fields must remain unchanged.
	if lastFp := subscriptionLastAppliedCreateFingerprint(cr); lastFp != "" {
		if fp, fpErr := subscriptionCreateFingerprint(cr); fpErr == nil && lastFp != fp {
			return DeployResult{}, &gatewayclient.NonRetryableError{
				Err: fmt.Errorf("create-only fields for Subscription (spec.apiId, spec.subscriptionToken, spec.applicationId, spec.subscriptionPlanId, spec.billingCustomerId, spec.billingSubscriptionId) are immutable once status.id is set; delete and recreate the Subscription to apply changes"),
			}
		}
	} else {
		// No prior fingerprint stored (e.g. pre-upgrade CR): establish a baseline.
		if fp, fpErr := subscriptionCreateFingerprint(cr); fpErr == nil {
			_ = setSubscriptionLastAppliedCreateFingerprint(ctx, k8sClient, cr, fp)
		}
	}

	// Secret-backed token: management PUT only supports status, not token rotation.
	fpNow, err := subscriptionTokenSecretBindingFingerprint(ctx, k8sClient, cr)
	if err != nil {
		return DeployResult{}, err
	}
	if fpNow != "" {
		prev := subscriptionLastAppliedTokenSecretBindingFingerprint(cr)
		if prev != "" && prev != fpNow {
			return DeployResult{}, &gatewayclient.NonRetryableError{
				Err: fmt.Errorf("spec.subscriptionToken valueFrom Secret data changed but the gateway management API cannot update an existing subscription token (only status). Delete this Subscription CR and recreate it to register the new token"),
			}
		}
	}

	if cr.Spec.Status != nil {
		updatePayload := gatewayclient.SubscriptionUpdatePayload{Status: cr.Spec.Status}
		if _, err := gatewayclient.UpdateSubscription(ctx, gatewayEndpoint, cr.Status.Id, updatePayload, authFn); err != nil {
			return DeployResult{}, err
		}
	}
	return DeployResult{Id: cr.Status.Id}, nil
}

// resolveSubscriptionPlanIDForCreate allows Subscription.spec.subscriptionPlanId
// to be either a literal gateway UUID or a same-namespace SubscriptionPlan name.
// If a SubscriptionPlan CR with that name exists but has no status.id yet, we
// return a retryable error so reconciliation naturally retries after the plan is
// created in the gateway.
func resolveSubscriptionPlanIDForCreate(ctx context.Context, k8sClient client.Client, cr *apiv1.Subscription) (*string, error) {
	if cr.Spec.SubscriptionPlanId == nil || *cr.Spec.SubscriptionPlanId == "" {
		return nil, nil
	}
	planRef := *cr.Spec.SubscriptionPlanId
	var sp apiv1.SubscriptionPlan
	err := k8sClient.Get(ctx, types.NamespacedName{Name: planRef, Namespace: cr.Namespace}, &sp)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Treat as a literal plan UUID/ID and pass through untouched.
			return cr.Spec.SubscriptionPlanId, nil
		}
		return nil, &gatewayclient.RetryableError{StatusCode: 0, Err: fmt.Errorf("get SubscriptionPlan %q: %w", planRef, err)}
	}
	if sp.Status.Id == "" {
		return nil, &gatewayclient.RetryableError{StatusCode: 0, Err: fmt.Errorf("SubscriptionPlan %q exists but status.id is not set yet; waiting for gateway deployment", planRef)}
	}
	id := sp.Status.Id
	return &id, nil
}

type subscriptionCreateFingerprintData struct {
	ApiId                 string                  `json:"apiId"`
	SubscriptionToken     apiv1.SecretValueSource `json:"subscriptionToken"`
	ApplicationId         *string                 `json:"applicationId,omitempty"`
	SubscriptionPlanId    *string                 `json:"subscriptionPlanId,omitempty"`
	BillingCustomerId     *string                 `json:"billingCustomerId,omitempty"`
	BillingSubscriptionId *string                 `json:"billingSubscriptionId,omitempty"`
}

func subscriptionCreateFingerprint(cr *apiv1.Subscription) (string, error) {
	data := subscriptionCreateFingerprintData{
		ApiId:                 cr.Spec.ApiId,
		SubscriptionToken:     cr.Spec.SubscriptionToken,
		ApplicationId:         cr.Spec.ApplicationId,
		SubscriptionPlanId:    cr.Spec.SubscriptionPlanId,
		BillingCustomerId:     cr.Spec.BillingCustomerId,
		BillingSubscriptionId: cr.Spec.BillingSubscriptionId,
	}
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return base64.RawStdEncoding.EncodeToString(sum[:]), nil
}

func subscriptionLastAppliedCreateFingerprint(cr *apiv1.Subscription) string {
	if cr.Annotations == nil {
		return ""
	}
	return cr.Annotations[annSubscriptionLastAppliedCreateFingerprint]
}

func setSubscriptionLastAppliedCreateFingerprint(ctx context.Context, k8sClient client.Client, cr *apiv1.Subscription, fingerprint string) error {
	orig := cr.DeepCopy()
	if cr.Annotations == nil {
		cr.Annotations = map[string]string{}
	}
	cr.Annotations[annSubscriptionLastAppliedCreateFingerprint] = fingerprint
	return k8sClient.Patch(ctx, cr, client.MergeFrom(orig))
}

func subscriptionTokenSecretBindingFingerprint(ctx context.Context, c client.Client, cr *apiv1.Subscription) (string, error) {
	vf := cr.Spec.SubscriptionToken.ValueFrom
	if vf == nil {
		return "", nil
	}
	var sec corev1.Secret
	if err := c.Get(ctx, types.NamespacedName{Namespace: cr.Namespace, Name: vf.Name}, &sec); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s@%s", vf.Name, vf.Key, sec.ResourceVersion), nil
}

func subscriptionLastAppliedTokenSecretBindingFingerprint(cr *apiv1.Subscription) string {
	if cr.Annotations == nil {
		return ""
	}
	return cr.Annotations[annSubscriptionLastAppliedTokenSecretBindingRV]
}

func (a *subscriptionAdapter) needsRedeployForExternalDeps(ctx context.Context, c client.Client, obj client.Object) (bool, error) {
	cr := obj.(*apiv1.Subscription)
	fp, err := subscriptionTokenSecretBindingFingerprint(ctx, c, cr)
	if err != nil {
		return false, err
	}
	if fp == "" {
		return false, nil
	}
	cur := ""
	if obj.GetAnnotations() != nil {
		cur = obj.GetAnnotations()[annSubscriptionLastAppliedTokenSecretBindingRV]
	}
	return cur != fp, nil
}

func (a *subscriptionAdapter) onExternalDepsApplied(ctx context.Context, c client.Client, obj client.Object, _ string) error {
	var latest apiv1.Subscription
	if err := c.Get(ctx, types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}, &latest); err != nil {
		return err
	}
	fp, err := subscriptionTokenSecretBindingFingerprint(ctx, c, &latest)
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
	ann[annSubscriptionLastAppliedTokenSecretBindingRV] = fp
	latest.SetAnnotations(ann)
	return c.Patch(ctx, &latest, client.MergeFrom(base))
}

func (a *subscriptionAdapter) Delete(ctx context.Context, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) error {
	cr := obj.(*apiv1.Subscription)
	if cr.Status.Id == "" {
		return nil
	}
	return gatewayclient.DeleteSubscription(ctx, gatewayEndpoint, cr.Status.Id, authFn)
}
