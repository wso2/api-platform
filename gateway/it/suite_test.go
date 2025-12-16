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
	"path/filepath"
	"testing"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

var (
	// composeManager is the global Docker Compose manager
	composeManager *ComposeManager

	// testState is the global test state shared between steps
	testState *TestState

	// httpSteps provides common HTTP request steps
	httpSteps *steps.HTTPSteps

	// assertSteps provides common assertion steps
	assertSteps *steps.AssertSteps

	// coverageCollector manages coverage data collection
	coverageCollector *CoverageCollector
)

// TestFeatures is the main entry point for BDD tests
func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		TestSuiteInitializer: InitializeTestSuite,
		ScenarioInitializer:  InitializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}

// InitializeTestSuite sets up the test suite (runs once before all scenarios)
func InitializeTestSuite(ctx *godog.TestSuiteContext) {
	ctx.BeforeSuite(func() {
		log.Println("=== Integration Test Suite Starting ===")

		// Initialize coverage collector (always enabled)
		coverageCollector = NewCoverageCollector(DefaultCoverageConfig())
		if err := coverageCollector.Setup(); err != nil {
			log.Printf("Warning: Failed to setup coverage: %v", err)
		}

		// Pre-flight checks
		if err := CheckDockerAvailable(); err != nil {
			log.Fatalf("Pre-flight check failed: %v", err)
		}

		if err := CheckPortsAvailable(); err != nil {
			log.Fatalf("Pre-flight check failed: %v", err)
		}

		// Create and start compose manager
		composeFile := getComposeFilePath()
		log.Printf("Using compose file: %s", composeFile)
		var err error
		composeManager, err = NewComposeManager(composeFile)
		if err != nil {
			log.Fatalf("Failed to create compose manager: %v", err)
		}

		if err := composeManager.Start(); err != nil {
			log.Fatalf("Failed to start services: %v", err)
		}

		// Initialize global test state
		testState = NewTestState()

		// Initialize common step handlers
		httpSteps = steps.NewHTTPSteps(testState.HTTPClient, map[string]string{
			"gateway-controller": testState.Config.GatewayControllerURL,
			"router":             testState.Config.RouterURL,
			"policy-engine":      testState.Config.PolicyEngineURL,
			"sample-backend":     testState.Config.SampleBackendURL,
		})
		assertSteps = steps.NewAssertSteps(httpSteps)

		log.Println("=== Test Suite Ready ===")
	})

	ctx.AfterSuite(func() {
		log.Println("=== Integration Test Suite Cleanup ===")

		// Stop containers first - this flushes coverage data to the bind mount
		if composeManager != nil {
			composeManager.Cleanup()
		}

		// Generate coverage reports after containers stop (coverage data is now flushed)
		if coverageCollector != nil {
			log.Println("Generating coverage reports...")
			if err := coverageCollector.MergeAndGenerateReport(); err != nil {
				log.Printf("Warning: Failed to generate coverage report: %v", err)
			}
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
	if testState != nil {
		RegisterHealthSteps(ctx, testState)
	}

	// Register common HTTP and assertion steps
	if httpSteps != nil {
		httpSteps.Register(ctx)
	}
	if assertSteps != nil {
		assertSteps.Register(ctx)
	}
}

// getComposeFilePath returns the path to docker-compose.test.yaml
func getComposeFilePath() string {
	// Try to find compose file relative to this test file
	// When running tests, the working directory is the package directory
	candidates := []string{
		"docker-compose.test.yaml",
		filepath.Join(".", "docker-compose.test.yaml"),
	}

	// Also check COMPOSE_FILE env var
	if envFile := os.Getenv("COMPOSE_FILE"); envFile != "" {
		candidates = append([]string{envFile}, candidates...)
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			absPath, _ := filepath.Abs(candidate)
			return absPath
		}
	}

	// Default to relative path
	return "docker-compose.test.yaml"
}
