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
	return newTransientHTTPRouteConfigError(
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
		if fg != "" && fg != gatewayv1.GroupName {
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
