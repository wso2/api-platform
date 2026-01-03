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

package it

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestState holds global test state
type TestState struct {
	// Kubernetes clients
	K8sClient     client.Client
	Clientset     *kubernetes.Clientset
	DynamicClient dynamic.Interface
	RestMapper    meta.RESTMapper
	Config        *rest.Config

	// HTTP client for API invocation tests
	HTTPClient *http.Client

	// Port forward management
	portForwards   map[string]*exec.Cmd
	portForwardsMu sync.Mutex

	// Current test context
	CurrentNamespace string
	Token            string // JWT token for auth tests
}

// NewTestState creates a new TestState with initialized clients
func NewTestState() (*TestState, error) {
	// Load kubeconfig
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes config: %w", err)
	}

	// Create kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Create controller-runtime client
	k8sClient, err := client.New(config, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Create REST mapper
	restMapper := meta.NewDefaultRESTMapper(nil)

	// Create HTTP client
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &TestState{
		K8sClient:        k8sClient,
		Clientset:        clientset,
		DynamicClient:    dynamicClient,
		RestMapper:       restMapper,
		Config:           config,
		HTTPClient:       httpClient,
		portForwards:     make(map[string]*exec.Cmd),
		CurrentNamespace: "default",
	}, nil
}

// Reset clears state between scenarios
func (s *TestState) Reset() {
	s.Token = ""
	// Don't reset namespace or port forwards - they persist across scenarios
}

// Cleanup performs final cleanup
func (s *TestState) Cleanup() {
	s.StopAllPortForwards()
}

// StartPortForward starts a port forward to a service
func (s *TestState) StartPortForward(namespace, serviceName string, localPort, remotePort int) error {
	s.portForwardsMu.Lock()
	defer s.portForwardsMu.Unlock()

	key := fmt.Sprintf("%s/%s:%d", namespace, serviceName, localPort)

	// Check if already running
	if cmd, exists := s.portForwards[key]; exists && cmd.Process != nil {
		return nil // Already running
	}

	cmd := exec.Command("kubectl", "port-forward",
		fmt.Sprintf("svc/%s", serviceName),
		fmt.Sprintf("%d:%d", localPort, remotePort),
		"-n", namespace)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start port forward: %w", err)
	}

	s.portForwards[key] = cmd

	// Wait for port forward to be ready
	time.Sleep(3 * time.Second)

	return nil
}

// StopPortForward stops a specific port forward
func (s *TestState) StopPortForward(namespace, serviceName string, localPort int) {
	s.portForwardsMu.Lock()
	defer s.portForwardsMu.Unlock()

	key := fmt.Sprintf("%s/%s:%d", namespace, serviceName, localPort)

	if cmd, exists := s.portForwards[key]; exists && cmd.Process != nil {
		cmd.Process.Kill()
		cmd.Wait()
		delete(s.portForwards, key)
	}
}

// StopAllPortForwards stops all port forwards
func (s *TestState) StopAllPortForwards() {
	s.portForwardsMu.Lock()
	defer s.portForwardsMu.Unlock()

	for key, cmd := range s.portForwards {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
		delete(s.portForwards, key)
	}
}

// WaitForCondition waits for a condition on a resource
func (s *TestState) WaitForCondition(ctx context.Context, gvk, namespace, name, conditionType string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// Use kubectl wait for simplicity
		cmd := exec.CommandContext(ctx, "kubectl", "wait",
			fmt.Sprintf("--for=condition=%s", conditionType),
			fmt.Sprintf("%s/%s", gvk, name),
			"-n", namespace,
			"--timeout=10s")

		if err := cmd.Run(); err == nil {
			return nil
		}

		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("timeout waiting for condition %s on %s/%s", conditionType, gvk, name)
}
