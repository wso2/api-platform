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
	cryptotls "crypto/tls"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// supportedProtocols defines the protocols supported by this gateway implementation.
var supportedProtocols = map[gatewayv1.ProtocolType]bool{
	gatewayv1.HTTPProtocolType:  true,
	gatewayv1.HTTPSProtocolType: true,
}

// supportedRouteKinds defines the route kinds supported by HTTP/HTTPS listeners.
var supportedRouteKinds = map[gatewayv1.Kind]bool{
	"HTTPRoute": true,
}

// evaluateListeners validates each listener in the Gateway spec and returns per-listener
// statuses along with a boolean indicating whether at least one listener was accepted.
//
// For supported protocols (HTTP, HTTPS):
//   - Sets Accepted=True with reason Accepted
//   - Populates SupportedKinds with HTTPRoute (or subset based on allowedRoutes.kinds)
//   - Validates allowedRoutes.kinds and TLS certificateRefs
//   - Sets ResolvedRefs condition based on validation
//
// For unsupported protocols:
//   - Sets Accepted=False with reason UnsupportedProtocol
//   - Sets SupportedKinds to empty slice
//   - Sets ResolvedRefs=False
//
// Returns:
//   - listenerStatuses: slice of ListenerStatus for each listener in the Gateway
//   - hasAnyAccepted: true if at least one listener has a supported protocol
//   - error: if there's an error during validation (e.g., client operations)
func evaluateListeners(ctx context.Context, cl client.Client, gw *gatewayv1.Gateway) (listenerStatuses []gatewayv1.ListenerStatus, hasAnyAccepted bool, err error) {
	attachedRoutes, err := computeAttachedHTTPRoutesByListener(ctx, cl, gw)
	if err != nil {
		return nil, false, err
	}

	for _, l := range gw.Spec.Listeners {
		if !supportedProtocols[l.Protocol] {
			// Unsupported protocol
			listenerStatuses = append(listenerStatuses, gatewayv1.ListenerStatus{
				Name:           l.Name,
				SupportedKinds: []gatewayv1.RouteGroupKind{},
				Conditions: listenerConditions(gw.Generation,
					metav1.Condition{
						Type:               string(gatewayv1.ListenerConditionAccepted),
						Status:             metav1.ConditionFalse,
						Reason:             string(gatewayv1.ListenerReasonUnsupportedProtocol),
						Message:            fmt.Sprintf("Protocol %q is not supported; supported protocols are HTTP and HTTPS", l.Protocol),
					},
					metav1.Condition{
						Type:               string(gatewayv1.ListenerConditionResolvedRefs),
						Status:             metav1.ConditionFalse,
						Reason:             string(gatewayv1.ListenerReasonInvalid),
						Message:            "Listener has unsupported protocol",
					},
					listenerProgrammedCondition(metav1.ConditionFalse, string(gatewayv1.ListenerReasonInvalid), "Listener has unsupported protocol"),
				),
				AttachedRoutes: attachedRoutes[l.Name],
			})
			continue
		}

		// Supported protocol - listener is Accepted
		hasAnyAccepted = true

		// Validate allowedRoutes.kinds
		supportedKinds, resolvedRefsStatus, resolvedRefsReason, resolvedRefsMsg := validateAllowedRouteKinds(l, gw)

		// For HTTPS listeners, validate TLS certificateRefs
		if l.Protocol == gatewayv1.HTTPSProtocolType && resolvedRefsStatus == metav1.ConditionTrue {
			valid, reason, msg, err := validateTLSCertificateRefs(ctx, cl, gw.Namespace, l)
			if err != nil {
				// Transient lookup failure (ReferenceGrant list or Secret read): bubble up so the
				// Gateway reconcile requeues instead of recording a permanent listener failure.
				return nil, false, err
			}
			if !valid {
				resolvedRefsStatus = metav1.ConditionFalse
				resolvedRefsReason = reason
				resolvedRefsMsg = msg
			}
		}

		acceptedCond := metav1.Condition{
			Type:    string(gatewayv1.ListenerConditionAccepted),
			Status:  metav1.ConditionTrue,
			Reason:  string(gatewayv1.ListenerReasonAccepted),
			Message: "Listener protocol is supported",
		}
		resolvedRefsCond := metav1.Condition{
			Type:    string(gatewayv1.ListenerConditionResolvedRefs),
			Status:  resolvedRefsStatus,
			Reason:  resolvedRefsReason,
			Message: resolvedRefsMsg,
		}
		var programmedCond metav1.Condition
		if resolvedRefsStatus == metav1.ConditionTrue {
			programmedCond = listenerProgrammedCondition(metav1.ConditionTrue, string(gatewayv1.ListenerReasonProgrammed), "Listener references are resolved")
		} else {
			programmedCond = listenerProgrammedCondition(metav1.ConditionFalse, string(gatewayv1.ListenerReasonInvalid), resolvedRefsMsg)
		}

		listenerStatuses = append(listenerStatuses, gatewayv1.ListenerStatus{
			Name:           l.Name,
			SupportedKinds: supportedKinds,
			Conditions: listenerConditions(gw.Generation,
				acceptedCond,
				resolvedRefsCond,
				programmedCond,
			),
			AttachedRoutes: attachedRoutes[l.Name],
		})
	}
	return listenerStatuses, hasAnyAccepted, nil
}

func listenerConditions(generation int64, conds ...metav1.Condition) []metav1.Condition {
	out := make([]metav1.Condition, len(conds))
	now := metav1.Now()
	for i, c := range conds {
		c.ObservedGeneration = generation
		c.LastTransitionTime = now
		out[i] = c
	}
	return out
}

func listenerProgrammedCondition(status metav1.ConditionStatus, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:    string(gatewayv1.ListenerConditionProgrammed),
		Status:  status,
		Reason:  reason,
		Message: message,
	}
}

// hasAllListenersAccepted returns true if all listeners in the provided statuses
// have Accepted=True. This is used to determine the appropriate Gateway Accepted reason.
func hasAllListenersAccepted(listenerStatuses []gatewayv1.ListenerStatus) bool {
	for _, ls := range listenerStatuses {
		for _, cond := range ls.Conditions {
			if cond.Type == string(gatewayv1.ListenerConditionAccepted) && cond.Status == metav1.ConditionFalse {
				return false
			}
		}
	}
	return true
}

// validateAllowedRouteKinds validates the allowedRoutes.kinds for a listener.
// Returns (supportedKinds, status, reason, message).
func validateAllowedRouteKinds(l gatewayv1.Listener, gw *gatewayv1.Gateway) ([]gatewayv1.RouteGroupKind, metav1.ConditionStatus, string, string) {
	// Default supported kinds for HTTP/HTTPS
	defaultKinds := []gatewayv1.RouteGroupKind{{
		Group: (*gatewayv1.Group)(&gatewayv1.GroupVersion.Group),
		Kind:  gatewayv1.Kind("HTTPRoute"),
	}}

	// If no allowedRoutes or no kinds specified, use defaults
	if l.AllowedRoutes == nil || len(l.AllowedRoutes.Kinds) == 0 {
		return defaultKinds, metav1.ConditionTrue, string(gatewayv1.ListenerReasonResolvedRefs), "All references resolved"
	}

	// Check which requested kinds are supported
	validKinds := []gatewayv1.RouteGroupKind{}
	hasInvalidKind := false

	for _, requestedKind := range l.AllowedRoutes.Kinds {
		// Check if the group matches (or is nil/empty for default group)
		groupMatches := false
		if requestedKind.Group == nil || string(*requestedKind.Group) == "" || string(*requestedKind.Group) == gatewayv1.GroupVersion.Group {
			groupMatches = true
		}

		if groupMatches && supportedRouteKinds[requestedKind.Kind] {
			validKinds = append(validKinds, requestedKind)
		} else {
			hasInvalidKind = true
		}
	}

	if hasInvalidKind {
		// Some kinds are not supported
		if len(validKinds) == 0 {
			// No valid kinds at all
			return []gatewayv1.RouteGroupKind{}, metav1.ConditionFalse,
				string(gatewayv1.ListenerReasonInvalidRouteKinds),
				"No supported route kinds specified in allowedRoutes"
		}
		// Mixed case: some valid, some invalid
		return validKinds, metav1.ConditionFalse,
			string(gatewayv1.ListenerReasonInvalidRouteKinds),
			"Some route kinds in allowedRoutes are not supported"
	}

	// All requested kinds are supported
	return validKinds, metav1.ConditionTrue, string(gatewayv1.ListenerReasonResolvedRefs), "All references resolved"
}

// validateTLSCertificateRefs validates TLS certificate references for an HTTPS listener.
// Returns (valid bool, reason string, message string, err error). A non-nil err signals a
// TRANSIENT lookup failure (ReferenceGrant list or Secret read failed) that the caller should
// retry; in that case valid/reason/message are not meaningful. A completed validation returns a
// nil err with valid reflecting the outcome (e.g. a genuine RefNotPermitted or NotFound is a
// permanent valid=false with nil err).
func validateTLSCertificateRefs(ctx context.Context, cl client.Client, gwNamespace string, l gatewayv1.Listener) (bool, string, string, error) {
	if l.TLS == nil || len(l.TLS.CertificateRefs) == 0 {
		// No TLS config, treat as valid (HTTP listener or HTTPS without explicit cert ref)
		return true, string(gatewayv1.ListenerReasonResolvedRefs), "All references resolved", nil
	}

	for _, certRef := range l.TLS.CertificateRefs {
		// Validate the cert ref group/kind
		if certRef.Group != nil && string(*certRef.Group) != "" && string(*certRef.Group) != "core" {
			return false, string(gatewayv1.ListenerReasonInvalidCertificateRef),
				fmt.Sprintf("Unsupported group %q for certificateRef", *certRef.Group), nil
		}

		if certRef.Kind != nil && string(*certRef.Kind) != "Secret" {
			return false, string(gatewayv1.ListenerReasonInvalidCertificateRef),
				fmt.Sprintf("Unsupported kind %q for certificateRef; only Secret is supported", *certRef.Kind), nil
		}

		// Determine the namespace
		ns := gwNamespace
		if certRef.Namespace != nil {
			ns = string(*certRef.Namespace)
		}

		if ns != gwNamespace {
			permitted, err := crossNamespaceSecretReferenceGrantPermitted(ctx, cl, gwNamespace, ns, string(certRef.Name))
			if err != nil {
				// Transient: the ReferenceGrant lookup failed. Surface for retry rather than
				// reporting a permanent RefNotPermitted.
				return false, "", "", err
			}
			if !permitted {
				return false, string(gatewayv1.ListenerReasonRefNotPermitted),
					fmt.Sprintf("no ReferenceGrant in %q allowing Gateway from %q to Secret %q", ns, gwNamespace, certRef.Name), nil
			}
		}

		// Try to get the secret
		secret := &corev1.Secret{}
		if err := cl.Get(ctx, client.ObjectKey{Name: string(certRef.Name), Namespace: ns}, secret); err != nil {
			if apierrors.IsNotFound(err) {
				return false, string(gatewayv1.ListenerReasonInvalidCertificateRef),
					fmt.Sprintf("Secret %s/%s not found", ns, certRef.Name), nil
			}
			// Transient read failure (API server unavailable, throttling, etc.): surface for
			// retry rather than latching a permanent InvalidCertificateRef.
			return false, "", "", fmt.Errorf("get Secret %s/%s: %w", ns, certRef.Name, err)
		}

		// Validate that the secret has required TLS data
		if secret.Type != corev1.SecretTypeTLS {
			return false, string(gatewayv1.ListenerReasonInvalidCertificateRef),
				fmt.Sprintf("Secret %s/%s is not of type kubernetes.io/tls", ns, certRef.Name), nil
		}

		if _, hasCert := secret.Data["tls.crt"]; !hasCert {
			return false, string(gatewayv1.ListenerReasonInvalidCertificateRef),
				fmt.Sprintf("Secret %s/%s does not contain tls.crt", ns, certRef.Name), nil
		}

		if _, hasKey := secret.Data["tls.key"]; !hasKey {
			return false, string(gatewayv1.ListenerReasonInvalidCertificateRef),
				fmt.Sprintf("Secret %s/%s does not contain tls.key", ns, certRef.Name), nil
		}

		if _, err := cryptotls.X509KeyPair(secret.Data["tls.crt"], secret.Data["tls.key"]); err != nil {
			return false, string(gatewayv1.ListenerReasonInvalidCertificateRef),
				fmt.Sprintf("Secret %s/%s contains malformed TLS certificate data", ns, certRef.Name), nil
		}
	}

	return true, string(gatewayv1.ListenerReasonResolvedRefs), "All references resolved", nil
}

// validateParametersRef checks if the Gateway's spec.infrastructure.parametersRef is valid.
// Returns (valid bool, reason string, error).
// If parametersRef is nil or empty, returns (true, "", nil).
// If the referenced resource doesn't exist or is invalid, returns (false, "InvalidParameters", nil).
func validateParametersRef(ctx context.Context, cl client.Client, gw *gatewayv1.Gateway) (bool, string, error) {
	if gw.Spec.Infrastructure == nil || gw.Spec.Infrastructure.ParametersRef == nil {
		return true, "", nil
	}

	ref := gw.Spec.Infrastructure.ParametersRef

	// Build GVK from the parametersRef
	group := string(ref.Group)

	gvk := schema.GroupVersionKind{
		Group: group,
		Kind:  string(ref.Kind),
		// We'll try to discover the version
	}

	// Attempt to get the resource using unstructured client
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)

	// Determine namespace - parametersRef is in the same namespace as Gateway
	ns := gw.Namespace
	if ns == "" {
		ns = "default"
	}

	objKey := client.ObjectKey{
		Name:      ref.Name,
		Namespace: ns,
	}

	if err := cl.Get(ctx, objKey, u); err != nil {
		if apierrors.IsNotFound(err) {
			return false, "InvalidParameters", nil
		}
		// For other errors (e.g., permission issues, unknown GVK), treat as invalid
		return false, "InvalidParameters", nil
	}

	return true, "", nil
}
