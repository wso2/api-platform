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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/helm"
)

func TestResolveGatewayAddressesFromService(t *testing.T) {
	tests := []struct {
		name      string
		svc       *corev1.Service
		nodePort  []gatewayv1.GatewayStatusAddress
		wantLen   int
		wantType  gatewayv1.AddressType
		wantValue string
	}{
		{
			name: "LoadBalancer with IP",
			svc: &corev1.Service{
				Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{{IP: "203.0.113.10"}},
					},
				},
			},
			wantLen:   1,
			wantType:  gatewayv1.IPAddressType,
			wantValue: "203.0.113.10",
		},
		{
			name: "LoadBalancer with hostname",
			svc: &corev1.Service{
				Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{{Hostname: "lb.example.com"}},
					},
				},
			},
			wantLen:   1,
			wantType:  gatewayv1.HostnameAddressType,
			wantValue: "lb.example.com",
		},
		{
			name: "LoadBalancer pending",
			svc: &corev1.Service{
				Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
			},
			wantLen: 0,
		},
		{
			name: "ClusterIP",
			svc: &corev1.Service{
				Spec: corev1.ServiceSpec{
					Type:      corev1.ServiceTypeClusterIP,
					ClusterIP: "10.96.0.42",
				},
			},
			wantLen:   1,
			wantType:  gatewayv1.IPAddressType,
			wantValue: "10.96.0.42",
		},
		{
			name: "NodePort uses caller-resolved addresses",
			svc: &corev1.Service{
				Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeNodePort},
			},
			nodePort:  []gatewayv1.GatewayStatusAddress{gatewayIPAddress("198.51.100.7")},
			wantLen:   1,
			wantType:  gatewayv1.IPAddressType,
			wantValue: "198.51.100.7",
		},
		{
			name: "NodePort with no resolved addresses is empty",
			svc: &corev1.Service{
				Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeNodePort},
			},
			nodePort: nil,
			wantLen:  0,
		},
		{
			name: "ExternalName",
			svc: &corev1.Service{
				Spec: corev1.ServiceSpec{
					Type:         corev1.ServiceTypeExternalName,
					ExternalName: "gateway.external.example.com",
				},
			},
			wantLen:   1,
			wantType:  gatewayv1.HostnameAddressType,
			wantValue: "gateway.external.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addrs := resolveGatewayAddressesFromService(tt.svc, tt.nodePort)
			if len(addrs) != tt.wantLen {
				t.Fatalf("len(addrs) = %d, want %d", len(addrs), tt.wantLen)
			}
			if tt.wantLen == 0 {
				return
			}
			if addrs[0].Type == nil || *addrs[0].Type != tt.wantType {
				t.Fatalf("addrs[0].Type = %v, want %v", addrs[0].Type, tt.wantType)
			}
			if addrs[0].Value != tt.wantValue {
				t.Fatalf("addrs[0].Value = %q, want %q", addrs[0].Value, tt.wantValue)
			}
		})
	}
}

func TestDiscoverGatewayRuntimeService(t *testing.T) {
	releaseName := helm.GetReleaseName("test-gateway")
	runtimeSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-gateway-gateway-runtime",
			Namespace: "infra",
			Labels: map[string]string{
				"app.kubernetes.io/instance":  releaseName,
				"app.kubernetes.io/component": "gateway-runtime",
			},
		},
	}
	controllerSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-gateway-controller",
			Namespace: "infra",
			Labels: map[string]string{
				"app.kubernetes.io/instance":  releaseName,
				"app.kubernetes.io/component": "controller",
			},
		},
	}

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(runtimeSvc, controllerSvc).Build()

	got, err := discoverGatewayRuntimeService(context.Background(), cl, "test-gateway", "infra")
	if err != nil {
		t.Fatalf("discoverGatewayRuntimeService returned error: %v", err)
	}
	if got.Name != runtimeSvc.Name {
		t.Fatalf("got service %q, want %q", got.Name, runtimeSvc.Name)
	}
}

func node(name string, addrs map[corev1.NodeAddressType]string) corev1.Node {
	n := corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}}
	for typ, val := range addrs {
		n.Status.Addresses = append(n.Status.Addresses, corev1.NodeAddress{Type: typ, Address: val})
	}
	return n
}

func TestNodePortGatewayAddressesFromNodes(t *testing.T) {
	tests := []struct {
		name  string
		nodes []corev1.Node
		want  []string
	}{
		{
			name:  "no nodes -> empty",
			nodes: nil,
			want:  nil,
		},
		{
			name: "external IP preferred over internal",
			nodes: []corev1.Node{
				node("n1", map[corev1.NodeAddressType]string{
					corev1.NodeExternalIP: "203.0.113.5",
					corev1.NodeInternalIP: "10.0.0.5",
				}),
			},
			want: []string{"203.0.113.5"},
		},
		{
			name: "falls back to internal when no external",
			nodes: []corev1.Node{
				node("n1", map[corev1.NodeAddressType]string{corev1.NodeInternalIP: "10.0.0.9"}),
			},
			want: []string{"10.0.0.9"},
		},
		{
			name: "multiple nodes, external addresses deduped",
			nodes: []corev1.Node{
				node("n1", map[corev1.NodeAddressType]string{corev1.NodeExternalIP: "203.0.113.1"}),
				node("n2", map[corev1.NodeAddressType]string{corev1.NodeExternalIP: "203.0.113.2"}),
				node("n3", map[corev1.NodeAddressType]string{corev1.NodeExternalIP: "203.0.113.1"}),
			},
			want: []string{"203.0.113.1", "203.0.113.2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addrs := nodePortGatewayAddressesFromNodes(tt.nodes)
			if len(addrs) != len(tt.want) {
				t.Fatalf("len(addrs) = %d, want %d", len(addrs), len(tt.want))
			}
			for i, w := range tt.want {
				if addrs[i].Type == nil || *addrs[i].Type != gatewayv1.IPAddressType {
					t.Fatalf("addrs[%d].Type = %v, want IPAddress", i, addrs[i].Type)
				}
				if addrs[i].Value != w {
					t.Fatalf("addrs[%d].Value = %q, want %q", i, addrs[i].Value, w)
				}
			}
		})
	}
}

func TestResolveNodePortGatewayAddresses(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	t.Run("override is published verbatim without listing nodes", func(t *testing.T) {
		// No nodes in the cluster: if the override is honored we never need them.
		cl := fake.NewClientBuilder().WithScheme(scheme).Build()
		r := &K8sGatewayReconciler{
			Client: cl,
			Config: &config.OperatorConfig{
				GatewayAPI: config.GatewayAPIConfig{NodePortAddressOverride: "127.0.0.1"},
			},
		}
		addrs, err := r.resolveNodePortGatewayAddresses(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(addrs) != 1 || addrs[0].Value != "127.0.0.1" {
			t.Fatalf("addrs = %+v, want single 127.0.0.1", addrs)
		}
	})

	t.Run("no override derives node addresses from the cluster", func(t *testing.T) {
		n := node("n1", map[corev1.NodeAddressType]string{corev1.NodeExternalIP: "203.0.113.20"})
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&n).Build()
		r := &K8sGatewayReconciler{
			Client: cl,
			Config: &config.OperatorConfig{GatewayAPI: config.GatewayAPIConfig{}},
		}
		addrs, err := r.resolveNodePortGatewayAddresses(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(addrs) != 1 || addrs[0].Value != "203.0.113.20" {
			t.Fatalf("addrs = %+v, want single 203.0.113.20", addrs)
		}
	})
}
