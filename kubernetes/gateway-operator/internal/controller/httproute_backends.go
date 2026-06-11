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
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// serviceConnectionPort returns the port the gateway-runtime should connect to for a
// resolved Service port.
//
// For a normal (ClusterIP) Service this is the Service port itself: the runtime connects to
// the ClusterIP and kube-proxy translates the Service port to the pods' target port.
//
// For a HEADLESS Service (ClusterIP "None") there is no ClusterIP/kube-proxy translation —
// the Service DNS name resolves directly to pod IPs and the runtime connects to those pods
// on the port it specifies. It must therefore use the port the pods actually listen on, i.e.
// the matching ServicePort's targetPort. Connecting to a headless backend on the Service port
// (when targetPort differs) fails the TCP connect and surfaces as a 503.
//
// When the targetPort is a named port or is left unset (Kubernetes then defaults it to the
// Service port), it cannot be resolved to a number from the Service spec alone, so we fall
// back to the Service port.
func serviceConnectionPort(svc *corev1.Service, servicePort int32) int32 {
	if svc == nil || svc.Spec.ClusterIP != corev1.ClusterIPNone {
		return servicePort
	}
	for _, p := range svc.Spec.Ports {
		if p.Port != servicePort {
			continue
		}
		if p.TargetPort.Type == intstr.Int && p.TargetPort.IntVal > 0 {
			return p.TargetPort.IntVal
		}
		break
	}
	return servicePort
}

// HTTPRouteBackendRefError is a permanent backend reference resolution failure with a Gateway API reason.
type HTTPRouteBackendRefError struct {
	Reason gatewayv1.RouteConditionReason
	Err    error
}

func (e *HTTPRouteBackendRefError) Error() string {
	if e == nil || e.Err == nil {
		return "httproute backend ref error"
	}
	return e.Err.Error()
}

func (e *HTTPRouteBackendRefError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func newBackendRefError(reason gatewayv1.RouteConditionReason, format string, args ...any) error {
	return &HTTPRouteBackendRefError{
		Reason: reason,
		Err:    fmt.Errorf(format, args...),
	}
}

// IsHTTPRouteBackendRefError reports permanent backend ref resolution failures.
func IsHTTPRouteBackendRefError(err error) bool {
	var e *HTTPRouteBackendRefError
	return errors.As(err, &e)
}

// BackendRefResolutionReason extracts the Gateway API reason from a backend ref error.
func BackendRefResolutionReason(err error) gatewayv1.RouteConditionReason {
	var e *HTTPRouteBackendRefError
	if errors.As(err, &e) && e.Reason != "" {
		return e.Reason
	}
	return gatewayv1.RouteReasonResolvedRefs
}

// ResolvedBackendRef holds the outcome of resolving one HTTPRoute backendRef.
type ResolvedBackendRef struct {
	RuleIndex    int
	BackendIndex int
	ServiceNS    string
	ServiceName  string
	Port         int32
	URL          string
	Weight       *int32
	OK           bool
	Reason       gatewayv1.RouteConditionReason
	Message      string
}

// HTTPRouteBackendResolution aggregates per-ref results for status and compilation.
type HTTPRouteBackendResolution struct {
	Refs                []ResolvedBackendRef
	AllResolved         bool
	FirstFailureReason  gatewayv1.RouteConditionReason
	FirstFailureMessage string
	PlaceholderURL      string
}

// resolveHTTPRouteBackendRefs resolves every backendRef on the route independently.
func resolveHTTPRouteBackendRefs(ctx context.Context, c client.Client, route *gatewayv1.HTTPRoute, clusterDomain string) *HTTPRouteBackendResolution {
	out := &HTTPRouteBackendResolution{AllResolved: true}
	dnsBase := effectiveClusterDNSBase(clusterDomain)
	routeNS := route.Namespace

	for ruleIdx, rule := range route.Spec.Rules {
		for backendIdx, b := range rule.BackendRefs {
			ref := ResolvedBackendRef{RuleIndex: ruleIdx, BackendIndex: backendIdx}
			if b.Weight != nil {
				w := int32(*b.Weight)
				ref.Weight = &w
			}

			kind := ""
			if b.Kind != nil {
				kind = string(*b.Kind)
			}
			group := ""
			if b.Group != nil {
				group = string(*b.Group)
			}

			if kind != "" && kind != "Service" {
				ref.OK = false
				ref.Reason = gatewayv1.RouteReasonInvalidKind
				ref.Message = fmt.Sprintf("unsupported backendRef kind %q", kind)
				out.Refs = append(out.Refs, ref)
				out.recordFailure(ref.Reason, ref.Message)
				continue
			}
			// Core Service backends must omit group or use the core API group (empty).
			if group != "" {
				ref.OK = false
				ref.Reason = gatewayv1.RouteReasonInvalidKind
				ref.Message = fmt.Sprintf("unsupported backendRef group %q", group)
				out.Refs = append(out.Refs, ref)
				out.recordFailure(ref.Reason, ref.Message)
				continue
			}

			svcNS := routeNS
			if b.Namespace != nil && *b.Namespace != "" {
				svcNS = string(*b.Namespace)
			}
			svcName := string(b.Name)
			if svcName == "" {
				continue
			}

			if err := ensureCrossNamespaceServiceReferenceGrant(ctx, c, routeNS, svcNS, svcName); err != nil {
				ref.OK = false
				ref.Reason = BackendRefResolutionReason(err)
				ref.Message = err.Error()
				out.Refs = append(out.Refs, ref)
				out.recordFailure(ref.Reason, ref.Message)
				continue
			}

			svc := &corev1.Service{}
			key := types.NamespacedName{Namespace: svcNS, Name: svcName}
			if err := c.Get(ctx, key, svc); err != nil {
				ref.OK = false
				if apierrors.IsNotFound(err) {
					ref.Reason = gatewayv1.RouteReasonBackendNotFound
					ref.Message = fmt.Sprintf("backend Service %s not found", key.String())
				} else {
					ref.Reason = gatewayv1.RouteReasonBackendNotFound
					ref.Message = fmt.Sprintf("get backend Service %s: %v", key.String(), err)
				}
				out.Refs = append(out.Refs, ref)
				out.recordFailure(ref.Reason, ref.Message)
				continue
			}

			portNum, err := resolveServicePort(svc, b.Port)
			if err != nil {
				ref.OK = false
				ref.Reason = gatewayv1.RouteReasonBackendNotFound
				ref.Message = err.Error()
				out.Refs = append(out.Refs, ref)
				out.recordFailure(ref.Reason, ref.Message)
				continue
			}

			ref.OK = true
			ref.ServiceNS = svcNS
			ref.ServiceName = svcName
			ref.Port = portNum
			// Connect on the Service port for ClusterIP Services (kube-proxy translates to
			// the target port); for headless Services connect directly to pods on the
			// target port. ref.Port keeps the Service port for stable cluster naming.
			connPort := serviceConnectionPort(svc, portNum)
			ref.URL = fmt.Sprintf("http://%s.%s.svc.%s:%d", svcName, svcNS, dnsBase, connPort)
			out.Refs = append(out.Refs, ref)
			if out.PlaceholderURL == "" {
				out.PlaceholderURL = ref.URL
			}
		}
	}

	if len(out.Refs) == 0 {
		out.AllResolved = true
	}
	return out
}

func (r *HTTPRouteBackendResolution) recordFailure(reason gatewayv1.RouteConditionReason, message string) {
	r.AllResolved = false
	if r.FirstFailureReason == "" {
		r.FirstFailureReason = reason
		r.FirstFailureMessage = message
	}
}

// resolvedBackendForRule returns the first resolved backend for a rule, or nil.
func (r *HTTPRouteBackendResolution) resolvedBackendForRule(ruleIdx int) *ResolvedBackendRef {
	if r == nil {
		return nil
	}
	for i := range r.Refs {
		if r.Refs[i].RuleIndex == ruleIdx && r.Refs[i].OK {
			return &r.Refs[i]
		}
	}
	return nil
}

// ruleHasBackendRefs reports whether a rule declares any backendRefs entries.
func ruleHasBackendRefs(rule gatewayv1.HTTPRouteRule) bool {
	return len(rule.BackendRefs) > 0
}
