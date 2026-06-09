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
	corev1 "k8s.io/api/core/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const localClusterNodeAddressFallback = "127.0.0.1"

// resolveGatewayAddressesFromService derives Gateway status addresses from the
// gateway-runtime Service fronting the data plane.
func resolveGatewayAddressesFromService(svc *corev1.Service) []gatewayv1.GatewayStatusAddress {
	if svc == nil {
		return nil
	}

	switch svc.Spec.Type {
	case corev1.ServiceTypeLoadBalancer:
		return loadBalancerGatewayAddresses(svc)
	case corev1.ServiceTypeNodePort:
		return nodePortGatewayAddresses()
	case corev1.ServiceTypeExternalName:
		if svc.Spec.ExternalName == "" {
			return nil
		}
		return []gatewayv1.GatewayStatusAddress{gatewayHostnameAddress(svc.Spec.ExternalName)}
	default:
		return clusterIPGatewayAddresses(svc.Spec.ClusterIP)
	}
}

func loadBalancerGatewayAddresses(svc *corev1.Service) []gatewayv1.GatewayStatusAddress {
	if svc.Status.LoadBalancer.Ingress == nil {
		return nil
	}
	var addrs []gatewayv1.GatewayStatusAddress
	for _, ing := range svc.Status.LoadBalancer.Ingress {
		if ing.IP != "" {
			addrs = append(addrs, gatewayIPAddress(ing.IP))
		}
		if ing.Hostname != "" {
			addrs = append(addrs, gatewayHostnameAddress(ing.Hostname))
		}
	}
	return addrs
}

func clusterIPGatewayAddresses(clusterIP string) []gatewayv1.GatewayStatusAddress {
	if clusterIP == "" || clusterIP == corev1.ClusterIPNone {
		return nil
	}
	return []gatewayv1.GatewayStatusAddress{gatewayIPAddress(clusterIP)}
}

func nodePortGatewayAddresses() []gatewayv1.GatewayStatusAddress {
	return []gatewayv1.GatewayStatusAddress{gatewayIPAddress(localClusterNodeAddressFallback)}
}

func gatewayIPAddress(value string) gatewayv1.GatewayStatusAddress {
	t := gatewayv1.IPAddressType
	return gatewayv1.GatewayStatusAddress{Type: &t, Value: value}
}

func gatewayHostnameAddress(value string) gatewayv1.GatewayStatusAddress {
	t := gatewayv1.HostnameAddressType
	return gatewayv1.GatewayStatusAddress{Type: &t, Value: value}
}
