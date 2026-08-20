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

package helm

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
)

// gatewayHelmChartPath points at the gateway-helm-chart source relative to this
// package (kubernetes/gateway-operator/internal/helm) — the same chart the
// operator installs via InstallOrUpgrade/UpgradeOrInstall.
const gatewayHelmChartPath = "../../../helm/gateway-helm-chart"

// operatorHelmChartValuesPath points at the operator chart's values.yaml, whose
// gateway.values block is written to the gateway-values ConfigMap and handed to
// the gateway chart install as user-supplied values (helm_values_file_path).
const operatorHelmChartValuesPath = "../../../helm/operator-helm-chart/values.yaml"

// routerOverrides describes the gateway.config.router fields relevant to health
// probes. A zero value for listenerPort/httpsPort leaves the chart default.
type routerOverrides struct {
	httpsEnabled bool
	listenerPort int
	httpsPort    int
}

// renderGatewayRuntimeDeployment renders the gateway-helm-chart with the given
// gateway.config.router overrides and returns the parsed gateway-runtime
// Deployment manifest.
func renderGatewayRuntimeDeployment(t *testing.T, ro routerOverrides) map[string]interface{} {
	t.Helper()

	router := map[string]interface{}{
		"https_enabled": ro.httpsEnabled,
	}
	if ro.listenerPort != 0 {
		router["listener_port"] = ro.listenerPort
	}
	if ro.httpsPort != 0 {
		router["https_port"] = ro.httpsPort
	}

	overrides := map[string]interface{}{
		"gateway": map[string]interface{}{
			"controller": map[string]interface{}{
				// Required: the controller deployment template fails the render
				// outright when encryption keys are not configured.
				"encryptionKeys": map[string]interface{}{
					"enabled":    true,
					"secretName": "dummy-secret",
				},
			},
			"config": map[string]interface{}{
				"router": router,
			},
		},
	}

	rendered := renderGatewayChart(t, overrides)
	return gatewayChartDeployment(t, rendered, "gateway/templates/gateway/gateway-runtime/deployment.yaml")
}

// renderGatewayChart coalesces overrides over the gateway chart's own defaults
// exactly the way `helm upgrade --install -f values.yaml` does, and renders it.
func renderGatewayChart(t *testing.T, overrides map[string]interface{}) map[string]string {
	t.Helper()

	chrt, err := loader.Load(gatewayHelmChartPath)
	if err != nil {
		t.Fatalf("load chart: %v", err)
	}
	vals, err := chartutil.CoalesceValues(chrt, overrides)
	if err != nil {
		t.Fatalf("coalesce values: %v", err)
	}
	renderValues, err := chartutil.ToRenderValues(chrt, vals, chartutil.ReleaseOptions{
		Name:      "test",
		Namespace: "default",
	}, nil)
	if err != nil {
		t.Fatalf("build render values: %v", err)
	}
	rendered, err := engine.Render(chrt, renderValues)
	if err != nil {
		t.Fatalf("render chart: %v", err)
	}
	return rendered
}

func gatewayChartDeployment(t *testing.T, rendered map[string]string, templateName string) map[string]interface{} {
	t.Helper()

	manifest, ok := rendered[templateName]
	if !ok {
		t.Fatalf("template %q not found in rendered output; available: %v", templateName, keysOf(rendered))
	}
	var deployment map[string]interface{}
	if err := yaml.Unmarshal([]byte(manifest), &deployment); err != nil {
		t.Fatalf("parse rendered deployment manifest: %v\n%s", err, manifest)
	}
	return deployment
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		if strings.TrimSpace(m[k]) != "" {
			out = append(out, k)
		}
	}
	return out
}

func probeHTTPGet(t *testing.T, deployment map[string]interface{}, probeName string) map[string]interface{} {
	t.Helper()
	containers := navigate(t, deployment, "spec", "template", "spec", "containers")
	list, ok := containers.([]interface{})
	if !ok || len(list) == 0 {
		t.Fatalf("expected non-empty containers list, got %#v", containers)
	}
	container, ok := list[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected container map, got %#v", list[0])
	}
	probe, ok := container[probeName].(map[string]interface{})
	if !ok {
		t.Fatalf("expected %s to be a map on the gateway-runtime container, got %#v", probeName, container[probeName])
	}
	httpGet, ok := probe["httpGet"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected %s.httpGet to be a map, got %#v", probeName, probe["httpGet"])
	}
	return httpGet
}

func navigate(t *testing.T, m map[string]interface{}, path ...string) interface{} {
	t.Helper()
	var cur interface{} = m
	for _, p := range path {
		asMap, ok := cur.(map[string]interface{})
		if !ok {
			t.Fatalf("cannot navigate to %q: %#v is not a map", p, cur)
		}
		cur, ok = asMap[p]
		if !ok {
			t.Fatalf("key %q not found while navigating %v", p, path)
		}
	}
	return cur
}

func assertProbePort(t *testing.T, deployment map[string]interface{}, probeName string, wantPort int, wantScheme string) {
	t.Helper()
	httpGet := probeHTTPGet(t, deployment, probeName)
	if got := httpGet["port"]; got != wantPort {
		t.Errorf("%s.httpGet.port = %v (%T), want %d", probeName, got, got, wantPort)
	}
	if got := httpGet["scheme"]; got != wantScheme {
		t.Errorf("%s.httpGet.scheme = %v, want %q", probeName, got, wantScheme)
	}
}

// TestGatewayRuntimeProbes_HTTPSEnabled verifies the current, unchanged behavior:
// when the router's HTTPS listener is enabled (the chart default), liveness and
// readiness probes target the router's actual HTTPS bind port over HTTPS.
func TestGatewayRuntimeProbes_HTTPSEnabled(t *testing.T) {
	deployment := renderGatewayRuntimeDeployment(t, routerOverrides{httpsEnabled: true})

	assertProbePort(t, deployment, "livenessProbe", 8443, "HTTPS")
	assertProbePort(t, deployment, "readinessProbe", 8443, "HTTPS")
}

// TestGatewayRuntimeProbes_HTTPSDisabled is the regression test for the fix: when
// the router's HTTPS listener is disabled, the router only binds listener_port
// (plain HTTP), so liveness/readiness probes must target that port over HTTP
// instead of a listener that doesn't exist — otherwise the pod never becomes
// ready/alive.
func TestGatewayRuntimeProbes_HTTPSDisabled(t *testing.T) {
	deployment := renderGatewayRuntimeDeployment(t, routerOverrides{httpsEnabled: false})

	assertProbePort(t, deployment, "livenessProbe", 8080, "HTTP")
	assertProbePort(t, deployment, "readinessProbe", 8080, "HTTP")
}

// TestGatewayRuntimeProbes_CustomRouterPorts guards against deriving the probe
// port from the "http"/"https" named container ports (gatewayRuntime.service.
// ports.*): those are an independently configurable value that nothing forces
// to stay in sync with gateway.config.router.listener_port/https_port, which is
// what the router actually binds to (see gateway-config.yaml). The probe must
// track the router config directly, so a listener_port/https_port override
// with the service ports left at their defaults must still produce a matching
// probe port.
func TestGatewayRuntimeProbes_CustomRouterPorts(t *testing.T) {
	deployment := renderGatewayRuntimeDeployment(t, routerOverrides{
		httpsEnabled: true,
		listenerPort: 9999,
		httpsPort:    9443,
	})

	assertProbePort(t, deployment, "livenessProbe", 9443, "HTTPS")
	assertProbePort(t, deployment, "readinessProbe", 9443, "HTTPS")
}

// probeHandlerKeys are the mutually exclusive Probe handler fields; Kubernetes
// rejects a Deployment whose probe sets more than one of them ("may not specify
// more than 1 handler type").
var probeHandlerKeys = []string{"exec", "httpGet", "tcpSocket", "grpc"}

// TestOperatorChartValues_ProbesHaveSingleHandler guards the operator chart's
// gateway.values block (mounted as gateway_values.yaml and passed to the gateway
// chart install) against handler-type drift from the gateway chart's own defaults.
//
// Helm coalesces user-supplied values over chart defaults by deep-merging maps, so a
// probe overridden with a *different* handler type does not replace the chart's — both
// end up in the rendered Deployment and the API server rejects it, leaving the Gateway
// stuck Programmed=False ("failed to create resource: ... livenessProbe.httpGet:
// Forbidden: may not specify more than 1 handler type"). That is exactly how the
// operator chart's `exec: health-check.sh` runtime probes broke the Gateway API
// conformance run after the gateway chart switched to httpGet health routes.
func TestOperatorChartValues_ProbesHaveSingleHandler(t *testing.T) {
	raw, err := os.ReadFile(operatorHelmChartValuesPath)
	if err != nil {
		t.Fatalf("read operator chart values: %v", err)
	}
	var operatorValues struct {
		Gateway struct {
			Values map[string]interface{} `yaml:"values"`
		} `yaml:"gateway"`
	}
	if err := yaml.Unmarshal(raw, &operatorValues); err != nil {
		t.Fatalf("parse operator chart values: %v", err)
	}
	overlay := operatorValues.Gateway.Values
	if len(overlay) == 0 {
		t.Fatalf("gateway.values missing/empty in %s", operatorHelmChartValuesPath)
	}
	// Mirrors conformance/install-wso2-gateway.sh: encryption keys are mandatory
	// for the chart to render at all.
	if gw, ok := overlay["gateway"].(map[string]interface{}); ok {
		controller, _ := gw["controller"].(map[string]interface{})
		if controller == nil {
			controller = map[string]interface{}{}
			gw["controller"] = controller
		}
		controller["encryptionKeys"] = map[string]interface{}{
			"enabled":    true,
			"secretName": "dummy-secret",
		}
	}

	rendered := renderGatewayChart(t, overlay)
	for _, templateName := range []string{
		"gateway/templates/gateway/gateway-runtime/deployment.yaml",
		"gateway/templates/gateway/controller/deployment.yaml",
	} {
		deployment := gatewayChartDeployment(t, rendered, templateName)
		containers, ok := navigate(t, deployment, "spec", "template", "spec", "containers").([]interface{})
		if !ok || len(containers) == 0 {
			t.Fatalf("%s: expected a non-empty containers list", templateName)
		}
		for _, c := range containers {
			container, ok := c.(map[string]interface{})
			if !ok {
				t.Fatalf("%s: expected container map, got %#v", templateName, c)
			}
			name, _ := container["name"].(string)
			for _, probeName := range []string{"livenessProbe", "readinessProbe", "startupProbe"} {
				probe, ok := container[probeName].(map[string]interface{})
				if !ok {
					continue // probe not configured — nothing to validate
				}
				var handlers []string
				for _, key := range probeHandlerKeys {
					if _, present := probe[key]; present {
						handlers = append(handlers, key)
					}
				}
				if len(handlers) != 1 {
					t.Errorf("%s: container %q %s has handlers %v, want exactly 1 — "+
						"the operator chart's gateway.values probe must use the same handler type as "+
						"gateway-helm-chart/values.yaml, since Helm merges (not replaces) the two maps",
						templateName, name, probeName, handlers)
				}
			}
		}
	}
}
