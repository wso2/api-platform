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

	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// ensureCrossNamespaceServiceReferenceGrant enforces Gateway API ReferenceGrant
// for HTTPRoute → Service backends when the Service namespace differs from the route's namespace.
// ReferenceGrants must be defined in the Service namespace (the referent namespace).
func ensureCrossNamespaceServiceReferenceGrant(ctx context.Context, c client.Client, routeNS, serviceNS, serviceName string) error {
	if serviceNS == routeNS {
		return nil
	}
	var list gatewayv1beta1.ReferenceGrantList
	if err := c.List(ctx, &list, client.InNamespace(serviceNS)); err != nil {
		return newTransientHTTPRouteConfigError("list ReferenceGrant in namespace %q: %w", serviceNS, err)
	}
	for i := range list.Items {
		if referenceGrantAllowsHTTPRouteToService(&list.Items[i].Spec, routeNS, serviceName) {
			return nil
		}
	}
	return newBackendRefError(
		gatewayv1.RouteReasonRefNotPermitted,
		"cross-namespace backend Service %s/%s: no ReferenceGrant in namespace %q allowing HTTPRoute from namespace %q to core/Service %q",
		serviceNS, serviceName, serviceNS, routeNS, serviceName,
	)
}

func referenceGrantAllowsHTTPRouteToService(spec *gatewayv1beta1.ReferenceGrantSpec, routeNS, svcName string) bool {
	if spec == nil {
		return false
	}
	fromOK := false
	for _, f := range spec.From {
		if string(f.Namespace) != routeNS {
			continue
		}
		if string(f.Kind) != "HTTPRoute" {
			continue
		}
		fg := string(f.Group)
		if fg != gatewayv1.GroupName {
			continue
		}
		fromOK = true
		break
	}
	if !fromOK {
		return false
	}
	for _, t := range spec.To {
		if string(t.Kind) != "Service" {
			continue
		}
		// Core Service API: group empty in Gateway API = Kubernetes core.
		if string(t.Group) != "" {
			continue
		}
		if t.Name == nil || string(*t.Name) == svcName {
			return true
		}
	}
	return false
}

// crossNamespaceSecretReferenceGrantPermitted checks whether a Gateway may reference a Secret
// in another namespace per Gateway API ReferenceGrant rules. Grants must exist in the Secret
// namespace. It returns a non-nil error only when the ReferenceGrant lookup itself fails (a
// transient API read problem the caller should retry); a completed check that finds no matching
// grant returns (false, nil) — a real, permanent deny. This lets callers distinguish a temporary
// read failure from an actual RefNotPermitted.
func crossNamespaceSecretReferenceGrantPermitted(ctx context.Context, c client.Client, gwNS, secretNS, secretName string) (bool, error) {
	if secretNS == gwNS {
		return true, nil
	}
	var list gatewayv1beta1.ReferenceGrantList
	if err := c.List(ctx, &list, client.InNamespace(secretNS)); err != nil {
		return false, fmt.Errorf("list ReferenceGrant in namespace %q: %w", secretNS, err)
	}
	for i := range list.Items {
		if referenceGrantAllowsGatewayToSecret(&list.Items[i].Spec, gwNS, secretName) {
			return true, nil
		}
	}
	return false, nil
}

func referenceGrantAllowsGatewayToSecret(spec *gatewayv1beta1.ReferenceGrantSpec, gwNS, secretName string) bool {
	if spec == nil {
		return false
	}
	fromOK := false
	for _, f := range spec.From {
		if string(f.Namespace) != gwNS {
			continue
		}
		if string(f.Kind) != "Gateway" {
			continue
		}
		if string(f.Group) != gatewayv1.GroupName {
			continue
		}
		fromOK = true
		break
	}
	if !fromOK {
		return false
	}
	for _, t := range spec.To {
		if string(t.Kind) != "Secret" {
			continue
		}
		if string(t.Group) != "" {
			continue
		}
		if t.Name == nil || string(*t.Name) == secretName {
			return true
		}
	}
	return false
}
