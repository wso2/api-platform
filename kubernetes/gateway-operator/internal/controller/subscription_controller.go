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
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/secretsource"
)

const (
	subscriptionFinalizer                          = "gateway.api-platform.wso2.com/subscription-finalizer"
	annSubscriptionLastAppliedImmutableFingerprint = "gateway.api-platform.wso2.com/subscription-last-applied-immutable-fingerprint"
	// annSubscriptionLegacyCreateFingerprint was written by earlier operator versions, when plan and token were
	// also immutable. It is removed when the immutable fingerprint is first stored.
	annSubscriptionLegacyCreateFingerprint         = "gateway.api-platform.wso2.com/subscription-last-applied-create-fingerprint"
	annSubscriptionLastAppliedTokenSecretBindingRV = "gateway.api-platform.wso2.com/last-applied-subscription-token-secret-rv"
)

// SubscriptionReconciler reconciles Subscription CRs against /subscriptions.
// The gateway issues a UUID id on first POST (or uses spec.externalRefId) that
// the controller persists to .status.id.
type SubscriptionReconciler struct {
	GenericReconciler
}

// NewSubscriptionReconciler constructs a fully wired Subscription reconciler.
func NewSubscriptionReconciler(c client.Client, cfg *config.OperatorConfig, logger *slog.Logger, tracker *ResourceTracker) *SubscriptionReconciler {
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
func enqueueSubscriptionsReferencingSecret(c client.Client, log *slog.Logger) handlerMapFunc {
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
			if log != nil {
				log.Error("Secret watch: listing Subscriptions in namespace failed, fail-open to full Subscription reconcile",
					slog.Any("error", err),
					slog.String("secretNamespace", ns),
					slog.String("secretName", s.GetName()))
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

	// A deployed subscription still gets its status when the token or plan can't be resolved, so the
	// resolution error is held until the status has been applied.
	var resolveErr error
	token, err := secretsource.Resolve(ctx, k8sClient, "spec.subscriptionToken", cr.Spec.SubscriptionToken, cr.Namespace)
	if err != nil {
		resolveErr = classifySecretSourceResolveError(err)
	}
	var resolvedPlanID *string
	if resolveErr == nil {
		resolvedPlanID, resolveErr = resolveSubscriptionPlanID(ctx, k8sClient, cr)
	}

	if cr.Status.Id == "" {
		if resolveErr != nil {
			return DeployResult{}, resolveErr
		}
		payload := gatewayclient.SubscriptionCreatePayload{
			ExternalRefId:         cr.Spec.ExternalRefId,
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
				recoveredID, recErr := recoverSubscriptionIDAfterConflict(ctx, gatewayEndpoint, cr, authFn)
				if recErr != nil {
					return DeployResult{}, recErr
				}
				if recoveredID != "" {
					// Adopting a subscription another CR already manages would let this CR overwrite its
					// token and plan, and deleting either CR would delete it for both.
					owned, ownErr := subscriptionIDOwnedByAnotherCR(ctx, k8sClient, gatewayEndpoint, cr, recoveredID)
					if ownErr != nil {
						return DeployResult{}, ownErr
					}
					if owned {
						return DeployResult{}, subscriptionConflictError()
					}
					// The existing subscription may predate spec changes; bring its mutable fields in line.
					if err := applySubscriptionUpdate(ctx, gatewayEndpoint, recoveredID, cr, token, resolvedPlanID, authFn); err != nil {
						return DeployResult{}, err
					}
					if fp, fpErr := subscriptionImmutableFingerprint(cr); fpErr == nil {
						_ = setSubscriptionLastAppliedImmutableFingerprint(ctx, k8sClient, cr, fp)
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
		// Persist a fingerprint of immutable fields so later reconciles can detect forbidden updates.
		if fp, fpErr := subscriptionImmutableFingerprint(cr); fpErr == nil {
			// Best-effort; failure to persist the fingerprint should not cause gateway duplicate creates.
			_ = setSubscriptionLastAppliedImmutableFingerprint(ctx, k8sClient, cr, fp)
		}
		return DeployResult{Id: resp.Id}, nil
	}

	// If this Subscription is already deployed (Status.Id set), then immutable fields must remain unchanged.
	if lastFp := subscriptionLastAppliedImmutableFingerprint(cr); lastFp != "" {
		if fp, fpErr := subscriptionImmutableFingerprint(cr); fpErr == nil && lastFp != fp {
			return DeployResult{}, &gatewayclient.NonRetryableError{
				Err: fmt.Errorf("spec.externalRefId, spec.apiId, spec.applicationId, spec.billingCustomerId and spec.billingSubscriptionId are immutable once status.id is set; delete and recreate the Subscription to apply changes"),
			}
		}
	} else {
		// No prior fingerprint stored (a CR from an earlier operator version, or a lost annotation write).
		// The spec may hold an edit an earlier version rejected, so it becomes the baseline only once the
		// gateway subscription is confirmed to still match it.
		if err := verifyDeployedSubscriptionMatchesSpec(ctx, gatewayEndpoint, cr, authFn); err != nil {
			return DeployResult{}, err
		}
		if fp, fpErr := subscriptionImmutableFingerprint(cr); fpErr == nil {
			_ = setSubscriptionLastAppliedImmutableFingerprint(ctx, k8sClient, cr, fp)
		}
	}

	if resolveErr != nil {
		// Only a lasting failure, such as a missing Secret, applies the status alone. A transient one, such
		// as a SubscriptionPlan not deployed yet, is retried and then applies everything together.
		var nr *gatewayclient.NonRetryableError
		if errors.As(resolveErr, &nr) {
			return DeployResult{}, updateSubscriptionStatusOnFailure(ctx, gatewayEndpoint, cr.Status.Id, cr, resolveErr, authFn)
		}
		return DeployResult{}, resolveErr
	}
	// status, subscriptionPlanId and subscriptionToken are updated in place. Sending the token on every
	// update also applies a rotated valueFrom Secret, since Secret drift triggers this redeploy.
	if err := applySubscriptionUpdate(ctx, gatewayEndpoint, cr.Status.Id, cr, token, resolvedPlanID, authFn); err != nil {
		return DeployResult{}, err
	}
	return DeployResult{Id: cr.Status.Id}, nil
}

// applySubscriptionUpdate PUTs the CR's status, plan and token to the subscription. When the gateway rejects
// the plan or the token, the status alone is still applied.
func applySubscriptionUpdate(ctx context.Context, gatewayEndpoint, subscriptionID string, cr *apiv1.Subscription, token string, planID *string, authFn gatewayclient.AuthHeaderFunc) error {
	err := updateSubscriptionMutableFields(ctx, gatewayEndpoint, subscriptionID, cr, token, planID, authFn)
	var nr *gatewayclient.NonRetryableError
	if errors.As(err, &nr) && (nr.StatusCode == http.StatusBadRequest || nr.StatusCode == http.StatusConflict) {
		return updateSubscriptionStatusOnFailure(ctx, gatewayEndpoint, subscriptionID, cr, err, authFn)
	}
	return err
}

// updateSubscriptionStatusOnFailure applies spec.status alone after the plan or token could not be applied,
// so a broken plan or token reference never blocks revoking or deactivating the subscription. It returns
// cause, so the CR still reports why the full update failed, unless the status update fails transiently:
// then a retryable error is returned so the status is applied on a later attempt.
func updateSubscriptionStatusOnFailure(ctx context.Context, gatewayEndpoint, subscriptionID string, cr *apiv1.Subscription, cause error, authFn gatewayclient.AuthHeaderFunc) error {
	if cr.Spec.Status == nil {
		return cause
	}
	_, err := gatewayclient.UpdateSubscriptionStatus(ctx, gatewayEndpoint, subscriptionID, cr.Spec.Status, authFn)
	var retryable *gatewayclient.RetryableError
	if errors.As(err, &retryable) {
		return &gatewayclient.RetryableError{
			StatusCode: retryable.StatusCode,
			Err:        fmt.Errorf("applying spec.status after the update failed (%v): %w", cause, err),
		}
	}
	return cause
}

// verifyDeployedSubscriptionMatchesSpec checks that the gateway subscription in status.id belongs to
// spec.apiId and spec.applicationId, has spec.billingCustomerId and spec.billingSubscriptionId, and, on
// gateways that report externalRefId, matches spec.externalRefId.
func verifyDeployedSubscriptionMatchesSpec(ctx context.Context, gatewayEndpoint string, cr *apiv1.Subscription, authFn gatewayclient.AuthHeaderFunc) error {
	subscriptions, err := gatewayclient.ListSubscriptions(ctx, gatewayEndpoint, cr.Spec.ApiId, nil, authFn)
	if err != nil {
		return err
	}
	for i := range subscriptions {
		sub := &subscriptions[i]
		if sub.Id != cr.Status.Id {
			continue
		}
		externalRefIDMatches := sub.ExternalRefId == nil || cr.Spec.ExternalRefId == nil ||
			*cr.Spec.ExternalRefId == "" || *cr.Spec.ExternalRefId == sub.Id
		if gatewayclient.SameApplication(sub.ApplicationId, cr.Spec.ApplicationId) && externalRefIDMatches &&
			sameOptionalValue(sub.BillingCustomerId, cr.Spec.BillingCustomerId) &&
			sameOptionalValue(sub.BillingSubscriptionId, cr.Spec.BillingSubscriptionId) {
			return nil
		}
		break
	}
	return &gatewayclient.NonRetryableError{
		Err: fmt.Errorf("the gateway subscription in status.id does not match spec.externalRefId, spec.apiId, " +
			"spec.applicationId, spec.billingCustomerId and spec.billingSubscriptionId; these fields are immutable " +
			"once status.id is set, so delete and recreate the Subscription to apply changes"),
	}
}

// subscriptionIDOwnedByAnotherCR reports whether another Subscription CR that targets the same gateway
// already holds id in status.id.
func subscriptionIDOwnedByAnotherCR(ctx context.Context, k8sClient client.Client, gatewayEndpoint string, cr *apiv1.Subscription, id string) (bool, error) {
	var list apiv1.SubscriptionList
	if err := k8sClient.List(ctx, &list); err != nil {
		return false, &gatewayclient.RetryableError{Err: fmt.Errorf("list Subscriptions: %w", err)}
	}
	for i := range list.Items {
		other := &list.Items[i]
		if other.Status.Id != id || (other.Namespace == cr.Namespace && other.Name == cr.Name) {
			continue
		}
		// Deleting a CR deletes its subscription from every gateway it matches, so any shared gateway counts.
		for _, gw := range registry.GetGatewayRegistry().FindMatchingGateways(other.Namespace, other.Labels) {
			if gw.GetGatewayServiceEndpoint() == gatewayEndpoint {
				return true, nil
			}
		}
	}
	return false, nil
}

// subscriptionConflictError reports a create conflict without saying whether the id or the token collided,
// so the error never confirms that a token is in use.
func subscriptionConflictError() error {
	return &gatewayclient.NonRetryableError{
		StatusCode: http.StatusConflict,
		Err: fmt.Errorf("the subscription conflicts with an existing subscription on the gateway; " +
			"check that spec.externalRefId and spec.subscriptionToken are not used by another subscription"),
	}
}

// updateSubscriptionMutableFields PUTs the CR's status, plan and token to the gateway subscription.
// The plan is always sent, so a plan removed from the CR is cleared on the gateway.
func updateSubscriptionMutableFields(ctx context.Context, gatewayEndpoint, subscriptionID string, cr *apiv1.Subscription, token string, planID *string, authFn gatewayclient.AuthHeaderFunc) error {
	payload := gatewayclient.SubscriptionUpdatePayload{
		Status:             cr.Spec.Status,
		SubscriptionPlanId: planID,
		SubscriptionToken:  &token,
	}
	resp, err := gatewayclient.UpdateSubscription(ctx, gatewayEndpoint, subscriptionID, payload, authFn)
	if err != nil {
		return err
	}
	// Gateways without in-place plan updates accept the request but keep the old plan, so the plan they
	// report back is checked before the update counts as applied.
	if !sameOptionalValue(resp.SubscriptionPlanId, planID) {
		return &gatewayclient.NonRetryableError{
			Err: fmt.Errorf("the gateway did not apply spec.subscriptionPlanId; this gateway can't change a " +
				"subscription's plan in place, so delete and recreate the Subscription to change it"),
		}
	}
	return nil
}

// sameOptionalValue compares two optional values, treating unset, empty and blank as the same.
func sameOptionalValue(a, b *string) bool {
	trimmed := func(v *string) string {
		if v == nil {
			return ""
		}
		return strings.TrimSpace(*v)
	}
	return trimmed(a) == trimmed(b)
}

// recoverSubscriptionIDAfterConflict finds the gateway subscription that a conflicting create collided
// with. With spec.externalRefId the subscription id is known, so it is adopted only when that id is one of
// this API's subscriptions and belongs to the CR's application; otherwise the id or the token belongs to a
// different subscription, and the conflict is reported without saying which, so the error never confirms
// that a token is in use. Gateways that predate externalRefId ignore it and assign their own ids, which they
// show by leaving externalRefId out of their responses; those are recovered as if it were not set. Without
// spec.externalRefId the id is looked up by apiId and applicationId, and is returned only when exactly one
// subscription matches; otherwise no id is returned and the create conflict is retried.
func recoverSubscriptionIDAfterConflict(ctx context.Context, gatewayEndpoint string, cr *apiv1.Subscription, authFn gatewayclient.AuthHeaderFunc) (string, error) {
	if cr.Spec.ExternalRefId != nil && *cr.Spec.ExternalRefId != "" {
		subscriptions, err := gatewayclient.ListSubscriptions(ctx, gatewayEndpoint, cr.Spec.ApiId, nil, authFn)
		if err != nil {
			return "", err
		}
		gatewaySupportsExternalRefID := false
		for i := range subscriptions {
			if subscriptions[i].ExternalRefId != nil {
				gatewaySupportsExternalRefID = true
			}
			if subscriptions[i].Id == *cr.Spec.ExternalRefId &&
				gatewayclient.SameApplication(subscriptions[i].ApplicationId, cr.Spec.ApplicationId) {
				return subscriptions[i].Id, nil
			}
		}
		if len(subscriptions) == 0 || gatewaySupportsExternalRefID {
			return "", subscriptionConflictError()
		}
	}
	return gatewayclient.FindSubscriptionIDByAPIAndApplication(ctx, gatewayEndpoint, cr.Spec.ApiId, cr.Spec.ApplicationId, authFn)
}

// resolveSubscriptionPlanID allows Subscription.spec.subscriptionPlanId
// to be either a literal gateway UUID or a same-namespace SubscriptionPlan name.
// If a SubscriptionPlan CR with that name exists but has no status.id yet, we
// return a retryable error so reconciliation naturally retries after the plan is
// created in the gateway.
func resolveSubscriptionPlanID(ctx context.Context, k8sClient client.Client, cr *apiv1.Subscription) (*string, error) {
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

// subscriptionImmutableFingerprintData holds the fields the management API can't change after create.
type subscriptionImmutableFingerprintData struct {
	ExternalRefId         *string `json:"externalRefId,omitempty"`
	ApiId                 string  `json:"apiId"`
	ApplicationId         *string `json:"applicationId,omitempty"`
	BillingCustomerId     *string `json:"billingCustomerId,omitempty"`
	BillingSubscriptionId *string `json:"billingSubscriptionId,omitempty"`
}

func subscriptionImmutableFingerprint(cr *apiv1.Subscription) (string, error) {
	data := subscriptionImmutableFingerprintData{
		ExternalRefId:         cr.Spec.ExternalRefId,
		ApiId:                 cr.Spec.ApiId,
		ApplicationId:         cr.Spec.ApplicationId,
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

func subscriptionLastAppliedImmutableFingerprint(cr *apiv1.Subscription) string {
	if cr.Annotations == nil {
		return ""
	}
	return cr.Annotations[annSubscriptionLastAppliedImmutableFingerprint]
}

// setSubscriptionLastAppliedImmutableFingerprint stores the fingerprint and drops the legacy
// create-fingerprint annotation, which also covered the now-mutable plan and token.
func setSubscriptionLastAppliedImmutableFingerprint(ctx context.Context, k8sClient client.Client, cr *apiv1.Subscription, fingerprint string) error {
	orig := cr.DeepCopy()
	if cr.Annotations == nil {
		cr.Annotations = map[string]string{}
	}
	cr.Annotations[annSubscriptionLastAppliedImmutableFingerprint] = fingerprint
	delete(cr.Annotations, annSubscriptionLegacyCreateFingerprint)
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
