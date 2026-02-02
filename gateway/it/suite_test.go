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
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	// jwtSteps provides JWT authentication steps
	jwtSteps *JWTSteps

	// coverageCollector manages coverage data collection
	coverageCollector *CoverageCollector

	// testReporter manages test report generation
	testReporter *TestReporter

	// hasFailures tracks if any scenario failed
	hasFailures bool
)

// TestFeatures is the main entry point for BDD tests
func TestFeatures(t *testing.T) {
	// Initialize test reporter
	testReporter = NewTestReporter(DefaultReporterConfig())
	if err := testReporter.Setup(); err != nil {
		log.Printf("Warning: Failed to setup test reporter: %v", err)
	}

	suite := godog.TestSuite{
		TestSuiteInitializer: InitializeTestSuite,
		ScenarioInitializer:  InitializeScenario,
		Options: &godog.Options{
			Strict: true,
			Format: "pretty",
			Paths: []string{
				"features/health.feature",
				"features/metrics.feature",
				"features/api_deploy.feature",
				"features/backend_timeout.feature",
				"features/mcp_deploy.feature",
				"features/ratelimit.feature",
				"features/jwt-auth.feature",
				"features/cors.feature",
				"features/word-count-guardrail.feature",
				"features/sentence-count-guardrail.feature",
				"features/url-guardrail.feature",
				"features/regex-guardrail.feature",
				"features/prompt-decorator.feature",
				"features/prompt-template.feature",
				"features/pii-masking-regex.feature",
				"features/model-weighted-round-robin.feature",
				"features/model-round-robin.feature",
				"features/json-schema-guardrail.feature",
				"features/llm-provider-templates.feature",
				"features/analytics-header-filter.feature",
				"features/lazy-resources-xds.feature",
				"features/content-length-guardrail.feature",
				"features/azure-content-safety.feature",
				"features/aws-bedrock-guardrail.feature",
				"features/semantic-cache.feature",
				"features/semantic-prompt-guard.feature",
				"features/modify-headers.feature",
				"features/respond.feature",
				"features/llm-provider.feature",
				"features/certificates.feature",
				"features/config-dump.feature",
				"features/api-management.feature",
				"features/api-error-responses.feature",
				"features/list-policies.feature",
				"features/api-keys.feature",
				"features/api-with-policies.feature",
				"features/llm-proxies.feature",
				"features/search-deployments.feature",
				"features/policy-engine-admin.feature",
				"features/cel-conditions.feature",
				"features/analytics-basic.feature",
			},
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
		log.Println("=== Integration Test Suite Starting ===")

		// Initialize coverage collector (always enabled)
		coverageCollector = NewCoverageCollector(DefaultCoverageConfig())
		if err := coverageCollector.Setup(); err != nil {
			log.Printf("Warning: Failed to setup coverage: %v", err)
		}

		// Pre-flight checks
		checkColimaAndSetupEnv()

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
			"gateway-controller":         testState.Config.GatewayControllerURL,
			"router":                     testState.Config.RouterURL,
			"policy-engine":              testState.Config.PolicyEngineURL,
			"sample-backend":             testState.Config.SampleBackendURL,
			"echo-backend":               testState.Config.EchoBackendURL,
			"mock-jwks":                  testState.Config.MockJWKSURL,
			"mock-azure-content-safety":  testState.Config.MockAzureContentSafetyURL,
			"mock-aws-bedrock-guardrail": testState.Config.MockAWSBedrockGuardrailURL,
			"mock-embedding-provider":    testState.Config.MockEmbeddingProviderURL,
		})
		assertSteps = steps.NewAssertSteps(httpSteps)

		// Initialize JWT steps
		jwtSteps = NewJWTSteps(testState, httpSteps, testState.Config.MockJWKSURL)

		log.Println("=== Test Suite Ready ===")
	})

	ctx.AfterSuite(func() {
		log.Println("=== Integration Test Suite Cleanup ===")

		// If failures occurred, dump container logs
		if hasFailures {
			log.Println("Failures detected, dumping container logs...")
			outDir := "logs"
			if mkErr := os.MkdirAll(outDir, 0755); mkErr != nil {
				log.Printf("Warning: Failed to create logs directory: %v", mkErr)
			} else if composeManager != nil {
				ts := time.Now().Format("20060102-150405")
				fileName := "suite_failure_" + ts + ".log"
				outPath := filepath.Join(outDir, fileName)
				if dumpErr := composeManager.DumpLogs(outPath); dumpErr != nil {
					log.Printf("Warning: Failed to dump container logs: %v", dumpErr)
				} else {
					log.Printf("Container logs saved to: %s", outPath)
				}
			}
		}

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

		// Generate test reports
		if testReporter != nil {
			log.Println("Generating test reports...")
			if err := testReporter.GenerateReport(); err != nil {
				log.Printf("Warning: Failed to generate test report: %v", err)
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
		if jwtSteps != nil {
			jwtSteps.Reset()
		}
		// Record scenario start for reporting
		if testReporter != nil {
			testReporter.StartScenario(sc)
		}
		return ctx, nil
	})

	// Log after each scenario
	ctx.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
		if err != nil {
			log.Printf("Scenario failed: %s - Error: %v", sc.Name, err)
			hasFailures = true
		} else {
			log.Printf("Scenario passed: %s", sc.Name)
		}
		// Record scenario result for reporting
		if testReporter != nil {
			testReporter.EndScenario(sc, err)
		}
		return ctx, nil
	})

	// Register step definitions
	if testState != nil {
		RegisterHealthSteps(ctx, testState, httpSteps)
		RegisterMetricsSteps(ctx, testState, httpSteps)
		RegisterAuthSteps(ctx, testState, httpSteps)
		RegisterAPISteps(ctx, testState, httpSteps)
		RegisterMCPSteps(ctx, testState, httpSteps)
		RegisterLLMSteps(ctx, testState, httpSteps)
		RegisterJWTSteps(ctx, testState, httpSteps, jwtSteps)
		RegisterPolicyEngineSteps(ctx, testState, httpSteps)
		RegisterAnalyticsSteps(ctx, testState, httpSteps)
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

// checkColimaAndSetupEnv detects if colima is used and sets up environment variables
func checkColimaAndSetupEnv() {
	// If DOCKER_HOST is already set, don't override
	if os.Getenv("DOCKER_HOST") != "" {
		return
	}

	// Check if colima context exists
	cmd := exec.Command("docker", "context", "ls", "--format", "{{.Name}}")
	out, err := cmd.Output()
	if err == nil && strings.Contains(string(out), "colima") {
		log.Println("Colima detected, setting up environment variables...")

		// Set DOCKER_HOST
		dockerHost := "unix://" + os.Getenv("HOME") + "/.colima/default/docker.sock"
		if _, err := os.Stat(strings.TrimPrefix(dockerHost, "unix://")); err == nil {
			os.Setenv("DOCKER_HOST", dockerHost)
			log.Printf("Set DOCKER_HOST=%s", dockerHost)
		}

		// Disable Ryuk if not already specified
		if os.Getenv("TESTCONTAINERS_RYUK_DISABLED") == "" {
			os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
			log.Println("Set TESTCONTAINERS_RYUK_DISABLED=true")
		}
	}
}
