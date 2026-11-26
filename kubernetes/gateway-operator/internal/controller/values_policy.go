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
	"log/slog"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/valuespolicy"
)

// Events need create/patch in the gateway's namespace to report values policy refusals.
//+kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

const (
	// EventReasonValuesPolicyViolation is the Event reason used when per-gateway
	// Helm values are refused by the values policy.
	EventReasonValuesPolicyViolation = "ValuesPolicyViolation"

	// maxEventNoteLen keeps Event notes under the API server's 1 KiB limit.
	maxEventNoteLen = 1000
)

// checkGatewayValuesPolicy validates the per-gateway values overlay against the
// configured policy. A nil config means no policy.
func checkGatewayValuesPolicy(cfg *config.OperatorConfig, valuesYAML string) error {
	if cfg == nil {
		return nil
	}
	return cfg.Gateway.ValuesPolicy.Policy().Check(valuesYAML)
}

// recordValuesPolicyViolation emits a Warning Event on obj naming the refused keys.
func recordValuesPolicyViolation(rec events.EventRecorder, obj runtime.Object, violation *valuespolicy.ViolationError) {
	if rec == nil || violation == nil {
		return
	}
	note := violation.Error()
	if len(note) > maxEventNoteLen {
		note = note[:maxEventNoteLen-3] + "..."
	}
	rec.Eventf(obj, nil, corev1.EventTypeWarning, EventReasonValuesPolicyViolation, "Render", "%s", note)
}

// handleValuesPolicyViolation records a values policy refusal for an APIGateway.
//
// Nothing is rendered or applied, so the release that is already installed (if
// any) keeps running unchanged. The refusal is terminal for this generation and
// configuration: both conditions carry the current generation and the attempted
// config hash is recorded, so the gateway is reconciled again only once the CR
// or its ConfigMap changes - retrying the same values cannot succeed.
func (r *GatewayReconciler) handleValuesPolicyViolation(
	ctx context.Context,
	gatewayConfig *apiv1.APIGateway,
	trackingKey string,
	entry *GatewayTrackingEntry,
	violation *valuespolicy.ViolationError,
	selectedCount int,
	configHash string,
) (ctrl.Result, error) {
	log := r.Logger.With(slog.String("controller", "APIGateway"), slog.String("name", gatewayConfig.Name))
	log.Warn("Gateway values refused by values policy; keeping the current release",
		slog.String("keys", strings.Join(violation.Keys(), ",")),
		slog.String("error", violation.Error()))

	recordValuesPolicyViolation(r.Recorder, gatewayConfig, violation)

	entry.Status = GatewayTrackingStatusDeployed
	r.gatewayTracker.Set(trackingKey, entry)

	if err := r.setGatewayAcceptedCondition(ctx, gatewayConfig, metav1.Condition{
		Type:               apiv1.GatewayConditionAccepted,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: entry.Generation,
		Reason:             apiv1.GatewayAcceptedReasonInvalidConfiguration,
		Message:            violation.Error(),
		LastTransitionTime: metav1.Now(),
	}); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.updateGatewayProgrammedCondition(ctx, gatewayConfig, metav1.Condition{
		Type:               apiv1.GatewayConditionProgrammed,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: entry.Generation,
		Reason:             apiv1.GatewayProgrammedReasonInvalid,
		Message:            violation.Error(),
		LastTransitionTime: metav1.Now(),
	}, &selectedCount, &configHash); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// setGatewayAcceptedCondition patches only the Accepted condition.
func (r *GatewayReconciler) setGatewayAcceptedCondition(ctx context.Context, gatewayConfig *apiv1.APIGateway, cond metav1.Condition) error {
	latest := &apiv1.APIGateway{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(gatewayConfig), latest); err != nil {
		return err
	}
	base := latest.DeepCopy()
	meta.SetStatusCondition(&latest.Status.Conditions, cond)
	return r.Status().Patch(ctx, latest, client.MergeFrom(base))
}
