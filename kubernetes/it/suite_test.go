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
	"log"
	"os"
	"testing"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/kubernetes/it/steps"
)

var (
	// testState is the global test state shared between steps
	testState *TestState

	// k8sSteps provides Kubernetes resource operations
	k8sSteps *steps.K8sSteps

	// httpSteps provides HTTP request operations
	httpSteps *steps.HTTPSteps

	// assertSteps provides assertion operations
	assertSteps *steps.AssertSteps

	// helmSteps provides Helm and operator lifecycle operations
	helmSteps *steps.HelmSteps
)

// TestFeatures is the main entry point for BDD tests
func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		TestSuiteInitializer: InitializeTestSuite,
		ScenarioInitializer:  InitializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			Output:   os.Stdout, // Force real-time output streaming
			TestingT: t,
		},
	}

	exitCode := suite.Run()
	if exitCode != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}

// InitializeTestSuite sets up the test suite (runs once before all scenarios)
func InitializeTestSuite(ctx *godog.TestSuiteContext) {
	ctx.BeforeSuite(func() {
		log.Println("=== Operator Integration Test Suite Starting ===")

		// Initialize test state
		var err error
		testState, err = NewTestState()
		if err != nil {
			log.Fatalf("Failed to initialize test state: %v", err)
		}

		// Initialize step handlers
		k8sSteps = steps.NewK8sSteps(testState.K8sClient, testState.DynamicClient, testState.RestMapper)
		httpSteps = steps.NewHTTPSteps(testState.HTTPClient)
		assertSteps = steps.NewAssertSteps(httpSteps)
		helmSteps = steps.NewHelmSteps()

		log.Println("=== Test Suite Ready ===")
	})

	ctx.AfterSuite(func() {
		log.Println("=== Operator Integration Test Suite Cleanup ===")

		if testState != nil {
			testState.Cleanup()
		}

		log.Println("=== Test Suite Complete ===")
	})
}

// InitializeScenario sets up each scenario (runs before each scenario)
func InitializeScenario(ctx *godog.ScenarioContext) {
	// Reset state before each scenario
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		log.Printf("Starting scenario: %s", sc.Name)
		if testState != nil {
			testState.Reset()
		}
		if httpSteps != nil {
			httpSteps.Reset()
		}
		if k8sSteps != nil {
			k8sSteps.Reset()
		}
		return ctx, nil
	})

	// Log after each scenario
	ctx.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
		if err != nil {
			log.Printf("Scenario failed: %s - Error: %v", sc.Name, err)
		} else {
			log.Printf("Scenario passed: %s", sc.Name)
		}
		return ctx, nil
	})

	// Register step definitions
	if k8sSteps != nil {
		k8sSteps.Register(ctx)
	}
	if httpSteps != nil {
		httpSteps.Register(ctx)
	}
	if assertSteps != nil {
		assertSteps.Register(ctx)
	}
	if helmSteps != nil {
		helmSteps.Register(ctx)
	}
}
