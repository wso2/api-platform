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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
)

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
	e, ok := err.(*HTTPRouteConfigError)
	return ok && e.Kind == httpRouteConfigErrorInvalid
}

// BuildAPIConfigFromHTTPRoute maps HTTPRoute rules to APIConfigData (MVP: single Service backend across rules).
func BuildAPIConfigFromHTTPRoute(ctx context.Context, c client.Client, route *gatewayv1.HTTPRoute) (*apiv1.APIConfigData, error) {
	if len(route.Spec.Rules) == 0 {
		return nil, newInvalidHTTPRouteConfigError("HTTPRoute has no rules")
	}

	displayName := route.Name
	if v := route.Annotations[AnnHTTPRouteDisplayName]; v != "" {
		displayName = v
	}

	version := route.Annotations[AnnHTTPRouteAPIVersion]
	if version == "" {
		version = "v1"
	}

	// Resolve backend URL (same Service required for all rules in MVP).
	backendURL, err := firstBackendURL(ctx, c, route)
	if err != nil {
		return nil, err
	}

	var ops []apiv1.Operation
	for _, rule := range route.Spec.Rules {
		if len(rule.Matches) == 0 {
			ops = append(ops, apiv1.Operation{
				Method: apiv1.OperationMethodGET,
				Path:   "/",
			})
			continue
		}
		for _, m := range rule.Matches {
			pathVal := "/"
			if m.Path != nil && m.Path.Value != nil {
				p := strings.TrimSpace(*m.Path.Value)
				if p != "" {
					pathVal = p
					if !strings.HasPrefix(pathVal, "/") {
						pathVal = "/" + pathVal
					}
				}
			}
			method := apiv1.OperationMethodGET
			if m.Method != nil {
				method = apiv1.OperationMethod(*m.Method)
			}
			ops = append(ops, apiv1.Operation{
				Method: method,
				Path:   pathVal,
			})
		}
	}

	if len(ops) == 0 {
		return nil, newInvalidHTTPRouteConfigError("no operations derived from HTTPRoute")
	}

	contextPath := route.Annotations[AnnHTTPRouteContext]
	if contextPath == "" {
		contextPath = commonPathPrefix(ops)
		if contextPath == "" {
			contextPath = "/"
		}
	}

	spec := &apiv1.APIConfigData{
		Context:     contextPath,
		DisplayName: displayName,
		Operations:  ops,
		Upstream: apiv1.UpstreamConfig{
			Main: apiv1.Upstream{Url: backendURL},
		},
		Version: version,
	}
	return spec, nil
}

func firstBackendURL(ctx context.Context, c client.Client, route *gatewayv1.HTTPRoute) (string, error) {
	ns := route.Namespace
	for _, rule := range route.Spec.Rules {
		for _, b := range rule.BackendRefs {
			if b.Kind != nil && string(*b.Kind) != "" && string(*b.Kind) != "Service" {
				continue
			}
			svcNS := ns
			if b.Namespace != nil && *b.Namespace != "" {
				svcNS = string(*b.Namespace)
			}
			svcName := string(b.Name)
			if svcName == "" {
				continue
			}
			svc := &corev1.Service{}
			key := types.NamespacedName{Namespace: svcNS, Name: svcName}
			if err := c.Get(ctx, key, svc); err != nil {
				return "", newTransientHTTPRouteConfigError("get backend Service %s: %w", key.String(), err)
			}
			portNum, err := resolveServicePort(svc, b.Port)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", svcName, svcNS, portNum), nil
		}
	}
	return "", newInvalidHTTPRouteConfigError("no Service backendRef found on HTTPRoute")
}

func resolveServicePort(svc *corev1.Service, refPort *gatewayv1.PortNumber) (int32, error) {
	if refPort != nil && *refPort > 0 {
		return int32(*refPort), nil
	}
	if len(svc.Spec.Ports) == 0 {
		return 0, newInvalidHTTPRouteConfigError("service %s/%s has no ports", svc.Namespace, svc.Name)
	}
	return svc.Spec.Ports[0].Port, nil
}

func commonPathPrefix(ops []apiv1.Operation) string {
	if len(ops) == 0 {
		return "/"
	}
	prefix := ops[0].Path
	for _, o := range ops[1:] {
		prefix = sharedPrefix(prefix, o.Path)
		if prefix == "" || prefix == "/" {
			return "/"
		}
	}
	if prefix == "" {
		return "/"
	}
	// Strip trailing slash except root
	for len(prefix) > 1 && strings.HasSuffix(prefix, "/") {
		prefix = strings.TrimSuffix(prefix, "/")
	}
	return prefix
}

func sharedPrefix(a, b string) string {
	if !strings.HasPrefix(a, "/") {
		a = "/" + a
	}
	if !strings.HasPrefix(b, "/") {
		b = "/" + b
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var i int
	for i < n && a[i] == b[i] {
		i++
	}
	if i == 0 {
		return "/"
	}
	s := a[:i]
	// End at path segment boundary
	if last := strings.LastIndex(s, "/"); last > 0 {
		return s[:last]
	}
	return s
}

// DefaultHTTPRouteAPIHandle returns a stable handle for rest-apis when no annotation is set.
func DefaultHTTPRouteAPIHandle(route *gatewayv1.HTTPRoute) string {
	if h := route.Annotations[AnnHTTPRouteAPIHandle]; h != "" {
		return h
	}
	return strings.ReplaceAll(route.Namespace+"-"+route.Name, "/", "-")
}
