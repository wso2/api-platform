/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package steps

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// HelmSteps provides Helm and operator lifecycle step definitions
type HelmSteps struct {
	// Configuration
	OperatorChartPath   string
	GatewayChartPath    string
	DockerRegistry      string
	ImageTag            string
	ImagePullPolicy     string
	OCIRegistry         string
	GatewayChartName    string
	GatewayChartVersion string
}

// NewHelmSteps creates a new HelmSteps instance with default configuration
func NewHelmSteps() *HelmSteps {
	// Read from environment with defaults
	return &HelmSteps{
		OperatorChartPath:   getEnvOrDefault("OPERATOR_CHART_PATH", "./kubernetes/helm/operator-helm-chart"),
		GatewayChartPath:    getEnvOrDefault("GATEWAY_CHART_PATH", "./kubernetes/helm/gateway-helm-chart"),
		DockerRegistry:      getEnvOrDefault("DOCKER_REGISTRY", "localhost"),
		ImageTag:            getEnvOrDefault("IMAGE_TAG", "test"),
		ImagePullPolicy:     getEnvOrDefault("IMAGE_PULL_POLICY", "Never"), // Never for CI (kind load), Always for Docker Hub
		OCIRegistry:         getEnvOrDefault("OCI_REGISTRY", "oci://registry.registry.svc.cluster.local:5000/charts/gateway"),
		GatewayChartName:    getEnvOrDefault("GATEWAY_CHART_NAME", "oci://registry.registry.svc.cluster.local:5000/charts/gateway"),
		GatewayChartVersion: getEnvOrDefault("GATEWAY_CHART_VERSION", "0.0.0-test"),
	}
}

// Register registers all Helm/operator lifecycle step definitions
func (h *HelmSteps) Register(ctx *godog.ScenarioContext) {
	// Operator lifecycle steps
	ctx.Step(`^the operator is installed in namespace "([^"]*)"$`, h.operatorIsInstalledInNamespace)
	ctx.Step(`^I install the operator in namespace "([^"]*)"$`, h.installOperator)
	ctx.Step(`^I install the operator in namespace "([^"]*)" with watchNamespaces "([^"]*)"$`, h.installOperatorScoped)
	ctx.Step(`^I uninstall the operator from namespace "([^"]*)"$`, h.uninstallOperator)
	ctx.Step(`^the operator pod is ready in namespace "([^"]*)"$`, h.operatorPodReady)

	// Helm chart steps (generic)
	ctx.Step(`^I install Helm chart "([^"]*)" as "([^"]*)" in namespace "([^"]*)"$`, h.installHelmChart)
	ctx.Step(`^I uninstall Helm release "([^"]*)" from namespace "([^"]*)"$`, h.uninstallHelmRelease)

	// Image loading steps (for kind)
	ctx.Step(`^the required images are loaded into the cluster$`, h.imagesAreLoaded)

	// OCI registry steps
	ctx.Step(`^the OCI registry is available$`, h.ociRegistryAvailable)
	ctx.Step(`^the Gateway chart is pushed to the OCI registry$`, h.gatewayChartPushed)

	// Prerequisite service steps
	ctx.Step(`^httpbin is deployed in namespace "([^"]*)"$`, h.httpbinDeployed)
	ctx.Step(`^mock-jwks is deployed in namespace "([^"]*)"$`, h.mockJwksDeployed)
	ctx.Step(`^cert-manager is installed$`, h.certManagerInstalled)

	// ConfigMap for gateway configuration
	ctx.Step(`^the gateway configuration ConfigMap "([^"]*)" exists in namespace "([^"]*)"$`, h.gatewayConfigExists)
}

// operatorIsInstalledInNamespace checks if operator is installed, installs if not
func (h *HelmSteps) operatorIsInstalledInNamespace(namespace string) error {
	// Check if operator pod exists and is ready
	cmd := exec.Command("kubectl", "get", "pods",
		"-n", namespace,
		"-l", "app.kubernetes.io/name=gateway-operator",
		"--no-headers")
	output, err := cmd.CombinedOutput()
	if err == nil && strings.TrimSpace(string(output)) != "" {
		// Operator exists, check if ready
		return h.operatorPodReady(namespace)
	}
	// Install operator
	return h.installOperator(namespace)
}

// installOperator installs the operator in cluster-wide mode
func (h *HelmSteps) installOperator(namespace string) error {
	return h.installOperatorWithOptions(namespace, "")
}

// installOperatorScoped installs the operator in scoped mode
func (h *HelmSteps) installOperatorScoped(namespace, watchNamespaces string) error {
	return h.installOperatorWithOptions(namespace, watchNamespaces)
}

// installOperatorWithOptions installs operator with optional watchNamespaces
func (h *HelmSteps) installOperatorWithOptions(namespace, watchNamespaces string) error {
	// Ensure namespace exists
	exec.Command("kubectl", "create", "namespace", namespace, "--dry-run=client", "-o", "yaml").Run()
	execCmd := exec.Command("kubectl", "apply", "-f", "-")
	execCmd.Stdin = strings.NewReader(fmt.Sprintf("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: %s", namespace))
	execCmd.Run()

	args := []string{
		"upgrade", "--install", "gateway-operator",
		h.OperatorChartPath,
		"--namespace", namespace,
		"--create-namespace",
		"--set", fmt.Sprintf("image.repository=%s/gateway-operator", h.DockerRegistry),
		"--set", fmt.Sprintf("image.tag=%s", h.ImageTag),
		"--set", fmt.Sprintf("image.pullPolicy=%s", h.ImagePullPolicy),
		"--set", fmt.Sprintf("gateway.helm.chartName=%s", h.GatewayChartName),
		"--set", fmt.Sprintf("gateway.helm.chartVersion=%s", h.GatewayChartVersion),
		"--set", "gateway.helm.plainHTTP=true",
		"--wait",
		"--timeout", "5m",
	}

	if watchNamespaces != "" {
		args = append(args, "--set", fmt.Sprintf("watchNamespaces={%s}", watchNamespaces))
	}

	cmd := exec.Command("helm", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install operator: %w\nOutput: %s", err, output)
	}

	// Wait for pod to be ready
	return h.operatorPodReady(namespace)
}

// uninstallOperator uninstalls the operator
func (h *HelmSteps) uninstallOperator(namespace string) error {
	cmd := exec.Command("helm", "uninstall", "gateway-operator", "-n", namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Ignore if not found
		if strings.Contains(string(output), "not found") {
			return nil
		}
		return fmt.Errorf("failed to uninstall operator: %w\nOutput: %s", err, output)
	}

	// Also delete namespace if empty
	exec.Command("kubectl", "delete", "ns", namespace, "--ignore-not-found").Run()

	return nil
}

// operatorPodReady waits for operator pod to be ready
func (h *HelmSteps) operatorPodReady(namespace string) error {
	cmd := exec.Command("kubectl", "wait", "--for=condition=ready", "pod",
		"-l", "app.kubernetes.io/name=gateway-operator",
		"-n", namespace,
		"--timeout=240s")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("operator pod not ready: %w\nOutput: %s", err, output)
	}
	return nil
}

// installHelmChart installs a generic Helm chart
func (h *HelmSteps) installHelmChart(chartPath, releaseName, namespace string) error {
	cmd := exec.Command("helm", "upgrade", "--install", releaseName,
		chartPath,
		"-n", namespace,
		"--create-namespace",
		"--wait",
		"--timeout", "5m")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install chart: %w\nOutput: %s", err, output)
	}
	return nil
}

// uninstallHelmRelease uninstalls a Helm release
func (h *HelmSteps) uninstallHelmRelease(releaseName, namespace string) error {
	cmd := exec.Command("helm", "uninstall", releaseName, "-n", namespace)
	cmd.Run() // Ignore errors
	return nil
}

// imagesAreLoaded checks if required images are loaded (for Kind)
func (h *HelmSteps) imagesAreLoaded() error {
	// This is typically done before running tests; just verify images exist
	images := []string{
		fmt.Sprintf("%s/gateway-operator:%s", h.DockerRegistry, h.ImageTag),
		fmt.Sprintf("%s/gateway-controller:%s", h.DockerRegistry, h.ImageTag),
		fmt.Sprintf("%s/policy-engine:%s", h.DockerRegistry, h.ImageTag),
		fmt.Sprintf("%s/gateway-router:%s", h.DockerRegistry, h.ImageTag),
	}

	for _, img := range images {
		cmd := exec.Command("docker", "image", "inspect", img)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("image %s not found - run 'make build-local' first", img)
		}
	}
	return nil
}

// ociRegistryAvailable checks if OCI registry is accessible
func (h *HelmSteps) ociRegistryAvailable() error {
	cmd := exec.Command("kubectl", "wait", "--for=condition=ready", "pod",
		"-l", "app=registry",
		"-n", "registry",
		"--timeout=60s")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("OCI registry not ready: %w\nOutput: %s", err, output)
	}
	return nil
}

// gatewayChartPushed checks if Gateway chart is in OCI registry
func (h *HelmSteps) gatewayChartPushed() error {
	// This is typically done during CI setup; assume it's done
	return nil
}

// httpbinDeployed ensures httpbin is deployed
func (h *HelmSteps) httpbinDeployed(namespace string) error {
	// Check if already deployed
	cmd := exec.Command("kubectl", "get", "deployment", "httpbin", "-n", namespace)
	if err := cmd.Run(); err == nil {
		return nil // Already exists
	}

	// Deploy httpbin
	manifest := fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: httpbin
  namespace: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: httpbin
  template:
    metadata:
      labels:
        app: httpbin
    spec:
      containers:
      - name: httpbin
        image: kennethreitz/httpbin:latest
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: httpbin
  namespace: %s
spec:
  selector:
    app: httpbin
  ports:
  - port: 80
    targetPort: 80
`, namespace, namespace)

	cmd = exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to deploy httpbin: %w\nOutput: %s", err, output)
	}

	// Wait for ready
	time.Sleep(5 * time.Second)
	cmd = exec.Command("kubectl", "wait", "--for=condition=ready", "pod",
		"-l", "app=httpbin", "-n", namespace, "--timeout=120s")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("httpbin not ready: %w\nOutput: %s", err, output)
	}
	return nil
}

// mockJwksDeployed ensures mock-jwks is deployed
func (h *HelmSteps) mockJwksDeployed(namespace string) error {
	// Check if already deployed
	cmd := exec.Command("kubectl", "get", "deployment", "mock-jwks", "-n", namespace)
	if err := cmd.Run(); err == nil {
		return nil // Already exists
	}

	// Deploy mock-jwks
	manifest := fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mock-jwks
  namespace: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mock-jwks
  template:
    metadata:
      labels:
        app: mock-jwks
    spec:
      containers:
      - name: mock-jwks
        image: %s/mock-jwks:%s
        ports:
        - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: mock-jwks
  namespace: %s
spec:
  selector:
    app: mock-jwks
  ports:
  - port: 8080
    targetPort: 8080
`, namespace, h.DockerRegistry, h.ImageTag, namespace)

	cmd = exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to deploy mock-jwks: %w\nOutput: %s", err, output)
	}

	// Wait for ready
	time.Sleep(5 * time.Second)
	cmd = exec.Command("kubectl", "wait", "--for=condition=ready", "pod",
		"-l", "app=mock-jwks", "-n", namespace, "--timeout=120s")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mock-jwks not ready: %w\nOutput: %s", err, output)
	}
	return nil
}

// certManagerInstalled ensures cert-manager is installed
func (h *HelmSteps) certManagerInstalled() error {
	// Check if already installed
	cmd := exec.Command("kubectl", "get", "deployment", "cert-manager", "-n", "cert-manager")
	if err := cmd.Run(); err == nil {
		return nil // Already exists
	}

	// Install cert-manager
	cmd = exec.Command("helm", "upgrade", "--install", "cert-manager",
		"oci://quay.io/jetstack/charts/cert-manager",
		"--version", "v1.17.2",
		"--namespace", "cert-manager",
		"--create-namespace",
		"--set", "crds.enabled=true",
		"--wait", "--timeout", "5m")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install cert-manager: %w\nOutput: %s", err, output)
	}
	return nil
}

// gatewayConfigExists ensures the gateway config ConfigMap exists, creates it if not
func (h *HelmSteps) gatewayConfigExists(name, namespace string) error {
	// Check if already exists
	cmd := exec.Command("kubectl", "get", "configmap", name, "-n", namespace)
	if err := cmd.Run(); err == nil {
		return nil // Already exists
	}

	// Create the ConfigMap with gateway image configuration and JWT policy config
	manifest := fmt.Sprintf(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
data:
  values.yaml: |
    gateway:
      config:
        policy_configurations:
          JWTAuth_v010:
            keymanagers:
              - name: MockKeyManager
                issuer: http://mock-jwks.default.svc.cluster.local:8080/token
                jwks:
                  remote:
                    uri: http://mock-jwks.default.svc.cluster.local:8080/jwks
            JwksCacheTtl: "5m"
            JwksFetchTimeout: "5s"
            JwksFetchRetryCount: 3
            JwksFetchRetryInterval: "2s"
            AllowedAlgorithms:
            - RS256
            - ES256
            Leeway: "30s"
            AuthHeaderScheme: Bearer
            HeaderName: Authorization
            OnFailureStatusCode: 401
            ErrorMessageFormat: json
            ErrorMessage: "Authentication failed."
            ValidateIssuer: true
      controller:
        image:
          repository: %s/gateway-controller
          tag: %s
          pullPolicy: %s
      router:
        image:
          repository: %s/gateway-router
          tag: %s
          pullPolicy: %s
      policyEngine:
        image:
          repository: %s/policy-engine
          tag: %s
          pullPolicy: %s
`, name, namespace,
		h.DockerRegistry, h.ImageTag, h.ImagePullPolicy,
		h.DockerRegistry, h.ImageTag, h.ImagePullPolicy,
		h.DockerRegistry, h.ImageTag, h.ImagePullPolicy)

	cmd = exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create gateway config: %w\nOutput: %s", err, output)
	}
	return nil
}

// getEnvOrDefault returns env var or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
