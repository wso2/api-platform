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

// Package steps provides step definitions for operator integration tests
package steps

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/cucumber/godog"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// K8sSteps provides Kubernetes resource operation step definitions
type K8sSteps struct {
	client        client.Client
	dynamicClient dynamic.Interface
	restMapper    meta.RESTMapper

	// Current namespace context
	currentNamespace string

	// Port forward state
	portForwards map[string]*exec.Cmd
}

// NewK8sSteps creates a new K8sSteps instance
func NewK8sSteps(c client.Client, dc dynamic.Interface, rm meta.RESTMapper) *K8sSteps {
	return &K8sSteps{
		client:           c,
		dynamicClient:    dc,
		restMapper:       rm,
		currentNamespace: "default",
		portForwards:     make(map[string]*exec.Cmd),
	}
}

// Register registers all Kubernetes step definitions
func (k *K8sSteps) Register(ctx *godog.ScenarioContext) {
	// Namespace steps
	ctx.Step(`^namespace "([^"]*)" exists$`, k.namespaceExists)
	ctx.Step(`^I create namespace "([^"]*)"$`, k.createNamespace)
	ctx.Step(`^I delete namespace "([^"]*)"$`, k.deleteNamespace)
	ctx.Step(`^I use namespace "([^"]*)"$`, k.useNamespace)

	// CR lifecycle steps
	ctx.Step(`^I apply the following CR:$`, k.applyYAML)
	ctx.Step(`^I apply the following YAML:$`, k.applyYAML)
	ctx.Step(`^I delete the "([^"]*)" "([^"]*)" in namespace "([^"]*)"$`, k.deleteResource)
	ctx.Step(`^I delete the "([^"]*)" "([^"]*)"$`, k.deleteResourceInCurrentNamespace)
	ctx.Step(`^I update the "([^"]*)" "([^"]*)" in namespace "([^"]*)" with:$`, k.updateResource)

	// Status condition steps
	ctx.Step(`^the "([^"]*)" "([^"]*)" should have condition "([^"]*)"$`, k.resourceShouldHaveCondition)
	ctx.Step(`^the "([^"]*)" "([^"]*)" should have condition "([^"]*)" within (\d+) seconds$`, k.resourceShouldHaveConditionWithTimeout)
	ctx.Step(`^the "([^"]*)" "([^"]*)" in namespace "([^"]*)" should have condition "([^"]*)" within (\d+) seconds$`, k.resourceInNamespaceShouldHaveConditionWithTimeout)
	ctx.Step(`^the "([^"]*)" "([^"]*)" status should be empty$`, k.resourceStatusShouldBeEmpty)
	ctx.Step(`^the "([^"]*)" "([^"]*)" in namespace "([^"]*)" status should be empty$`, k.resourceStatusInNamespaceShouldBeEmpty)

	// Shortcut steps
	ctx.Step(`^Gateway "([^"]*)" should be Programmed$`, k.gatewayShouldBeProgrammed)
	ctx.Step(`^Gateway "([^"]*)" should be Programmed within (\d+) seconds$`, k.gatewayShouldBeProgrammedWithTimeout)
	ctx.Step(`^Gateway "([^"]*)" is Programmed in namespace "([^"]*)"$`, k.gatewayIsProgrammedInNamespace)
	ctx.Step(`^RestApi "([^"]*)" should be Programmed$`, k.restapiShouldBeProgrammed)
	ctx.Step(`^RestApi "([^"]*)" should be Programmed within (\d+) seconds$`, k.restapiShouldBeProgrammedWithTimeout)

	// Pod verification steps
	ctx.Step(`^pods with label "([^"]*)" should be running in namespace "([^"]*)"$`, k.podsShouldBeRunning)
	ctx.Step(`^pods with label "([^"]*)" should not exist in namespace "([^"]*)"$`, k.podsShouldNotExist)

	// Service verification steps
	ctx.Step(`^service "([^"]*)" should exist in namespace "([^"]*)"$`, k.serviceShouldExist)
	ctx.Step(`^service "([^"]*)" should not exist in namespace "([^"]*)"$`, k.serviceShouldNotExist)

	// RBAC verification steps
	ctx.Step(`^Role "([^"]*)" should exist in namespace "([^"]*)"$`, k.roleShouldExist)
	ctx.Step(`^RoleBinding "([^"]*)" should exist in namespace "([^"]*)"$`, k.roleBindingShouldExist)
	ctx.Step(`^ClusterRole "([^"]*)" should exist$`, k.clusterRoleShouldExist)
	ctx.Step(`^ClusterRole "([^"]*)" should not exist$`, k.clusterRoleShouldNotExist)
	ctx.Step(`^ClusterRoleBinding "([^"]*)" should exist$`, k.clusterRoleBindingShouldExist)
	ctx.Step(`^ClusterRoleBinding "([^"]*)" should not exist$`, k.clusterRoleBindingShouldNotExist)

	// Port forward steps
	ctx.Step(`^I port-forward service "([^"]*)" in namespace "([^"]*)" to local port (\d+)$`, k.portForwardService)
	ctx.Step(`^I port-forward service "([^"]*)" to local port (\d+)$`, k.portForwardServiceCurrentNamespace)
	ctx.Step(`^I stop port-forwarding$`, k.stopAllPortForwards)

	// Utility steps
	ctx.Step(`^I wait for (\d+) seconds$`, k.waitSeconds)
}

// Reset clears state between scenarios
func (k *K8sSteps) Reset() {
	// Stop all port forwards
	k.stopAllPortForwards()
}

// namespaceExists ensures a namespace exists
func (k *K8sSteps) namespaceExists(name string) error {
	ns := &corev1.Namespace{}
	err := k.client.Get(context.Background(), client.ObjectKey{Name: name}, ns)
	if errors.IsNotFound(err) {
		return k.createNamespace(name)
	}
	return err
}

// createNamespace creates a namespace
func (k *K8sSteps) createNamespace(name string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	err := k.client.Create(context.Background(), ns)
	if errors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

// deleteNamespace deletes a namespace
func (k *K8sSteps) deleteNamespace(name string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	return client.IgnoreNotFound(k.client.Delete(context.Background(), ns))
}

// useNamespace sets the current namespace context
func (k *K8sSteps) useNamespace(name string) error {
	k.currentNamespace = name
	return nil
}

// applyYAML applies a YAML document to the cluster
func (k *K8sSteps) applyYAML(docString *godog.DocString) error {
	// Parse YAML into unstructured object
	obj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal([]byte(docString.Content), &obj.Object); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Set namespace if not specified
	if obj.GetNamespace() == "" {
		obj.SetNamespace(k.currentNamespace)
	}

	// Use kubectl apply for simplicity (handles CRDs properly)
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(docString.Content)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl apply failed: %w\nOutput: %s", err, output)
	}

	return nil
}

// deleteResource deletes a resource
func (k *K8sSteps) deleteResource(kind, name, namespace string) error {
	cmd := exec.Command("kubectl", "delete", strings.ToLower(kind), name, "-n", namespace, "--ignore-not-found")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl delete failed: %w\nOutput: %s", err, output)
	}
	return nil
}

// deleteResourceInCurrentNamespace deletes a resource in current namespace
func (k *K8sSteps) deleteResourceInCurrentNamespace(kind, name string) error {
	return k.deleteResource(kind, name, k.currentNamespace)
}

// updateResource updates a resource with a patch
func (k *K8sSteps) updateResource(kind, name, namespace string, docString *godog.DocString) error {
	cmd := exec.Command("kubectl", "patch", strings.ToLower(kind), name,
		"-n", namespace,
		"--type=merge",
		"-p", docString.Content)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl patch failed: %w\nOutput: %s", err, output)
	}
	return nil
}

// resourceShouldHaveCondition checks if a resource has a condition
func (k *K8sSteps) resourceShouldHaveCondition(kind, name, conditionType string) error {
	return k.resourceShouldHaveConditionWithTimeout(kind, name, conditionType, 60)
}

// resourceShouldHaveConditionWithTimeout checks if a resource has a condition with timeout
func (k *K8sSteps) resourceShouldHaveConditionWithTimeout(kind, name, conditionType string, timeoutSeconds int) error {
	return k.resourceInNamespaceShouldHaveConditionWithTimeout(kind, name, k.currentNamespace, conditionType, timeoutSeconds)
}

// resourceInNamespaceShouldHaveConditionWithTimeout checks condition in specific namespace
func (k *K8sSteps) resourceInNamespaceShouldHaveConditionWithTimeout(kind, name, namespace, conditionType string, timeoutSeconds int) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "wait",
		fmt.Sprintf("--for=condition=%s", conditionType),
		fmt.Sprintf("%s/%s", strings.ToLower(kind), name),
		"-n", namespace,
		fmt.Sprintf("--timeout=%ds", timeoutSeconds))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("condition %s not met: %w\nOutput: %s", conditionType, err, output)
	}
	return nil
}

// resourceStatusShouldBeEmpty checks that resource has no status
func (k *K8sSteps) resourceStatusShouldBeEmpty(kind, name string) error {
	return k.resourceStatusInNamespaceShouldBeEmpty(kind, name, k.currentNamespace)
}

// resourceStatusInNamespaceShouldBeEmpty checks that resource in namespace has no status
func (k *K8sSteps) resourceStatusInNamespaceShouldBeEmpty(kind, name, namespace string) error {
	cmd := exec.Command("kubectl", "get", strings.ToLower(kind), name,
		"-n", namespace,
		"-o", "jsonpath={.status}")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get resource status: %w", err)
	}

	status := strings.TrimSpace(string(output))
	if status != "" && status != "<nil>" && status != "{}" {
		return fmt.Errorf("expected empty status, got: %s", status)
	}

	return nil
}

// gatewayShouldBeProgrammed checks Gateway is Programmed
func (k *K8sSteps) gatewayShouldBeProgrammed(name string) error {
	return k.resourceShouldHaveConditionWithTimeout("Gateway", name, "Programmed", 180)
}

// gatewayShouldBeProgrammedWithTimeout checks Gateway is Programmed with timeout
func (k *K8sSteps) gatewayShouldBeProgrammedWithTimeout(name string, timeoutSeconds int) error {
	return k.resourceShouldHaveConditionWithTimeout("Gateway", name, "Programmed", timeoutSeconds)
}

// gatewayIsProgrammedInNamespace checks Gateway is Programmed in namespace (prerequisite step)
func (k *K8sSteps) gatewayIsProgrammedInNamespace(name, namespace string) error {
	return k.resourceInNamespaceShouldHaveConditionWithTimeout("Gateway", name, namespace, "Programmed", 180)
}

// restapiShouldBeProgrammed checks RestApi is Programmed
func (k *K8sSteps) restapiShouldBeProgrammed(name string) error {
	return k.resourceShouldHaveConditionWithTimeout("RestApi", name, "Programmed", 120)
}

// restapiShouldBeProgrammedWithTimeout checks RestApi is Programmed with timeout
func (k *K8sSteps) restapiShouldBeProgrammedWithTimeout(name string, timeoutSeconds int) error {
	return k.resourceShouldHaveConditionWithTimeout("RestApi", name, "Programmed", timeoutSeconds)
}

// podsShouldBeRunning checks pods with label are running
func (k *K8sSteps) podsShouldBeRunning(labelSelector, namespace string) error {
	cmd := exec.Command("kubectl", "wait", "--for=condition=ready", "pod",
		"-l", labelSelector,
		"-n", namespace,
		"--timeout=120s")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pods not ready: %w\nOutput: %s", err, output)
	}
	return nil
}

// podsShouldNotExist checks no pods exist with label
// Pods in Terminating status are considered as not existing since they are being deleted
func (k *K8sSteps) podsShouldNotExist(labelSelector, namespace string) error {
	cmd := exec.Command("kubectl", "get", "pods",
		"-l", labelSelector,
		"-n", namespace,
		"--no-headers")

	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	// kubectl get returns exit code 0 with "No resources found" message when no pods exist
	// This is the expected case
	if err == nil && outputStr == "" {
		return nil
	}

	// Also accept "No resources found" messages as success
	if strings.Contains(outputStr, "No resources found") {
		return nil
	}

	if err != nil {
		// Genuine error occurred
		return fmt.Errorf("kubectl get pods failed: %w\nOutput: %s", err, outputStr)
	}

	// Check if all pods are in Terminating status
	// Pods in Terminating status are acceptable as they are being deleted
	lines := strings.Split(outputStr, "\n")
	var nonTerminatingPods []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// kubectl output format: NAME READY STATUS RESTARTS AGE
		// Check if the line contains "Terminating" status
		if !strings.Contains(line, "Terminating") {
			nonTerminatingPods = append(nonTerminatingPods, line)
		}
	}

	// If all pods are terminating, consider it as success
	if len(nonTerminatingPods) == 0 {
		return nil
	}

	// Non-terminating pods were found
	return fmt.Errorf("expected no pods, but found:\n%s", strings.Join(nonTerminatingPods, "\n"))
}

// serviceShouldExist checks service exists
func (k *K8sSteps) serviceShouldExist(name, namespace string) error {
	svc := &corev1.Service{}
	return k.client.Get(context.Background(), client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, svc)
}

// serviceShouldNotExist checks service doesn't exist
func (k *K8sSteps) serviceShouldNotExist(name, namespace string) error {
	svc := &corev1.Service{}
	err := k.client.Get(context.Background(), client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, svc)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("service %s/%s still exists", namespace, name)
}

// roleShouldExist checks Role exists
func (k *K8sSteps) roleShouldExist(name, namespace string) error {
	role := &rbacv1.Role{}
	return k.client.Get(context.Background(), client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, role)
}

// roleBindingShouldExist checks RoleBinding exists
func (k *K8sSteps) roleBindingShouldExist(name, namespace string) error {
	rb := &rbacv1.RoleBinding{}
	return k.client.Get(context.Background(), client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, rb)
}

// clusterRoleShouldExist checks ClusterRole exists
func (k *K8sSteps) clusterRoleShouldExist(name string) error {
	cr := &rbacv1.ClusterRole{}
	return k.client.Get(context.Background(), client.ObjectKey{Name: name}, cr)
}

// clusterRoleShouldNotExist checks ClusterRole doesn't exist
func (k *K8sSteps) clusterRoleShouldNotExist(name string) error {
	cr := &rbacv1.ClusterRole{}
	err := k.client.Get(context.Background(), client.ObjectKey{Name: name}, cr)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("ClusterRole %s exists but should not", name)
}

// clusterRoleBindingShouldExist checks ClusterRoleBinding exists
func (k *K8sSteps) clusterRoleBindingShouldExist(name string) error {
	crb := &rbacv1.ClusterRoleBinding{}
	return k.client.Get(context.Background(), client.ObjectKey{Name: name}, crb)
}

// clusterRoleBindingShouldNotExist checks ClusterRoleBinding doesn't exist
func (k *K8sSteps) clusterRoleBindingShouldNotExist(name string) error {
	crb := &rbacv1.ClusterRoleBinding{}
	err := k.client.Get(context.Background(), client.ObjectKey{Name: name}, crb)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("ClusterRoleBinding %s exists but should not", name)
}

// portForwardService starts port forwarding to a service
func (k *K8sSteps) portForwardService(serviceName, namespace string, localPort int) error {
	key := fmt.Sprintf("%s/%s:%d", namespace, serviceName, localPort)

	// Check if already running for this exact service
	if _, exists := k.portForwards[key]; exists {
		return nil
	}

	// Kill any existing port-forward on this local port (from previous runs)
	killCmd := exec.Command("sh", "-c", fmt.Sprintf("lsof -ti tcp:%d | xargs -r kill 2>/dev/null || true", localPort))
	killCmd.Run()

	// Small delay to let port be released
	time.Sleep(500 * time.Millisecond)

	cmd := exec.Command("kubectl", "port-forward",
		fmt.Sprintf("svc/%s", serviceName),
		fmt.Sprintf("%d:8080", localPort), // Assume remote port is 8080
		"-n", namespace)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start port forward: %w", err)
	}

	k.portForwards[key] = cmd

	// Wait for port forward to be ready
	time.Sleep(10 * time.Second)

	return nil
}

// portForwardServiceCurrentNamespace port forwards in current namespace
func (k *K8sSteps) portForwardServiceCurrentNamespace(serviceName string, localPort int) error {
	return k.portForwardService(serviceName, k.currentNamespace, localPort)
}

// stopAllPortForwards stops all port forwards
func (k *K8sSteps) stopAllPortForwards() error {
	for key, cmd := range k.portForwards {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
		delete(k.portForwards, key)
	}
	return nil
}

// waitSeconds waits for specified seconds
func (k *K8sSteps) waitSeconds(seconds int) error {
	time.Sleep(time.Duration(seconds) * time.Second)
	return nil
}

// getGVR gets the GroupVersionResource for a kind
func (k *K8sSteps) getGVR(kind string) schema.GroupVersionResource {
	// Map common kinds to GVRs
	switch strings.ToLower(kind) {
	case "gateway":
		return schema.GroupVersionResource{
			Group:    "gateway.api-platform.wso2.com",
			Version:  "v1alpha1",
			Resource: "gateways",
		}
	case "restapi":
		return schema.GroupVersionResource{
			Group:    "gateway.api-platform.wso2.com",
			Version:  "v1alpha1",
			Resource: "restapis",
		}
	default:
		return schema.GroupVersionResource{
			Resource: strings.ToLower(kind) + "s",
		}
	}
}
