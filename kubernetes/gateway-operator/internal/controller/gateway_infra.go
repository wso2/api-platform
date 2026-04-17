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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/helm"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
)

// configMapValuesYAML returns the values.yaml entry from a ConfigMap.
func configMapValuesYAML(ctx context.Context, c client.Client, configMapName, namespace string) (string, error) {
	configMap := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: configMapName, Namespace: namespace}, configMap); err != nil {
		return "", fmt.Errorf("failed to get ConfigMap %s/%s: %w", namespace, configMapName, err)
	}
	valuesYAML, ok := configMap.Data["values.yaml"]
	if !ok {
		return "", fmt.Errorf("ConfigMap %s/%s does not contain 'values.yaml' key", namespace, configMapName)
	}
	if valuesYAML == "" {
		return "", fmt.Errorf("'values.yaml' key in ConfigMap %s/%s is empty", namespace, configMapName)
	}
	return valuesYAML, nil
}

// discoverControllerService finds the gateway-controller Service for a Helm release.
func discoverControllerService(ctx context.Context, c client.Client, gatewayName, namespace string) (*corev1.Service, int32, error) {
	releaseName := helm.GetReleaseName(gatewayName)
	serviceList := &corev1.ServiceList{}
	if err := c.List(ctx, serviceList,
		client.InNamespace(namespace),
		client.MatchingLabels{
			"app.kubernetes.io/instance":  releaseName,
			"app.kubernetes.io/component": "controller",
		},
	); err != nil {
		return nil, 0, fmt.Errorf("failed to list gateway controller services: %w", err)
	}
	if len(serviceList.Items) == 0 {
		return nil, 0, fmt.Errorf("no gateway controller service found for release %s in namespace %s", releaseName, namespace)
	}
	svc := &serviceList.Items[0]
	var restPort int32 = 9090
	for _, port := range svc.Spec.Ports {
		if port.Name == "rest" || port.Port == 9090 {
			restPort = port.Port
			break
		}
	}
	return svc, restPort, nil
}

// registerGatewayInRegistry discovers the controller service and registers the gateway.
func registerGatewayInRegistry(ctx context.Context, c client.Client, name, namespace string, apiSelector *apiv1.APISelector, controlPlaneHost string, helmValuesCM string, fromK8sGW bool) error {
	svc, restPort, err := discoverControllerService(ctx, c, name, namespace)
	if err != nil {
		return err
	}
	info := &registry.GatewayInfo{
		Name:                    name,
		Namespace:               namespace,
		APISelector:             apiSelector,
		ServiceName:             svc.Name,
		ServicePort:             restPort,
		ControlPlaneHost:        controlPlaneHost,
		HelmValuesConfigMapName: helmValuesCM,
		FromGatewayAPI:          fromK8sGW,
	}
	registry.GetGatewayRegistry().Register(info)
	return nil
}

// gatewayHelmExpectedResourcesPresent reports whether cluster objects for the Helm release still exist
// (at least one Deployment labeled for the release and the gateway-controller Service used for discovery).
// A false result with a nil error means resources were likely removed out-of-band and Helm should re-run.
func gatewayHelmExpectedResourcesPresent(ctx context.Context, c client.Client, gatewayName, namespace string) (ok bool, detail string, err error) {
	releaseName := helm.GetReleaseName(gatewayName)
	deployments := &appsv1.DeploymentList{}
	if err := c.List(ctx, deployments, client.InNamespace(namespace), client.MatchingLabels(map[string]string{
		"app.kubernetes.io/instance": releaseName,
	})); err != nil {
		return false, "", fmt.Errorf("list gateway Deployments: %w", err)
	}
	if len(deployments.Items) == 0 {
		return false, "no Deployments labeled for this Helm release", nil
	}
	if _, _, err := discoverControllerService(ctx, c, gatewayName, namespace); err != nil {
		return false, fmt.Sprintf("gateway controller Service not found: %v", err), nil
	}
	return true, "", nil
}

// evaluateGatewayDeploymentsReady checks Deployments for the Helm release are fully ready.
func evaluateGatewayDeploymentsReady(ctx context.Context, c client.Client, gatewayName, namespace string) (bool, string, error) {
	releaseName := helm.GetReleaseName(gatewayName)
	deployments := &appsv1.DeploymentList{}
	if err := c.List(ctx, deployments, client.InNamespace(namespace), client.MatchingLabels(map[string]string{
		"app.kubernetes.io/instance": releaseName,
	})); err != nil {
		return false, "", fmt.Errorf("failed to list gateway deployments: %w", err)
	}
	if len(deployments.Items) == 0 {
		return false, "Gateway workloads have not been created yet", nil
	}
	var pending []string
	for _, deploy := range deployments.Items {
		desired := int32(1)
		if deploy.Spec.Replicas != nil {
			desired = *deploy.Spec.Replicas
		}
		observed := deploy.Status.ObservedGeneration
		generation := deploy.Generation
		updated := deploy.Status.UpdatedReplicas
		available := deploy.Status.AvailableReplicas
		ready := deploy.Status.ReadyReplicas
		if observed < generation || updated < desired || available < desired {
			pending = append(pending, fmt.Sprintf(
				"%s observedGeneration %d/%d, updated %d/%d, available %d/%d, ready %d/%d",
				deploy.Name, observed, generation, updated, desired, available, desired, ready, desired,
			))
		}
	}
	if len(pending) > 0 {
		return false, "Waiting for deployments to become ready: " + strings.Join(pending, ", "), nil
	}
	return true, "Gateway resources reconciled successfully", nil
}
