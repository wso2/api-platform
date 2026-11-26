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
	"maps"
	"net/http"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
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

// Annotations Helm sets on every object it installs or upgrades.
const (
	helmReleaseNameAnnotation      = "meta.helm.sh/release-name"
	helmReleaseNamespaceAnnotation = "meta.helm.sh/release-namespace"
)

// releaseServiceSource reads a Service's name and selector from the Helm release that
// created it. *helm.Client implements it; tests use a fake.
type releaseServiceSource interface {
	ReleaseService(namespace, releaseName, component string) (*helm.ManifestService, error)
}

// serviceReader reads controller Services straight from the API server for the checks that
// decide where credentials go. The manager's client reads through the informer cache, which
// can lag a change to the Service; main sets this to mgr.GetAPIReader() before the manager
// starts. Unset (unit tests), the caller's client is used.
var serviceReader client.Reader

// SetServiceReader sets the uncached reader used to verify gateway controller Services.
func SetServiceReader(r client.Reader) { serviceReader = r }

func liveServiceReader(fallback client.Reader) client.Reader {
	if serviceReader != nil {
		return serviceReader
	}
	return fallback
}

// errControllerServiceUnverified marks a management call held because the controller Service
// could not be verified. It is a wait for rediscovery, not a failed deployment, so callers do
// not count it against their retry budget.
var errControllerServiceUnverified = errors.New("gateway controller Service not verified")

// newReleaseServiceSource returns the Helm client the reconcilers read release records with.
func newReleaseServiceSource(cfg *config.OperatorConfig) (releaseServiceSource, error) {
	plainHTTP := cfg != nil && cfg.Gateway.PlainHTTP
	return helm.NewClientWithOptions(plainHTTP)
}

// discoverControllerService finds the gateway-controller Service the operator sends
// management calls, with their credentials, to.
//
// It does not trust labels: anyone who can create a Service in the namespace can copy
// them. The name comes from the manifest Helm recorded for the gateway's release, and the
// live Service under that name must still be the one Helm installed: owned by the release
// (Helm's ownership annotations), with the selector the chart rendered, and not an
// ExternalName. Anything else is an error, so the gateway is not registered rather than
// registered at an endpoint someone else chose.
func discoverControllerService(ctx context.Context, c client.Client, src releaseServiceSource, gatewayName, namespace string) (*corev1.Service, int32, error) {
	releaseName := helm.GetReleaseName(gatewayName)
	want, err := src.ReleaseService(namespace, releaseName, "controller")
	if err != nil {
		return nil, 0, fmt.Errorf("find the gateway controller Service in Helm release %s/%s: %w", namespace, releaseName, err)
	}
	svc := &corev1.Service{}
	if err := liveServiceReader(c).Get(ctx, client.ObjectKey{Namespace: namespace, Name: want.Name}, svc); err != nil {
		return nil, 0, fmt.Errorf("get gateway controller Service %s/%s: %w", namespace, want.Name, err)
	}
	if err := verifyReleaseService(svc, want, releaseName, namespace); err != nil {
		return nil, 0, err
	}
	port, err := controllerRESTPort(svc)
	if err != nil {
		return nil, 0, err
	}
	return svc, port, nil
}

// verifyReleaseService checks that svc is still the Service the release rendered as want.
func verifyReleaseService(svc *corev1.Service, want *helm.ManifestService, releaseName, namespace string) error {
	if svc.Spec.Type == corev1.ServiceTypeExternalName {
		return fmt.Errorf("gateway controller Service %s/%s is of type ExternalName; refusing to send management calls to it", namespace, svc.Name)
	}
	if svc.Annotations[helmReleaseNameAnnotation] != releaseName || svc.Annotations[helmReleaseNamespaceAnnotation] != namespace {
		return fmt.Errorf("gateway controller Service %s/%s is not owned by Helm release %s/%s", namespace, svc.Name, namespace, releaseName)
	}
	if len(want.Selector) > 0 && !maps.Equal(svc.Spec.Selector, want.Selector) {
		return fmt.Errorf("gateway controller Service %s/%s selects other pods than Helm release %s/%s rendered", namespace, svc.Name, namespace, releaseName)
	}
	return nil
}

// controllerRESTPort is the Service port the gateway controller's plain-HTTP management API
// listens on: the port named "rest", else one numbered 9090, else 9090.
//
// With controller TLS enabled the chart exposes only "rest-tls". The operator calls the
// management API over plain HTTP (registry.GatewayInfo.GetGatewayServiceEndpoint) and has no
// way yet to trust the controller's certificate, so such a Service is refused with an
// explicit error rather than registered on a port it does not serve.
func controllerRESTPort(svc *corev1.Service) (int32, error) {
	tlsOnly := false
	for _, port := range svc.Spec.Ports {
		if port.Name == "rest" {
			return port.Port, nil
		}
		if port.Name == "rest-tls" {
			tlsOnly = true
		}
	}
	for _, port := range svc.Spec.Ports {
		// A rest-tls port is never plain HTTP, whatever its number.
		if port.Port == 9090 && port.Name != "rest-tls" {
			return port.Port, nil
		}
	}
	if tlsOnly {
		return 0, fmt.Errorf("gateway controller Service %s/%s exposes its management API only over TLS (rest-tls); "+
			"the operator calls it over plain HTTP and does not support a TLS-only controller yet", svc.Namespace, svc.Name)
	}
	return 9090, nil
}

// discoverGatewayRuntimeService finds the gateway-runtime Service for a Helm release.
func discoverGatewayRuntimeService(ctx context.Context, c client.Client, gatewayName, namespace string) (*corev1.Service, error) {
	releaseName := helm.GetReleaseName(gatewayName)
	serviceList := &corev1.ServiceList{}
	if err := c.List(ctx, serviceList,
		client.InNamespace(namespace),
		client.MatchingLabels{
			"app.kubernetes.io/instance":  releaseName,
			"app.kubernetes.io/component": "gateway-runtime",
		},
	); err != nil {
		return nil, fmt.Errorf("failed to list gateway runtime services: %w", err)
	}
	if len(serviceList.Items) == 0 {
		return nil, fmt.Errorf("no gateway runtime service found for release %s in namespace %s", releaseName, namespace)
	}
	return &serviceList.Items[0], nil
}

// registerGatewayInRegistry discovers the controller service and registers the gateway.
//
// When discovery fails, an earlier entry for the gateway is kept. It is not trusted as it
// stands: every request checks the live Service against the identity recorded here
// (verifiedAuth, managementEndpoint), so a Service replaced or retargeted since is refused.
// Keeping the entry means a transient error (Helm storage, the API server) does not drop a
// healthy gateway, and deletions still find it as a cleanup target and keep their finalizer
// until the API is removed, instead of completing while the API stays deployed.
func registerGatewayInRegistry(ctx context.Context, c client.Client, src releaseServiceSource, name, namespace string, apiSelector *apiv1.APISelector, controlPlaneHost string, helmValuesCM string, fromK8sGW bool) error {
	svc, restPort, err := discoverControllerService(ctx, c, src, name, namespace)
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
		ServiceUID:              string(svc.UID),
		ServiceSelector:         maps.Clone(svc.Spec.Selector),
	}
	registry.GetGatewayRegistry().Register(info)
	return nil
}

// managementEndpoint returns the URL of the gateway's management API after checking that the
// Service it names is still the one discovery verified: same UID (not deleted and recreated),
// same selector, and not an ExternalName. Registration only runs while the gateway is ready, so
// an entry can outlive a change to the Service; every management call goes through this check
// so it never sends credentials to a Service that has not been verified.
//
// The Service is read from the API server (see serviceReader), not the informer cache. Call it
// immediately before each request that carries credentials; it narrows the window in which the
// Service can change before the request is sent, though the read and the request are not atomic.
// Errors wrap errControllerServiceUnverified.
func managementEndpoint(ctx context.Context, c client.Reader, gw *registry.GatewayInfo) (string, error) {
	svc := &corev1.Service{}
	if err := liveServiceReader(c).Get(ctx, client.ObjectKey{Namespace: gw.Namespace, Name: gw.ServiceName}, svc); err != nil {
		return "", fmt.Errorf("%w: get gateway %s/%s controller Service %s: %w",
			errControllerServiceUnverified, gw.Namespace, gw.Name, gw.ServiceName, err)
	}
	if gw.ServiceUID == "" || string(svc.UID) != gw.ServiceUID ||
		svc.Spec.Type == corev1.ServiceTypeExternalName ||
		!maps.Equal(svc.Spec.Selector, gw.ServiceSelector) {
		return "", fmt.Errorf("%w: gateway %s/%s controller Service %s changed since it was verified; "+
			"management calls are held until the gateway is rediscovered",
			errControllerServiceUnverified, gw.Namespace, gw.Name, gw.ServiceName)
	}
	return gw.GetGatewayServiceEndpoint(), nil
}

// verifiedAuth wraps a request's auth function so that, immediately before credentials are
// attached to any request, the gateway's controller Service is verified again (managementEndpoint)
// and the request is confirmed to target that verified endpoint. Every gateway client call passes
// its request through the auth function, so this covers callers that send several requests in
// one operation (for example an existence check followed by a create). A refusal wraps
// errControllerServiceUnverified and no credentials are added.
func verifiedAuth(c client.Reader, gw *registry.GatewayInfo, attach gatewayclient.AuthHeaderFunc) gatewayclient.AuthHeaderFunc {
	return func(ctx context.Context, req *http.Request) error {
		ep, err := managementEndpoint(ctx, c, gw)
		if err != nil {
			return err
		}
		if req.URL == nil || !strings.HasPrefix(req.URL.String(), ep+"/") {
			return fmt.Errorf("%w: request for gateway %s/%s does not target its controller Service endpoint",
				errControllerServiceUnverified, gw.Namespace, gw.Name)
		}
		if attach == nil {
			return nil
		}
		return attach(ctx, req)
	}
}

// gatewayHelmExpectedResourcesPresent reports whether cluster objects for the Helm release still exist
// (at least one Deployment labeled for the release, the gateway-controller Service used for discovery,
// and the gateway-runtime Service that syncGateway requires for the dataplane). A false result with a
// nil error means resources were likely removed out-of-band and Helm should re-run.
//
// The gateway-runtime Service must be included here: syncGateway treats it as required downstream (it
// errors out when discoverGatewayRuntimeService fails). If it were omitted, a runtime Service deleted
// out-of-band would leave resourcesOK (and thus skipHelm) true, so Helm would never re-run to recreate
// it, and reconcile would loop returning pending/error forever without ever healing.
//
// The controller Service is checked the way registration finds it (discoverControllerService),
// not by labels: a look-alike Service carrying the release's labels must not make the release
// look healthy while the Service Helm created is gone, or Helm would never recreate it and
// registration would fail forever.
func gatewayHelmExpectedResourcesPresent(ctx context.Context, c client.Client, src releaseServiceSource, gatewayName, namespace string) (ok bool, detail string, err error) {
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
	if _, _, err := discoverControllerService(ctx, c, src, gatewayName, namespace); err != nil {
		return false, fmt.Sprintf("gateway controller Service not found: %v", err), nil
	}
	if _, err := discoverGatewayRuntimeService(ctx, c, gatewayName, namespace); err != nil {
		return false, fmt.Sprintf("gateway runtime Service not found: %v", err), nil
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
