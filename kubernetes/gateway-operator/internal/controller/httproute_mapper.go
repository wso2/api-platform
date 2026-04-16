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

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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
	e, ok := err.(*HTTPRouteConfigError)
	return ok && e.Kind == httpRouteConfigErrorInvalid
}

// IsTransientHTTPRouteConfigError reports errors that should be retried (e.g. missing ReferenceGrant, API lookup).
func IsTransientHTTPRouteConfigError(err error) bool {
	if err == nil {
		return false
	}
	e, ok := err.(*HTTPRouteConfigError)
	return ok && e.Kind == httpRouteConfigErrorTransient
}

// BuildAPIConfigFromHTTPRoute maps HTTPRoute rules to APIConfigData (MVP: single Service backend across rules).
// clusterDomain is the cluster DNS suffix (e.g. cluster.local or from CLUSTER_DOMAIN / gateway_api.cluster_domain).
// log may be nil (tests); when set, emits structured diagnostics for policy loading and mapping.
func BuildAPIConfigFromHTTPRoute(ctx context.Context, c client.Client, route *gatewayv1.HTTPRoute, clusterDomain string, log *zap.Logger) (*apiv1.APIConfigData, error) {
	if len(route.Spec.Rules) == 0 {
		return nil, newInvalidHTTPRouteConfigError("HTTPRoute has no rules")
	}
	if log != nil {
		log.Info("build API config from HTTPRoute",
			zap.Int64("generation", route.Generation),
			zap.String("resourceVersion", route.ResourceVersion),
			zap.Int("ruleCount", len(route.Spec.Rules)))
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
	backendURL, err := firstBackendURL(ctx, c, route, clusterDomain)
	if err != nil {
		return nil, err
	}

	var ops []apiv1.Operation
	for ruleIdx, rule := range route.Spec.Rules {
		rulePolicies, err := policiesFromHTTPRouteRuleExtensionRefs(ctx, c, route, rule, ruleIdx, log)
		if err != nil {
			return nil, err
		}
		if len(rule.Matches) == 0 {
			return nil, newInvalidHTTPRouteConfigError(
				"method-agnostic HTTPRoute matches are not supported: rule[%d] has no matches; use explicit rule.matches entries with method set on each match",
				ruleIdx,
			)
		}
		for matchIdx, m := range rule.Matches {
			if m.Method == nil {
				return nil, newInvalidHTTPRouteConfigError(
					"method-agnostic HTTPRoute matches are not supported: rule[%d] match[%d] omits method; set match.method",
					ruleIdx, matchIdx,
				)
			}
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
			method := apiv1.OperationMethod(*m.Method)
			ops = append(ops, apiv1.Operation{
				Method:   method,
				Path:     pathVal,
				Policies: copyPolicies(rulePolicies),
			})
		}
	}

	if len(ops) == 0 {
		return nil, newInvalidHTTPRouteConfigError("no operations derived from HTTPRoute")
	}

	contextPath := strings.TrimSpace(route.Annotations[AnnHTTPRouteContext])
	if contextPath == "" {
		contextPath = commonPathPrefix(ops)
		if contextPath == "" {
			contextPath = "/"
		}
	} else if !strings.HasPrefix(contextPath, "/") {
		contextPath = "/" + contextPath
	}

	apiPolicies, err := loadHTTPRouteAPIPolicies(ctx, c, route, log)
	if err != nil {
		return nil, err
	}

	spec := &apiv1.APIConfigData{
		Context:     contextPath,
		DisplayName: displayName,
		Operations:  ops,
		Upstream: apiv1.UpstreamConfig{
			Main: apiv1.Upstream{Url: backendURL},
		},
		Version:  version,
		Policies: apiPolicies,
	}
	if err := resolveAPIConfigPolicyParamsSecrets(ctx, c, route.Namespace, spec, log); err != nil {
		return nil, err
	}
	opsWithPol := 0
	for i := range spec.Operations {
		if len(spec.Operations[i].Policies) > 0 {
			opsWithPol++
		}
	}
	if log != nil {
		log.Info("built API config from HTTPRoute",
			zap.Int("operations", len(spec.Operations)),
			zap.Int("apiLevelPolicies", len(spec.Policies)),
			zap.Int("operationsWithAttachedPolicies", opsWithPol),
			zap.Int("operationPolicyAnnotationEntries", 0))
	}
	return spec, nil
}

func firstBackendURL(ctx context.Context, c client.Client, route *gatewayv1.HTTPRoute, clusterDomain string) (string, error) {
	dnsBase := effectiveClusterDNSBase(clusterDomain)
	ns := route.Namespace
	var (
		baselineSet   bool
		baselineSvcNS string
		baselineSvc   string
		baselinePort  int32
	)
	for _, rule := range route.Spec.Rules {
		for _, b := range rule.BackendRefs {
			if b.Kind != nil && string(*b.Kind) != "" && string(*b.Kind) != "Service" {
				continue
			}
			if b.Group != nil && string(*b.Group) != "" {
				return "", newTransientHTTPRouteConfigError(
					"unsupported backendRef: core Service backends require group to be omitted or empty (got group %q)",
					string(*b.Group),
				)
			}
			svcNS := ns
			if b.Namespace != nil && *b.Namespace != "" {
				svcNS = string(*b.Namespace)
			}
			svcName := string(b.Name)
			if svcName == "" {
				continue
			}
			if err := ensureCrossNamespaceServiceReferenceGrant(ctx, c, ns, svcNS, svcName); err != nil {
				return "", err
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
			if !baselineSet {
				baselineSet = true
				baselineSvcNS = svcNS
				baselineSvc = svcName
				baselinePort = portNum
				continue
			}
			if svcNS != baselineSvcNS || svcName != baselineSvc || portNum != baselinePort {
				return "", newInvalidHTTPRouteConfigError(
					"HTTPRoute backendRefs must resolve to a single Service backend (first %s/%s:%d, found %s/%s:%d)",
					baselineSvcNS, baselineSvc, baselinePort, svcNS, svcName, portNum,
				)
			}
		}
	}
	if baselineSet {
		return fmt.Sprintf("http://%s.%s.svc.%s:%d", baselineSvc, baselineSvcNS, dnsBase, baselinePort), nil
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
