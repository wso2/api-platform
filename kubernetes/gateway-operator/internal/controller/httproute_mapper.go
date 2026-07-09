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
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
)

const defaultKubernetesClusterDNS = "cluster.local"

func effectiveClusterDNSBase(domain string) string {
	d := strings.Trim(strings.TrimSpace(domain), ".")
	if d == "" {
		return defaultKubernetesClusterDNS
	}
	return d
}

type httpRouteConfigErrorKind string

const (
	httpRouteConfigErrorInvalid   httpRouteConfigErrorKind = "invalid"
	httpRouteConfigErrorTransient httpRouteConfigErrorKind = "transient"
)

type HTTPRouteConfigError struct {
	Kind httpRouteConfigErrorKind
	Err  error
}

func (e *HTTPRouteConfigError) Error() string {
	if e == nil || e.Err == nil {
		return "httproute config error"
	}
	return e.Err.Error()
}

func (e *HTTPRouteConfigError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func newInvalidHTTPRouteConfigError(format string, args ...any) error {
	return &HTTPRouteConfigError{
		Kind: httpRouteConfigErrorInvalid,
		Err:  fmt.Errorf(format, args...),
	}
}

func newTransientHTTPRouteConfigError(format string, args ...any) error {
	return &HTTPRouteConfigError{
		Kind: httpRouteConfigErrorTransient,
		Err:  fmt.Errorf(format, args...),
	}
}

func IsInvalidHTTPRouteConfigError(err error) bool {
	if err == nil {
		return false
	}
	var e *HTTPRouteConfigError
	if errors.As(err, &e) {
		return e.Kind == httpRouteConfigErrorInvalid
	}
	return false
}

// IsTransientHTTPRouteConfigError reports errors that should be retried (e.g. missing ReferenceGrant, API lookup).
func IsTransientHTTPRouteConfigError(err error) bool {
	if err == nil {
		return false
	}
	var e *HTTPRouteConfigError
	if errors.As(err, &e) {
		return e.Kind == httpRouteConfigErrorTransient
	}
	return false
}

// allRESTAPIOperationMethods returns every HTTP verb modeled by APIConfigData / RestApi operations
// (used when an HTTPRoute match omits method, which is valid in Gateway API).
func allRESTAPIOperationMethods() []apiv1.OperationMethod {
	return []apiv1.OperationMethod{
		apiv1.OperationMethodGET,
		apiv1.OperationMethodPOST,
		apiv1.OperationMethodPUT,
		apiv1.OperationMethodPATCH,
		apiv1.OperationMethodDELETE,
		apiv1.OperationMethodHEAD,
		apiv1.OperationMethodOPTIONS,
	}
}

// restAPIOperationMethodsForHTTPRouteMatch returns a single explicit method or all supported methods
// when the Gateway API match leaves method unset.
func restAPIOperationMethodsForHTTPRouteMatch(m gatewayv1.HTTPRouteMatch) []apiv1.OperationMethod {
	if m.Method != nil {
		return []apiv1.OperationMethod{apiv1.OperationMethod(*m.Method)}
	}
	return allRESTAPIOperationMethods()
}

// restAPIPathForHTTPRouteMatch maps a Gateway API path match to an APIConfigData Operation.Path,
// honoring match.path.type (not just the value).
//
// Gateway API PathPrefix semantics — "/foo" matches "/foo" AND any path under "/foo/" — are
// expressed to the gateway-controller by the "/*" suffix: the xDS translator treats an
// Operation.Path ending in "/*" as a wildcard prefix route (gateway/gateway-controller/pkg/xds/
// translator.go), and the bare "/" as a catch-all root. So:
//   - PathPrefix "/foo"  -> "/foo/*"   (prefix route)
//   - PathPrefix "/"     -> "/"        (root catch-all; never "//*")
//   - Exact "/foo"       -> "/foo"     (verbatim, unchanged behavior)
//   - RegularExpression  -> Invalid config error (unsupported)
//
// A nil path, or a path with a nil type, defaults to PathPrefix per the Gateway API spec.
func restAPIPathForHTTPRouteMatch(m gatewayv1.HTTPRouteMatch) (string, error) {
	pathVal := "/"
	matchType := gatewayv1.PathMatchPathPrefix // Gateway API default when path/type is unset
	if m.Path != nil {
		if m.Path.Type != nil {
			matchType = *m.Path.Type
		}
		if m.Path.Value != nil {
			if p := strings.TrimSpace(*m.Path.Value); p != "" {
				pathVal = p
			}
		}
	}
	if !strings.HasPrefix(pathVal, "/") {
		pathVal = "/" + pathVal
	}

	switch matchType {
	case gatewayv1.PathMatchExact:
		return pathVal, nil
	case gatewayv1.PathMatchPathPrefix:
		if pathVal == "/" {
			// PathPrefix "/" is a catch-all; the controller special-cases bare "/".
			return pathVal, nil
		}
		// "/foo" or "/foo/" -> "/foo/*" so the controller routes it as a prefix.
		return strings.TrimRight(pathVal, "/") + "/*", nil
	case gatewayv1.PathMatchRegularExpression:
		return "", newInvalidHTTPRouteConfigError(
			"HTTPRoute path match type %q is not supported; use Exact or PathPrefix",
			gatewayv1.PathMatchRegularExpression)
	default:
		return "", newInvalidHTTPRouteConfigError(
			"unsupported HTTPRoute path match type %q (supported: Exact, PathPrefix)", string(matchType))
	}
}

func firstBackendURL(ctx context.Context, c client.Client, route *gatewayv1.HTTPRoute, clusterDomain string) (string, error) {
	res, err := resolveHTTPRouteBackendRefs(ctx, c, route, clusterDomain)
	if err != nil {
		return "", err
	}
	if res.PlaceholderURL != "" {
		return res.PlaceholderURL, nil
	}
	if !res.AllResolved && res.FirstFailureMessage != "" {
		return "", newBackendRefError(res.FirstFailureReason, "%s", res.FirstFailureMessage)
	}
	return "", newInvalidHTTPRouteConfigError("no Service backendRef found on HTTPRoute")
}

func resolveServicePort(svc *corev1.Service, refPort *gatewayv1.PortNumber) (int32, error) {
	ports := svc.Spec.Ports
	if len(ports) == 0 {
		return 0, newInvalidHTTPRouteConfigError("service %s/%s has no ports", svc.Namespace, svc.Name)
	}

	var want int32
	if refPort != nil && *refPort > 0 {
		want = int32(*refPort)
	}

	if want > 0 {
		for _, p := range ports {
			if p.Port == want {
				return p.Port, nil
			}
		}
		return 0, newInvalidHTTPRouteConfigError(
			"service %s/%s has no port %d in spec.ports (backendRefs[].port must match a Service spec.ports.port)",
			svc.Namespace, svc.Name, want,
		)
	}

	if len(ports) > 1 {
		return 0, newInvalidHTTPRouteConfigError(
			"service %s/%s has %d ports; set backendRefs[].port to a spec.ports.port value to disambiguate",
			svc.Namespace, svc.Name, len(ports),
		)
	}

	return ports[0].Port, nil
}

func sharedPrefix(a, b string) string {
	if !strings.HasPrefix(a, "/") {
		a = "/" + a
	}
	if !strings.HasPrefix(b, "/") {
		b = "/" + b
	}
	aSeg := splitPathSegments(a)
	bSeg := splitPathSegments(b)
	n := len(aSeg)
	if len(bSeg) < n {
		n = len(bSeg)
	}
	matched := 0
	for i := 0; i < n; i++ {
		if aSeg[i] != bSeg[i] {
			break
		}
		matched++
	}
	if matched == 0 {
		return "/"
	}
	return "/" + strings.Join(aSeg[:matched], "/")
}

func splitPathSegments(p string) []string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// DefaultHTTPRouteAPIHandle returns a stable handle for rest-apis when no annotation is set.
func DefaultHTTPRouteAPIHandle(route *gatewayv1.HTTPRoute) string {
	if h := route.Annotations[AnnHTTPRouteAPIHandle]; h != "" {
		return h
	}
	return strings.ReplaceAll(route.Namespace+"-"+route.Name, "/", "-")
}
